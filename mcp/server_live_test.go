package mcp

import (
	"path/filepath"
	"testing"

	"github.com/Course-Studio/skillex/internal/registry"
)

func TestResourceContent_IsReadLiveFromRegistry(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/a.md", Content: "ORIGINAL", Visibility: "repo", SourceType: "repo", Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}

	read := resourceContentFunc(reg, "skills/a.md")

	// Mutate AFTER the handler was built (simulates a mid-session refresh).
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/a.md", Content: "UPDATED", Visibility: "repo", SourceType: "repo", Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if got != "UPDATED" {
		t.Errorf("resource content = %q, want live %q", got, "UPDATED")
	}
}

func TestResourceContent_DeletedSkillReturnsEmptyNotPanic(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	// Build the reader for a path that does not exist in the registry.
	read := resourceContentFunc(reg, "skills/missing.md")
	got, err := read()
	if err != nil {
		t.Fatalf("missing skill must not error: %v", err)
	}
	if got != "" {
		t.Errorf("missing skill content = %q, want empty string", got)
	}
}

// TestResourceContent_EmptyContentIsPreserved verifies that a skill with
// empty string content is returned as-is (not confused with "not found").
func TestResourceContent_EmptyContentIsPreserved(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/empty.md", Content: "", Visibility: "repo", SourceType: "repo", Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}

	read := resourceContentFunc(reg, "skills/empty.md")
	got, err := read()
	if err != nil {
		t.Fatal(err)
	}
	// Empty content is a valid state; should not error or panic.
	if got != "" {
		t.Errorf("empty skill content = %q, want %q", got, "")
	}
}

// TestResourceContent_DBErrorPropagates verifies that a registry error is
// returned to the caller rather than swallowed.
func TestResourceContent_DBErrorPropagates(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Close the registry to force a DB error on the next query.
	reg.Close()

	read := resourceContentFunc(reg, "skills/a.md")
	_, err = read()
	if err == nil {
		t.Error("expected error when registry DB is closed, got nil")
	}
}
