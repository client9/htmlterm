package htmlterm

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

// newTestScreen returns a tcell.Screen backed by a real VT-emulator mock
// terminal (vt.MockTerm) sized cols x rows, plus the mock itself for
// inspecting what was actually painted (mt.GetCell) — this exercises the
// full path from SetContent through tcell's own renderer and out through
// real SGR/OSC8 bytes that the mock emulator independently re-parses, not
// just "did we call SetContent with the expected struct."
func newTestScreen(t *testing.T, cols, rows int) (tcell.Screen, vt.MockTerm) {
	t.Helper()
	mt := vt.NewMockTerm(vt.MockOptColors(1 << 24)) // full truecolor, so RGB spans round-trip exactly
	mt.SetSize(vt.Coord{X: vt.Col(cols), Y: vt.Row(rows)})
	scr, err := tcell.NewTerminfoScreenFromTty(mt)
	if err != nil {
		t.Fatalf("NewTerminfoScreenFromTty: %v", err)
	}
	if err := scr.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(scr.Fini)
	return scr, mt
}

func TestWriteANSILinePlainText(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	nextLinkID := 0
	writeANSILine(scr, 0, "hello", 20, &nextLinkID)
	scr.Show()

	want := "hello"
	for i, r := range want {
		cell := mt.GetCell(vt.Coord{X: vt.Col(i), Y: 0})
		if cell.C != string(r) {
			t.Errorf("col %d: got %q, want %q", i, cell.C, string(r))
		}
		if cell.S.Attr() != vt.Plain {
			t.Errorf("col %d: got attr %v, want Plain", i, cell.S.Attr())
		}
	}
}

func TestWriteANSILineBoldColor(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	// SGR: bold + red foreground, "hi", reset.
	line := "\x1b[1;31mhi\x1b[0m"
	nextLinkID := 0
	writeANSILine(scr, 0, line, 20, &nextLinkID)
	scr.Show()

	c0 := mt.GetCell(vt.Coord{X: 0, Y: 0})
	if c0.C != "h" {
		t.Errorf("got %q, want %q", c0.C, "h")
	}
	if c0.S.Attr()&vt.Bold == 0 {
		t.Errorf("expected Bold attr set, got %v", c0.S.Attr())
	}
	wantFg := tcell.PaletteColor(1) // SGR 31 = red = palette index 1
	if c0.S.Fg() != wantFg {
		t.Errorf("got fg %v, want %v", c0.S.Fg(), wantFg)
	}
}

func TestWriteANSILineTruecolor(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	line := "\x1b[38;2;12;34;56mx\x1b[0m"
	nextLinkID := 0
	writeANSILine(scr, 0, line, 20, &nextLinkID)
	scr.Show()

	c0 := mt.GetCell(vt.Coord{X: 0, Y: 0})
	want := tcell.NewRGBColor(12, 34, 56)
	if c0.S.Fg() != want {
		t.Errorf("got fg %v, want %v", c0.S.Fg(), want)
	}
}

func TestWriteANSILine256Color(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	line := "\x1b[48;5;200my\x1b[0m"
	nextLinkID := 0
	writeANSILine(scr, 0, line, 20, &nextLinkID)
	scr.Show()

	c0 := mt.GetCell(vt.Coord{X: 0, Y: 0})
	want := tcell.PaletteColor(200)
	if c0.S.Bg() != want {
		t.Errorf("got bg %v, want %v", c0.S.Bg(), want)
	}
}

func TestWriteANSILineResetClearsAttrs(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	line := "\x1b[1;4;31mab\x1b[0mcd"
	nextLinkID := 0
	writeANSILine(scr, 0, line, 20, &nextLinkID)
	scr.Show()

	styled := mt.GetCell(vt.Coord{X: 0, Y: 0})
	if styled.S.Attr()&vt.Bold == 0 || styled.S.Attr()&vt.Underline == 0 {
		t.Errorf("expected bold+underline before reset, got %v", styled.S.Attr())
	}
	plain := mt.GetCell(vt.Coord{X: 2, Y: 0})
	if plain.C != "c" {
		t.Fatalf("got %q at col 2, want %q", plain.C, "c")
	}
	if plain.S.Attr() != vt.Plain {
		t.Errorf("expected Plain after reset, got %v", plain.S.Attr())
	}
}

func TestWriteANSILineHyperlink(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	line := "\x1b]8;;https://example.com\x07link\x1b]8;;\x07 plain"
	nextLinkID := 0
	writeANSILine(scr, 0, line, 20, &nextLinkID)
	scr.Show()

	linked := mt.GetCell(vt.Coord{X: 0, Y: 0})
	url, id := linked.S.Url()
	if url != "https://example.com" {
		t.Errorf("got url %q, want %q", url, "https://example.com")
	}
	if id == "" {
		t.Errorf("expected a non-empty synthetic UrlId")
	}

	after := mt.GetCell(vt.Coord{X: 5, Y: 0}) // "link" is 4 cols, so col 5 is one past the space
	url2, _ := after.S.Url()
	if url2 != "" {
		t.Errorf("expected no url after reset+space, got %q", url2)
	}
}

