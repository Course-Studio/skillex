package registry

import (
	"path/filepath"
	"testing"
)

// TestGetSkillContent_ReturnsContentWithoutMetadata covers the lightweight
// content-only accessor used by the MCP resource read path: it returns the body
// for an existing skill and "" (no error) for a missing one, without running the
// topic/tag/scope metadata queries that GetSkillByPath issues.
func TestGetSkillContent_ReturnsContentWithoutMetadata(t *testing.T) {
	reg, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	if _, err := reg.InsertSkill(Skill{
		Path: "skills/a.md", Content: "hello body", Visibility: "repo", SourceType: "repo",
		Topics: []string{"t"}, Tags: []string{"g"}, Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := reg.GetSkillContent("skills/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello body" {
		t.Errorf("GetSkillContent = %q, want %q", got, "hello body")
	}

	missing, err := reg.GetSkillContent("skills/missing.md")
	if err != nil {
		t.Fatalf("GetSkillContent(missing) error = %v, want nil", err)
	}
	if missing != "" {
		t.Errorf("GetSkillContent(missing) = %q, want empty string", missing)
	}
}
