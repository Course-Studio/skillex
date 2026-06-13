package acceptance

import (
	"strings"
	"testing"

	"github.com/Course-Studio/skillex/test/helpers"
)

func TestImport_VisibilityFlagRemoved(t *testing.T) {
	dir := t.TempDir()
	res := helpers.Run(t, dir, "import", "whatever.md", "--visibility", "public")
	if res.ExitCode == 0 || !strings.Contains(res.Stderr, "unknown flag") {
		t.Errorf("want unknown-flag failure, got exit=%d stderr=%q", res.ExitCode, res.Stderr)
	}
}
