package htmlterm_test

import "testing"

// bordered wraps html in a table/td/th selector that pulls every cell into a
// shared border-collapse:collapse grid — the real-CSS replacement for the
// deleted border-header/border-columns/border-rows toggles. These tests
// care about colspan/rowspan grid geometry; the shared border is just a
// visual aid to see the merged cells' boundaries.
const collapseGridCSS = `table { border-collapse: collapse; } td, th { border: solid; }`

func TestColspan(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "basic colspan header",
			css:  collapseGridCSS,
			html: `<table><tr><th colspan="2">Header</th></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌───────┐\n│Header │\n├───┬───┤\n│A  │B  │\n└───┴───┘\n",
		},
		{
			name: "colspan cell gets combined width of its spanned columns, borderless",
			html: `<table><tr><td colspan="2">Long spanning text</td></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "Long spanning text\nA        B        \n",
		},
		{
			name: "colspan wider than its columns' own natural width forces them to grow, borderless",
			html: `<table><tr><td colspan="3">A very long spanning header cell</td></tr><tr><td>x</td><td>y</td><td>z</td></tr></table>`,
			want: "A very long spanning header cell\nx          y          z         \n",
		},
		{
			name: "column grid re-diverges below a colspan",
			css:  collapseGridCSS,
			html: `<table><tr><td colspan="2">Top</td></tr><tr><td>M1</td><td>M2</td></tr></table>`,
			want: "┌─────┐\n│Top  │\n├──┬──┤\n│M1│M2│\n└──┴──┘\n",
		},
		{
			name: "no colspan attribute behaves exactly as before (colSpan=1)",
			css:  collapseGridCSS,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n└─┴─┘\n",
		},
	})
}

func TestRowspan(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "basic rowspan merges border and flows content across rows",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="2">Tall</td><td>Top</td></tr><tr><td>Bottom</td></tr></table>`,
			want: "┌────┬──────┐\n" +
				"│Tall│Top   │\n" +
				"│    ├──────┤\n" +
				"│    │Bottom│\n" +
				"└────┴──────┘\n",
		},
		{
			name: "rowspan cell forces its spanned sibling rows to grow to fit its content, borderless",
			html: `<table><tr><td rowspan="2">Line one<br>Line two<br>Line three</td><td>A</td></tr><tr><td>B</td></tr></table>`,
			want: "Line one  A\nLine two   \nLine threeB\n",
		},
		{
			name: "adjacent independent rowspans keep their shared column boundary",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="2">A-tall</td><td rowspan="2">B-tall</td></tr><tr></tr><tr><td>C</td><td>D</td></tr></table>`,
			want: "┌──────┬──────┐\n" +
				"│A-tall│B-tall│\n" +
				"│      │      │\n" +
				"├──────┼──────┤\n" +
				"│C     │D     │\n" +
				"└──────┴──────┘\n",
		},
		{
			name: "rowspan=0 spans every remaining row",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="0">All</td><td>R1</td></tr><tr><td>R2</td></tr><tr><td>R3</td></tr></table>`,
			want: "┌───┬──┐\n" +
				"│All│R1│\n" +
				"│   ├──┤\n" +
				"│   │R2│\n" +
				"│   ├──┤\n" +
				"│   │R3│\n" +
				"└───┴──┘\n",
		},
		{
			name: "rowspan larger than the table's actual remaining rows is clamped, not a crash, borderless",
			html: `<table><tr><td rowspan="100">X</td><td>A</td></tr><tr><td>B</td></tr></table>`,
			want: "XA\n B\n",
		},
		{
			name: "vertical-align middle centers content across the whole merged block, not per physical row",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="3" style="vertical-align:middle">Mid</td><td>R1</td></tr><tr><td>R2</td></tr><tr><td>R3</td></tr></table>`,
			want: "┌───┬──┐\n" +
				"│   │R1│\n" +
				"│   ├──┤\n" +
				"│Mid│R2│\n" +
				"│   ├──┤\n" +
				"│   │R3│\n" +
				"└───┴──┘\n",
		},
		{
			name: "column after a rowspan lands in the correct column, not shifted left (the core regression case)",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="2">X</td><td>A1</td><td>B1</td></tr><tr><td>A2</td><td>B2</td></tr></table>`,
			want: "┌─┬──┬──┐\n│X│A1│B1│\n│ ├──┼──┤\n│ │A2│B2│\n└─┴──┴──┘\n",
		},
		{
			name: "no rowspan attribute behaves exactly as before (rowSpan=1)",
			css:  collapseGridCSS,
			html: `<table><tr><td>A</td></tr><tr><td>B</td></tr></table>`,
			want: "┌─┐\n│A│\n├─┤\n│B│\n└─┘\n",
		},
	})
}

func TestColspanRowspanCombined(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "colspan and rowspan together merge both border axes correctly",
			css:  collapseGridCSS,
			html: `<table><tr><td rowspan="2">Left</td><td colspan="2">TopRight</td></tr><tr><td>M1</td><td>M2</td></tr></table>`,
			want: "┌────┬─────────┐\n│Left│TopRight │\n│    ├────┬────┤\n│    │M1  │M2  │\n└────┴────┴────┘\n",
		},
	})
}
