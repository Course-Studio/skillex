package linker

import (
	"testing"

	"github.com/Course-Studio/skillex/internal/config"
	"github.com/Course-Studio/skillex/internal/scanner"
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
