package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRoot_WalksUpToConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "skillex.json"), []byte(`{"Version":4,"Rules":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "clients", "epgm", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := findRepoRoot(nested); got != root {
		t.Errorf("findRepoRoot(%q) = %q, want %q", nested, got, root)
	}
}

func TestFindRepoRoot_NoConfigReturnsStart(t *testing.T) {
	dir := t.TempDir()
	if got := findRepoRoot(dir); got != dir {
		t.Errorf("findRepoRoot(%q) = %q, want unchanged", dir, got)
	}
}
