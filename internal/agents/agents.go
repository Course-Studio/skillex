package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/course-studio/skillex/internal/registry"
)

const (
	markerStart = "<!-- skillex:start -->"
	markerEnd   = "<!-- skillex:end -->"
)

// DefaultCatalogCutoff is the maximum number of skills for which GenerateSection
// emits a per-skill catalog. Above it, only the taxonomy vocabulary is shown
// to keep the always-in-context AGENTS.md section bounded.
// This default applies when no CatalogCutoff is set in skillex.json/yaml (i.e. 0).
const DefaultCatalogCutoff = 40

// truncateDescription shortens s to at most 140 characters, adding an ellipsis.
// It counts runes (not bytes) so multibyte UTF-8 sequences are never split.
func truncateDescription(s string) string {
	const max = 140
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

// GenerateSection creates the AGENTS.md section content from registry data,
// using the DefaultCatalogCutoff for the catalog threshold.
func GenerateSection(reg *registry.Registry) (string, error) {
	return GenerateSectionWithCutoff(reg, DefaultCatalogCutoff)
}

// GenerateSectionWithCutoff creates the AGENTS.md section content from registry data.
// cutoff controls how many skills trigger a full per-skill catalog; if cutoff <= 0 it
// resolves to DefaultCatalogCutoff. Above the cutoff only the vocabulary is shown.
func GenerateSectionWithCutoff(reg *registry.Registry, cutoff int) (string, error) {
	if cutoff <= 0 {
		cutoff = DefaultCatalogCutoff
	}

	topics, err := reg.AllTopics()
	if err != nil {
		return "", fmt.Errorf("fetching topics: %w", err)
	}

	tags, err := reg.AllTags()
	if err != nil {
		return "", fmt.Errorf("fetching tags: %w", err)
	}

	scopes, err := reg.AllScopes()
	if err != nil {
		return "", fmt.Errorf("fetching scopes: %w", err)
	}

	packages, err := reg.AllPackages()
	if err != nil {
		return "", fmt.Errorf("fetching packages: %w", err)
	}

	skills, err := reg.AllSkills()
	if err != nil {
		return "", fmt.Errorf("fetching skills: %w", err)
	}

	var sb strings.Builder

	sb.WriteString(markerStart + "\n")
	sb.WriteString("## Skillex\n\n")
	sb.WriteString("This project uses Skillex for skill management. Use the skillex MCP server\n")
	sb.WriteString("if available (preferred), otherwise use the CLI commands below.\n\n")

	sb.WriteString("### MCP (preferred)\n\n")
	sb.WriteString("If the `skillex` MCP server is connected, use it directly:\n\n")
	sb.WriteString("- Use the `skillex_query` tool with parameters: path, topic, tags, package, search, format.\n")
	sb.WriteString("- `search` matches skill names, descriptions, topics, and tags — pass space/comma-separated concepts for intent-based discovery.\n")
	sb.WriteString("- `path` accepts a repo-relative path or an absolute path inside this repository; it returns the skills scoped to that location.\n")
	sb.WriteString("- A query with no parameters returns the available scopes, topics, tags, and packages (the vocabulary) to guide a real query.\n\n")

	sb.WriteString("### CLI (fallback)\n\n")
	sb.WriteString("If MCP is not available, query skills via the command line:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("  skillex query --search \"<concepts>\"\n")
	sb.WriteString("  skillex query --path <filepath>\n")
	sb.WriteString("  skillex query --topic <topic> --tags <tags>\n")
	sb.WriteString("  skillex query --package <package>\n")
	sb.WriteString("  skillex query --path <glob> --topic <topic> --format content\n")
	sb.WriteString("```\n\n")

	if len(skills) > 0 && len(skills) <= cutoff {
		sb.WriteString("### Skills\n\n")
		sb.WriteString("The skills available in this repository, grouped by the path scope that loads them:\n\n")
		for _, scope := range scopes {
			var lines []string
			for _, sk := range skills {
				if !containsString(sk.Scopes, scope) {
					continue
				}
				line := "  - " + sk.Path
				if sk.Name != "" {
					line += " — " + sk.Name
					if sk.Description != "" {
						line += ": " + truncateDescription(sk.Description)
					}
				} else if sk.Description != "" {
					line += " — " + truncateDescription(sk.Description)
				}
				lines = append(lines, line)
			}
			if len(lines) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("`%s`\n", scope))
			for _, l := range lines {
				sb.WriteString(l + "\n")
			}
			sb.WriteString("\n")
		}
	}

	if len(scopes) > 0 {
		sb.WriteString("### Available scopes\n\n")
		for _, scope := range scopes {
			sb.WriteString(fmt.Sprintf("  - %s\n", scope))
		}
		sb.WriteString("\n")
	}

	if len(topics) > 0 {
		sb.WriteString("### Available topics\n\n")
		sb.WriteString("  ")
		sb.WriteString(strings.Join(topics, ", "))
		sb.WriteString("\n\n")
	}

	if len(tags) > 0 {
		sb.WriteString("### Available tags\n\n")
		sb.WriteString("  ")
		sb.WriteString(strings.Join(tags, ", "))
		sb.WriteString("\n\n")
	}

	if len(packages) > 0 {
		sb.WriteString("### Packages with skills\n\n")
		for _, p := range packages {
			version := p.Version
			if version == "" {
				version = "unknown"
			}
			sb.WriteString(fmt.Sprintf("  %s (%s) — %d public, %d private\n",
				p.Name, version, p.Public, p.Private))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(markerEnd + "\n")

	return sb.String(), nil
}

// UpdateFile writes (or updates) the skillex section in the AGENTS.md file.
// If the file does not exist, it creates it.
// If it does exist, it replaces the content between markers.
func UpdateFile(agentsPath string, section string) error {
	var existing string
	data, err := os.ReadFile(agentsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading AGENTS.md: %w", err)
	}
	if err == nil {
		existing = string(data)
	}

	updated := replaceSection(existing, section)

	if err := os.MkdirAll(filepath.Dir(agentsPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(agentsPath, []byte(updated), 0o644)
}

// replaceSection replaces the content between markers, or appends if not found.
func replaceSection(existing, section string) string {
	return replaceMarkedSection(existing, section, markerStart, markerEnd)
}

func replaceMarkedSection(existing, section string, startMarker string, endMarker string) string {
	startIdx := strings.Index(existing, startMarker)
	endIdx := strings.Index(existing, endMarker)

	if startIdx == -1 || endIdx == -1 {
		// Markers not found — append
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		return existing + "\n" + section
	}

	before := existing[:startIdx]
	after := existing[endIdx+len(endMarker):]
	if strings.HasPrefix(after, "\n") {
		after = after[1:]
	}

	return before + section + after
}

// DefaultContent returns the initial AGENTS.md content for a new repo.
func DefaultContent() string {
	return "# AGENTS\n\nThis file documents how to work in this repository.\n\n"
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
