package frontmatter

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestFormatFrontmatter_RoundTripPreservesNameAndDescription(t *testing.T) {
	in := []byte("---\nname: My Skill\ndescription: \"Covers: a11y, focus traps\"\ntopics: [ui]\n---\n\nBody text\n")
	fm, body, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}

	out := FormatFrontmatter(fm) + "\n" + body
	fm2, body2, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("reparsing formatted output: %v\noutput:\n%s", err, out)
	}
	if fm2.Name != "My Skill" {
		t.Errorf("name lost in round trip: %q", fm2.Name)
	}
	if fm2.Description != "Covers: a11y, focus traps" {
		t.Errorf("description lost in round trip: %q", fm2.Description)
	}
	if body2 != body {
		t.Errorf("body changed in round trip")
	}
}

func TestFormatFrontmatter_ReviewedAloneIsKept(t *testing.T) {
	out := FormatFrontmatter(Frontmatter{Reviewed: "2026-06-12"})
	if !strings.Contains(out, "reviewed: 2026-06-12") {
		t.Errorf("reviewed-only frontmatter dropped: %q", out)
	}
}

func TestFormatFrontmatter_RoundTripPreservesTopicsAndTagsWithSpecialChars(t *testing.T) {
	// Topic/tag values can carry YAML flow indicators, either from --topic input
	// or by re-serializing a source file's own frontmatter. Each must survive a
	// FormatFrontmatter -> Parse round trip without corrupting or dropping the skill.
	in := Frontmatter{
		Topics: []string{"ui", "foo: bar", "a]b", "needs,comma", "#leadinghash"},
		Tags:   []string{"a11y", "x[y", "key: val"},
	}
	out := FormatFrontmatter(in) + "\n"
	fm2, _, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("reparsing topics/tags with special chars: %v\noutput:\n%s", err, out)
	}
	if !reflect.DeepEqual(fm2.Topics, in.Topics) {
		t.Errorf("topics corrupted in round trip: want %q, got %q\noutput:\n%s", in.Topics, fm2.Topics, out)
	}
	if !reflect.DeepEqual(fm2.Tags, in.Tags) {
		t.Errorf("tags corrupted in round trip: want %q, got %q\noutput:\n%s", in.Tags, fm2.Tags, out)
	}
}

func TestYamlScalar_QuotesNullAndBoolKeywordForms(t *testing.T) {
	keywords := []string{
		"null", "Null", "NULL", "~",
		"true", "True", "TRUE", "false", "False", "FALSE",
		"yes", "Yes", "YES", "no", "No", "NO",
		"on", "On", "ON", "off", "Off", "OFF",
	}
	for _, v := range keywords {
		if got := yamlScalar(v); got != strconv.Quote(v) {
			t.Errorf("yamlScalar(%q) = %q, want %q (plain form decodes as YAML null/bool)", v, got, strconv.Quote(v))
		}
	}
}

func TestFormatFrontmatter_RoundTripPreservesNullFormStrings(t *testing.T) {
	for _, v := range []string{"null", "~"} {
		out := FormatFrontmatter(Frontmatter{Name: v, Description: v}) + "\n"
		fm2, _, err := Parse([]byte(out))
		if err != nil {
			t.Fatalf("reparsing output for %q: %v\noutput:\n%s", v, err, out)
		}
		if fm2.Name != v {
			t.Errorf("name %q corrupted in round trip: got %q", v, fm2.Name)
		}
		if fm2.Description != v {
			t.Errorf("description %q corrupted in round trip: got %q", v, fm2.Description)
		}
	}
}

func TestFormatFrontmatter_RoundTripPreservesControlCharacters(t *testing.T) {
	for _, desc := range []string{"weird\x01value", "del\x7fchar"} {
		out := FormatFrontmatter(Frontmatter{Description: desc}) + "\n"
		fm2, _, err := Parse([]byte(out))
		if err != nil {
			t.Fatalf("reparsing output with control char %q: %v\noutput:\n%q", desc, err, out)
		}
		if fm2.Description != desc {
			t.Errorf("description with control char corrupted: want %q, got %q", desc, fm2.Description)
		}
	}
}

func TestFormatFrontmatter_RoundTripPreservesSourceAndReviewed(t *testing.T) {
	cases := []Frontmatter{
		{Source: "https://example.com/skill.md#section"},
		{Source: "#section"},
		{Reviewed: "~"},
		{Source: ",starts-with-comma"},
		{Reviewed: "?"},
		{Source: "nel\u0085break"}, // C1 control: NEL
	}
	for _, in := range cases {
		out := FormatFrontmatter(in) + "\n"
		fm2, _, err := Parse([]byte(out))
		if err != nil {
			t.Fatalf("reparsing %+v: %v\noutput:\n%s", in, err, out)
		}
		if fm2.Source != in.Source {
			t.Errorf("source corrupted in round trip: want %q, got %q\noutput:\n%s", in.Source, fm2.Source, out)
		}
		if fm2.Reviewed != in.Reviewed {
			t.Errorf("reviewed corrupted in round trip: want %q, got %q\noutput:\n%s", in.Reviewed, fm2.Reviewed, out)
		}
	}
}
