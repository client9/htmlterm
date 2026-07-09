package render

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// TestWordWrapANSIStyleCarry verifies that a single SGR-styled run spanning
// multiple wrapped lines is closed before each line break and reopened at
// the start of the next line, so every emitted line is self-contained.
func TestWordWrapANSIStyleCarry(t *testing.T) {
	text := "\x1b[4m" + "alpha beta gamma delta epsilon" + "\x1b[m"
	got := wordWrapANSI(text, 12, "")
	want := []string{
		"\x1b[4malpha beta\x1b[m",
		"\x1b[4mgamma delta\x1b[m",
		"\x1b[4mepsilon\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI() = %#v, want %#v", got, want)
	}
}

// TestWrapHyperlinkMarginNotUnderlined is the wrap.html bug scenario: a
// hyperlink's underlined text wraps inside a block with left/right margin.
// The margin spaces on each wrapped line must fall outside the underline
// and hyperlink span, and every wrapped line must remain independently
// styled and clickable.
func TestWrapHyperlinkMarginNotUnderlined(t *testing.T) {
	r, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(`<p style="margin: 0 3">` +
		`<a href="https://x.test/">alpha beta gamma delta epsilon</a></p>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	osc, oscReset := "\x1b]8;;https://x.test/\x07", "\x1b]8;;\x07"
	wantContent := strings.Join([]string{
		osc + "\x1b[4malpha beta\x1b[m" + oscReset,
		osc + "\x1b[4mgamma delta\x1b[m" + oscReset,
		osc + "\x1b[4mepsilon\x1b[m" + oscReset,
	}, "\n")
	// margin: 0 3 sets margin-top and margin-bottom to 0 (CSS 2-value
	// shorthand), overriding the UA stylesheet's default p margin-bottom:1,
	// so there's no extra trailing blank line here.
	want := applyLineEdges(wantContent, "   ", "   ") + "\n"
	if out != want {
		t.Fatalf("Render() =\n%q\nwant:\n%q", out, want)
	}
}

// TestSplitAtVisualWidthCarryHardBreak verifies that a hard break
// (break-word) inside a single overlong styled token also closes and
// reopens the style at each internal cut point.
func TestSplitAtVisualWidthCarryHardBreak(t *testing.T) {
	text := "\x1b[1m" + "Supercalifragilisticexpialidocious" + "\x1b[m"
	got := wordWrapANSI(text, 10, "break-word")
	want := []string{
		"\x1b[1mSupercalif\x1b[m",
		"\x1b[1mragilistic\x1b[m",
		"\x1b[1mexpialidoc\x1b[m",
		"\x1b[1mious\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI(break-word) = %#v, want %#v", got, want)
	}
}

// TestWordWrapANSINoDoubleOpenOnCombinedBreak guards against a naive
// close+reopen implementation double-emitting the open sequence when a
// normal width-overflow break and a hard mid-token break both fire for the
// same token (an over-width word arriving right after already-packed words).
func TestWordWrapANSINoDoubleOpenOnCombinedBreak(t *testing.T) {
	text := "\x1b[4m" + "hi Supercalifragilisticexpialidocious" + "\x1b[m"
	got := wordWrapANSI(text, 10, "break-word")
	want := []string{
		"\x1b[4mhi\x1b[m",
		"\x1b[4mSupercalif\x1b[m",
		"\x1b[4mragilistic\x1b[m",
		"\x1b[4mexpialidoc\x1b[m",
		"\x1b[4mious\x1b[m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wordWrapANSI() = %#v, want %#v (check for doubled \\x1b[4m open codes)", got, want)
	}
}
