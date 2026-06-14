package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/course-studio/skillex/test/helpers"
)

func TestPack_ProjectLocalFilesPresentActivation(t *testing.T) {
	dir := helpers.CopyFixture(t, "monorepo-pnpm")

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skillex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "pack.yaml"), []byte(`name: docker
version: 1.0.0
description: Docker guidance.
skills:
  - file: docker.md
    activate-when:
      files-present:
        - Dockerfile
    scope: repo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "docker.md"), []byte(`---
name: Docker
description: Dockerfile guidance.
topics: [docker]
tags: [containers]
---

# Docker

Use Docker guidance from the activated pack.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := helpers.Run(t, dir, "refresh")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	skills := queryResults(t, dir, "--topic", "docker", "--format", "summary")
	helpers.AssertSkillPresent(t, skills, "docker.md")
	if len(skills) != 1 {
		t.Fatalf("docker topic results = %d, want 1", len(skills))
	}
	if skills[0].SourceType != "pack" {
		t.Fatalf("SourceType = %q, want pack", skills[0].SourceType)
	}
}

func TestPack_ProjectLocalPackWithoutMatchDoesNotActivate(t *testing.T) {
	dir := helpers.CopyFixture(t, "monorepo-pnpm")

	if err := os.MkdirAll(filepath.Join(dir, "skillex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "pack.yaml"), []byte(`name: docker
version: 1.0.0
skills:
  - file: docker.md
    activate-when:
      files-present:
        - Dockerfile
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "docker.md"), []byte("# Docker\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := helpers.Run(t, dir, "refresh")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	resp, _ := helpers.RunQueryJSON(t, dir, "query", "--topic", "docker", "--format", "summary")
	if resp.Type != "no_match" {
		t.Fatalf("response type = %q, want no_match", resp.Type)
	}
}

func TestPack_SameFileListedTwiceMergesScopesAndInsertsTestOnce(t *testing.T) {
	dir := helpers.CopyFixture(t, "monorepo-pnpm")

	for _, sub := range []string{"services/api", "services/worker"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, sub, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "skillex"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The same skill file is listed in two manifest entries with different
	// activate-when/scope. Linking must take the UNION of both scopes, and the
	// paired .test.md must be inserted exactly once (not once per entry).
	if err := os.WriteFile(filepath.Join(dir, "skillex", "pack.yaml"), []byte(`name: docker
version: 1.0.0
description: Docker guidance.
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
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "docker.md"), []byte(`---
name: Docker
description: Dockerfile guidance.
topics: [docker]
tags: [containers]
---

# Docker

Use Docker guidance from the activated pack.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex", "docker.test.md"), []byte(`# Tests: docker.md

## Validation: Basic
Prompt: "How should I build this Dockerfile?"
Success criteria:
  - Mentions Docker guidance
`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := helpers.Run(t, dir, "refresh")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	db := helpers.OpenRegistry(t, dir)

	// Exactly one docker.md skill row (deduped), carrying the union of scopes.
	skillRows := helpers.QuerySkillsTable(t, db)
	dockerCount := 0
	for _, r := range skillRows {
		if strings.HasSuffix(r.Path, "skillex/docker.md") {
			dockerCount++
		}
	}
	if dockerCount != 1 {
		t.Fatalf("docker.md skill rows = %d, want 1 (deduped)", dockerCount)
	}

	scopes := helpers.QueryScopesFor(t, db, "skillex/docker.md")
	want := map[string]bool{"services/api/*": true, "services/worker/*": true}
	if len(scopes) != len(want) {
		t.Fatalf("scopes = %v, want union %v", scopes, want)
	}
	for _, s := range scopes {
		if !want[s] {
			t.Fatalf("unexpected scope %q in %v, want union %v", s, scopes, want)
		}
	}

	// The skill is visible from BOTH activated directories (union, not keep-first).
	apiSkills := queryResults(t, dir, "--path", "services/api/Dockerfile", "--topic", "docker", "--format", "summary")
	helpers.AssertSkillPresent(t, apiSkills, "docker.md")
	workerSkills := queryResults(t, dir, "--path", "services/worker/Dockerfile", "--topic", "docker", "--format", "summary")
	helpers.AssertSkillPresent(t, workerSkills, "docker.md")

	// The paired test scenario is inserted exactly once.
	tests := helpers.QueryTestsFor(t, db, "skillex/docker.md")
	if len(tests) != 1 {
		t.Fatalf("test scenarios for docker.md = %d, want 1 (deduped)", len(tests))
	}
}

func TestPack_NodeDependencyShippedPackActivation(t *testing.T) {
	dir := helpers.CopyFixture(t, "monorepo-pnpm")

	appPkgPath := filepath.Join(dir, "packages", "app-a", "package.json")
	data, err := os.ReadFile(appPkgPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), `"@test/utils": "workspace:*"`, `"@test/utils": "workspace:*",
    "with-pack": "1.0.0"`, 1)
	if err := os.WriteFile(appPkgPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgRoot := filepath.Join(dir, "packages", "app-a", "node_modules", "with-pack")
	if err := os.MkdirAll(filepath.Join(pkgRoot, "skillex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgRoot, "package.json"), []byte(`{
  "name": "with-pack",
  "version": "1.0.0",
  "skillex": true
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgRoot, "skillex", "pack.yaml"), []byte(`name: with-pack
version: 1.0.0
skills:
  - file: usage.md
    activate-when:
      dependency-declared:
        - source: npm-package
          name: with-pack
    scope: boundary
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgRoot, "skillex", "usage.md"), []byte(`---
name: With Pack
description: Guidance shipped from a dependency pack.
topics: [with-pack]
tags: [dependency-pack]
---

# With Pack

Use package-shipped pack guidance.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := helpers.Run(t, dir, "refresh")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	skills := queryResults(t, dir, "--path", "packages/app-a/src/index.ts", "--topic", "with-pack", "--format", "summary")
	helpers.AssertSkillPresent(t, skills, "usage.md")
	if len(skills) != 1 {
		t.Fatalf("with-pack results = %d, want 1", len(skills))
	}
	if skills[0].SourceType != "pack" {
		t.Fatalf("SourceType = %q, want pack", skills[0].SourceType)
	}
	if skills[0].PackageName != "with-pack" {
		t.Fatalf("PackageName = %q, want with-pack", skills[0].PackageName)
	}
}
