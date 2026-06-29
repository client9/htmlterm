package htmlterm_test

import "testing"

func TestTextOverflow(t *testing.T) {
	cell := func(attrs, content string) string {
		return `<table style="border-style:hidden"><tr><td ` + attrs + `>` + content + `</td></tr></table>`
	}
	runCases(t, []renderCase{
		{name: "ellipsis (default) truncates with …", html: cell(`width="5"`, "Hello World"), want: "Hell…\n"},
		{name: "clip truncates without marker", html: cell(`style="text-overflow:clip" width="5"`, "Hello World"), want: "Hello\n"},
		{name: "custom string marker", html: cell(`style='text-overflow:"+"' width="5"`, "Hello World"), want: "Hell+\n"},
		{name: "no truncation when content fits", html: cell(`width="11"`, "Hello World"), want: "Hello World\n"},
		{name: "ellipsis on exact fit needs no truncation", html: cell(`width="5"`, "Hello"), want: "Hello\n"},
		{name: "clip width=1 takes first rune", html: cell(`style="text-overflow:clip" width="1"`, "Hello"), want: "H\n"},
		{name: "text-overflow via CSS class", css: `.clip td { text-overflow: clip; }`, html: `<table class="clip" style="border-style:hidden"><tr><td width="5">Hello World</td></tr></table>`, want: "Hello\n"},
	})
}

func TestTableCellPadding(t *testing.T) {
	hidden := `style="border-style:hidden"`
	runCases(t, []renderCase{
		{name: "padding-left indents cell content", html: `<table ` + hidden + `><tr><td style="padding-left:1" width="6">ab</td></tr></table>`, want: " ab   \n"},
		{name: "padding-right adds space after cell content", html: `<table ` + hidden + `><tr><td style="padding-right:1" width="6">ab</td></tr></table>`, want: "ab    \n"},
		{name: "padding-left and padding-right both set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1" width="7">ab</td></tr></table>`, want: " ab    \n"},
		{name: "natural width includes padding when no explicit width set", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1">ab</td></tr></table>`, want: " ab \n"},
		{name: "padding-left truncates content to reduced content width", html: `<table ` + hidden + `><tr><td style="padding-left:1" width="5">Hello</td></tr></table>`, want: " Hel…\n"},
		{name: "padding-top adds blank line above content", html: `<table ` + hidden + `><tr><td style="padding-top:1" width="5">ab</td></tr></table>`, want: "     \nab   \n"},
		{name: "padding-bottom adds blank line below content", html: `<table ` + hidden + `><tr><td style="padding-bottom:1" width="5">ab</td></tr></table>`, want: "ab   \n     \n"},
		{name: "padding-top 2 adds two blank lines above", html: `<table ` + hidden + `><tr><td style="padding-top:2" width="4">X</td></tr></table>`, want: "    \n    \nX   \n"},
		{name: "padding-top with padding-left", html: `<table ` + hidden + `><tr><td style="padding-top:1; padding-left:1" width="6">ab</td></tr></table>`, want: "      \n ab   \n"},
		{name: "all four sides of padding", html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1; padding-top:1; padding-bottom:1" width="7">ab</td></tr></table>`, want: "       \n ab    \n       \n"},
		{name: "padding-top in one cell raises row height for sibling", html: `<table ` + hidden + `><tr><td style="padding-top:1" width="3">X</td><td width="3">Y</td></tr></table>`, want: "    Y  \nX      \n"},
		{name: "padding-left on th header", html: `<table ` + hidden + `><tr><th style="padding-left:1" width="7">Name</th></tr><tr><td width="7">val</td></tr></table>`, want: " Name  \nval    \n"},
		{name: "padding-top on th header adds blank row before header text", html: `<table ` + hidden + `><tr><th style="padding-top:1" width="4">Hi</th></tr><tr><td width="4">ok</td></tr></table>`, want: "    \nHi  \nok  \n"},
		{name: "padding-left with wrapping cell", html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:normal" width="7">Hello World</td></tr></table>`, want: " Hello \n World \n"},
	})
}

func TestTable(t *testing.T) {
	runCases(t, []renderCase{
		{name: "two-column hidden-border table", html: `<table style="border-style:hidden"><tr><td width="3">A</td><td width="5">Hello</td></tr></table>`, width: 40, want: "A   Hello\n"},
		{name: "normal border style: single header and data row", html: `<table><tr><th width="3">H1</th><th width="4">H2</th></tr><tr><td>A</td><td>Long</td></tr></table>`, width: 40, want: "┌───┬────┐\n│H1 │H2  │\n├───┼────┤\n│A  │Long│\n└───┴────┘\n"},
		{name: "table width:100% expands flexible column", css: `table { width: 100%; border-style: hidden; }`, html: `<table><tr><td width="5">fixed</td><td>flex</td></tr></table>`, width: 20, want: "fixed flex          \n"},
		{name: "table border-left none overrides border-style deterministically", css: `table { border-style: normal; border-left: none; }`, html: `<table><tr><td width="3">A</td><td width="3">B</td></tr></table>`, width: 40, want: "───┬───┐\nA  │B  │\n───┴───┘\n"},
	})
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
