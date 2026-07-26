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
			// Regression: edgeStyleName used to fall through to the cell's
			// own whole-box border-style whenever its edge-specific
			// declaration wasn't a *named* style, even though a literal
			// glyph is just as much an explicit override as a named one -
			// so a cell with both `border: solid` and a literal
			// `border-left` incorrectly got "solid"'s precedence for that
			// edge (real precedence 3) instead of "" (unnamed, precedence
			// 0), letting its literal glyph wrongly win every tie against a
			// genuinely-solid neighbor - including the table's own outer
			// edge and the adjacent cell's own unset-but-solid-by-default
			// edge. Every divider here (outer and interior) must render the
			// real "solid" style, not the stray literal.
			name: "a cell's literal-glyph edge override doesn't borrow its own whole-box border-style's precedence",
			css:  `table { border-collapse: collapse; border: solid; } td, th { border: solid; } td, th { border-left: "┆"; }`,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n└─┴─┘\n",
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

// TestTableBorderCollapseCorners covers border-*-corner under
// border-collapse:collapse - previously silently ignored (composeCollapsedGrid
// discarded resolveBoxBorders' corner return values), now consulted for the
// table's own true 4 outer corners, same as border-collapse:separate's own
// table box and plain blocks already do.
func TestTableBorderCollapseCorners(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "table's own outer corners can be overridden under collapse (regression: previously silently ignored)",
			css: `table { border-collapse: collapse; border-style: solid; border-spacing:0;
				border-top-left-corner: '1'; border-top-right-corner: '2';
				border-bottom-left-corner: '3'; border-bottom-right-corner: '4'; }`,
			html: `<table><tr><td>A</td></tr></table>`,
			want: "1─2\n│A│\n3─4\n",
		},
	})
}

// TestTableBorderCollapseCellShapeOverrides covers cell-level
// border-*-corner/border-*-junction under collapse - previously table-only,
// the one inconsistent exception among border properties (every other one
// already works on both <table> and <th>/<td>). A cell only has 4 corners
// total, so this reuses border-*-corner's existing 4 names (rather than
// inventing new ones) plus border-*-junction's existing 5 non-diagonal
// names; the 4 diagonal junction properties stay table-only (see
// docs/TABLES.md).
func TestTableBorderCollapseCellShapeOverrides(t *testing.T) {
	runCases(t, []renderCase{
		{
			// Regression: the table-level corner fix read tlCorner/etc.
			// from resolveBoxBorders(tableDecls), which blends an explicit
			// override with the table's own border-style preset's corner
			// glyph as a fallback - firing regardless of whether that
			// style actually won the surrounding edges. Here the table's
			// own border-style is rounded (precedence 2), but td's own
			// border-style is heavy (precedence 4) and correctly wins the
			// edges - the corners must be heavy's own, not a mismatched
			// rounded corner around heavy fill/edge characters.
			name: "table's own weaker style must not supply a mismatched corner around a cell's own stronger style",
			css:  `table { border-collapse: collapse; border-style: rounded; } td, th { border-style: heavy; }`,
			html: `<table><tr><td>A</td></tr></table>`,
			want: "┏━┓\n┃A┃\n┗━┛\n",
		},
		{
			name: "a cell's own border-top-left-corner overrides at its own corner position",
			css:  `table { border-collapse: collapse; border-style: solid; } td { border-top-left-corner: 'X'; }`,
			html: `<table><tr><td>A</td></tr></table>`,
			want: "X─┐\n│A│\n└─┘\n",
		},
		{
			name: "a cell's own border-center-junction overrides at its own interior cross",
			css:  `table { border-collapse: collapse; } td, th { border: solid; } th { border-center-junction: '+'; }`,
			html: `<table><tr><th>H</th><th>I</th></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌─┬─┐\n│H│I│\n├─+─┤\n│A│B│\n└─┴─┘\n",
		},
		{
			name: "a cell's own corner override beats a conflicting table-level one",
			css:  `table { border-collapse: collapse; border-style: solid; border-top-left-corner: 'T'; } td { border-top-left-corner: 'C'; }`,
			html: `<table><tr><td>A</td></tr></table>`,
			want: "C─┐\n│A│\n└─┘\n",
		},
		{
			name: "two adjacent cells conflict at a shared vertex: the earlier (left) cell's override wins",
			css: `table { border-collapse: collapse; } td, th { border: solid; }
				td.left { border-top-junction: 'L'; } td.right { border-top-junction: 'R'; }`,
			html: `<table><tr><td class="left">A</td><td class="right">B</td></tr><tr><td>C</td><td>D</td></tr></table>`,
			want: "┌─L─┐\n│A│B│\n├─┼─┤\n│C│D│\n└─┴─┘\n",
		},
		{
			// Regression: a colspan cell's own override must not leak onto
			// interior points along its own edges that aren't one of its 4
			// real corners. A (colspan=2) has no top-shaped vertex at any of
			// its own corners (nothing above it) - its border-top-junction
			// must not fire at the row0/row1 boundary between B and C, which
			// is a point along A's own *bottom* edge, not a corner.
			name: "a colspan cell's own override does not leak onto a vertex that isn't one of its own corners",
			css:  `table { border-collapse: collapse; } td { border: solid; } td.a { border-top-junction: 'X'; }`,
			html: `<table><tr><td class="a" colspan="2">A</td></tr><tr><td>B</td><td>C</td></tr></table>`,
			want: "┌───┐\n│A  │\n├─┬─┤\n│B│C│\n└─┴─┘\n",
		},
		{
			// Same bug class, rowspan axis: A (rowspan=2) has no left-shaped
			// vertex at any of its own corners (it's the leftmost column
			// entirely) - its border-left-junction must not fire at the
			// vertex where B/C's boundary meets A's own *right* edge.
			name: "a rowspan cell's own override does not leak onto a vertex that isn't one of its own corners",
			css:  `table { border-collapse: collapse; } td { border: solid; } td.a { border-left-junction: 'X'; }`,
			html: `<table><tr><td class="a" rowspan="2">A</td><td>B</td></tr><tr><td>C</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n│ ├─┤\n│ │C│\n└─┴─┘\n",
		},
	})
}

