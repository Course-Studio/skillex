package agents

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Course-Studio/skillex/internal/registry"
)

func newReg(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func TestGenerateSection_ListsSkillsPerScope(t *testing.T) {
	reg := newReg(t)
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/repo.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "Repo Conventions", Description: "Branch naming and commit format.",
		Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/ui.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "UI Patterns", Description: "Component composition.",
		Scopes: []string{"clients/**"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/shared.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "Shared", Description: "Cross-cutting helpers.",
		Scopes: []string{"**", "clients/**"},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := GenerateSection(reg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "### Skills") {
		t.Error("expected a Skills catalog heading")
	}
	if !strings.Contains(out, "skills/repo.md — Repo Conventions: Branch naming and commit format.") {
		t.Errorf("repo skill catalog line missing:\n%s", out)
	}
	if !strings.Contains(out, "skills/ui.md — UI Patterns: Component composition.") {
		t.Errorf("ui skill catalog line missing:\n%s", out)
	}
	repoIdx := strings.Index(out, "skills/repo.md")
	uiIdx := strings.Index(out, "skills/ui.md")
	scopeAllIdx := strings.Index(out, "`**`")
	scopeClientsIdx := strings.Index(out, "`clients/**`")
	if !(scopeAllIdx < repoIdx && scopeClientsIdx < uiIdx) {
		t.Error("skills should appear under their scope headings")
	}

	// A skill scoped to multiple scopes must appear under each heading.
	allBlock := out[scopeAllIdx:scopeClientsIdx]
	clientsBlock := out[scopeClientsIdx:]
	if !strings.Contains(allBlock, "skills/shared.md") {
		t.Errorf("multi-scope skill missing under `**`:\n%s", out)
	}
	if !strings.Contains(clientsBlock, "skills/shared.md") {
		t.Errorf("multi-scope skill missing under `clients/**`:\n%s", out)
	}
}

func TestGenerateSection_TruncatesLongDescriptionTo140(t *testing.T) {
	reg := newReg(t)
	long := strings.Repeat("a", 200)
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/x.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "X", Description: long, Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}
	out, err := GenerateSection(reg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, strings.Repeat("a", 141)) {
		t.Error("description not truncated to 140 chars")
	}
	if !strings.Contains(out, strings.Repeat("a", 137)+"...") {
		t.Errorf("expected 137 chars + ellipsis, got:\n%s", out)
	}
}

func TestGenerateSection_TruncatesMultibyteDescriptionSafely(t *testing.T) {
	reg := newReg(t)
	desc := strings.Repeat("é", 200)
	if _, err := reg.InsertSkill(registry.Skill{
		Path: "skills/m.md", Content: "x", Visibility: "repo", SourceType: "repo",
		Name: "M", Description: desc, Scopes: []string{"**"},
	}); err != nil {
		t.Fatal(err)
	}
	out, err := GenerateSection(reg)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(out) {
		t.Error("catalog output is not valid UTF-8 after truncation")
	}
	if !strings.Contains(out, strings.Repeat("é", 137)+"...") {
		t.Error("expected 137 runes + ellipsis")
	}
}

func TestGenerateSection_FallsBackToVocabularyAboveCutoff(t *testing.T) {
	reg := newReg(t)
	for i := 0; i < catalogCutoff+1; i++ {
		if _, err := reg.InsertSkill(registry.Skill{
			Path:    "skills/s-" + itoa(i) + ".md",
			Content: "x", Visibility: "repo", SourceType: "repo",
			Name: "N", Description: "D", Scopes: []string{"**"},
			Topics: []string{"common"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	out, err := GenerateSection(reg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "### Skills") {
		t.Errorf("above cutoff (%d) the catalog must be omitted", catalogCutoff)
	}
	if !strings.Contains(out, "### Available topics") {
		t.Error("vocabulary view must remain above cutoff")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
