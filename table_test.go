package htmlterm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/client9/htmlterm"
)

func trimRightPerLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func TestTextOverflow(t *testing.T) {
	cell := func(attrs, content string) string {
		return `<table style="border-spacing:0"><tr><td ` + attrs + `>` + content + `</td></tr></table>`
	}
	runCases(t, []renderCase{
		{name: "clip is the default truncation when nowrap set", html: cell(`style="white-space:nowrap;width:5"`, "Hello World"), want: "Hello\n"},
		{name: "explicit text-overflow:ellipsis truncates with …", html: cell(`style="white-space:nowrap; text-overflow:ellipsis;width:5"`, "Hello World"), want: "Hell…\n"},
		{name: "clip truncates without marker", html: cell(`style="white-space:nowrap; text-overflow:clip;width:5"`, "Hello World"), want: "Hello\n"},
		{name: "custom string marker", html: cell(`style='white-space:nowrap; text-overflow:"+";width:5'`, "Hello World"), want: "Hell+\n"},
		{name: "no truncation when content fits", html: cell(`style="white-space:nowrap;width:11"`, "Hello World"), want: "Hello World\n"},
		{name: "ellipsis on exact fit needs no truncation", html: cell(`style="white-space:nowrap;width:5"`, "Hello"), want: "Hello\n"},
		{name: "clip width=1 takes first rune", html: cell(`style="white-space:nowrap; text-overflow:clip;width:1"`, "Hello"), want: "H\n"},
		{name: "text-overflow via CSS class", css: `.clip td { white-space: nowrap; text-overflow: clip; }`, html: `<table class="clip" style="border-spacing:0"><tr><td style="width:5">Hello World</td></tr></table>`, want: "Hello\n"},
		{name: "no white-space set wraps instead of truncating", html: cell(`style="width:5"`, "Hello World"), want: "Hello\nWorld\n"},
	})
}