// TestTableBorderCollapseComfyTableStyle is an end-to-end reproduction of
// the real comfy-table (https://github.com/Nukesor/comfy-table) ASCII
// look that originally motivated cell-level shape overrides: the
// header/body divider needs its own left/right ends and interior crosses,
// distinct from what ordinary row dividers use - impossible with
// table-level-only border-*-junction, since both dividers share the exact
// same arm-presence shapes (only which *divider* differs). th supplies its
// own overrides at its own edges/corners; ordinary rows keep the
// table-level defaults.
func TestTableBorderCollapseComfyTableStyle(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "header/body divider gets its own seamless '=' run with '+' ends, distinct from the ordinary '|--+--|' row divider",
			css: `table {
				border-collapse: collapse; border: none;
				border-top: "-"; border-bottom: "-";
				border-center-junction: "+"; border-top-junction: "+"; border-bottom-junction: "+";
				border-left-junction: "|"; border-right-junction: "|";
				border-top-left-corner: "+"; border-top-right-corner: "+";
				border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
			}
			th, td { border-left: "|"; border-right: "|"; }
			th {
				border-bottom: "=";
				border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
				border-center-junction: "="; border-bottom-junction: "=";
				border-left-junction: "+"; border-right-junction: "+";
			}
			td { border-bottom: "-"; }`,
			html: `<table><tr><th>H1</th><th>H2</th></tr><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>`,
			want: "+--+--+\n|H1|H2|\n+=====+\n|a |b |\n|--+--|\n|c |d |\n+--+--+\n",
		},
	})
}

