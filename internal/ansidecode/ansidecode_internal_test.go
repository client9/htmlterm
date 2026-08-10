package ansidecode

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestParsePlainTextNoEscapes(t *testing.T) {
	got := Parse("Row 1\nRow 2\n")
	want := []Line{
		{Runs: []Run{{Text: "Row 1", Width: 5, Style: Style{}}}},
		{Runs: []Run{{Text: "Row 2", Width: 5, Style: Style{}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse = %+v, want %+v", got, want)
	}
}

func TestParseTrailingNewlineDoesNotAddEmptyLine(t *testing.T) {
	withNL := Parse("hi\n")
	withoutNL := Parse("hi")
	if !reflect.DeepEqual(withNL, withoutNL) {
		t.Errorf("Parse(%q) = %+v, want same as Parse(%q) = %+v", "hi\n", withNL, "hi", withoutNL)
	}
	if len(withNL) != 1 {
		t.Errorf("Parse(%q) has %d lines, want 1", "hi\n", len(withNL))
	}
}

func TestParseBlankLineIsPreserved(t *testing.T) {
	got := Parse("a\n\nb\n")
	if len(got) != 3 {
		t.Fatalf("Parse(%q) has %d lines, want 3: %+v", "a\n\nb\n", len(got), got)
	}
	if len(got[1].Runs) != 0 {
		t.Errorf("middle line = %+v, want no runs", got[1])
	}
}

// TestParseRoundTripsAnsiStyle builds strings with the same x/ansi.Style
// type internal/render/style.go uses, the codebase's actual encoder, and
// checks Parse recovers every field. This is the symmetry check: if the
// encoder ever changes its wire format, this test feels it without needing
// to touch internal/render at all.
func TestParseRoundTripsAnsiStyle(t *testing.T) {
	red := color.RGBA{R: 0xff, A: 0xff}
	tests := []struct {
		name  string
		style ansi.Style
		want  Style
	}{
		{
			name:  "bold italic underline strike truecolor fg",
			style: ansi.Style{}.Bold().Italic(true).Underline(true).Strikethrough(true).ForegroundColor(red),
			want:  Style{FG: red, Bold: true, Italic: true, Underline: true, Strike: true},
		},
		{
			name:  "reverse only",
			style: ansi.Style{}.Reverse(true),
			want:  Style{Reverse: true},
		},
		{
			name:  "fg and bg both set",
			style: ansi.Style{}.ForegroundColor(color.RGBA{G: 0xff, A: 0xff}).BackgroundColor(color.RGBA{B: 0xff, A: 0xff}),
			want:  Style{FG: color.RGBA{G: 0xff, A: 0xff}, BG: color.RGBA{B: 0xff, A: 0xff}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.style.Styled("hi") + "\n"
			lines := Parse(s)
			if len(lines) != 1 || len(lines[0].Runs) != 1 {
				t.Fatalf("Parse(%q) = %+v, want exactly one line with one run", s, lines)
			}
			run := lines[0].Runs[0]
			if run.Text != "hi" || run.Width != 2 {
				t.Errorf("run text/width = %q/%d, want \"hi\"/2", run.Text, run.Width)
			}
			if run.Style != tc.want {
				t.Errorf("run.Style = %+v, want %+v", run.Style, tc.want)
			}
		})
	}
}

func TestParseHyperlink(t *testing.T) {
	s := ansi.SetHyperlink("https://example.com") + "click" + ansi.ResetHyperlink() + "\n"
	lines := Parse(s)
	if len(lines) != 1 || len(lines[0].Runs) != 1 {
		t.Fatalf("Parse(%q) = %+v, want exactly one line with one run", s, lines)
	}
	run := lines[0].Runs[0]
	if run.Text != "click" {
		t.Errorf("run.Text = %q, want %q", run.Text, "click")
	}
	if run.Style.Href != "https://example.com" {
		t.Errorf("run.Style.Href = %q, want %q", run.Style.Href, "https://example.com")
	}
}

// TestParseCoalescesSameStyleRuns confirms two adjacent escape sequences
// that leave the effective style unchanged, here a redundant re-assertion
// of the same color, don't split what should be one run. Real htmlterm
// output never does this; a hand-crafted or third-party ANSI string might.
func TestParseCoalescesSameStyleRuns(t *testing.T) {
	red := ansi.Style{}.ForegroundColor(color.RGBA{R: 0xff, A: 0xff}).String()
	s := red + "ab" + red + "cd" + ansi.ResetStyle + "\n"
	lines := Parse(s)
	if len(lines) != 1 || len(lines[0].Runs) != 1 {
		t.Fatalf("Parse(%q) = %+v, want one coalesced run", s, lines)
	}
	if got := lines[0].Runs[0].Text; got != "abcd" {
		t.Errorf("run.Text = %q, want %q", got, "abcd")
	}
}

// TestParseWideGraphemeWidth confirms Width reflects terminal cell width,
// not rune count, for a double-width character. Getting this wrong is the
// exact bug docs/ARCHITECTURE.md warns about: "arithmetic that adds a rune
// count to a column is wrong even when its tests pass" on an ASCII corpus.
func TestParseWideGraphemeWidth(t *testing.T) {
	lines := Parse("日\n")
	if len(lines) != 1 || len(lines[0].Runs) != 1 {
		t.Fatalf("Parse(%q) = %+v", "日\n", lines)
	}
	if got := lines[0].Runs[0].Width; got != 2 {
		t.Errorf("Width(%q) = %d, want 2", "日", got)
	}
}

func TestApplySGRIndexedAndBasicColors(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   color.Color
		bg     bool
	}{
		{"basic fg red", "31", basic16[1], false},
		{"bright fg red", "91", basic16[9], false},
		{"basic bg blue", "44", basic16[4], true},
		{"256-color fg pure red (196)", "38;5;196", color.RGBA{R: 0xff, A: 0xff}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s Style
			applySGR("\x1b["+tc.params+"m", &s)
			got := s.FG
			if tc.bg {
				got = s.BG
			}
			if got != tc.want {
				t.Errorf("applySGR(%q) color = %#v, want %#v", tc.params, got, tc.want)
			}
		})
	}
}

func TestPlainTextMatchesOriginalModuloStyle(t *testing.T) {
	s := "Row 1\n\nRow 3\n"
	if got := PlainText(Parse(s)); got != strings.TrimSuffix(s, "\n") {
		t.Errorf("PlainText(Parse(%q)) = %q, want %q", s, got, strings.TrimSuffix(s, "\n"))
	}
}
