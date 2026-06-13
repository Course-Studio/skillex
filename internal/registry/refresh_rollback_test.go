package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/course-studio/skillex/internal/config"
)

func writeSkillFixture(t *testing.T, path, name, topic string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + name + " skill\ntopics: [" + topic + "]\n---\n\n# " + name + "\n\nBody.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRefresh_InsertFailureRollsBackAndReturnsError locks the fail-closed
// integrity contract: if a write fails partway through the rebuild transaction,
// Refresh aborts and leaves the previously committed index intact — it must never
// commit a half-rebuilt catalog. Input-level problems (unparseable frontmatter)
// are filtered by the scanner before the transaction and remain best-effort; this
// covers the in-transaction DB-integrity path only.
func TestRefresh_InsertFailureRollsBackAndReturnsError(t *testing.T) {
	root := t.TempDir()
	writeSkillFixture(t, filepath.Join(root, "skills", "foo.md"), "Foo", "alpha")

	reg, err := Open(filepath.Join(root, ".skillex", "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	cfg1 := &config.Config{Version: 4, Rules: []config.Rule{{Scope: "**", Skills: []string{"skills/foo.md"}}}}
	if _, err := Refresh(reg, cfg1, RefreshOptions{Root: root}); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	if n, _ := reg.SkillCount(); n != 1 {
		t.Fatalf("after initial refresh SkillCount = %d, want 1", n)
	}

	// Add two more skills: baz inserts cleanly, bar is rigged to fail. A trigger
	// aborts the INSERT of bar's skills row (DELETE/clear is unaffected), so the
	// failure lands partway through the rebuild loop after baz has been written.
	writeSkillFixture(t, filepath.Join(root, "skills", "baz.md"), "Baz", "beta")
	writeSkillFixture(t, filepath.Join(root, "skills", "bar.md"), "Bar", "gamma")
	if _, err := reg.db.Exec(
		`CREATE TRIGGER fail_bar BEFORE INSERT ON skills
		 WHEN NEW.path = 'skills/bar.md'
		 BEGIN SELECT RAISE(ABORT, 'injected integrity failure'); END;`,
	); err != nil {
		t.Fatal(err)
	}

	cfg2 := &config.Config{Version: 4, Rules: []config.Rule{{Scope: "**", Skills: []string{
		"skills/foo.md", "skills/baz.md", "skills/bar.md",
	}}}}
	if _, err := Refresh(reg, cfg2, RefreshOptions{Root: root}); err == nil {
		t.Error("Refresh returned nil after a mid-transaction insert failure; want a non-nil error (fail-closed, no partial commit)")
	}

	// Rollback invariant: the registry still holds exactly the previously committed
	// single-skill index — neither the new baz (which inserted before the failure)
	// nor a cleared-but-not-rebuilt empty catalog.
	if n, _ := reg.SkillCount(); n != 1 {
		t.Errorf("after failed refresh SkillCount = %d, want 1 (a rolled-back refresh must leave the prior index intact)", n)
	}
}
