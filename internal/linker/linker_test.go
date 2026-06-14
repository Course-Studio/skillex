package linker

import (
	"testing"

	"github.com/course-studio/skillex/internal/config"
	"github.com/course-studio/skillex/internal/scanner"
)

func TestLink_DeduplicatesRepoSkillsListedInSeveralRules(t *testing.T) {
	cfg := &config.Config{
		Version: 4,
		Rules: []config.Rule{
			{Scope: "**", Skills: []string{"skills/a.md"}},
			{Scope: "clients/**", Skills: []string{"skills/a.md"}},
		},
	}
	sf := scanner.SkillFile{RelPath: "skills/a.md", Visibility: "repo", SourceType: "repo"}
	// The scanner reads the file once per rule that lists it.
	result := &scanner.ScanResult{RepoSkills: []scanner.SkillFile{sf, sf}}

	linked := New("/repo", cfg).Link(result)

	count := 0
	for _, ls := range linked {
		if ls.IsTest {
			continue
		}
		count++
		if len(ls.Scopes) != 2 {
			t.Errorf("scopes = %v, want the merged pair [** clients/**]", ls.Scopes)
		}
	}
	if count != 1 {
		t.Fatalf("linked %d repo skills, want 1 after dedup", count)
	}
}

func TestLink_MergesPackSkillScopesAcrossDuplicateRelPaths(t *testing.T) {
	cfg := &config.Config{Version: 4}

	// A project pack that lists the same file in two manifest entries with
	// different scopes emits two SkillFiles sharing one RelPath but carrying
	// DIFFERENT ExplicitScopes. They must merge to the union, not keep-first.
	first := scanner.SkillFile{
		RelPath:        "skillex/docker.md",
		Visibility:     "repo",
		SourceType:     "pack",
		ExplicitScopes: []string{"services/api/**"},
	}
	second := first
	second.ExplicitScopes = []string{"services/worker/**"}

	result := &scanner.ScanResult{RepoSkills: []scanner.SkillFile{first, second}}

	linked := New("/repo", cfg).Link(result)

	count := 0
	for _, ls := range linked {
		if ls.IsTest {
			continue
		}
		count++
		want := []string{"services/api/**", "services/worker/**"}
		if !sameStringSet(ls.Scopes, want) {
			t.Errorf("scopes = %v, want union %v", ls.Scopes, want)
		}
	}
	if count != 1 {
		t.Fatalf("linked %d pack skills, want 1 after merge", count)
	}
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}
