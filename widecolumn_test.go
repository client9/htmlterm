package htmlterm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
	"github.com/clipperhouse/displaywidth"
)

// This file is the column invariant suite: it renders the same set of
// awkward-width payloads through every construct that produces a box, and
// checks that what came out is the width the layout said it would be.
//
// Why it exists as its own suite rather than more cases in the per-topic
// files: the bugs it guards against all have the same shape — some helper
// measured a string in *runes* where the terminal charges *columns* — and
// they are invisible to an all-ASCII test corpus, where the two numbers are
// always equal. Every construct here was, at some point, wrong in exactly
// that way (a wide border glyph overflowing its box, a custom list marker
// misaligning wrapped lines, a <select> popup with a ragged highlight bar,
// visibility:hidden freeing fewer columns than it occupied). Adding a
// construct to this file is the cheapest way to keep the next one from
// shipping.
//
// Width is measured with displaywidth directly rather than through
// internal/render's own textVisualWidth, so a bug in the renderer's width
// measurement can't hide itself by also being the yardstick.

// widePayloads is the shared set of strings whose column count differs from
// their rune count (except the ASCII control, which pins that the assertions
// aren't accidentally trivial). Each is one grapheme cluster per entry so
// callers can reason about "one character" without cluster arithmetic.
var widePayloads = []struct {
	name  string
	text  string
	width int // terminal columns
}{
	{"ascii", "abc", 3},    // control: runes == columns
	{"cjk", "日本語", 6},      // 3 runes, 2 columns each
	{"emoji", "🔥", 2},      // 1 rune, 2 columns
	{"vs16", "❤️", 2},      // 2 runes (base + VS16), one cluster
	{"combining", "é", 1}, // 2 runes (e + combining acute), 1 column
}

// A ZWJ sequence (👨‍👩‍👧) is deliberately absent from the list above: the
// renderer strips U+200D as an invisible format character before it ever
// reaches layout (sanitizeTerminalText, internal/render/csstext.go), so the cluster
// decomposes into its component emoji and there is no single-cluster
// invariant left to assert on the render path. That behavior is pinned by
// TestZWJSequenceIsDecomposedBySanitizer below, and cluster-level width
// handling itself is covered where it actually matters — NextGrapheme and
// the cellbridge painter (internal/render/grapheme_internal_test.go,
// tui/cellbridge_test.go).

// quoteAttr wraps s in double quotes for embedding in an HTML attribute or a
// CSS string literal. Deliberately not fmt's %q: that escapes non-printable
// runes, so a payload containing U+200D would reach the renderer as the six
// literal characters backslash-u-2-0-0-d — testing Go's quoting rather than
// htmlterm's layout. None of the payloads contain a quote of their own.
func quoteAttr(s string) string {
	return `"` + s + `"`
}

// visualWidth is the suite's own yardstick — see the file comment on why it
// doesn't call into internal/render.
func visualWidth(s string) int {
	return displaywidth.String(stripANSI(s))
}

// TestWidePayloadWidths pins the yardstick itself: if displaywidth disagrees
// with the widths declared above (a Unicode data update, an East Asian width
// env var leaking into the test process), every other assertion in this file
// would be measuring against the wrong number, and it's far clearer to fail
// here than to fail six construct tests with confusing diffs.
func TestWidePayloadWidths(t *testing.T) {
	for _, p := range widePayloads {
		if got := visualWidth(p.text); got != p.width {
			t.Errorf("%s: visualWidth(%q) = %d, want %d", p.name, p.text, got, p.width)
		}
	}
}

// renderWide renders src at width cols and returns its lines, ANSI stripped.
func renderWide(t *testing.T, css, src string, cols int) []string {
	t.Helper()
	r, err := htmlterm.New(htmlterm.Options{CSS: css, Width: cols})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return splitRendered(out)
}

// splitRendered splits a rendered frame into lines, dropping the trailing
// empty line every block-level render ends with.
func splitRendered(out string) []string {
	lines := strings.Split(strings.TrimRight(stripANSI(out), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// assertAllLinesWidth fails if any line isn't exactly want columns wide,
// reporting the offending line's own rune count alongside — the two numbers
// diverging is the signature of the bug class this file exists for.
func assertAllLinesWidth(t *testing.T, lines []string, want int) {
	t.Helper()
	for i, l := range lines {
		if got := visualWidth(l); got != want {
			t.Errorf("line %d = %d columns, want %d (runes=%d): %q",
				i, got, want, len([]rune(l)), l)
		}
	}
}

// TestColumnInvariantBorderedBlock renders a wide payload inside a
// fixed-width bordered block: every line, border rules included, must come
// out exactly as wide as the box.
func TestColumnInvariantBorderedBlock(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			src := fmt.Sprintf(`<div style="border-style:solid;width:20">%s</div>`, p.text)
			assertAllLinesWidth(t, renderWide(t, "", src, 40), 20)
		})
	}
}

