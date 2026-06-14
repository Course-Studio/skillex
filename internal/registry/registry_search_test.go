package registry

import (
	"path/filepath"
	"testing"
)

func openReg(t *testing.T) *Registry {
	t.Helper()
	reg, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func TestQueryBySearch_MatchesTopicsAndTags(t *testing.T) {
	reg := openReg(t)
	if _, err := reg.InsertSkill(Skill{
		Path: "a.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "Accessibility", Description: "Focus management.",
		Topics: []string{"accessibility"}, Tags: []string{"a11y", "aria"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InsertSkill(Skill{
		Path: "b.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "Routing", Description: "Nav.", Topics: []string{"routing"}, Tags: []string{"nav"},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := reg.QueryBySearch("a11y")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "a.md" {
		t.Errorf("search a11y (a tag): want [a.md], got %v", paths(got))
	}

	got, _ = reg.QueryBySearch("accessibility")
	if len(got) != 1 || got[0].Path != "a.md" {
		t.Errorf("search accessibility (a topic): want [a.md], got %v", paths(got))
	}
}

func TestQueryBySearch_EscapesLikeWildcards(t *testing.T) {
	reg := openReg(t)
	if _, err := reg.InsertSkill(Skill{
		Path: "a.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "Plain", Description: "no wildcards here",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := reg.QueryBySearch("%")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("search %q should match nothing literally, got %v", "%", paths(got))
	}
}

func paths(skills []Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Path
	}
	return out
}
