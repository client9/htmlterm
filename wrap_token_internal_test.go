package htmlterm

import (
	"reflect"
	"testing"

	"golang.org/x/net/html"
)

// TestWordWrapTokensBoxMidParagraph verifies a multi-line box token forces a
// line break before and after itself, embeds its own lines verbatim (no
// reflow), and its Rect reflects where it landed in the caller's own output.
func TestWordWrapTokensBoxMidParagraph(t *testing.T) {
	node := &html.Node{Data: "div"}
	bx := box{lines: []string{"X", "Y"}, width: 1}
	tokens := []wrapToken{
		{text: "before"},
		{box: &bx, node: node},
		{text: "after"},
	}
	got, positions := wordWrapTokens(tokens, 20, "", 0)
	want := []string{"before", "X", "Y", "after"}
	if !reflect.DeepEqual(got.lines, want) {
		t.Fatalf("lines = %#v, want %#v", got.lines, want)
	}
	wantRect := Rect{Row: 1, Col: 0, Width: 1, Height: 2}
	if positions[node] != wantRect {
		t.Fatalf("positions[node] = %#v, want %#v", positions[node], wantRect)
	}
}

// TestWordWrapTokensInlineBlockSiblingsShareLine verifies two single-line box
// tokens with no text between them (mirroring two adjacent inline-block
// elements with no source whitespace) sit glued on the same output line, and
// each gets its own correctly-offset Rect.
func TestWordWrapTokensInlineBlockSiblingsShareLine(t *testing.T) {
	nodeA := &html.Node{Data: "span"}
	nodeB := &html.Node{Data: "span"}
	boxA := box{lines: []string{"AAAAA"}, width: 5}
	boxB := box{lines: []string{"BBBBB"}, width: 5}
	tokens := []wrapToken{
		{box: &boxA, node: nodeA},
		{box: &boxB, node: nodeB},
	}
	got, positions := wordWrapTokens(tokens, 20, "", 0)
	want := []string{"AAAAABBBBB"}
	if !reflect.DeepEqual(got.lines, want) {
		t.Fatalf("lines = %#v, want %#v", got.lines, want)
	}
	if want := (Rect{Row: 0, Col: 0, Width: 5, Height: 1}); positions[nodeA] != want {
		t.Fatalf("positions[nodeA] = %#v, want %#v", positions[nodeA], want)
	}
	if want := (Rect{Row: 0, Col: 5, Width: 5, Height: 1}); positions[nodeB] != want {
		t.Fatalf("positions[nodeB] = %#v, want %#v", positions[nodeB], want)
	}
}

// TestWordWrapTokensOversizedBoxOverflows verifies a box token wider than the
// available width is embedded as-is, not clipped — it has already made its
// own overflow decision (renderBlockContentBox only clips when
// overflow:hidden and an explicit width are both set); re-clipping every
// wider box unconditionally at this level would silently truncate ordinary
// unbreakable content (e.g. overflow-wrap:normal with a long word) that's
// supposed to overflow its container instead.
func TestWordWrapTokensOversizedBoxOverflows(t *testing.T) {
	bx := box{lines: []string{"0123456789"}, width: 10}
	tokens := []wrapToken{{box: &bx}}
	got, _ := wordWrapTokens(tokens, 5, "", 0)
	want := []string{"0123456789"}
	if !reflect.DeepEqual(got.lines, want) {
		t.Fatalf("lines = %#v, want %#v", got.lines, want)
	}
	if got.width != 10 {
		t.Fatalf("width = %d, want 10", got.width)
	}
}

// TestWordWrapTokensTextForcesBreakAroundTallBox verifies text that would
// otherwise fit alongside a box on the same line is still pushed to its own
// line when the box is multi-line tall (no flowing text around a tall
// embedded object, per RENDERING.md's stated scope).
func TestWordWrapTokensTextForcesBreakAroundTallBox(t *testing.T) {
	bx := box{lines: []string{"X", "Y", "Z"}, width: 1}
	tokens := []wrapToken{
		{text: "left"},
		{box: &bx},
		{text: "right"},
	}
	got, _ := wordWrapTokens(tokens, 20, "", 0)
	want := []string{"left", "X", "Y", "Z", "right"}
	if !reflect.DeepEqual(got.lines, want) {
		t.Fatalf("lines = %#v, want %#v", got.lines, want)
	}
}

// TestWordWrapTokensBrkForcesBreak verifies a brk token (e.g. <br>) always
// ends the current line regardless of available width.
func TestWordWrapTokensBrkForcesBreak(t *testing.T) {
	tokens := []wrapToken{
		{text: "a"},
		{brk: true},
		{text: "b"},
	}
	got, _ := wordWrapTokens(tokens, 20, "", 0)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got.lines, want) {
		t.Fatalf("lines = %#v, want %#v", got.lines, want)
	}
}
