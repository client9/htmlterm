package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// TestInputSelectionRendersReverseVideoByDefault pins the ::selection UA
// default (reverse video, no author color/background-color) for a focused
// text <input> with a non-collapsed selection recorded via the reserved
// selectionStart/EndAttr marker attributes — see
// docs/proposals/CARET_SELECTION.md and formcontrol.go's selectionRange.
func TestInputSelectionRendersReverseVideoByDefault(t *testing.T) {
	src := `<input value="hello" ` + defaultFocusAttr + ` ` +
		defaultSelectionStartAttr + `="1" ` + defaultSelectionEndAttr + `="4">`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if got != "[hello]" {
		t.Fatalf("stripped output = %q, want %q", got, "[hello]")
	}
	// Only "ell" (runes [1,4)) is under the highlight — the "[h" prefix and
	// "o]" suffix render as plain, unstyled text either side of it. The
	// style resets via the general "\x1b[m" (not a per-attribute "\x1b[27m"),
	// same as every other inlineStyle.render call.
	if !strings.Contains(result.Output, "\x1b[7mell\x1b[m") {
		t.Errorf("expected exactly \"ell\" wrapped in reverse video, got %q", result.Output)
	}
}

// TestInputSelectionReclampsStaleAttrsAgainstCurrentValue pins that
// selectionRange reclamps the mirrored selectionStart/EndAttr values
// against the input's *current* value length, the same way
// Document.selection/Element.SelectionStart/SelectionEnd already do —
// rather than trusting attributes that can go stale after a direct
// SetAttribute("value", ...) bypassing SetValue's clearSelection (see
// docs/proposals/CARET_SELECTION.md). Regression test: this used to trust
// the raw attribute values verbatim, which stayed merely non-crashing
// (renderInput/block.go clamp defensively before slicing) but could
// highlight a stale, incorrect range.
func TestInputSelectionReclampsStaleAttrsAgainstCurrentValue(t *testing.T) {
	// selectionStart/End (1, 4) refer to a value that's since shrunk to
	// "hi" (2 runes) — reclamped, that's [1, 2), i.e. just "i".
	src := `<input value="hi" ` + defaultFocusAttr + ` ` +
		defaultSelectionStartAttr + `="1" ` + defaultSelectionEndAttr + `="4">`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if got != "[hi]" {
		t.Fatalf("stripped output = %q, want %q", got, "[hi]")
	}
	if !strings.Contains(result.Output, "\x1b[7mi\x1b[m") {
		t.Errorf("expected exactly \"i\" wrapped in reverse video, got %q", result.Output)
	}
	if strings.Contains(result.Output, "\x1b[7mhi\x1b[m") {
		t.Errorf("highlight should not include \"h\" (outside the reclamped range), got %q", result.Output)
	}
}

// TestInputSelectionCollapsedByReclampingRendersNoHighlight pins the case
// where reclamping a stale range against a shorter value collapses it
// entirely (start == end after clamping) — selectionRange must report
// has=false, not a zero-width or malformed range.
func TestInputSelectionCollapsedByReclampingRendersNoHighlight(t *testing.T) {
	// selectionStart/End (1, 4) refer to a value that's since shrunk to a
	// single rune "h" — reclamped, start and end both become 1, collapsed.
	src := `<input value="h" ` + defaultFocusAttr + ` ` +
		defaultSelectionStartAttr + `="1" ` + defaultSelectionEndAttr + `="4">`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if strings.Contains(result.Output, "\x1b[7m") {
		t.Errorf("a range collapsed by reclamping should render no highlight, got %q", result.Output)
	}
}

// TestInputSelectionAuthorColorOverridesReverseVideo pins that an author
// ::selection rule setting color/background-color wins outright, with no
// reverse-video fallback layered underneath it.
func TestInputSelectionAuthorColorOverridesReverseVideo(t *testing.T) {
	src := `<style>input::selection { background-color: #ff0000; color: #000000; }</style>` +
		`<input value="hello" ` + defaultFocusAttr + ` ` +
		defaultSelectionStartAttr + `="1" ` + defaultSelectionEndAttr + `="4">`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if strings.Contains(result.Output, "\x1b[7m") {
		t.Errorf("author ::selection color should suppress the reverse-video fallback, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "38;2;0;0;0") || !strings.Contains(result.Output, "48;2;255;0;0") {
		t.Errorf("expected author fg/bg SGR codes, got %q", result.Output)
	}
}

// TestInputSelectionRequiresFocus pins that a non-collapsed selection on an
// unfocused text entry renders with no highlight at all — real UAs don't
// paint a selection highlight on a field that isn't focused.
func TestInputSelectionRequiresFocus(t *testing.T) {
	src := `<input value="hello" ` + defaultSelectionStartAttr + `="1" ` + defaultSelectionEndAttr + `="4">`
	e, err := New(Options{Width: 20, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if strings.Contains(result.Output, "\x1b[7m") {
		t.Errorf("unfocused selection should not render a highlight, got %q", result.Output)
	}
}

// TestTextareaSelectionSpansAcrossLines pins that a <textarea> selection
// range spanning an embedded "\n" highlights both lines' overlapping
// portions correctly.
func TestTextareaSelectionSpansAcrossLines(t *testing.T) {
	// value = "foo\nbar" (runes 0-6): "o\nb" is runes [2,5).
	src := `<textarea style="border-style:none;padding:0" value="foo&#10;bar" ` +
		defaultFocusAttr + ` ` + defaultSelectionStartAttr + `="2" ` + defaultSelectionEndAttr + `="5"></textarea>`
	e, err := New(Options{Width: 10, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want at least 2:\n%q", len(lines), result.Output)
	}
	if got := strings.TrimRight(stripPopupANSI(lines[0]), " "); got != "foo" {
		t.Fatalf("line 0 stripped = %q, want %q", got, "foo")
	}
	if got := strings.TrimRight(stripPopupANSI(lines[1]), " "); got != "bar" {
		t.Fatalf("line 1 stripped = %q, want %q", got, "bar")
	}
	if !strings.Contains(lines[0], "\x1b[7mo\x1b[m") {
		t.Errorf("line 0 missing highlighted \"o\": %q", lines[0])
	}
	if !strings.Contains(lines[1], "\x1b[7mb\x1b[m") {
		t.Errorf("line 1 missing highlighted \"b\": %q", lines[1])
	}
}
