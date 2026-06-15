package acceptance

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/course-studio/skillex/test/helpers"
)

const threeSkillConfig = `{
  "Version": 4,
  "Rules": [
    {"Scope": "**", "Skills": ["skills/a.md", "skills/b.md", "skills/c.md"]}
  ]
}`

// query auto-builds a missing index instead of erroring.
func TestQuery_AutoBuildsMissingIndex(t *testing.T) {
	dir := writeCorruptionFixture(t, threeSkillConfig)
	dbPath := filepath.Join(dir, ".skillex", "index.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: index should be missing, got %v", err)
	}

	var q queryJSON
	res := helpers.RunJSON(t, dir, &q, "query", "--topic", "topic-a", "--json")
	if res.ExitCode != 0 {
		t.Fatalf("query should auto-build a missing index, got exit %d:\nstderr: %s", res.ExitCode, res.Stderr)
	}
	if q.Type != "results" || len(q.Results) != 1 || q.Results[0].Path != "skills/a.md" {
		t.Fatalf("query --topic topic-a after auto-build: want skills/a.md, got %+v", q)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("index should exist on disk after auto-build: %v", err)
	}
}

// mcp auto-builds a missing index AND keeps the JSON-RPC stdout stream byte-clean
// (build progress must go to stderr only).
func TestMCP_AutoBuildsMissingIndexWithCleanStdout(t *testing.T) {
	dir := writeCorruptionFixture(t, threeSkillConfig)
	if _, err := os.Stat(filepath.Join(dir, ".skillex", "index.db")); !os.IsNotExist(err) {
		t.Fatalf("precondition: index should be missing, got %v", err)
	}

	cmd := exec.Command(helpers.SkilexBinary(), "mcp")
	cmd.Dir = dir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting mcp: %v", err)
	}

	// Drive a minimal session, then close stdin so the server exits cleanly.
	reqs := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}`,
	}, "\n") + "\n"
	if _, err := io.WriteString(stdin, reqs); err != nil {
		t.Fatalf("writing requests: %v", err)
	}
	stdin.Close()

	out, _ := io.ReadAll(stdout)
	_ = cmd.Wait()

	// 1. Every non-empty stdout line must be valid JSON-RPC — no build chatter.
	foundResources := false
	for i, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if !json.Valid([]byte(ln)) {
			t.Fatalf("stdout line %d is not valid JSON-RPC — non-protocol bytes leaked onto stdout:\n%q\n\nfull stdout:\n%s\n\nstderr:\n%s", i, ln, out, stderrBuf.String())
		}
		var msg struct {
			ID     int `json:"id"`
			Result struct {
				Resources []json.RawMessage `json:"resources"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(ln), &msg); err == nil && msg.ID == 2 && len(msg.Result.Resources) > 0 {
			foundResources = true
		}
	}

	// 2. The auto-build actually populated the index (resources were served).
	if !foundResources {
		t.Fatalf("resources/list returned no resources — auto-build did not populate the index\nstdout:\n%s\nstderr:\n%s", out, stderrBuf.String())
	}

	// 3. The index now exists on disk.
	if _, err := os.Stat(filepath.Join(dir, ".skillex", "index.db")); err != nil {
		t.Errorf("index should exist after mcp auto-build: %v", err)
	}
}

// Multiple processes cold-starting on a fresh tree must each succeed without
// corrupting the index (concurrency-safe cold start).
func TestEdge_ConcurrentColdStartQuery(t *testing.T) {
	dir := writeCorruptionFixture(t, threeSkillConfig)

	const n = 4
	type outcome struct {
		exit   int
		stderr string
	}
	ch := make(chan outcome, n)
	for i := 0; i < n; i++ {
		go func() {
			r := helpers.Run(t, dir, "query", "--topic", "topic-a", "--json")
			ch <- outcome{r.ExitCode, r.Stderr}
		}()
	}
	for i := 0; i < n; i++ {
		o := <-ch
		if o.exit != 0 {
			t.Errorf("concurrent cold-start query failed (exit %d): %s", o.exit, o.stderr)
		}
	}

	// The final index must be intact and queryable.
	var q queryJSON
	res := helpers.RunJSON(t, dir, &q, "query", "--topic", "topic-a", "--json")
	if res.ExitCode != 0 || len(q.Results) != 1 || q.Results[0].Path != "skills/a.md" {
		t.Fatalf("final query after concurrent cold start: exit %d, got %+v\nstderr: %s", res.ExitCode, q, res.Stderr)
	}
}

// With opt-out set, mcp does NOT auto-build and preserves its exact legacy error
// (which, unlike query's, includes the index path).
func TestMCP_OptOutErrorsOnMissingIndex(t *testing.T) {
	t.Setenv("SKILLEX_NO_AUTO_REFRESH", "1")
	dir := writeCorruptionFixture(t, threeSkillConfig)
	dbPath := filepath.Join(dir, ".skillex", "index.db")

	// EnsureIndex returns the sentinel before the server starts reading stdin, so
	// `mcp` exits immediately without a handshake.
	res := helpers.Run(t, dir, "mcp")
	if res.ExitCode == 0 {
		t.Error("mcp should fail on a missing index when auto-refresh is disabled")
	}
	want := "registry not found at " + dbPath + " — run 'skillex refresh' first"
	if !strings.Contains(res.Stderr, want) {
		t.Errorf("mcp opt-out error: want %q in stderr, got: %q", want, res.Stderr)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("opt-out must not create the index")
	}
}
