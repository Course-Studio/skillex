package query

import (
	"path/filepath"
	"testing"

	"github.com/course-studio/skillex/internal/registry"
)

func newQueryReg(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func TestRepoRelativePath(t *testing.T) {
	cases := []struct {
		path, root, wantRel string
		wantOutside         bool
	}{
		{"clients/epgm/App.tsx", "/repo", "clients/epgm/App.tsx", false},
		{"/repo/clients/epgm/App.tsx", "/repo", "clients/epgm/App.tsx", false},
		{"/elsewhere/x.ts", "/repo", "", true},
		{"/repo", "/repo", "", false},
		{"clients\\epgm\\App.tsx", "/repo", "clients/epgm/App.tsx", false},
	}
	for _, c := range cases {
		rel, outside := repoRelativePath(c.path, c.root)
		if outside != c.wantOutside || (!outside && rel != c.wantRel) {
			t.Errorf("repoRelativePath(%q,%q) = (%q,%v), want (%q,%v)",
				c.path, c.root, rel, outside, c.wantRel, c.wantOutside)
		}
	}
}

func TestExecute_AbsolutePathOutsideRepoReturnsNoMatchWithNote(t *testing.T) {
	reg := newQueryReg(t)
	eng := New(reg, "/repo")
	resp, err := eng.Execute(Params{Path: "/somewhere/else/x.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Type != ResponseTypeNoMatch {
		t.Fatalf("want no_match, got %s", resp.Type)
	}
	if resp.Note == "" {
		t.Error("expected a note explaining the path is outside the repository")
	}
}

func TestExecute_AbsolutePathInsideRepoReturnsScopedSkills(t *testing.T) {
	reg := newQueryReg(t)
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/global.md", Content: "g", Visibility: "repo", SourceType: "repo", Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/clients.md", Content: "c", Visibility: "repo", SourceType: "repo", Scopes: []string{"clients/**"},
	}); err != nil {
		t.Fatal(err)
	}
	eng := New(reg, "/repo")
	resp, err := eng.Execute(Params{Path: "/repo/clients/epgm/App.tsx"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Type != ResponseTypeResults {
		t.Fatalf("want results, got %s (note: %q)", resp.Type, resp.Note)
	}
	got := map[string]bool{}
	for _, r := range resp.Results {
		got[r.Path] = true
	}
	if !got["skills/clients.md"] {
		t.Errorf("absolute path inside repo must return the clients/** scoped skill (F10); got %v", got)
	}
	if !got["skills/global.md"] {
		t.Errorf("absolute path inside repo must also return the **-scoped skill; got %v", got)
	}
}
