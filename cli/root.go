package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	flagJSON  bool
	flagQuiet bool
	rootCmd   *cobra.Command
)

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd = &cobra.Command{
		Use:   "skillex",
		Short: "Skill management for AI agents",
		Long: `Skillex manages skills — versioned, queryable documentation —
for AI agent workflows in Node.js projects.

Skills are Markdown files with YAML frontmatter that teach agents how to use
packages, follow repo conventions, and work safely in a codebase.`,
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output structured JSON to stdout")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress stderr output")

	rootCmd.AddCommand(
		newInitCmd(),
		newQueryCmd(),
		newRefreshCmd(),
		newGetCmd(),
		newImportCmd(),
		newTestCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newMCPCmd(),
	)
}

// findRepoRoot walks upward from startDir looking for skillex.json or
// skillex.yaml. Returns the nearest directory containing one, or startDir
// unchanged when no config exists anywhere above (commands then fail with
// their usual "config not found" / "registry not found" errors).
func findRepoRoot(startDir string) string {
	dir := startDir
	for {
		for _, name := range []string{"skillex.json", "skillex.yaml"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// repoRoot returns the skillex repo root for the current working directory.
func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return findRepoRoot(wd)
}

// initRoot returns the directory `skillex init` operates on: always the
// current working directory — init must be able to create a new repo root
// inside a larger tree without being captured by an ancestor config.
func initRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
