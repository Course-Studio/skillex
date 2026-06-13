package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_JSONConfig(t *testing.T) {
	dir := t.TempDir()
	data := `{
  "Version": 4,
  "Rules": [
    {
      "Scope": "**",
      "Skills": ["skills/repo.md"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, JSONFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(json): %v", err)
	}
	if cfg.Version != 4 {
		t.Fatalf("Version: got %d, want 4", cfg.Version)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Scope != "**" {
		t.Fatalf("unexpected rules: %+v", cfg.Rules)
	}
}

func TestLoad_YAMLConfig(t *testing.T) {
	dir := t.TempDir()
	data := "Version: 4\nRules:\n  - Scope: \"**\"\n    Skills:\n      - skills/repo.md\n"
	if err := os.WriteFile(filepath.Join(dir, YAMLFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(yaml): %v", err)
	}
	if cfg.Version != 4 {
		t.Fatalf("Version: got %d, want 4", cfg.Version)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Scope != "**" {
		t.Fatalf("unexpected rules: %+v", cfg.Rules)
	}
}

func TestLoad_JSONConfig_WithCatalogCutoff(t *testing.T) {
	dir := t.TempDir()
	data := `{
  "Version": 4,
  "CatalogCutoff": 5,
  "Rules": [
    {
      "Scope": "**",
      "Skills": ["skills/repo.md"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, JSONFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(json with CatalogCutoff): %v", err)
	}
	if cfg.CatalogCutoff != 5 {
		t.Fatalf("CatalogCutoff: got %d, want 5", cfg.CatalogCutoff)
	}
}

func TestLoad_YAMLConfig_WithCatalogCutoff(t *testing.T) {
	dir := t.TempDir()
	data := "Version: 4\nCatalogCutoff: 5\nRules:\n  - Scope: \"**\"\n    Skills:\n      - skills/repo.md\n"
	if err := os.WriteFile(filepath.Join(dir, YAMLFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(yaml with CatalogCutoff): %v", err)
	}
	if cfg.CatalogCutoff != 5 {
		t.Fatalf("CatalogCutoff: got %d, want 5", cfg.CatalogCutoff)
	}
}

func TestLoad_JSONConfig_NoCatalogCutoff_IsZero(t *testing.T) {
	dir := t.TempDir()
	data := `{
  "Version": 4,
  "Rules": [
    {
      "Scope": "**",
      "Skills": ["skills/repo.md"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, JSONFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(json without CatalogCutoff): %v", err)
	}
	if cfg.CatalogCutoff != 0 {
		t.Fatalf("CatalogCutoff absent in JSON: got %d, want 0", cfg.CatalogCutoff)
	}
}

func TestLoad_YAMLConfig_NoCatalogCutoff_IsZero(t *testing.T) {
	dir := t.TempDir()
	data := "Version: 4\nRules:\n  - Scope: \"**\"\n    Skills:\n      - skills/repo.md\n"
	if err := os.WriteFile(filepath.Join(dir, YAMLFilename), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(yaml without CatalogCutoff): %v", err)
	}
	if cfg.CatalogCutoff != 0 {
		t.Fatalf("CatalogCutoff absent in YAML: got %d, want 0", cfg.CatalogCutoff)
	}
}

func TestMarshal_DefaultConfig_OmitsCatalogCutoff(t *testing.T) {
	// DefaultConfig does not set CatalogCutoff; it must NOT appear in marshaled JSON
	// (omitempty ensures generated skillex.json is unchanged)
	cfg := DefaultConfig()
	data, err := Marshal(cfg, FormatJSON)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "CatalogCutoff") {
		t.Errorf("DefaultConfig marshal should not emit CatalogCutoff, got:\n%s", data)
	}
}

func TestMarshal_DefaultConfig_OmitsCatalogCutoff_YAML(t *testing.T) {
	data, err := Marshal(DefaultConfig(), FormatYAML)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CatalogCutoff") {
		t.Errorf("generated YAML must not contain CatalogCutoff when unset:\n%s", data)
	}
}

func TestLoad_BothJSONAndYAMLRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, JSONFilename), []byte(`{"Version":4,"Rules":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, YAMLFilename), []byte("Version: 4\nRules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when both skillex.json and skillex.yaml exist")
	}
	if !strings.Contains(err.Error(), JSONFilename) || !strings.Contains(err.Error(), YAMLFilename) {
		t.Fatalf("unexpected error: %v", err)
	}
}
