package htmlterm_test

import (
	"strings"
	"testing"

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
		return `<table style="border-style:hidden"><tr><td ` + attrs + `>` + content + `</td></tr></table>`
	}
	runCases(t, []renderCase{
		{name: "ellipsis truncates with … when nowrap set", html: cell(`style="white-space:nowrap" width="5"`, "Hello World"), want: "Hell…\n"},
		{name: "clip truncates without marker", html: cell(`style="white-space:nowrap; text-overflow:clip" width="5"`, "Hello World"), want: "Hello\n"},
		{name: "custom string marker", html: cell(`style='white-space:nowrap; text-overflow:"+"' width="5"`, "Hello World"), want: "Hell+\n"},
		{name: "no truncation when content fits", html: cell(`style="white-space:nowrap" width="11"`, "Hello World"), want: "Hello World\n"},
		{name: "ellipsis on exact fit needs no truncation", html: cell(`style="white-space:nowrap" width="5"`, "Hello"), want: "Hello\n"},
		{name: "clip width=1 takes first rune", html: cell(`style="white-space:nowrap; text-overflow:clip" width="1"`, "Hello"), want: "H\n"},
		{name: "text-overflow via CSS class", css: `.clip td { white-space: nowrap; text-overflow: clip; }`, html: `<table class="clip" style="border-style:hidden"><tr><td width="5">Hello World</td></tr></table>`, want: "Hello\n"},
		{name: "no white-space set wraps instead of truncating", html: cell(`width="5"`, "Hello World"), want: "Hello\nWorld\n"},
	})
}

