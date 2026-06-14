package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/course-studio/skillex/internal/frontmatter"
)

func newGetCmd() *cobra.Command {
	var (
		topicsFlag string
		skipReview bool
	)

	cmd := &cobra.Command{
		Use:   "get <url>",
		Short: "Fetch and vendor a skill from a remote source",
		Long: `Fetch a skill or skill pack from a remote URL, review it for safety,
and vendor it into skillex/vendor/.

The skill passes through a safety review that checks for:
  - Prompt injection patterns
  - File system manipulation instructions
  - Exfiltration attempts
  - Unusual runtime instructions

After review, the skill is converted to skillex format (frontmatter, test stubs)
and placed in skillex/vendor/<source>/.

Use --skip-review to bypass the review step for trusted sources.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			root := repoRoot()

			var topics []string
			if topicsFlag != "" {
				for _, t := range strings.Split(topicsFlag, ",") {
					if t = strings.TrimSpace(t); t != "" {
						topics = append(topics, t)
					}
				}
			}

			return runGet(root, url, topics, skipReview)
		},
	}

	cmd.Flags().StringVar(&topicsFlag, "topic", "", "Comma-separated topics to assign to fetched skills")
	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip the safety review step (not recommended)")

	return cmd
}

func runGet(root, url string, topics []string, skipReview bool) error {
	// Normalize GitHub URLs before anything else
	{
		rawURL, err := normalizeGitHubURL(url)
		if err != nil {
			return err
		}
		url = rawURL
	}

	if !flagQuiet {
		fmt.Fprintln(os.Stderr, styleHeader.Render("  skillex get  "))
		fmt.Fprintf(os.Stderr, "  Fetching %s\n", styleDim.Render(url))
	}

	// 1. Fetch
	content, err := fetchURL(url)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "  %s Fetched %d bytes\n", styleSuccess.Render("✓"), len(content))
	}

	// Reject HTML responses (e.g. directory listings, login pages)
	if isHTMLContent(content) {
		return fmt.Errorf("%s returned an HTML page, not a Markdown skill file; for GitHub files use the raw.githubusercontent.com URL", url)
	}

	// 2. Review (structural only — agent does semantic review)
	if !skipReview {
		issues := reviewContent(content)
		if len(issues) > 0 {
			fmt.Fprintf(os.Stderr, "\n%s Safety review flagged %d issue(s):\n",
				styleWarn.Render("!"), len(issues))
			for _, iss := range issues {
				fmt.Fprintf(os.Stderr, "  %s %s\n", styleDim.Render("•"), iss)
			}
			if flagQuiet || !stdinIsInteractive() {
				return fmt.Errorf("safety review flagged %d issue(s); rerun interactively to confirm, or use --skip-review for trusted sources", len(issues))
			}
			fmt.Fprint(os.Stderr, "\nProceed anyway? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				return fmt.Errorf("aborted by user")
			}
		} else if !flagQuiet {
			fmt.Fprintf(os.Stderr, "  %s Safety review passed\n", styleSuccess.Render("✓"))
		}
	}

	// 3. Convert and vendor
	vendorDir := skillVendorDir(root, url)
	filename := skillFilenameFromURL(url)
	skillPath := filepath.Join(vendorDir, filename)
	testPath := filepath.Join(vendorDir, strings.TrimSuffix(filename, ".md")+".test.md")

	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return fmt.Errorf("creating vendor directory: %w", err)
	}

	// Ensure frontmatter with topics and source
	normalized := normalizeSkillContent(content, topics, url)

	if err := os.WriteFile(skillPath, []byte(normalized), 0o644); err != nil {
		return fmt.Errorf("writing skill: %w", err)
	}

	// Write test stub if not exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		stub := generateTestStub(filename)
		if err := os.WriteFile(testPath, []byte(stub), 0o644); err != nil {
			return fmt.Errorf("writing test stub: %w", err)
		}
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "\n%s Vendored to %s\n",
			styleSuccess.Render("✓"),
			styleDim.Render(skillPath),
		)
		fmt.Fprintln(os.Stderr, "\nNext steps:")
		fmt.Fprintln(os.Stderr, "  • Review the vendored skill and edit as needed")
		fmt.Fprintln(os.Stderr, "  • Run 'skillex refresh' to index the new skill")
	}

	return nil
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func fetchURL(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "skillex/"+Version)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, err
	}
	return data, nil
}

var errTreeURL = errors.New("github /tree/ URLs return an HTML directory listing, not a file; pass the file's raw URL (https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>) or the /blob/ URL")

// normalizeGitHubURL rewrites github.com /blob/ URLs to raw.githubusercontent.com
// and rejects /tree/ URLs. Non-GitHub URLs pass through untouched.
func normalizeGitHubURL(url string) (string, error) {
	rest, found := strings.CutPrefix(url, "https://github.com/")
	if !found {
		return url, nil
	}
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) < 4 {
		return url, nil
	}
	switch parts[2] {
	case "blob":
		return "https://raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/" + parts[3], nil
	case "tree":
		return "", errTreeURL
	}
	return url, nil
}

// isHTMLContent reports whether the payload looks like an HTML document
// rather than a Markdown skill file.
func isHTMLContent(content []byte) bool {
	head := content
	if len(head) > 512 {
		head = head[:512]
	}
	head = bytes.TrimPrefix(head, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
	s := strings.ToLower(strings.TrimSpace(string(head)))
	return strings.HasPrefix(s, "<!doctype html") || strings.HasPrefix(s, "<html")
}

// stdinIsInteractive reports whether stdin is a terminal; when it is not,
// the review prompt cannot be answered and review failures are fatal.
func stdinIsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// reviewContent performs basic structural safety checks on skill content.
// Semantic review is the responsibility of the agent.
func reviewContent(content []byte) []string {
	var issues []string
	s := strings.ToLower(string(content))

	patterns := []struct {
		pattern string
		desc    string
	}{
		{"ignore previous", "potential prompt injection: 'ignore previous'"},
		{"ignore all previous", "potential prompt injection: 'ignore all previous'"},
		{"disregard", "potential prompt injection: 'disregard'"},
		{"system prompt", "references system prompt"},
		{"curl ", "contains curl command"},
		{"wget ", "contains wget command"},
		{"exfiltrate", "potential exfiltration mention"},
		{"rm -rf", "dangerous file system command"},
	}

	for _, p := range patterns {
		if strings.Contains(s, p.pattern) {
			issues = append(issues, p.desc)
		}
	}

	return issues
}

func skillVendorDir(root, url string) string {
	source := sanitizeSource(url)
	return filepath.Join(root, "skillex", "vendor", source)
}

func sanitizeSource(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	// Replace special chars with filesystem-safe chars
	replacer := strings.NewReplacer(
		"/", string(filepath.Separator),
		"?", "_",
		"#", "_",
		":", "_",
	)
	return replacer.Replace(url)
}

func skillFilenameFromURL(url string) string {
	parts := strings.Split(url, "/")
	last := parts[len(parts)-1]
	if last == "" && len(parts) > 1 {
		last = parts[len(parts)-2]
	}
	if !strings.HasSuffix(last, ".md") {
		last += ".md"
	}
	return last
}

func normalizeSkillContent(content []byte, topics []string, sourceURL string) string {
	fm, body, err := frontmatter.Parse(content)
	if err != nil || (len(fm.Topics) == 0 && len(topics) > 0) {
		fm.Topics = append(fm.Topics, topics...)
	}
	if sourceURL != "" {
		fm.Source = sourceURL
	}

	fmStr := frontmatter.FormatFrontmatter(fm)
	if fmStr == "" {
		return body
	}
	return fmStr + "\n" + body
}

func generateTestStub(skillFilename string) string {
	return fmt.Sprintf(`# Tests: %s

## Validation: Basic usage
Prompt: "TODO: write a test prompt for this skill"
Success criteria:
  - TODO: add success criteria

`, skillFilename)
}
