package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/course-studio/skillex/test/helpers"
)

// writeCorruptionFixture builds a minimal repo with three skills and the
// given skillex.json content, returning the repo dir.
func writeCorruptionFixture(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c"} {
		content := "---\nname: Skill " + name +
			"\ndescription: Test skill " + name +
			"\ntopics: [topic-" + name + "]\ntags: [tag-" + name + "]\n---\n\n# Skill " + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "skills", name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "skillex.json"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type refreshJSON struct {
	SkillsAdded int `json:"skills_added"`
	Errors      int `json:"errors"`
}

type queryJSON struct {
	Type    string `json:"type"`
	Results []struct {
		Path   string   `json:"path"`
		Topics []string `json:"topics"`
		Scopes []string `json:"scopes"`
	} `json:"results"`
}

// A skill duplicated inside one rule used to silently cross-write its
// topics/tags onto an unrelated skill (LastInsertId after upsert).
func TestRefresh_DuplicateSkillInOneRule_NoCorruption(t *testing.T) {
	dir := writeCorruptionFixture(t, `{
  "Version": 4,
  "Rules": [
    {"Scope": "**", "Skills": ["skills/a.md", "skills/b.md", "skills/c.md", "skills/a.md"]}
  ]
}`)

	var out refreshJSON
	res := helpers.RunJSON(t, dir, &out, "refresh", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed: %s", res.Stderr)
	}
	if out.SkillsAdded != 3 {
		t.Errorf("skills_added = %d, want 3 (three unique skills)", out.SkillsAdded)
	}
	if out.Errors != 0 {
		t.Errorf("errors = %d, want 0", out.Errors)
	}

	var q queryJSON
	helpers.RunJSON(t, dir, &q, "query", "--topic", "topic-c", "--json")
	if q.Type != "results" || len(q.Results) != 1 || q.Results[0].Path != "skills/c.md" {
		t.Fatalf("query --topic topic-c: want exactly skills/c.md, got %+v", q)
	}
	helpers.RunJSON(t, dir, &q, "query", "--topic", "topic-a", "--json")
	if len(q.Results) != 1 || q.Results[0].Path != "skills/a.md" {
		t.Errorf("query --topic topic-a: want exactly skills/a.md, got %+v", q)
	}
}

// A skill listed in several rules (the only way to give it several scopes)
// used to produce FOREIGN KEY warnings on every refresh.
func TestRefresh_SkillInMultipleRules_MergesScopesCleanly(t *testing.T) {
	dir := writeCorruptionFixture(t, `{
  "Version": 4,
  "Rules": [
    {"Scope": "**", "Skills": ["skills/a.md", "skills/b.md", "skills/c.md"]},
    {"Scope": "x/**", "Skills": ["skills/a.md"]},
    {"Scope": "y/**", "Skills": ["skills/a.md"]}
  ]
}`)

	var out refreshJSON
	res := helpers.RunJSON(t, dir, &out, "refresh", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("refresh failed: %s", res.Stderr)
	}
	if out.SkillsAdded != 3 {
		t.Errorf("skills_added = %d, want 3", out.SkillsAdded)
	}
	if out.Errors != 0 {
		t.Errorf("errors = %d, want 0 (upstream emits FK constraint warnings here)", out.Errors)
	}

	var q queryJSON
	helpers.RunJSON(t, dir, &q, "query", "--topic", "topic-a", "--json")
	if len(q.Results) != 1 || len(q.Results[0].Scopes) != 3 {
		t.Errorf("skill a should carry 3 merged scopes (**, x/**, y/**), got %+v", q.Results)
	}
}