// TestPaintLinesRepeatedHyperlinkGetsFreshIDPerLine confirms paintLines
// mints a *different* synthetic UrlId every time the same href is reopened
// on a later line, rather than reusing one id per href. This is
// deliberate, not a missed optimization: tcell v3.4.0's renderer has a
// confirmed bug where an identical, multi-cell style (including url+id)
// repeated across a row boundary silently drops the second row's
// hyperlink entirely (verified directly against Screen.SetContent, no
// htmlterm code involved) — see paintLines' doc comment. Minting a fresh
// id per occurrence guarantees every line's hyperlink actually renders,
// at the cost of tcell's own cross-line-grouping feature, which isn't
// reliable enough here to depend on.
func TestPaintLinesRepeatedHyperlinkGetsFreshIDPerLine(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	lines := []string{
		"\x1b]8;;https://example.com\x07one\x1b]8;;\x07",
		"\x1b]8;;https://example.com\x07two\x1b]8;;\x07",
	}
	paintLines(scr, lines)
	scr.Show()

	url0, id0 := mt.GetCell(vt.Coord{X: 0, Y: 0}).S.Url()
	url1, id1 := mt.GetCell(vt.Coord{X: 0, Y: 1}).S.Url()
	if url0 != "https://example.com" || url1 != "https://example.com" {
		t.Fatalf("expected both lines to carry the hyperlink, got url0=%q url1=%q", url0, url1)
	}
	if id0 == "" || id1 == "" || id0 == id1 {
		t.Errorf("expected distinct non-empty ids across lines, got %q and %q", id0, id1)
	}
}

// TestPaintLinesDistinctHyperlinksGetDistinctIDs confirms two different
// hrefs never collide onto the same synthetic id.
func TestPaintLinesDistinctHyperlinksGetDistinctIDs(t *testing.T) {
	scr, mt := newTestScreen(t, 20, 3)
	lines := []string{
		"\x1b]8;;https://a.example\x07a\x1b]8;;\x07",
		"\x1b]8;;https://b.example\x07b\x1b]8;;\x07",
	}
	paintLines(scr, lines)
	scr.Show()

	_, id0 := mt.GetCell(vt.Coord{X: 0, Y: 0}).S.Url()
	_, id1 := mt.GetCell(vt.Coord{X: 0, Y: 1}).S.Url()
	if id0 == "" || id1 == "" || id0 == id1 {
		t.Errorf("expected distinct non-empty ids, got %q and %q", id0, id1)
	}
}

// TestPaintLinesMatchesRenderedOutput drives a handful of representative
// CSS/HTML combinations through the real rendering pipeline
// (New(...).Render), then through paintLines, and checks the resulting
// cells' plain text (ANSI stripped, same as htmlterm_test.go's renderCase
// table already does for the string-based output) matches — a systematic
// check that the bridge doesn't lose or reorder characters for real
// rendered frames, not just hand-picked ANSI fixtures.
func TestPaintLinesMatchesRenderedOutput(t *testing.T) {
	cases := []struct {
		name  string
		css   string
		html  string
		width int
	}{
		{"plain paragraph", "", `<p>hello world</p>`, 40},
		{"bold and color", "b { font-weight: bold; } .red { color: red; }", `<p><b>bold</b> <span class="red">red text</span></p>`, 40},
		{"background color", "span { background-color: blue; color: white; }", `<p><span>highlighted</span> plain</p>`, 40},
		{"hyperlink wraps across lines", "", `<p><a href="https://example.com">a rather long link label that should wrap across more than one output line</a></p>`, 20},
		{"table with borders", "table, td { border: 1px solid; }", `<table><tr><td>a</td><td>b</td></tr></table>`, 40},
		{"list", "", `<ul><li>one</li><li>two</li></ul>`, 40},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New(Options{CSS: tc.css, Width: tc.width})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			frame, err := r.Render(tc.html)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			lines := strings.Split(frame, "\n")

			scr, mt := newTestScreen(t, tc.width, len(lines)+1)
			paintLines(scr, lines)
			scr.Show()

			for row, line := range lines {
				want := stripANSI(line)
				var got strings.Builder
				for col := 0; col < tc.width && col < utf8.RuneCountInString(want); col++ {
					// A cell painted with a plain space (default style) is
					// optimized by tcell's renderer as an erase, which the
					// mock VT backend represents as an empty-content cell
					// rather than a literal " " — visually identical to a
					// real terminal, so normalize it back for comparison.
					c := mt.GetCell(vt.Coord{X: vt.Col(col), Y: vt.Row(row)}).C
					if c == "" {
						c = " "
					}
					got.WriteString(c)
				}
				gotTrimmed := strings.TrimRight(got.String(), " ")
				wantTrimmed := strings.TrimRight(want, " ")
				if gotTrimmed != wantTrimmed {
					t.Errorf("row %d: got %q, want %q", row, gotTrimmed, wantTrimmed)
				}
			}
		})
	}
}
