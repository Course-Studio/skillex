package registry

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeIndexTestRepo creates a temp repo with a skillex.json listing the named
// skills (one file per name under skills/) and returns its root. The index is
// NOT built — callers exercise EnsureIndex against the missing/fresh state.
func writeIndexTestRepo(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	skills := make([]string, 0, len(names))
	for _, n := range names {
		writeSkillFixture(t, filepath.Join(root, "skills", n+".md"), "Skill "+n, "topic-"+n)
		skills = append(skills, "skills/"+n+".md")
	}
	cfg := map[string]any{
		"Version": 4,
		"Rules":   []map[string]any{{"Scope": "**", "Skills": skills}},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skillex.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEnsureIndex_BuildsMissingIndex(t *testing.T) {
	root := writeIndexTestRepo(t, "foo")
	dbPath := filepath.Join(root, ".skillex", "index.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: index should be missing, stat err = %v", err)
	}

	reg, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("EnsureIndex on a missing index: %v", err)
	}
	defer reg.Close()

	if n, _ := reg.SkillCount(); n != 1 {
		t.Errorf("SkillCount = %d, want 1 (auto-build should have indexed the skill)", n)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("index file should exist after EnsureIndex: %v", err)
	}
}

func TestEnsureIndex_OpensHealthyWithoutRebuild(t *testing.T) {
	root := writeIndexTestRepo(t, "foo")

	reg1, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("initial EnsureIndex: %v", err)
	}
	// Inject a marker row that a rebuild (ClearTx) would wipe.
	if _, err := reg1.db.Exec(
		`INSERT INTO skills (path, content, visibility, source_type, indexed_at)
		 VALUES ('MARKER.md', 'x', 'public', 'repo', 'marker')`,
	); err != nil {
		t.Fatalf("inserting marker row: %v", err)
	}
	reg1.Close()

	// A healthy index must be opened as-is, not rebuilt — the marker must survive.
	reg2, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("second EnsureIndex: %v", err)
	}
	defer reg2.Close()

	var n int
	if err := reg2.db.QueryRow(`SELECT COUNT(*) FROM skills WHERE path = 'MARKER.md'`).Scan(&n); err != nil {
		t.Fatalf("querying marker: %v", err)
	}
	if n != 1 {
		t.Errorf("marker row gone (count=%d): a healthy index must not be rebuilt by EnsureIndex", n)
	}
}

func TestEnsureIndex_RebuildsUnreadableIndex(t *testing.T) {
	root := writeIndexTestRepo(t, "foo")
	dbPath := filepath.Join(root, ".skillex", "index.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write garbage so the file exists but is not a valid SQLite database.
	if err := os.WriteFile(dbPath, []byte("not a sqlite database at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("EnsureIndex should rebuild an unreadable index, got error: %v", err)
	}
	defer reg.Close()

	if n, _ := reg.SkillCount(); n != 1 {
		t.Errorf("SkillCount = %d, want 1 (corrupt index should be rebuilt)", n)
	}
}

func TestEnsureIndex_OptOutReturnsSentinelOnMissing(t *testing.T) {
	t.Setenv("SKILLEX_NO_AUTO_REFRESH", "1")
	root := writeIndexTestRepo(t, "foo")
	dbPath := filepath.Join(root, ".skillex", "index.db")

	reg, err := EnsureIndex(root, io.Discard)
	if reg != nil {
		reg.Close()
		t.Fatal("EnsureIndex must not build when SKILLEX_NO_AUTO_REFRESH is set")
	}
	if !errors.Is(err, ErrAutoRefreshDisabled) {
		t.Fatalf("want ErrAutoRefreshDisabled, got %v", err)
	}
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Errorf("opt-out must not create the index file")
	}
}

func TestEnsureIndex_OptOutOpensExistingIndex(t *testing.T) {
	root := writeIndexTestRepo(t, "foo")

	// Build the index first (auto-refresh enabled).
	reg, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("initial build: %v", err)
	}
	reg.Close()

	// With opt-out set, an existing healthy index must still open (opt-out only
	// disables building, not using an index that is already present).
	t.Setenv("SKILLEX_NO_AUTO_REFRESH", "1")
	reg2, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("opt-out with an existing index should open it: %v", err)
	}
	defer reg2.Close()
	if n, _ := reg2.SkillCount(); n != 1 {
		t.Errorf("SkillCount = %d, want 1", n)
	}
}

func TestEnsureIndex_ConcurrentColdStart(t *testing.T) {
	root := writeIndexTestRepo(t, "a", "b", "c")

	const goroutines = 8
	var wg sync.WaitGroup
	type outcome struct {
		count int
		err   error
	}
	results := make(chan outcome, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg, err := EnsureIndex(root, io.Discard)
			if err != nil {
				results <- outcome{err: err}
				return
			}
			n, _ := reg.SkillCount()
			reg.Close()
			results <- outcome{count: n}
		}()
	}
	wg.Wait()
	close(results)

	for o := range results {
		if o.err != nil {
			t.Errorf("concurrent EnsureIndex returned error: %v", o.err)
			continue
		}
		// A cold-start race must never expose a half-built index to any caller.
		if o.count != 3 {
			t.Errorf("concurrent EnsureIndex returned a registry with %d skills, want 3", o.count)
		}
	}

	reg, err := Open(filepath.Join(root, ".skillex", "index.db"))
	if err != nil {
		t.Fatalf("opening final index: %v", err)
	}
	defer reg.Close()
	if n, _ := reg.SkillCount(); n != 3 {
		t.Errorf("final SkillCount = %d, want 3", n)
	}
}
