package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// TestWrapHyperlinkBoxMultiLineSelfContained is the regression test for the
// bug fixed alongside this test: wrapHyperlinkBox used to prepend the OSC 8
// open sequence only to the box's first line and append the close only to
// its last line. ../tui/cellbridge.go's writeANSILine decodes each screen
// row independently from a fresh state (the same way SGR style is
// re-derived per row rather than carried across rows — see
// TestWrapHyperlinkMarginNotUnderlined's inline-wrap equivalent), so every
// wrapped continuation line of a multi-line display:block/flex <a> (common
// for HTML-email "read more"/CTA buttons) ended up with no URL attached to
// any of its cells: still underlined, but not clickable — and if the first
// line scrolled out of view, no visible row of the link was clickable at
// all. Every line must now carry its own open+close pair.
func TestWrapHyperlinkBoxMultiLineSelfContained(t *testing.T) {
	r, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(`<a href="https://x.test/" style="display:block">alpha beta gamma delta epsilon zeta eta</a>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	osc, oscReset := "\x1b]8;;https://x.test/\x07", "\x1b]8;;\x07"
	want := strings.Join([]string{
		osc + "\x1b[4malpha beta gamma\x1b[m" + oscReset,
		osc + "\x1b[4mdelta epsilon zeta\x1b[m" + oscReset,
		osc + "\x1b[4meta\x1b[m" + oscReset,
	}, "\n") + "\n"
	if out != want {
		t.Fatalf("Render() =\n%q\nwant:\n%q", out, want)
	}

	// Belt-and-suspenders: every non-empty line must independently contain
	// both an open and a close, matching how writeANSILine will decode it —
	// a line with underline but no hyperlink markers is exactly the bug.
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, osc) {
			t.Errorf("line %d missing hyperlink open: %q", i, line)
		}
		if !strings.HasSuffix(line, oscReset) {
			t.Errorf("line %d missing hyperlink close at end: %q", i, line)
		}
	}
}

// TestWrapHyperlinkBoxSingleLineUnchanged guards the box's original
// (already-correct) single-line behavior: line 0 and "the last line" are
// the same line, so it must get exactly one open and one close, not two of
// each.
func TestWrapHyperlinkBoxSingleLineUnchanged(t *testing.T) {
	r, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(`<a href="https://x.test/" style="display:block">short link</a>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	osc, oscReset := "\x1b]8;;https://x.test/\x07", "\x1b]8;;\x07"
	want := osc + "\x1b[4mshort link\x1b[m" + oscReset + "\n"
	if out != want {
		t.Fatalf("Render() =\n%q\nwant:\n%q", out, want)
	}
	if n := strings.Count(out, "\x1b]8;;"); n != 2 {
		t.Fatalf("expected exactly one open + one close (2 OSC8 sequences), got %d in %q", n, out)
	}
}

// TestWrapHyperlinkBoxRespectsNoOSC8AndLowProfile confirms the box variant
// honors the same suppression gate as the inline wrapHyperlink: no OSC 8 at
// all when links are explicitly disabled, or when the color profile can't
// support them (colorprofile.Ascii and below — see wrapHyperlinkBox's guard
// clause).
func TestWrapHyperlinkBoxRespectsNoOSC8AndLowProfile(t *testing.T) {
	html := `<a href="https://x.test/" style="display:block">alpha beta gamma delta epsilon zeta eta</a>`

	t.Run("NoOSC8Links", func(t *testing.T) {
		r, err := New(Options{Width: 20, Profile: colorprofile.TrueColor, NoOSC8Links: true})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		out, err := r.Render(html)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(out, "]8;") {
			t.Fatalf("NoOSC8Links: unexpected OSC8 in output: %q", out)
		}
	})

	t.Run("AsciiProfile", func(t *testing.T) {
		r, err := New(Options{Width: 20, Profile: colorprofile.Ascii})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		out, err := r.Render(html)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(out, "]8;") {
			t.Fatalf("Ascii profile: unexpected OSC8 in output: %q", out)
		}
	})
}