func TestTableCellPadding(t *testing.T) {
	hidden := `style="border-style:hidden"`
	runCases(t, []renderCase{
		{name: "padding-left indents cell content", html: `<table ` + hidden + `><tr><td style="padding-left:1" width="6">ab</td></tr></table>`, want: " ab   \n"},
		{name: "padding-right adds space after cell content", html: `<table ` + hidden + `><tr><td style="padding-right:1" width="6">ab</td></tr></table>`, want: "ab    \n"},
		{name: "padding-left and padding-right both set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1" width="7">ab</td></tr></table>`, want: " ab    \n"},
		{name: "natural width includes padding when no explicit width set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1">ab</td></tr></table>`, want: " ab \n"},
		{name: "padding-left truncates content to reduced content width", html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:nowrap" width="5">Hello</td></tr></table>`, want: " Hel…\n"},
		{name: "padding-top adds blank line above content", html: `<table ` + hidden + `><tr><td style="padding-top:1" width="5">ab</td></tr></table>`, want: "     \nab   \n"},
		{name: "padding-bottom adds blank line below content", html: `<table ` + hidden + `><tr><td style="padding-bottom:1" width="5">ab</td></tr></table>`, want: "ab   \n     \n"},
		{name: "padding-top 2 adds two blank lines above", html: `<table ` + hidden + `><tr><td style="padding-top:2" width="4">X</td></tr></table>`, want: "    \n    \nX   \n"},
		{name: "padding-top with padding-left", html: `<table ` + hidden + `><tr><td style="padding-top:1; padding-left:1" width="6">ab</td></tr></table>`, want: "      \n ab   \n"},
		{name: "all four sides of padding", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1; padding-top:1; padding-bottom:1" width="7">ab</td></tr></table>`, want: "       \n ab    \n       \n"},
		{name: "padding-top in one cell raises row height for sibling", html: `<table ` + hidden + `><tr><td style="padding-top:1" width="3">X</td><td width="3">Y</td></tr></table>`, want: "    Y  \nX      \n"},
		{name: "padding-left on th header", html: `<table ` + hidden + `><tr><th style="padding-left:1" width="7">Name</th></tr><tr><td width="7">val</td></tr></table>`, want: " Name  \nval    \n"},
		{name: "padding-top on th header adds blank row before header text", html: `<table ` + hidden + `><tr><th style="padding-top:1" width="4">Hi</th></tr><tr><td width="4">ok</td></tr></table>`, want: "    \nHi  \nok  \n"},
		{name: "padding-left with wrapping cell", html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:normal" width="7">Hello World</td></tr></table>`, want: " Hello \n World \n"},
		{name: "padding exceeds column width clamps to keep 1-char content", html: `<table ` + hidden + `><tr><td style="padding-left:2; padding-right:2" width="3">X</td></tr></table>`, want: "  X\n"},
	})
}

func TestTableMarginPadding(t *testing.T) {
	hidden := `style="border-style:hidden"`
	runCases(t, []renderCase{
		{name: "margin-left indents entire table", html: `<table style="margin-left:2; border-style:hidden"><tr><td>hi</td></tr></table>`, want: "  hi\n"},
		{name: "margin-right adds trailing space", html: `<table style="margin-right:2; border-style:hidden"><tr><td width="4">hi</td></tr></table>`, want: "hi    \n"},
		{name: "margin-left and margin-right both set", html: `<table style="margin-left:2; margin-right:2; border-style:hidden"><tr><td width="2">hi</td></tr></table>`, width: 10, want: "  hi  \n"},
		{name: "margin-left percent resolves against available width", html: `<table style="margin-left:50%; border-style:hidden"><tr><td>hi</td></tr></table>`, width: 10, want: "     hi\n"},
		{name: "padding-left adds space inside margin", html: `<table style="padding-left:2; border-style:hidden"><tr><td>hi</td></tr></table>`, want: "  hi\n"},
		{name: "padding and margin combine on the left", html: `<table style="margin-left:1; padding-left:2; border-style:hidden"><tr><td>hi</td></tr></table>`, want: "   hi\n"},
		{name: "padding-top adds a blank line before the table", html: `<table style="padding-top:1; border-style:hidden"><tr><td>hi</td></tr></table>`, want: "  \nhi\n"},
		{name: "padding-bottom adds a blank line after the table", html: `<table style="padding-bottom:1; border-style:hidden"><tr><td>hi</td></tr></table>`, want: "hi\n  \n"},
		{name: "margin and padding shrink column sizing for width:100% table", html: `<table style="width:100%; margin-left:2; margin-right:2; padding-left:1; padding-right:1; border-style:hidden"><tr><td>x</td></tr></table>`, width: 10, want: "   x      \n"},
		{name: "no margin or padding leaves table unchanged", html: `<table ` + hidden + `><tr><td>hi</td></tr></table>`, want: "hi\n"},
		// margin: auto on a table centers it (or pushes it to one side),
		// matching the same behavior already supported for block elements.
		{name: "margin auto both centers table", html: `<table style="margin:0 auto; border-style:hidden"><tr><td width="2">hi</td></tr></table>`, width: 10, want: "    hi    \n"},
		{name: "margin-left auto pushes table right", html: `<table style="margin-left:auto; border-style:hidden"><tr><td width="2">hi</td></tr></table>`, width: 10, want: "        hi\n"},
		{name: "margin-right auto fills trailing space", html: `<table style="margin-right:auto; border-style:hidden"><tr><td width="2">hi</td></tr></table>`, width: 10, want: "hi        \n"},
	})
}

func TestTable(t *testing.T) {
	runCases(t, []renderCase{
		{name: "two-column hidden-border table", html: `<table style="border-style:hidden"><tr><td width="3">A</td><td width="5">Hello</td></tr></table>`, width: 40, want: "A   Hello\n"},
		{name: "display table uses table renderer", html: `<table style="display:table; border-style:hidden"><tr><td width="2">A</td><td width="2">B</td></tr></table>`, width: 40, want: "A  B \n"},
		{name: "display block linearizes table descendants", css: `table, tr, td { display: block; } td { white-space: normal; width: auto; } td + td { margin-top: 1; } h2 { font-weight: normal; margin-bottom: 1; } h2::before { content: "## "; }`, html: `<table><tr><td><h2>Left Headline very long and keeps going</h2></td><td><h2>Right Headline very long and keeps going</h2></td></tr></table>`, width: 60, want: "## Left Headline very long and keeps going\n\n## Right Headline very long and keeps going\n"},
		{name: "comment before table border style none is ignored", css: `/* table, tr, td { display: block; } */ table { border-style: none; }`, html: `<table><tr><td width="2">A</td><td width="2">B</td></tr></table>`, width: 40, want: "A  B \n"},
		{name: "normal border style: single header and data row", html: `<table><tr><th width="3">H1</th><th width="4">H2</th></tr><tr><td>A</td><td>Long</td></tr></table>`, width: 40, want: "┌───┬────┐\n│H1 │H2  │\n├───┼────┤\n│A  │Long│\n└───┴────┘\n"},
		{name: "table width:100% expands flexible column", css: `table { width: 100%; border-style: hidden; }`, html: `<table><tr><td width="5">fixed</td><td>flex</td></tr></table>`, width: 20, want: "fixed flex          \n"},
		{name: "table border-left none overrides border-style deterministically", css: `table { border-style: solid; border-left: none; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, width: 40, want: "───┬───┐\nA  │B  │\n───┴───┘\n"},
		{name: "table width percent overrides html width attribute", html: `<table style="border-style:hidden"><tr><td width="8" style="width:50%">abc</td><td>xy</td></tr></table>`, width: 20, want: "abc       xy\n"},
		{name: "table min-width and max-width influence flexible columns", html: `<table style="border-style:hidden; width:100%"><tr><td style="min-width:6">a</td><td style="max-width:4">bb</td></tr></table>`, width: 16, want: "a           bb  \n"},
		{name: "later row can define additional columns", html: `<table style="border-style:hidden"><tr><td>A</td></tr><tr><td>B</td><td>C</td></tr></table>`, want: "A  \nB C\n"},
		{name: "display none table cell is skipped", css: `.gone { display: none; }`, html: `<table style="border-style:hidden"><tr><td>A</td><td class="gone">B</td><td>C</td></tr></table>`, want: "A C\n"},
		{name: "display none table row is skipped", css: `.gone { display: none; }`, html: `<table style="border-style:hidden"><tr><td>A</td></tr><tr class="gone"><td>B</td></tr><tr><td>C</td></tr></table>`, want: "A\nC\n"},
		{name: "thead row is header, tbody th row is not", html: `<table><thead><tr><th width="3">H</th></tr></thead><tbody><tr><th width="3">R</th></tr></tbody></table>`, width: 40, want: "┌───┐\n│H  │\n├───┤\n│R  │\n└───┘\n"},
		{name: "no thead: first all-th row is implicit header", html: `<table><tr><th width="3">H</th></tr><tr><td width="3">D</td></tr></table>`, width: 40, want: "┌───┐\n│H  │\n├───┤\n│D  │\n└───┘\n"},
		// tfoot all-<th> rows must NOT be promoted to implicit headers.
		{name: "tfoot all-th row is not promoted to implicit header", html: `<table><tfoot><tr><th width="3">F</th></tr></tfoot><tbody><tr><td width="3">D</td></tr></tbody></table>`, width: 40, want: "┌───┐\n│F  │\n│D  │\n└───┘\n"},
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

	got := render("", `<table style="border-style:hidden"><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "┌─┐\n│x│\n└─┘\n" {
		t.Fatalf("nested table did not render as table:\ngot  %q\nwant %q", got, "┌─┐\n│x│\n└─┘\n")
	}

	got = render(`table { border-style: solid; }`, `<table style="border-style:hidden"><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "┌─┐\n│x│\n└─┘\n" {
		t.Fatalf("nested table did not apply table CSS:\ngot  %q\nwant %q", got, "┌─┐\n│x│\n└─┘\n")
	}

	got = render(`table { border-style: none; }`, `<table><tr><td><table><tr><td>x</td></tr></table></td></tr></table>`)
	if got != "x\n" {
		t.Fatalf("nested borderless table was not compact:\ngot  %q\nwant %q", got, "x\n")
	}

	got = render(`table { border-style: none; } table::before { content: "<TABLE>"; } table::after { content: "</TABLE>"; } tr::before { content: "<TR>"; } tr::after { content: "</TR>"; } td::before { content: "<TD>"; } td::after { content: "</TD>"; }`,
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
	got2, err := r.Render(`<table style="width:100%"><tr><td><table style="width:100%"><tr><td>Hi</td></tr></table></td></tr></table>`)
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
	r3, err := htmlterm.New(htmlterm.Options{Width: 30})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got3, err := r3.Render(`<table style="border-style:hidden"><tr><td><table style="margin-left:2; margin-right:2; padding-left:2; padding-right:2" ><tr><td>hi</td></tr></table></td></tr></table>`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got3 = stripANSI(got3)
	want3 := "  ┌──────┐  \n  │  hi  │  \n  └──────┘  \n"
	if got3 != want3 {
		t.Fatalf("nested table's own right margin/padding was lost:\ngot  %q\nwant %q", got3, want3)
	}
}

func TestTablePreservesInlineChildStyling(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<table style="border-style:hidden"><tr><td><b>B</b> C</td></tr></table>`)
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
		{name: "white-space:normal wraps cell content", html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="5">Hello World</td></tr></table>`, want: "Hello\nWorld\n"},
		{name: "white-space:nowrap still truncates", html: `<table style="border-style:hidden"><tr><td style="white-space:nowrap" width="5">Hello World</td></tr></table>`, want: "Hell…\n"},
		{name: "multi-column row where one cell wraps", html: `<table style="border-style:hidden"><tr><td width="3">A</td><td style="white-space:normal" width="5">Hi there</td></tr></table>`, want: "A   Hi   \n    there\n"},
		{name: "long word is hard-broken", html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="4">Superlongword</td></tr></table>`, want: "Supe\nrlon\ngwor\nd   \n"},
		{name: "short content still fits on one line", html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="10">Hello</td></tr></table>`, want: "Hello     \n"},
		{name: "wrapping with bordered table", html: `<table><tr><th width="5">Name</th></tr><tr><td style="white-space:normal" width="5">Al Bob</td></tr></table>`, want: "┌─────┐\n│Name │\n├─────┤\n│Al   │\n│Bob  │\n└─────┘\n"},
		{name: "table cell preserves br line breaks", html: `<table style="border-style:hidden"><tr><td width="4">a<br>b</td></tr></table>`, want: "a   \nb   \n"},
		{name: "table cell skips display none descendants", html: `<table style="border-style:hidden"><tr><td width="4"><span style="display:none">x</span>y</td></tr></table>`, want: "y   \n"},
		{name: "percentage block children in cells fit page width", css: `h2::before { content: "## "; } table { border-style: none; } td { width: 100%; white-space: normal; } td::after { content: "\A"; }`, html: `<table><tr><td><h2>Left Headline Very Long and Takes Up Space</h2></td><td><h2>Right Headline is also very long and takes up space</h2></td></tr></table>`, width: 30, want: "## Left        ## Right       \nHeadline Very  Headline is    \nLong and Takes also very long \nUp Space       and takes up   \n               space          \n"},
	})
}

func TestTableVerticalAlign(t *testing.T) {
	hidden := `style="border-style:hidden"`
	runCases(t, []renderCase{
		{name: "vertical-align default is top", html: `<table ` + hidden + `><tr><td width="2">X</td><td style="white-space:normal" width="3">A B C D E</td></tr></table>`, want: "X  A B\n   C D\n   E  \n"},
		{name: "vertical-align:top", html: `<table ` + hidden + `><tr><td width="2" style="vertical-align:top">X</td><td style="white-space:normal" width="3">A B C D E</td></tr></table>`, want: "X  A B\n   C D\n   E  \n"},
		{name: "vertical-align:bottom", html: `<table ` + hidden + `><tr><td width="2" style="vertical-align:bottom">X</td><td style="white-space:normal" width="3">A B C D E</td></tr></table>`, want: "   A B\n   C D\nX  E  \n"},
		{name: "vertical-align:middle", html: `<table ` + hidden + `><tr><td width="2" style="vertical-align:middle">X</td><td style="white-space:normal" width="3">A B C D E</td></tr></table>`, want: "   A B\nX  C D\n   E  \n"},
		{name: "vertical-align:bottom two-line tall cell", html: `<table ` + hidden + `><tr><td width="2" style="vertical-align:bottom">X</td><td style="white-space:normal" width="3">A B C D</td></tr></table>`, want: "   A B\nX  C D\n"},
	})
}

func TestTableBorderCSS(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-rows solid adds separators between data rows", css: `table { border-style: solid; border-rows: solid; }`, html: `<table><tr><td width="3">A</td></tr><tr><td width="3">B</td></tr></table>`, want: "┌───┐\n│A  │\n├───┤\n│B  │\n└───┘\n"},
		{name: "border-header none suppresses header divider", css: `table { border-style: solid; border-header: none; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "┌───┐\n│H  │\n│A  │\n└───┘\n"},
		{name: "border-columns none removes cell separators in rows", css: `table { border-style: solid; border-columns: none; }`, html: `<table><tr><td width="2">A</td><td width="2">B</td></tr></table>`, want: "┌──┬──┐\n│A B │\n└──┴──┘\n"},
		{name: "border-rows none on a table that had row separators enabled", css: `table { border-style: solid; border-rows: solid; border-rows: none; }`, html: `<table><tr><td width="3">A</td></tr><tr><td width="3">B</td></tr></table>`, want: "┌───┐\n│A  │\n│B  │\n└───┘\n"},
		{name: "border-left none removes left outer edge and corners", css: `table { border-style: solid; border-left: none; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "───┐\nH  │\n───┤\nA  │\n───┘\n"},
		{name: "border-right none removes right outer edge and corners", css: `table { border-style: solid; border-right: none; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "┌───\n│H  \n├───\n│A  \n└───\n"},
	})
}

func TestTableBorderEdgeShorthand(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-top literal glyph on a table", css: `table { border-top: "═"; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "┌═══┬═══┐\n│A  │B  │\n└───┴───┘\n"},
		{name: "border-top shorthand style-only picks that preset's top glyph", css: `table { border-top: double; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "┌═══┬═══┐\n│A  │B  │\n└───┴───┘\n"},
		{name: "border-top shorthand width-style-color drops width", css: `table { border-top: 1px double red; }`, html: `<table><tr><td width="3">A</td></tr></table>`, want: "┌═══┐\n│A  │\n└───┘\n"},
		{name: "border-top:none on a solid table removes just the top edge (regression parity with block's fix)", css: `table { border-style: solid; border-top: none; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "│A  │B  │\n└───┴───┘\n"},
		{name: "border-top shorthand resurrects a top edge on a preset with none by default", css: `table { border-style: markdown; border-top: solid; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "───\n|H  |\n|───|\n|A  |\n"},
		{name: "internal separator fill reuses border-top's overridden fill", css: `table { border-top: double; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "┌═══┐\n│H  │\n├═══┤\n│A  │\n└───┘\n"},
		{name: "border-top-mid overrides the outer top T-junction", css: `table { border-top-mid: 'v'; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "┌───v───┐\n│A  │B  │\n└───┴───┘\n"},
		{name: "border-bottom-mid overrides the outer bottom T-junction", css: `table { border-bottom-mid: '^'; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "┌───┬───┐\n│A  │B  │\n└───^───┘\n"},
		{name: "border-left-mid overrides the header/row-separator left junction", css: `table { border-left-mid: 'L'; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "┌───┐\n│H  │\nL───┤\n│A  │\n└───┘\n"},
		{name: "border-right-mid overrides the header/row-separator right junction", css: `table { border-right-mid: 'R'; }`, html: `<table><tr><th width="3">H</th></tr><tr><td width="3">A</td></tr></table>`, want: "┌───┐\n│H  │\n├───R\n│A  │\n└───┘\n"},
		{name: "border-center overrides the header/row-separator cross junction", css: `table { border-center: '+'; }`, html: `<table><tr><th width="3">H</th><th width="3">I</th></tr><tr><td width="3">A</td><td width="3">B</td></tr></table>`, want: "┌───┬───┐\n│H  │I  │\n├───+───┤\n│A  │B  │\n└───┴───┘\n"},
		{name: "table outer corners can be overridden independently", css: `table { border-top-left-corner: '1'; border-top-right-corner: '2'; border-bottom-left-corner: '3'; border-bottom-right-corner: '4'; }`, html: `<table><tr><td width="3">A</td></tr></table>`, want: "1───2\n│A  │\n3───4\n"},
	})
}

func TestTableBorderStyles(t *testing.T) {
	oneCol := func(style string) string {
		return `<table style="border-style:` + style + `"><tr><th width="3">H</th></tr><tr><td>A</td></tr></table>`
	}
	runCases(t, []renderCase{
		{name: "thick border style", html: oneCol("thick"), want: "┏━━━┓\n┃H  ┃\n┣━━━┫\n┃A  ┃\n┗━━━┛\n"},
		{name: "double border style", html: oneCol("double"), want: "╔═══╗\n║H  ║\n╠═══╣\n║A  ║\n╚═══╝\n"},
		{name: "markdown border style", html: oneCol("markdown"), want: "|H  |\n|---|\n|A  |\n"},
		{name: "standard border style", html: oneCol("standard"), want: "H  \n───\nA  \n"},
	})
}

func TestTableCellTextAlign(t *testing.T) {
	hidden := `style="border-style:hidden"`
	runCases(t, []renderCase{
		{name: "text-align right in cell", html: `<table ` + hidden + `><tr><td style="text-align:right" width="6">hi</td></tr></table>`, want: "    hi\n"},
		{name: "text-align center in cell", html: `<table ` + hidden + `><tr><td style="text-align:center" width="6">hi</td></tr></table>`, want: "  hi  \n"},
		{name: "text-align left is explicit default", html: `<table ` + hidden + `><tr><td style="text-align:left" width="6">hi</td></tr></table>`, want: "hi    \n"},
	})
}

func TestColWithCSSAndWidthAttr(t *testing.T) {
	// col element has a CSS property (color via class) and a width HTML attribute
	// but no CSS width — exercises the copyMap(non-empty) path in collectColDecls
	runCases(t, []renderCase{
		{
			name: "col with CSS class and width attribute",
			css:  `.narrow { color: #888888; }`,
			html: `<table style="border-style:hidden"><colgroup><col class="narrow" width="5"></colgroup><tr><th>Name</th></tr><tr><td>Alice</td></tr></table>`,
			want: "Name \nAlice\n",
		},
	})
}
