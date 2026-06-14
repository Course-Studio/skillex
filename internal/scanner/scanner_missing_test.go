package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/course-studio/skillex/internal/config"
)

func TestScan_WarnsWhenConfigListsMissingSkill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "exists.md"), []byte("# Exists\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: 4,
		Rules: []config.Rule{
			{Scope: "**", Skills: []string{"skills/exists.md", "skills/deleted.md"}},
		},
	}

	result, err := New(root, cfg, true).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RepoSkills) != 1 {
		t.Errorf("RepoSkills = %d, want 1", len(result.RepoSkills))
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "skills/deleted.md") {
		t.Errorf("want one warning naming skills/deleted.md, got %v", result.Errors)
	}
}
