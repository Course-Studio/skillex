package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/course-studio/skillex/internal/config"
)

func TestScannerProjectPackActivatesWithFilesPresent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\n")
	writeFile(t, filepath.Join(root, "skillex", "pack.yaml"), `name: docker
version: 1.0.0
skills:
  - file: docker.md
    activate-when:
      files-present:
        - Dockerfile
    scope: repo
`)
	writeFile(t, filepath.Join(root, "skillex", "docker.md"), `---
name: Docker
description: Docker guidance.
topics: [docker]
tags: []
---

# Docker
`)
	writeFile(t, filepath.Join(root, "skillex", "docker.test.md"), `# Tests: docker.md

## Validation: Basic
Prompt: "How should I build this Dockerfile?"
Success criteria:
  - Mentions Docker guidance
`)

	sc := NewWithResolvers(root, &config.Config{Version: 4}, true, nil)
	result, err := sc.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Scan() result errors = %v", result.Errors)
	}
	if len(result.RepoSkills) != 2 {
		t.Fatalf("RepoSkills = %d, want 2", len(result.RepoSkills))
	}

	got := result.RepoSkills[0]
	if got.SourceType != "pack" {
		t.Fatalf("SourceType = %q, want pack", got.SourceType)
	}
	if len(got.ExplicitScopes) != 1 || got.ExplicitScopes[0] != "**" {
		t.Fatalf("ExplicitScopes = %v, want [**]", got.ExplicitScopes)
	}
	if !result.RepoSkills[1].IsTest {
		t.Fatalf("second pack file should be test, got %#v", result.RepoSkills[1])
	}
}

func TestScannerProjectPackEmitsPairedTestOncePerUniqueFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "services", "api", "Dockerfile"), "FROM scratch\n")
	writeFile(t, filepath.Join(root, "services", "worker", "Dockerfile"), "FROM scratch\n")
	// The same skill file is listed twice with different scope strategies.
	writeFile(t, filepath.Join(root, "skillex", "pack.yaml"), `name: docker
version: 1.0.0
skills:
  - file: docker.md
    activate-when:
      files-matching:
        - services/api/Dockerfile
    scope: directory
  - file: docker.md
    activate-when:
      files-matching:
        - services/worker/Dockerfile
    scope: directory
`)
	writeFile(t, filepath.Join(root, "skillex", "docker.md"), `---
name: Docker
description: Docker guidance.
topics: [docker]
tags: []
---

# Docker
`)
	writeFile(t, filepath.Join(root, "skillex", "docker.test.md"), `# Tests: docker.md

## Validation: Basic
Prompt: "How should I build this Dockerfile?"
Success criteria:
  - Mentions Docker guidance
`)

	sc := NewWithResolvers(root, &config.Config{Version: 4}, true, nil)
	result, err := sc.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Scan() result errors = %v", result.Errors)
	}

	tests := 0
	for _, sf := range result.RepoSkills {
		if sf.IsTest {
			tests++
		}
	}
	if tests != 1 {
		t.Fatalf("paired test SkillFiles = %d, want 1 (deduped across manifest entries)", tests)
	}
}

func TestScannerProjectPackRejectsSymlinkEscapingPackDir(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	writeFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\n")

	// Secret file outside the pack dir.
	writeFile(t, filepath.Join(base, "secret.md"), `---
name: Secret
description: Should not be read.
---

# Secret
`)

	// docker.md inside the pack symlinks out to the secret.
	if err := os.MkdirAll(filepath.Join(root, "skillex"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(filepath.Join(base, "secret.md"), filepath.Join(root, "skillex", "docker.md")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	writeFile(t, filepath.Join(root, "skillex", "pack.yaml"), `name: docker
version: 1.0.0
skills:
  - file: docker.md
    activate-when:
      files-present:
        - Dockerfile
    scope: repo
`)

	sc := NewWithResolvers(root, &config.Config{Version: 4}, true, nil)
	result, err := sc.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	// The escaping pack must be rejected (a scan error) and no skill read.
	if len(result.Errors) == 0 {
		t.Fatal("Scan() errors = none, want symlink-escape rejection")
	}
	for _, sf := range result.RepoSkills {
		if !sf.IsTest {
			t.Fatalf("escaping pack skill was read: %#v", sf)
		}
	}
}

func TestScannerProjectPackDoesNotActivateWithoutMatchingFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skillex", "pack.yaml"), `name: docker
version: 1.0.0
skills:
  - file: docker.md
    activate-when:
      files-present:
        - Dockerfile
`)
	writeFile(t, filepath.Join(root, "skillex", "docker.md"), "# Docker\n")

	sc := NewWithResolvers(root, &config.Config{Version: 4}, true, nil)
	result, err := sc.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("Scan() result errors = %v", result.Errors)
	}
	if len(result.RepoSkills) != 0 {
		t.Fatalf("RepoSkills = %d, want 0", len(result.RepoSkills))
	}
}
