package frontmatter

import (
	"bytes"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the YAML frontmatter metadata from a skill file.
type Frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Topics      []string `yaml:"topics"`
	Tags        []string `yaml:"tags"`
	Source      string   `yaml:"source"`
	Reviewed    string   `yaml:"reviewed"`
}

// Parse separates YAML frontmatter from Markdown body.
// Returns the parsed frontmatter and the body content.
// If no frontmatter is found, returns an empty Frontmatter and the full content as body.
func Parse(content []byte) (Frontmatter, string, error) {
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return Frontmatter{}, s, nil
	}

	// Find the closing ---
	rest := s[3:]
	// Skip optional newline after opening ---
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		// No closing delimiter found; treat whole thing as body
		return Frontmatter{}, s, nil
	}

	fmRaw := rest[:idx]
	body := rest[idx+4:] // skip \n---
	// Trim leading newline from body
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	} else if strings.HasPrefix(body, "\r\n") {
		body = body[2:]
	}

	var fm Frontmatter
	if err := yaml.NewDecoder(bytes.NewBufferString(fmRaw)).Decode(&fm); err != nil {
		return Frontmatter{}, s, err
	}

	return fm, body, nil
}

// yamlScalar renders s as a YAML scalar: plain when unambiguous,
// double-quoted otherwise. strconv.Quote output is valid YAML
// double-quote syntax for every escape it produces from valid UTF-8
// input; our inputs come from parsed YAML/UTF-8 files, so the
// invalid-UTF-8 \xNN byte-escape divergence never applies.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	// YAML 1.1/1.2 null and boolean keyword forms decode as non-strings
	// when emitted plain; quote every case variant (ToLower folds them all).
	switch strings.ToLower(s) {
	case "~", "null", "true", "false", "yes", "no", "on", "off":
		return strconv.Quote(s)
	}
	// The char list covers YAML indicators plus comment/anchor/alias/directive markers and quotes.
	// Leading '?'/',' are key/flow indicators; the IndexFunc catches C0 controls, DEL, and C1 controls
	// (U+0080-U+009F; yaml.v3 treats NEL U+0085 as a line break). Over-quoting "?foo" is harmless.
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`\n\t") ||
		strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.HasPrefix(s, "-") ||
		strings.HasPrefix(s, "?") || strings.HasPrefix(s, ",") ||
		strings.IndexFunc(s, func(r rune) bool {
			return (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (0x80 <= r && r <= 0x9f)
		}) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

// yamlFlowSeq renders a string slice as a YAML flow sequence so values containing
// flow indicators ('[', ']', '#', ':', ',') survive a FormatFrontmatter -> Parse
// round trip instead of corrupting or silently truncating the sequence. A comma
// separates elements in flow context, so a comma-bearing value must be quoted even
// though yamlScalar leaves it plain in block context; strconv.Quote also covers any
// other indicators such a value might hold. Everything else defers to yamlScalar.
func yamlFlowSeq(items []string) string {
	quoted := make([]string, len(items))
	for i, it := range items {
		if strings.Contains(it, ",") {
			quoted[i] = strconv.Quote(it)
		} else {
			quoted[i] = yamlScalar(it)
		}
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// FormatFrontmatter serializes frontmatter fields into a YAML block.
func FormatFrontmatter(fm Frontmatter) string {
	if fm.Name == "" && fm.Description == "" && len(fm.Topics) == 0 &&
		len(fm.Tags) == 0 && fm.Source == "" && fm.Reviewed == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	if fm.Name != "" {
		sb.WriteString("name: " + yamlScalar(fm.Name) + "\n")
	}
	if fm.Description != "" {
		sb.WriteString("description: " + yamlScalar(fm.Description) + "\n")
	}
	if len(fm.Topics) > 0 {
		sb.WriteString("topics: " + yamlFlowSeq(fm.Topics) + "\n")
	}
	if len(fm.Tags) > 0 {
		sb.WriteString("tags: " + yamlFlowSeq(fm.Tags) + "\n")
	}
	if fm.Source != "" {
		sb.WriteString("source: " + yamlScalar(fm.Source) + "\n")
	}
	if fm.Reviewed != "" {
		sb.WriteString("reviewed: " + yamlScalar(fm.Reviewed) + "\n")
	}
	sb.WriteString("---")
	return sb.String()
}
