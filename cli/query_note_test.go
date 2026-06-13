package cli

import (
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/course-studio/skillex/internal/query"
)

// captureStderr swaps os.Stderr for a pipe while fn runs and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stderr = old
	return <-done
}

// TestPrintNoMatch_SurfacesOutsideRepoNote: query.Execute sets resp.Note to explain
// an out-of-repo --path, and the changelog advertises it. That note reaches agents
// via --json/MCP, but the human CLI renderer must surface it too — otherwise an
// interactive user only sees "No skills matched" with no reason.
func TestPrintNoMatch_SurfacesOutsideRepoNote(t *testing.T) {
	resp := &query.Response{
		Type:       query.ResponseTypeNoMatch,
		Query:      &query.QueryEcho{Path: "/abs/outside/x.ts"},
		Note:       "path is outside this repository: /abs/outside/x.ts",
		Vocabulary: &query.Vocabulary{TotalSkills: 0},
	}
	out := captureStderr(t, func() { printNoMatch(resp) })
	if !strings.Contains(out, "outside this repository") {
		t.Errorf("printNoMatch did not surface resp.Note to stderr.\ngot:\n%s", out)
	}
}

// TestQueryCmd_SearchHelpMentionsTopicsAndTags: search now also matches topics and
// tags (and the AGENTS.md/MCP descriptions say so), so the CLI --search help must
// not understate it.
func TestQueryCmd_SearchHelpMentionsTopicsAndTags(t *testing.T) {
	usage := newQueryCmd().Flags().Lookup("search").Usage
	if !strings.Contains(usage, "topics") || !strings.Contains(usage, "tags") {
		t.Errorf("--search usage is stale (missing topics/tags): %q", usage)
	}
}

// TestTruncateDescription_RuneSafe: the summary renderer must truncate on a rune
// boundary, not a raw byte offset, or a multibyte description becomes invalid UTF-8.
func TestTruncateDescription_RuneSafe(t *testing.T) {
	long := strings.Repeat("a", 116) + strings.Repeat("€", 10) // byte 117 lands mid-rune
	if out := truncateDescription(long); !utf8.ValidString(out) {
		t.Errorf("truncateDescription split a rune (invalid UTF-8): %q", out)
	}
	if got := truncateDescription("short"); got != "short" {
		t.Errorf("truncateDescription(short) = %q, want unchanged", got)
	}
}
