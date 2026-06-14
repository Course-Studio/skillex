package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureMCP_ClaudeCodeWritesRootMcpJsonNpx(t *testing.T) {
	root := t.TempDir()
	if err := configureMCP(root, "claude-code"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "mcp.json")); !os.IsNotExist(err) {
		t.Error("claude-code must not write .claude/mcp.json anymore")
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("root .mcp.json not written: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"npx"`) || !strings.Contains(s, `"skillex"`) || !strings.Contains(s, `"mcp"`) {
		t.Errorf("expected npx skillex mcp config, got:\n%s", s)
	}
}

func TestConfigureMCP_ClaudeCodeUsesPnpmWhenLockfilePresent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureMCP(root, "claude-code"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("root .mcp.json not written: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"pnpm"`) || !strings.Contains(s, `"exec"`) {
		t.Errorf("expected pnpm exec skillex mcp config, got:\n%s", s)
	}
}

func TestConfigureMCP_ClaudeCodeUsesPnpmWhenWorkspacePresent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureMCP(root, "claude-code"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("root .mcp.json not written: %v", err)
	}
	if !strings.Contains(string(data), `"pnpm"`) {
		t.Errorf("pnpm-workspace.yaml without a lockfile should still select pnpm, got:\n%s", data)
	}
}

func TestConfigureMCP_ClaudeCodeSkipsWhenRootMcpJsonExists(t *testing.T) {
	root := t.TempDir()
	existing := `{"mcpServers":{"other":{"command":"x"}}}`
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureMCP(root, "claude-code"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf("root .mcp.json missing after skip: %v", err)
	}
	if string(data) != existing {
		t.Errorf("existing .mcp.json must not be overwritten; got:\n%s", data)
	}
}

func TestConfigureMCP_CursorUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := configureMCP(root, "cursor"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("cursor config should be at .cursor/mcp.json: %v", err)
	}
	if !strings.Contains(string(data), `"skillex"`) {
		t.Error("cursor config should keep command skillex")
	}
}

func TestConfigureMCP_WindsurfUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := configureMCP(root, "windsurf"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".windsurf", "mcp.json"))
	if err != nil {
		t.Fatalf("windsurf config should be at .windsurf/mcp.json: %v", err)
	}
	if !strings.Contains(string(data), `"skillex"`) {
		t.Error("windsurf config should keep command skillex")
	}
}

func TestConfigureMCP_UnknownHarnessErrors(t *testing.T) {
	root := t.TempDir()
	if err := configureMCP(root, "bogus"); err == nil {
		t.Error("unknown harness must return an error")
	}
}
