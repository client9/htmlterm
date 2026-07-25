package htmlterm_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm"
)

// TestTableBorderCollapseCollapse covers `border-collapse: collapse` (the
// real per-cell border-conflict-resolution model, internal/render/
// table_collapse.go) - see docs/TABLES.md for the full write-up of the
// conflict-resolution rules and junction-glyph synthesis this exercises.
func TestTableBorderCollapseCollapse(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "bare table, no CSS at all, is completely borderless",
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "AB\n",
		},
		{
			name: "unset border-collapse renders identically to explicit separate for the same markup",
			css:  `td, th { border: solid; }`,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┐┌─┐\n│A││B│\n└─┘└─┘\n",
		},
		{
			name: "border-collapse: separate produces the same output as unset for the same markup",
			css:  `table { border-collapse: separate; } td, th { border: solid; }`,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┐┌─┐\n│A││B│\n└─┘└─┘\n",
		},
		{
			name: "same style/color on adjacent cells merges into one shared line and junction",
			css:  `table { border-collapse: collapse; } td, th { border: solid; }`,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n└─┴─┘\n",
		},
		{
			name: "hidden on one cell's edge suppresses the shared segment regardless of the neighbor's own edge",
			css:  `table { border-collapse: collapse; } td { border: solid; } td.h { border-right: hidden; }`,
			html: `<table><tr><td class="h">A</td><td>B</td></tr></table>`,
			want: "┌──┐\n│AB│\n└──┘\n",
		},
		{
			name: "neither side sets a border: the shared segment is absent, consuming no space",
			css:  `table { border-collapse: collapse; }`,
			html: `<table><tr><td>A</td><td style="border:solid">B</td></tr></table>`,
			want: " ┌─┐\nA│B│\n └─┘\n",
		},
		{
			name: "differing styles at a shared edge: higher precedence (double) wins over solid",
			css:  `table { border-collapse: collapse; } td.a { border-style: solid; } td.b { border-style: double; }`,
			html: `<table><tr><td class="a">A</td><td class="b">B</td></tr></table>`,
			want: "┌─╦═╗\n│A║B║\n└─╩═╝\n",
		},
		{
			name: "differing styles at a shared edge: heavy wins over solid",
			css:  `table { border-collapse: collapse; } td.a { border-style: heavy; } td.b { border-style: solid; }`,
			html: `<table><tr><td class="a">A</td><td class="b">B</td></tr></table>`,
			want: "┏━┳─┐\n┃A┃B│\n┗━┻─┘\n",
		},
		{
			name: "differing styles at a shared edge: solid wins over rounded",
			css:  `table { border-collapse: collapse; } td.a { border-style: solid; } td.b { border-style: rounded; }`,
			html: `<table><tr><td class="a">A</td><td class="b">B</td></tr></table>`,
			want: "┌─┬─╮\n│A│B│\n└─┴─╯\n",
		},
		{
			name: "a literal-glyph border with no named style renders its junctions as blank space, not a borrowed box-drawing glyph",
			css:  `table { border-collapse: collapse; } td, th { border-left: "|"; border-right: "|"; } th { border-bottom: "-"; }`,
			html: `<table><tr><th>H</th><th>I</th></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "|H|I|\n - - \n|A|B|\n",
		},
		{
			name: "colspan suppresses the interior segment its own box covers",
			css:  `table { border-collapse: collapse; } td, th { border: solid; }`,
			html: `<table><tr><th colspan="2">Header</th></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌───────┐\n│Header │\n├───┬───┤\n│A  │B  │\n└───┴───┘\n",
		},
		{
			name: "rowspan suppresses the interior segment its own box covers",
			css:  `table { border-collapse: collapse; } td, th { border: solid; }`,
			html: `<table><tr><td rowspan="2">Tall</td><td>Top</td></tr><tr><td>Bottom</td></tr></table>`,
			want: "┌────┬──────┐\n│Tall│Top   │\n│    ├──────┤\n│    │Bottom│\n└────┴──────┘\n",
		},
	})
}

// TestTableBorderCollapseColor covers same-style tie-breaking by color
// source order (cell beats table, matching real CSS's own hierarchy).
func TestTableBorderCollapseColor(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<table style="border-collapse:collapse;border-style:solid;border-color:#ff0000"><tr><td style="width:3;border-style:solid;border-color:#00ff00">A</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;0;255;0m") {
		t.Fatalf("cell's own border-color did not win over the table's: %q", got)
	}
	if strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("table's border-color leaked through where the cell set its own: %q", got)
	}
}
