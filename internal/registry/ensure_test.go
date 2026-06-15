package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// A build must not announce "building"/"rebuilding" before config is confirmed to
// load — otherwise a config-less directory prints a contradictory build line right
// before failing with "run skillex init".
func TestEnsureIndex_NoBuildMessageWhenConfigMissing(t *testing.T) {
	root := t.TempDir() // no skillex.json, no skills

	var progress strings.Builder
	reg, err := EnsureIndex(root, &progress)
	if reg != nil {
		reg.Close()
		t.Fatal("expected no registry when config is missing")
	}
	if err == nil {
		t.Fatal("expected an error when config is missing")
	}
	if strings.Contains(progress.String(), "building") || strings.Contains(progress.String(), "rebuilding") {
		t.Errorf("must not announce a build before config is confirmed; progress was: %q", progress.String())
	}
}

// Best-effort scanner warnings (e.g. a configured skill file missing from disk)
// must be surfaced during auto-build, matching `skillex refresh` — not silently
// dropped.
func TestEnsureIndex_SurfacesRefreshWarnings(t *testing.T) {
	root := t.TempDir()
	writeSkillFixture(t, filepath.Join(root, "skills", "a.md"), "A", "alpha")
	cfg := map[string]any{
		"Version": 4,
		"Rules":   []map[string]any{{"Scope": "**", "Skills": []string{"skills/a.md", "skills/ghost.md"}}},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "skillex.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	var progress strings.Builder
	reg, err := EnsureIndex(root, &progress)
	if err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	defer reg.Close()
	if !strings.Contains(progress.String(), "ghost") {
		t.Errorf("expected a warning about the missing skill on progress, got: %q", progress.String())
	}
}

// On Windows a concurrent cold start can fail to rename over an index.db that a
// peer process holds open. EnsureIndex must recover by using the peer-installed
// index rather than erroring.
func TestInstallIndex_UsesPeerIndexWhenRenameFails(t *testing.T) {
	root := writeIndexTestRepo(t, "a", "b")
	dbDir := filepath.Join(root, ".skillex")
	dbPath := filepath.Join(dbDir, "index.db")

	// A peer has already installed a healthy 2-skill index at dbPath.
	peer, err := EnsureIndex(root, io.Discard)
	if err != nil {
		t.Fatalf("peer build: %v", err)
	}
	peer.Close()

	// A throwaway temp DB to "install".
	tmp, err := os.CreateTemp(dbDir, "index-*.db.tmp")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	// Simulate Windows: rename onto the peer-held dbPath fails.
	orig := osRename
	osRename = func(_, _ string) error { return fmt.Errorf("simulated sharing violation") }
	defer func() { osRename = orig }()

	reg, err := installIndex(tmpPath, dbPath)
	if err != nil {
		t.Fatalf("installIndex should recover via the peer index, got: %v", err)
	}
	defer reg.Close()
	if n, _ := reg.SkillCount(); n != 2 {
		t.Errorf("recovered SkillCount = %d, want 2 (should use the peer index)", n)
	}
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Errorf("temp index should be discarded after recovery")
	}
}

// Opt-out must not rebuild or delete a corrupt index, and the error must report
// the open failure (not the misleading "registry not found" used for a genuinely
// missing index).
func TestEnsureIndex_OptOutDoesNotRebuildCorrupt(t *testing.T) {
	t.Setenv("SKILLEX_NO_AUTO_REFRESH", "1")
	root := writeIndexTestRepo(t, "foo")
	dbPath := filepath.Join(root, ".skillex", "index.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	garbage := []byte("not a sqlite database at all")
	if err := os.WriteFile(dbPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := EnsureIndex(root, io.Discard)
	if reg != nil {
		reg.Close()
		t.Fatal("opt-out must not rebuild a corrupt index")
	}
	if err == nil {
		t.Fatal("expected an error for a corrupt index under opt-out")
	}
	if errors.Is(err, ErrAutoRefreshDisabled) {
		t.Errorf("a corrupt (present) index under opt-out should report the open failure, not the missing-index sentinel; got %v", err)
	}
	if got, _ := os.ReadFile(dbPath); string(got) != string(garbage) {
		t.Errorf("opt-out must not modify the corrupt index on disk")
	}
}
