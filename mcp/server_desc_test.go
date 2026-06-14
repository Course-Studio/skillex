package mcp

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/course-studio/skillex/internal/registry"
)

// TestSkillDescription_RuneSafeTruncation: a long multibyte description must be
// truncated on a rune boundary, not a raw byte offset, or the MCP listing emits
// invalid UTF-8. The 116 ASCII + multibyte construction places byte 117 in the
// middle of a 3-byte rune, which a desc[:117] slice would split.
func TestSkillDescription_RuneSafeTruncation(t *testing.T) {
	long := strings.Repeat("a", 116) + strings.Repeat("€", 10) // 126 runes, 146 bytes
	out := skillDescription(registry.Skill{Name: "N", Description: long, Visibility: "repo"})
	if !utf8.ValidString(out) {
		t.Errorf("skillDescription produced invalid UTF-8 (truncation split a rune): %q", out)
	}
}