func TestTableCellPadding(t *testing.T) {
	hidden := `style="border-spacing:0"`
	runCases(t, []renderCase{
		{name: "padding-left indents cell content", html: `<table ` + hidden + `><tr><td style="padding-left:1;width:6">ab</td></tr></table>`, want: " ab   \n"},
		{name: "padding-right adds space after cell content", html: `<table ` + hidden + `><tr><td style="padding-right:1;width:6">ab</td></tr></table>`, want: "ab    \n"},
		{name: "padding-left and padding-right both set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1;width:7">ab</td></tr></table>`, want: " ab    \n"},
		{name: "natural width includes padding when no explicit width set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1">ab</td></tr></table>`, want: " ab \n"},
		{name: "padding-left truncates content to reduced content width", html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:nowrap;width:5">Hello</td></tr></table>`, want: " Hell\n"},
		{name: "padding-top adds blank line above content", html: `<table ` + hidden + `><tr><td style="padding-top:1;width:5">ab</td></tr></table>`, want: "     \nab   \n"},
		{name: "padding-bottom adds blank line below content", html: `<table ` + hidden + `><tr><td style="padding-bottom:1;width:5">ab</td></tr></table>`, want: "ab   \n     \n"},
		{name: "padding-top 2 adds two blank lines above", html: `<table ` + hidden + `><tr><td style="padding-top:2;width:4">X</td></tr></table>`, want: "    \n    \nX   \n"},
		{name: "padding-top with padding-left", html: `<table ` + hidden + `><tr><td style="padding-top:1; padding-left:1;width:6">ab</td></tr></table>`, want: "      \n ab   \n"},
		{name: "all four sides of padding", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1; padding-top:1; padding-bottom:1;width:7">ab</td></tr></table>`, want: "       \n ab    \n       \n"},
		{name: "padding-top in one cell raises row height for sibling", html: `<table ` + hidden + `><tr><td style="padding-top:1;width:3">X</td><td style="width:3">Y</td></tr></table>`, want: "   Y  \nX     \n"},
		{name: "padding-left on th header", html: `<table ` + hidden + `><tr><th style="padding-left:1;width:7">Name</th></tr><tr><td style="width:7">val</td></tr></table>`, want: " Name  \nval    \n"},
		{name: "padding-top on th header adds blank row before header text", html: `<table ` + hidden + `><tr><th style="padding-top:1;width:4">Hi</th></tr><tr><td style="width:4">ok</td></tr></table>`, want: "    \nHi  \nok  \n"},
		{name: "padding-left with wrapping cell", html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:normal;width:7">Hello World</td></tr></table>`, want: " Hello \n World \n"},
		{name: "padding exceeds column width clamps to keep 1-char content", html: `<table ` + hidden + `><tr><td style="padding-left:2; padding-right:2;width:3">X</td></tr></table>`, want: "  X\n"},
	})
}

func TestTableMarginPadding(t *testing.T) {
	hidden := `style="border-spacing:0"`
	runCases(t, []renderCase{
		{name: "margin-left indents entire table", html: `<table style="margin-left:2; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "  hi\n"},
		{name: "margin-right adds trailing space", html: `<table style="margin-right:2; border-spacing:0"><tr><td style="width:4">hi</td></tr></table>`, want: "hi    \n"},
		{name: "margin-left and margin-right both set", html: `<table style="margin-left:2; margin-right:2; border-spacing:0"><tr><td style="width:2">hi</td></tr></table>`, width: 10, want: "  hi  \n"},
		{name: "margin-left percent resolves against available width", html: `<table style="margin-left:50%; border-spacing:0"><tr><td>hi</td></tr></table>`, width: 10, want: "     hi\n"},
		{name: "padding-left adds space inside margin", html: `<table style="padding-left:2; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "  hi\n"},
		{name: "padding and margin combine on the left", html: `<table style="margin-left:1; padding-left:2; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "   hi\n"},
		{name: "padding-top adds a blank line before the table", html: `<table style="padding-top:1; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "  \nhi\n"},
		{name: "padding-bottom adds a blank line after the table", html: `<table style="padding-bottom:1; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "hi\n  \n"},
		{name: "margin and padding shrink column sizing for width:100% table", html: `<table style="width:100%; margin-left:2; margin-right:2; padding-left:1; padding-right:1; border-spacing:0"><tr><td>x</td></tr></table>`, width: 10, want: "   x      \n"},
		{name: "no margin or padding leaves table unchanged", html: `<table ` + hidden + `><tr><td>hi</td></tr></table>`, want: "hi\n"},
		// margin: auto on a table centers it (or pushes it to one side),
		// matching the same behavior already supported for block elements.
		{name: "margin auto both centers table", html: `<table style="margin:0 auto; border-spacing:0"><tr><td style="width:2">hi</td></tr></table>`, width: 10, want: "    hi    \n"},
		{name: "margin-left auto pushes table right", html: `<table style="margin-left:auto; border-spacing:0"><tr><td style="width:2">hi</td></tr></table>`, width: 10, want: "        hi\n"},
		{name: "margin-right auto fills trailing space", html: `<table style="margin-right:auto; border-spacing:0"><tr><td style="width:2">hi</td></tr></table>`, width: 10, want: "hi        \n"},
	})
}

// TestTableVerticalMargin covers margin-top/margin-bottom on <table>, both
// at the root and nested inside a block - previously silently ignored in
// both dispatch paths (render.go's root-level "table" case and inline.go's
// nested-child "table" case each skipped straight to rendering the table's
// own box, unlike the "ol"/"ul"/"menu"/"block"/"flex" cases right next to
// them, which all consult margin-top/margin-bottom via the same
// ensureBreaks/pushBoxDirect convention tested here).
func TestTableVerticalMargin(t *testing.T) {
	runCases(t, []renderCase{
		{name: "root: larger margin wins the collapse", html: `<table style="margin-bottom:2; border-spacing:0"><tr><td>above</td></tr></table><table style="margin-top:1; border-spacing:0"><tr><td>below</td></tr></table>`, want: "above\n\n\nbelow\n"},
		{name: "root: equal margins collapse to one blank line", html: `<table style="margin-bottom:1; border-spacing:0"><tr><td>above</td></tr></table><table style="margin-top:1; border-spacing:0"><tr><td>below</td></tr></table>`, want: "above\n\nbelow\n"},
		{name: "root: leading margin-top has no effect on the first element", html: `<table style="margin-top:5; border-spacing:0"><tr><td>hi</td></tr></table>`, want: "hi\n"},
		{name: "nested: margin-top separates a table from preceding inline content", html: `<div>before<table style="margin-top:2; border-spacing:0"><tr><td>mid</td></tr></table>after</div>`, want: "before\n\n\nmid\nafter\n"},
		{name: "nested: margin-top/margin-bottom collapse with surrounding <p> margins", html: `<div><p>above</p><table style="margin-top:1; margin-bottom:1; border-spacing:0"><tr><td>mid</td></tr></table><p>below</p></div>`, want: "above\n\nmid\n\nbelow\n\n"},
	})
}

func TestTable(t *testing.T) {
	runCases(t, []renderCase{
		{name: "two-column hidden-border table", html: `<table style="border-spacing:0"><tr><td style="width:3">A</td><td style="width:5">Hello</td></tr></table>`, width: 40, want: "A  Hello\n"},
		{name: "display table uses table renderer", html: `<table style="display:table; border-spacing:0"><tr><td style="width:2">A</td><td style="width:2">B</td></tr></table>`, width: 40, want: "A B \n"},
		{name: "display block linearizes table descendants", css: `table, tr, td { display: block; } td { white-space: normal; width: auto; } td + td { margin-top: 1; } h2 { font-weight: normal; margin-bottom: 1; } h2::before { content: "## "; }`, html: `<table><tr><td><h2>Left Headline very long and keeps going</h2></td><td><h2>Right Headline very long and keeps going</h2></td></tr></table>`, width: 60, want: "## Left Headline very long and keeps going\n\n## Right Headline very long and keeps going\n"},
		{name: "comment before table border style none is ignored", css: `/* table, tr, td { display: block; } */ table { border-spacing: 0; }`, html: `<table><tr><td style="width:2">A</td><td style="width:2">B</td></tr></table>`, width: 40, want: "A B \n"},
		{name: "normal border style: single header and data row", css: collapseGridCSS, html: `<table><tr><th style="width:3">H1</th><th style="width:4">H2</th></tr><tr><td>A</td><td>Long</td></tr></table>`, width: 40, want: "┌───┬────┐\n│H1 │H2  │\n├───┼────┤\n│A  │Long│\n└───┴────┘\n"},
		{name: "table width:100% expands flexible column", css: `table { width: 100%; border-spacing: 0; }`, html: `<table><tr><td style="width:5">fixed</td><td>flex</td></tr></table>`, width: 20, want: "fixedflex           \n"},
		{name: "table border-left none strips just the vertical rule, keeping corners (unified with block border handling)", css: `table { border-style: solid; border-left: none; border-spacing:0; }`, html: `<table><tr><td style="width:3">A</td><td style="width:3">B</td></tr></table>`, width: 40, want: "┌─────┐\nA  B  │\n└─────┘\n"},
		{name: "html width attribute on td is ignored, CSS width wins", html: `<table style="border-spacing:0"><tr><td width="1" style="width:50%">abc</td><td>xy</td></tr></table>`, width: 20, want: "abc       xy\n"},
		{name: "table min-width and max-width influence flexible columns", html: `<table style="border-spacing:0; width:100%"><tr><td style="min-width:6">a</td><td style="max-width:4">bb</td></tr></table>`, width: 16, want: "a           bb  \n"},
		{name: "later row can define additional columns", html: `<table style="border-spacing:0"><tr><td>A</td></tr><tr><td>B</td><td>C</td></tr></table>`, want: "A \nBC\n"},
		{name: "display none table cell is skipped", css: `.gone { display: none; }`, html: `<table style="border-spacing:0"><tr><td>A</td><td class="gone">B</td><td>C</td></tr></table>`, want: "AC\n"},
		{name: "display none table row is skipped", css: `.gone { display: none; }`, html: `<table style="border-spacing:0"><tr><td>A</td></tr><tr class="gone"><td>B</td></tr><tr><td>C</td></tr></table>`, want: "A\nC\n"},
		{name: "thead row is header, tbody th row is not", css: collapseGridCSS, html: `<table><thead><tr><th style="width:3">H</th></tr></thead><tbody><tr><th style="width:3">R</th></tr></tbody></table>`, width: 40, want: "┌───┐\n│H  │\n├───┤\n│R  │\n└───┘\n"},
		{name: "no thead: first all-th row is implicit header", css: collapseGridCSS, html: `<table><tr><th style="width:3">H</th></tr><tr><td style="width:3">D</td></tr></table>`, width: 40, want: "┌───┐\n│H  │\n├───┤\n│D  │\n└───┘\n"},
		// tfoot all-<th> rows must NOT be promoted to implicit headers.
		{name: "tfoot all-th row is not promoted to implicit header", css: collapseGridCSS, html: `<table><tfoot><tr><th style="width:3">F</th></tr></tfoot><tbody><tr><td style="width:3">D</td></tr></tbody></table>`, width: 40, want: "┌───┐\n│F  │\n├───┤\n│D  │\n└───┘\n"},
	})
}

func TestNestedTablesInCells(t *testing.T) {
	render := func(css, htmlStr string) string {
		t.Helper()
		r, err := htmlterm.New(htmlterm.Options{CSS: css, Width: 80})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		got, err := r.Render(htmlStr)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return trimRightPerLine(stripANSI(got))
	}

	got := render(`table { border:solid; border-spacing:0; }`, `<table><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "┌───┐\n│┌─┐│\n││x││\n│└─┘│\n└───┘\n" {
		t.Fatalf("nested table did not render as table:\ngot  %q\nwant %q", got, "┌───┐\n│┌─┐│\n││x││\n│└─┘│\n└───┘\n")
	}

	got = render(`table { border-style: solid; }`, `<table style="border-spacing:0"><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "┌───┐\n│┌─┐│\n││x││\n│└─┘│\n└───┘\n" {
		t.Fatalf("nested table did not apply table CSS:\ngot  %q\nwant %q", got, "┌───┐\n│┌─┐│\n││x││\n│└─┘│\n└───┘\n")
	}

	got = render(`table { border-spacing: 0; }`, `<table><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "x\n" {
		t.Fatalf("nested borderless table was not compact:\ngot  %q\nwant %q", got, "x\n")
	}

	got = render(`table { border-spacing: 0; } table::before { content: "<TABLE>"; } table::after { content: "</TABLE>"; } tr::before { content: "<TR>"; } tr::after { content: "</TR>"; } td::before { content: "<TD>"; } td::after { content: "</TD>"; }`,
		`<table><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "<TD>\n<TD>x</TD>\n</TD>\n" {
		t.Fatalf("nested table structural pseudo-elements leaked:\ngot  %q\nwant %q", got, "<TD>\n<TD>x</TD>\n</TD>\n")
	}

	// A width:100% table nested in a width:100% outer cell must be sized to
	// the outer cell's content width, not the full renderer width, or its
	// border overflows the outer cell and gets ellipsis-truncated by the
	// outer td's default nowrap.
	r, err := htmlterm.New(htmlterm.Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got2, err := r.Render(`<table style="width:100%; border:solid; border-spacing:0"><tr><td><table style="width:100%; border:solid; border-spacing:0"><tr><td>Hi</td></tr></table></td></tr></table>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got2 = stripANSI(got2)
	want := "┌──────────────────┐\n│┌────────────────┐│\n││Hi              ││\n│└────────────────┘│\n└──────────────────┘\n"
	if got2 != want {
		t.Fatalf("nested width:100%% table overflowed its cell:\ngot  %q\nwant %q", got2, want)
	}

	// A nested table's own margin-right/padding-right must survive being
	// embedded in the outer table's cell text, which right-trims plain
	// trailing spaces (see plainInlineText in table_render.go). Deliberately
	// not trimming trailing spaces here — that's exactly what regressed.
	// Padding sits inside the border, margin outside it.
	r3, err := htmlterm.New(htmlterm.Options{Width: 32})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got3, err := r3.Render(`<table style="border-spacing:0"><tr><td><table style="border:solid; border-spacing:0; margin-left:2; margin-right:2; padding-left:2; padding-right:2" ><tr><td>hi</td></tr></table></td></tr></table>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got3 = stripANSI(got3)
	want3 := "  ┌──────┐  \n  │  hi  │  \n  └──────┘  \n"
	if got3 != want3 {
		t.Fatalf("nested table's own right margin/padding was lost:\ngot  %q\nwant %q", got3, want3)
	}

	// An outer table with two unconstrained flex columns - one empty, one
	// holding a nested table - has no CSS/HTML constraints to estimate column
	// widths from up front. The empty column should collapse and the nested
	// table should get (almost) the full outer width, not an even 50/50
	// split guess: its content should stay on one line instead of wrapping.
	got4 := render("", `<table><tr><td></td><td><table><tr><td>Hello World</td></tr></table></td></tr></table>`)
	if strings.Contains(got4, "Hello\nWorld") || strings.Contains(got4, "Hello \nWorld") {
		t.Fatalf("nested table wrapped despite the sibling column being empty:\ngot %q", got4)
	}
	if !strings.Contains(got4, "Hello World") {
		t.Fatalf("nested table content missing or split across lines:\ngot %q", got4)
	}

	// A fullWidth (width:100%) outer table with an empty flex column and a
	// content flex column whose text is short enough to NOT need to wrap:
	// the leftover space distributed to fill the table width must go to the
	// content column, not be split evenly with the empty one.
	r5, err := htmlterm.New(htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got5, err := r5.Render(`<table style="width:100%; border-spacing:0"><tr><td></td><td>Hi</td></tr></table>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got5 = stripANSI(got5)
	// A single leading space is expected (the hidden border style's own
	// column separator, since column 0 is 0-wide) - anything more means the
	// empty column absorbed part of the leftover fullWidth space.
	if strings.HasPrefix(got5, "  ") {
		t.Fatalf("empty flex column absorbed leftover fullWidth space instead of the content column:\ngot %q", got5)
	}
}

func TestTablePreservesInlineChildStyling(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<table style="border-spacing:0"><tr><td><b>B</b> C</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[1mB\x1b[m") {
		t.Fatalf("bold child styling not preserved in table cell: %q", got)
	}
	if stripANSI(got) != "B C\n" {
		t.Fatalf("plain table text got %q", stripANSI(got))
	}
}

func TestTableMultiLine(t *testing.T) {
	runCases(t, []renderCase{
		{name: "white-space:normal wraps cell content", html: `<table style="border-spacing:0"><tr><td style="white-space:normal;width:5">Hello World</td></tr></table>`, want: "Hello\nWorld\n"},
		{name: "white-space:nowrap still truncates", html: `<table style="border-spacing:0"><tr><td style="white-space:nowrap;width:5">Hello World</td></tr></table>`, want: "Hello\n"},
		{name: "multi-column row where one cell wraps", html: `<table style="border-spacing:0"><tr><td style="width:3">A</td><td style="white-space:normal;width:5">Hi there</td></tr></table>`, want: "A  Hi   \n   there\n"},
		{name: "long word is hard-broken", html: `<table style="border-spacing:0"><tr><td style="white-space:normal;width:4">Superlongword</td></tr></table>`, want: "Supe\nrlon\ngwor\nd   \n"},
		{name: "short content still fits on one line", html: `<table style="border-spacing:0"><tr><td style="white-space:normal;width:10">Hello</td></tr></table>`, want: "Hello     \n"},
		{name: "wrapping with bordered table", css: collapseGridCSS, html: `<table><tr><th style="width:5">Name</th></tr><tr><td style="white-space:normal;width:5">Al Bob</td></tr></table>`, want: "┌─────┐\n│Name │\n├─────┤\n│Al   │\n│Bob  │\n└─────┘\n"},
		{name: "table cell preserves br line breaks", html: `<table style="border-spacing:0"><tr><td style="width:4">a<br>b</td></tr></table>`, want: "a   \nb   \n"},
		{name: "table cell skips display none descendants", html: `<table style="border-spacing:0"><tr><td style="width:4"><span style="display:none">x</span>y</td></tr></table>`, want: "y   \n"},
		{name: "percentage block children in cells fit page width", css: `h2::before { content: "## "; } table { border-spacing: 0; } td { width: 100%; white-space: normal; } td::after { content: "\A"; }`, html: `<table><tr><td><h2>Left Headline Very Long and Takes Up Space</h2></td><td><h2>Right Headline is also very long and takes up space</h2></td></tr></table>`, width: 30, want: "## Left        ## Right       \nHeadline Very  Headline is    \nLong and Takes also very long \nUp Space       and takes up   \n               space          \n"},
	})
}

func TestTableVerticalAlign(t *testing.T) {
	hidden := `style="border-spacing:0"`
	runCases(t, []renderCase{
		{name: "vertical-align default is top", html: `<table ` + hidden + `><tr><td style="width:2">X</td><td style="white-space:normal;width:3">A B C D E</td></tr></table>`, want: "X A B\n  C D\n  E  \n"},
		{name: "vertical-align:top", html: `<table ` + hidden + `><tr><td style="vertical-align:top;width:2">X</td><td style="white-space:normal;width:3">A B C D E</td></tr></table>`, want: "X A B\n  C D\n  E  \n"},
		{name: "vertical-align:bottom", html: `<table ` + hidden + `><tr><td style="vertical-align:bottom;width:2">X</td><td style="white-space:normal;width:3">A B C D E</td></tr></table>`, want: "  A B\n  C D\nX E  \n"},
		{name: "vertical-align:middle", html: `<table ` + hidden + `><tr><td style="vertical-align:middle;width:2">X</td><td style="white-space:normal;width:3">A B C D E</td></tr></table>`, want: "  A B\nX C D\n  E  \n"},
		{name: "vertical-align:bottom two-line tall cell", html: `<table ` + hidden + `><tr><td style="vertical-align:bottom;width:2">X</td><td style="white-space:normal;width:3">A B C D</td></tr></table>`, want: "  A B\nX C D\n"},
	})
}

// TestTableBorderEdgeShorthand covers the table's own border (border-top/
// border-left/etc. shorthand and per-corner overrides) via the same
// resolveBoxBorders primitive any block element uses (see layout_test.go
// for the exhaustive shorthand-grammar coverage) - these cases just confirm
// <table> dispatches into that shared machinery correctly. The old
// border-*-mid/border-center junction-override properties and the
// border-header/border-columns/border-rows toggles they were tested with
// (TestTableBorderCSS) no longer exist: border-collapse:collapse resolves
// junction glyphs from real per-cell borders instead (see
// TestTableBorderStyles and table_collapse_test.go).
func TestTableBorderEdgeShorthand(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-top literal glyph on a table's own border", css: `table { border-top: "═"; border-spacing:0; }`, html: `<table><tr><td style="width:3">A</td><td style="width:3">B</td></tr></table>`, want: "══════\nA  B  \n"},
		{name: "border-top:none on a solid table removes just the top edge (regression parity with block's fix)", css: `table { border-style: solid; border-top: none; border-spacing:0; }`, html: `<table><tr><td style="width:3">A</td><td style="width:3">B</td></tr></table>`, want: "│A  B  │\n└──────┘\n"},
		{name: "table's own outer corners can be overridden independently", css: `table { border-top-left-corner: '1'; border-top-right-corner: '2'; border-bottom-left-corner: '3'; border-bottom-right-corner: '4'; border-style:solid; border-spacing:0; }`, html: `<table><tr><td style="width:3">A</td></tr></table>`, want: "1───2\n│A  │\n3───4\n"},
	})
}

func TestTableBorderStyles(t *testing.T) {
	oneColCSS := func(style string) string {
		return `table { border-collapse: collapse; border-style: ` + style + `; } td, th { border-style: ` + style + `; }`
	}
	html := `<table><tr><th style="width:3">H</th></tr><tr><td>A</td></tr></table>`
	runCases(t, []renderCase{
		{name: "heavy border style", css: oneColCSS("heavy"), html: html, want: "┏━━━┓\n┃H  ┃\n┣━━━┫\n┃A  ┃\n┗━━━┛\n"},
		{name: "double border style", css: oneColCSS("double"), html: html, want: "╔═══╗\n║H  ║\n╠═══╣\n║A  ║\n╚═══╝\n"},
		{name: "markdown border style", css: oneColCSS("markdown"), html: html, want: "|H  |\n|A  |\n"},
		{name: "standard border style", css: oneColCSS("standard"), html: html, want: "H  \nA  \n"},
	})
}

func TestTableCellTextAlign(t *testing.T) {
	hidden := `style="border-spacing:0"`
	runCases(t, []renderCase{
		{name: "text-align right in cell", html: `<table ` + hidden + `><tr><td style="text-align:right;width:6">hi</td></tr></table>`, want: "    hi\n"},
		{name: "text-align center in cell", html: `<table ` + hidden + `><tr><td style="text-align:center;width:6">hi</td></tr></table>`, want: "  hi  \n"},
		{name: "text-align left is explicit default", html: `<table ` + hidden + `><tr><td style="text-align:left;width:6">hi</td></tr></table>`, want: "hi    \n"},
	})
}

// TestDeeplyNestedTablesRenderQuickly guards against exponential blowup when
// measuring a nested table's natural width (see measureCellNaturalWidth):
// a table with an empty <td> beside a <td> holding an unconstrained
// two-column table (no CSS/HTML width anywhere) has no way to estimate
// column widths from constraints alone, so every level must measure its
// content - and if measuring a nested table meant fully rendering it (rather
// than just computing its natural width), that measure-and-fully-render
// duplication would compound at every nesting level, doubling total work
// per level. 18 levels deep would take minutes if that regressed; it should
// take well under a second.
func TestDeeplyNestedTablesRenderQuickly(t *testing.T) {
	const depth = 18
	html := `<table><tr><td>leaf</td><td>leaf2</td></tr></table>`
	for range depth {
		html = `<table><tr><td></td><td>` + html + `</td></tr></table>`
	}

	done := make(chan struct{})
	go func() {
		r, err := htmlterm.New(htmlterm.Options{Width: 80})
		if err != nil {
			t.Errorf("New: %v", err)
			close(done)
			return
		}
		if _, err := r.Render(html); err != nil {
			t.Errorf("Render: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("rendering deeply nested tables took too long - likely exponential blowup in nested-table width measurement")
	}
}

// TestMeasuringBlockContentRendersQuickly guards against a different
// blowup in the same nested-table-width-measurement path (see
// measureCellNaturalWidth): measuring a cell's natural width renders its
// content at an effectively unbounded width (naturalWidthCap, 1<<30) since
// that's free for plain text (it only suppresses width-driven wrapping). But
// a block-level child (e.g. a <div>) resolves its own default width
// directly from that budget - genuine CSS block behavior (width:auto fills
// the container) - and text-align (or any other alignment) then pads every
// line out to that width, materializing a roughly billion-character string.
// Real-world HTML almost always sets text-align somewhere inside a table
// cell, so this is a common trigger, not an edge case.
func TestMeasuringBlockContentRendersQuickly(t *testing.T) {
	// Two unconstrained flex columns (no CSS/HTML width anywhere) force the
	// ambiguous measurement path; the second column's block child has
	// text-align set, which is what triggers the width-padding blowup.
	html := `<table><tr><td></td><td><div style="text-align:center">hi</div></td></tr></table>`

	done := make(chan struct{})
	go func() {
		r, err := htmlterm.New(htmlterm.Options{Width: 80})
		if err != nil {
			t.Errorf("New: %v", err)
			close(done)
			return
		}
		if _, err := r.Render(html); err != nil {
			t.Errorf("Render: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("rendering a text-align block inside a measured table cell took too long - likely materializing a huge padded string")
	}
}

func TestColWidthAttrIgnored(t *testing.T) {
	// The legacy width HTML attribute on <col> is ignored (almost always a
	// pixel value in real-world markup); only CSS width has any effect.
	runCases(t, []renderCase{
		{
			name: "col width attribute has no effect, CSS width does",
			css:  `.narrow { color: #888888; }`,
			html: `<table style="border-spacing:0"><colgroup><col class="narrow" width="50"></colgroup><tr><th>Name</th></tr><tr><td>Alice</td></tr></table>`,
			want: "Name \nAlice\n",
		},
	})
}
