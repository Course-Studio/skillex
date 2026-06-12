package registry

import (
	"path/filepath"
	"testing"
)

func TestInsertSkill_UpsertReturnsOwnIDAndLeavesOthersAlone(t *testing.T) {
	reg, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	id1, err := reg.InsertSkill(Skill{
		Path: "skills/a.md", Content: "A", Visibility: "repo", SourceType: "repo",
		Topics: []string{"one"}, Scopes: []string{"**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InsertSkill(Skill{
		Path: "skills/b.md", Content: "B", Visibility: "repo", SourceType: "repo",
		Topics: []string{"b-topic"}, Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}

	// Re-insert a (upsert path). Upstream returned a foreign rowid here.
	id2, err := reg.InsertSkill(Skill{
		Path: "skills/a.md", Content: "A2", Visibility: "repo", SourceType: "repo",
		Topics: []string{"two"}, Scopes: []string{"**", "x/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id1 {
		t.Errorf("upsert returned id %d, want %d", id2, id1)
	}

	a, err := reg.GetSkillByPath("skills/a.md")
	if err != nil || a == nil {
		t.Fatalf("GetSkillByPath(a): %v", err)
	}
	if a.Content != "A2" || len(a.Topics) != 1 || a.Topics[0] != "two" || len(a.Scopes) != 2 {
		t.Errorf("skill a not fully updated: %+v", a)
	}

	b, err := reg.GetSkillByPath("skills/b.md")
	if err != nil || b == nil {
		t.Fatalf("GetSkillByPath(b): %v", err)
	}
	if len(b.Topics) != 1 || b.Topics[0] != "b-topic" {
		t.Errorf("skill b metadata was disturbed by the upsert of a: %+v", b)
	}
}
