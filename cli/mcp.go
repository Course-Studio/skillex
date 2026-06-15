package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/course-studio/skillex/internal/registry"
	mcpserver "github.com/course-studio/skillex/mcp"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio transport)",
		Long: `Start the skillex MCP server using stdio transport.

The MCP server exposes:
  - Resources: each skill as a discoverable MCP resource
  - Tool: skillex_query for structured skill queries

Configure in your agent harness:

  {
    "mcpServers": {
      "skillex": {
        "command": "skillex",
        "args": ["mcp"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := repoRoot()

			// Auto-build a missing/unreadable index so the MCP server works in a
			// fresh checkout or worktree. Build progress MUST go to stderr only —
			// stdout carries the JSON-RPC protocol stream and must stay byte-clean.
			reg, err := registry.EnsureIndex(root, os.Stderr)
			if err != nil {
				if errors.Is(err, registry.ErrAutoRefreshDisabled) {
					dbPath := filepath.Join(root, ".skillex", "index.db")
					return fmt.Errorf("registry not found at %s — run 'skillex refresh' first", dbPath)
				}
				return err
			}
			defer reg.Close()

			return mcpserver.Serve(reg, root, Version)
		},
	}
}