// TestColumnInvariantBorderGlyph pins that a *border glyph itself* can be a
// wide character. Literal-glyph border values are a documented
// terminal-native feature (see CSS.md), so this is reachable from ordinary
// author CSS — and the border's own width has to be charged in columns like
// any other content, or the box's right edge lands past its declared width.
func TestColumnInvariantBorderGlyph(t *testing.T) {
	for _, p := range widePayloads {
		if strings.ContainsAny(p.text, "abc") {
			continue // the ASCII control is a 3-glyph string, not a border glyph
		}
		t.Run(p.name, func(t *testing.T) {
			css := fmt.Sprintf(`div { border-style: solid; border-left: %s }`, quoteAttr(p.text))
			assertAllLinesWidth(t, renderWide(t, css, `<div>hi</div>`, 40), 40)
		})
	}
}

// TestColumnInvariantTable pins that a table's rows all agree on width when
// a cell holds a wide payload — a column estimated in runes makes later
// columns start at different screen positions on different rows.
func TestColumnInvariantTable(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			src := fmt.Sprintf(
				`<table><tr><td>%s</td><td>b</td></tr><tr><td>x</td><td>y</td></tr></table>`, p.text)
			lines := renderWide(t, "", src, 40)
			if len(lines) < 2 {
				t.Fatalf("expected at least 2 rendered rows, got %d: %q", len(lines), lines)
			}
			assertAllLinesWidth(t, lines, visualWidth(lines[0]))
		})
	}
}

// TestColumnInvariantTextInput pins that a text input fills exactly its
// resolved width (the size attribute) whatever its value is made of — the
// padding that fills a short field's tail has to be counted in columns.
func TestColumnInvariantTextInput(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			src := fmt.Sprintf(`<input value=%s size="12">`, quoteAttr(p.text))
			assertAllLinesWidth(t, renderWide(t, "", src, 40), 12)
		})
	}
}

// endMarker is the sentinel the visibility test looks for after the hidden
// payload; markerColumn reports which terminal column it starts at.
const endMarker = "|end"

func markerColumn(line string) int {
	before, _, ok := strings.Cut(line, endMarker)
	if !ok {
		return -1
	}
	return visualWidth(before)
}

// TestColumnInvariantVisibilityHidden pins that visibility:hidden preserves
// layout, which is its entire reason to exist over display:none: hiding a
// payload must free exactly the columns it occupied, so the text after it
// stays put.
func TestColumnInvariantVisibilityHidden(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			visible := fmt.Sprintf(`<div><span>%s</span>%s</div>`, p.text, endMarker)
			hidden := fmt.Sprintf(`<div><span style="visibility:hidden">%s</span>%s</div>`,
				p.text, endMarker)
			vl := renderWide(t, "", visible, 40)
			hl := renderWide(t, "", hidden, 40)
			if len(vl) != len(hl) {
				t.Fatalf("visible rendered %d lines, hidden rendered %d", len(vl), len(hl))
			}
			for i := range vl {
				if got, want := visualWidth(hl[i]), visualWidth(vl[i]); got != want {
					t.Errorf("line %d: hidden is %d columns, visible is %d — layout shifted: %q vs %q",
						i, got, want, hl[i], vl[i])
				}
				// The marker text must also start at the same column, not
				// merely leave the line the same total width. Compared in
				// columns, not byte offsets: "日本語" is 9 bytes and 6
				// columns, so byte positions legitimately differ between a
				// line holding the payload and one holding spaces.
				if got, want := markerColumn(hl[i]), markerColumn(vl[i]); got != want {
					t.Errorf("line %d: %q begins at column %d hidden, %d visible: %q vs %q",
						i, endMarker, got, want, hl[i], vl[i])
				}
			}
		})
	}
}

