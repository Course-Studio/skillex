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

// TestInitRoot_NotCapturedByAncestorConfig locks the load-bearing F4 invariant:
// `skillex init` operates on the current directory even when an ancestor already
// holds a skillex config. repoRoot() walks up and is captured by that ancestor;
// initRoot() must not be, or running init in a subtree would re-initialize the
// ancestor instead of creating a nested root. A refactor that points init at
// repoRoot() would regress silently without this test.
func TestInitRoot_NotCapturedByAncestorConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "skillex.json"), []byte(`{"Version":4,"Rules":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "packages", "app")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	// os.Getwd resolves symlinks (macOS /var -> /private/var), so compare against it.
	wantChild, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if got := initRoot(); got != wantChild {
		t.Errorf("initRoot() = %q, want cwd %q (init must not be captured by the ancestor config)", got, wantChild)
	}
	// Sanity: repoRoot() IS captured by the ancestor, proving the two genuinely differ.
	if got := repoRoot(); got == wantChild {
		t.Errorf("repoRoot() = %q, expected it to walk up to the ancestor config; test no longer exercises the init-vs-repoRoot distinction", got)
	}
}