// TestTableBorderCollapseJunctions covers the border-*-junction family: a
// literal-glyph override for a T-shape/cross/corner-shape wherever that
// exact arm combination occurs in the grid, table-level only - the
// counterpart to the named style presets' own built-in junction tables, for
// custom/literal borders that otherwise render junctions as blank space
// (see docs/TABLES.md).
func TestTableBorderCollapseJunctions(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "border-top-junction overrides the T-shape where a column divider meets the table's own top edge",
			css:  `table { border-collapse: collapse; border-top: "="; border-top-junction: '+'; } td { border-left: "|"; border-right: "|"; }`,
			html: `<table><tr><td>A</td><td>B</td></tr></table>`,
			want: " =+= \n|A|B|\n",
		},
		{
			name: "border-center-junction overrides an interior 4-arm cross",
			css:  `table { border-collapse: collapse; border-center-junction: '+'; } td, th { border: solid; }`,
			html: `<table><tr><td>A</td><td>B</td></tr><tr><td>C</td><td>D</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n├─+─┤\n│C│D│\n└─┴─┘\n",
		},
		{
			name: "border-top-right-junction overrides a corner-shape wherever it occurs, not just the table's own outer corner",
			// row0 is jagged (1 cell, row1 has 2). Both marked vertices are
			// genuine top-right-shaped corners away from the table's true
			// top-right corner (far right of row0's own line, which stays
			// blank here since row0 doesn't reach that far): one where
			// row0's own short top edge ends, one where row0's short right
			// edge meets row1's column divider below it.
			css:  `table { border-collapse: collapse; border-top-right-junction: '+'; } td, th { border: solid; }`,
			html: `<table><tr><td>A</td></tr><tr><td>B</td><td>C</td></tr></table>`,
			want: "┌─+ \n│A│  \n├─┼─+\n│B│C│\n└─┴─┘\n",
		},
		{
			name: "unset reverts border-center-junction back to the computed glyph",
			css:  `table { border-collapse: collapse; border-center-junction: '+'; } table.x { border-center-junction: unset; } td, th { border: solid; }`,
			html: `<table class="x"><tr><td>A</td><td>B</td></tr><tr><td>C</td><td>D</td></tr></table>`,
			want: "┌─┬─┐\n│A│B│\n├─┼─┤\n│C│D│\n└─┴─┘\n",
		},
		{
			name: "a vertex whose shape has no matching override falls through to the computed glyph unchanged",
			css:  `table { border-collapse: collapse; border-center-junction: '+'; } td, th { border: solid; }`,
			html: `<table><tr><th colspan="2">Header</th></tr><tr><td>A</td><td>B</td></tr></table>`,
			want: "┌───────┐\n│Header │\n├───┬───┤\n│A  │B  │\n└───┴───┘\n",
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

// TestTableBorderCollapseSameTypeTieBreak covers the cell-vs-cell (same
// element type) tie-break direction: verified directly against Firefox,
// Safari, and Chrome - a same-style tie goes to the *earlier* element in
// reading order (the row above wins a horizontal tie, the column to the
// left wins a vertical one), not the later one. This is a different rule
// from - and doesn't affect - "cell beats table" (TestTableBorderCollapseColor
// above), which is real CSS's own spec-mandated color hierarchy, not an
// unspecified same-type tie.
func TestTableBorderCollapseSameTypeTieBreak(t *testing.T) {
	t.Run("horizontal: the row above wins a same-style tie, matching real browsers", func(t *testing.T) {
		r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
		if err != nil {
			t.Fatal(err)
		}
		// Both edges are solid (a genuine style tie), distinguishable only
		// by th's explicit color - td sets no color at all, so if td's
		// edge won instead, the boundary would render with no color code.
		got, err := r.Render(`<table style="border-collapse:collapse"><tr><th style="border-bottom:solid;border-bottom-color:blue">H</th></tr><tr><td style="border-top:solid;border-bottom:solid">A</td></tr></table>`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "\x1b[38;2;0;0;255m") {
			t.Fatalf("th's own (earlier, blue) edge should have won the tie over td's (later, uncolored) edge: %q", got)
		}
	})

	t.Run("vertical: the left column wins a same-style tie, matching real browsers", func(t *testing.T) {
		r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.Render(`<table style="border-collapse:collapse"><tr><th style="border-left:solid;border-right:solid;border-left-color:red;border-right-color:blue">A</th><th style="border-left:solid;border-right:solid;border-left-color:red;border-right-color:blue">B</th><th style="border-left:solid;border-right:solid;border-left-color:red;border-right-color:blue">C</th></tr></table>`)
		if err != nil {
			t.Fatal(err)
		}
		plain := stripANSI(got)
		if plain != "│A│B│C│\n" {
			t.Fatalf("unexpected plain layout: %q", plain)
		}
		// Left exterior: cell beats table (unaffected by this fix) - red.
		if !strings.Contains(got, "\x1b[38;2;255;0;0m│\x1b[m\x1b[1mA") {
			t.Fatalf("left exterior should be red (cell beats table): %q", got)
		}
		// Both interior dividers: the left-hand cell's own right edge
		// (blue) should win over the right-hand cell's own left edge
		// (red) - matching the real-browser "earlier wins" result.
		if strings.Count(got, "\x1b[38;2;0;0;255m│\x1b[m") != 3 {
			t.Fatalf("expected exactly 3 blue dividers (2 interior + right exterior): %q", got)
		}
	})
}

// TestTableBorderCollapseColStyle covers a <col>'s own border participating
// in real conflict resolution against its column's cells, instead of being
// silently overridden outright by any cell-level border declaration on the
// same property. Regression for a bug where mergedCellDecls' flat "col is a
// fallback base, cell overrides" merge let a cell's lower-precedence style
// win a property key it happened to also declare explicitly (border-right,
// here), even though real CSS 2.1 §17.6.2.1 ranks style precedence
// (double > solid) above element-type precedence (cell > col) - a <col>'s
// higher-precedence style must still win against a cell that also set that
// exact edge.
func TestTableBorderCollapseColStyle(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "a col's higher-precedence style wins over a cell's own lower-precedence style on the same edge",
			html: `<table style="border-collapse:collapse"><colgroup><col style="border-right:double"><col></colgroup><tr><td style="border-right:solid">A</td><td style="border-left:solid">B</td></tr></table>`,
			want: "A║B\n",
		},
		{
			name: "a cell's own border still wins when the col has no competing declaration on that edge",
			html: `<table style="border-collapse:collapse"><colgroup><col><col></colgroup><tr><td style="border-right:solid">A</td><td>B</td></tr></table>`,
			want: "A│B\n",
		},
	})
}