// TestColumnInvariantListMarkerWrapIndent pins that a custom list marker's
// width is charged in columns: the hanging indent on a wrapped list item has
// to line up under the first line's content, and a marker measured in runes
// indents the continuation too little.
func TestColumnInvariantListMarkerWrapIndent(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			css := fmt.Sprintf(`ul { list-style-type: %s }`, quoteAttr(p.text+" "))
			lines := renderWide(t, css, `<ul><li>aaa bbb ccc ddd eee</li></ul>`, 18)
			if len(lines) < 2 {
				t.Fatalf("expected the item to wrap onto a second line, got %q", lines)
			}
			// Content on line 0 starts right after the marker; the wrapped
			// line's indent must put its content at that same column.
			wantCol := visualWidth(lines[0][:strings.Index(lines[0], "aaa")])
			gotCol := visualWidth(lines[1]) - visualWidth(strings.TrimLeft(lines[1], " "))
			if gotCol != wantCol {
				t.Errorf("wrapped line indent = %d columns, want %d (marker %q):\n%q\n%q",
					gotCol, wantCol, p.text, lines[0], lines[1])
			}
		})
	}
}

// TestColumnInvariantSelectPopup pins that every row of an open <select>
// popup is the same width — the popup sizes itself to its widest option
// label, and a label measured in runes makes the highlight bar ragged
// (visibly so, since the highlight is reverse video across the full row).
func TestColumnInvariantSelectPopup(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			src := fmt.Sprintf(
				`<select id="s"><option>%s</option><option>bb</option></select>`, p.text)
			doc, err := document.ParseDocument(src, htmlterm.Options{Width: 40})
			if err != nil {
				t.Fatalf("ParseDocument: %v", err)
			}
			if _, err := doc.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}
			el := doc.GetElementByID("s")
			el.Focus()
			doc.DispatchKey("Enter", document.Modifiers{}) // open the popup
			out, err := doc.Render()
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			lines := splitRendered(out)
			if len(lines) < 3 {
				t.Fatalf("expected a closed-state line plus 2 popup rows, got %q", lines)
			}
			// lines[0] is the closed <select> itself; the popup rows follow.
			// They're compared against each other rather than a fixed number:
			// the popup's width is derived from its widest label, so what
			// matters is that every row agrees on it.
			want := visualWidth(lines[1])
			for i, l := range lines[2:] {
				if got := visualWidth(l); got != want {
					t.Errorf("popup row %d = %d columns, row 0 = %d — ragged highlight bar:\n%q\n%q",
						i+1, got, want, lines[1], l)
				}
			}
		})
	}
}

func TestColumnInvariantDatalistPopup(t *testing.T) {
	for _, p := range widePayloads {
		t.Run(p.name, func(t *testing.T) {
			src := fmt.Sprintf(
				`<input id="i" list="l"><datalist id="l"><option value="%s"><option value="%sbb"></datalist>`,
				p.text, p.text)
			doc, err := document.ParseDocument(src, htmlterm.Options{Width: 40})
			if err != nil {
				t.Fatalf("ParseDocument: %v", err)
			}
			if _, err := doc.Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}
			el := doc.GetElementByID("i")
			el.Focus()
			doc.DispatchKey("ArrowDown", document.Modifiers{}) // open the popup
			out, err := doc.Render()
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			lines := splitRendered(out)
			if len(lines) < 3 {
				t.Fatalf("expected the field's line plus 2 popup rows, got %q", lines)
			}
			// lines[0] is the input field itself; the suggestion rows follow.
			// Compared against each other rather than a fixed number, same as
			// the <select> popup above: the width comes from the widest label,
			// so what matters is that every row agrees on it.
			want := visualWidth(lines[1])
			for i, l := range lines[2:] {
				if got := visualWidth(l); got != want {
					t.Errorf("popup row %d = %d columns, row 0 = %d — ragged highlight bar:\n%q\n%q",
						i+1, got, want, lines[1], l)
				}
			}
		})
	}
}

// TestZWJSequenceIsDecomposedBySanitizer documents why widePayloads has no
// ZWJ entry: U+200D is an invisible format character, and the renderer drops
// every one of those before layout (sanitizeTerminalText — see SECURITY.md's
// reasoning about invisible characters in untrusted HTML). So a family emoji
// arrives on screen as its three component emoji, six columns rather than
// two. This is intended behavior, not a width bug, and pinning it here keeps
// someone from "fixing" the width math to chase it.
func TestZWJSequenceIsDecomposedBySanitizer(t *testing.T) {
	const family = "👨‍👩‍👧" // 5 runes: 3 emoji joined by 2 ZWJs
	lines := renderWide(t, "", "<div>"+family+"</div>", 40)
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %q", lines)
	}
	if strings.ContainsRune(lines[0], '‍') {
		t.Errorf("ZWJ survived into rendered output: %q", lines[0])
	}
	if got, want := visualWidth(lines[0]), 6; got != want {
		t.Errorf("decomposed family emoji = %d columns, want %d: %q", got, want, lines[0])
	}
}
