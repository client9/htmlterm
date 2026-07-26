package render

import (
	"strings"

	"golang.org/x/net/html"
)

// stylePrecedence ranks htmlterm's own border-style vocabulary for
// collapsed-border conflict resolution — CSS 2.1 §17.6.2.1 has its own
// fixed ranking for real CSS keywords (double > solid > dashed > dotted >
// ridge > outset > groove > inset); htmlterm needs its own since the
// preset vocabulary differs.
// Border-width is a no-op deviation here (documented in COMPATIBILITY.md),
// so the spec's "widest wins" tier is skipped entirely — this ranking,
// plus "hidden always wins"/"absent loses", is the whole tie-break.
// "markdown"/"standard" and any unnamed (literal-glyph) style share the
// lowest rank (the Go zero value), a deliberately low but not
// hidden/absent tier.
var stylePrecedence = map[string]int{
	"double":   5,
	"heavy":    4,
	"solid":    3,
	"rounded":  2,
	"markdown": 1,
	"standard": 0,
}

// edgeCandidate is one side's resolved border for a single collapsed-grid
// segment: the actual glyph/color to paint (from resolveBoxBorders) plus
// the named style that produced it (from edgeStyleName) — style is empty
// when the edge used a literal quoted glyph instead of a named preset,
// which has no precedence ranking or junction-glyph table of its own.
type edgeCandidate struct {
	border blockBorder
	style  string
}

// edgeStyleName returns the named border-style (solid/rounded/heavy/
// double/markdown/standard/hidden/none) that determines edgeProp's glyph,
// mirroring resolveBoxBorders' own per-edge-then-whole-box precedence:
// border-top/-right/-bottom/-left wins if it's explicitly set at all, else
// the whole-box border-style falls back. Empty means the edge's glyph (if
// any) came from a literal quoted character instead — resolveBoxBorders
// still resolves its actual character correctly either way; this is only
// ever consulted for conflict-resolution precedence and junction-table
// selection, both of which need a name, not a glyph.
//
// The edge-specific branch must stop here even when it isn't a named style
// (a literal glyph, or an unrecognized token) - falling through to
// border-style in that case would incorrectly borrow the whole box's style
// name for precedence purposes, even though border-style plays no part in
// this edge's actual rendered glyph (resolveBoxBorders only ever consults
// border-style for an edge that has no explicit declaration of its own at
// all - see resolveBorderEdgeChar/resolveBoxBorders' own !present gating).
func edgeStyleName(decls map[string]string, edgeProp string) string {
	if v := strings.TrimSpace(decls[edgeProp]); v != "" {
		if _, ok := namedTableStyle(v); ok {
			return v
		}
		return ""
	}
	if v := strings.TrimSpace(decls["border-style"]); v != "" {
		if _, ok := namedTableStyle(v); ok {
			return v
		}
	}
	return ""
}

// resolveSegmentBorder picks the winning border for one shared collapsed-
// grid segment between two candidates — CSS 2.1 §17.6.2.1's conflict
// resolution, adapted for htmlterm (border-width is a no-op deviation, so
// the "widest wins" tier is skipped entirely). Ties are broken in favor of
// second — callers order their arguments so whichever side should win a
// tie is passed second: (table, cell) for a perimeter segment (real CSS's
// spec-mandated "cell beats table" color hierarchy), or (later, earlier)
// in reading order for a cell-vs-cell interior segment - real CSS doesn't
// specify this exact case, but real browsers (Firefox/Safari/Chrome,
// checked directly) consistently give it to the earlier element: the row
// above wins a horizontal tie, the column to the left wins a vertical one.
func resolveSegmentBorder(first, second edgeCandidate) edgeCandidate {
	if first.style == "hidden" || second.style == "hidden" {
		return edgeCandidate{}
	}
	if first.border.char == "" {
		return second
	}
	if second.border.char == "" {
		return first
	}
	if stylePrecedence[first.style] > stylePrecedence[second.style] {
		return first
	}
	return second
}

// Bit positions for junctionGlyph's presence mask.
const (
	jUp = 1 << iota
	jDown
	jLeft
	jRight
)

// solidJunctions is the full box-drawing junction set for "solid" (and,
// via junctionGlyphTables' fallback, for "markdown"/"standard"/any
// unnamed style) keyed by an up|down|left|right presence mask. Single-arm
// entries (a border ending with no continuation — rare in practice)
// approximate to the matching two-arm straight character rather than
// Unicode's little-used dead-end stub glyphs (╴╵╶╷), which don't have
// complete heavy/double equivalents to match anyway.
var solidJunctions = [16]string{
	jUp:                          "│",
	jDown:                        "│",
	jUp | jDown:                  "│",
	jLeft:                        "─",
	jRight:                       "─",
	jLeft | jRight:               "─",
	jUp | jLeft:                  "┘",
	jUp | jRight:                 "└",
	jDown | jLeft:                "┐",
	jDown | jRight:               "┌",
	jUp | jDown | jLeft:          "┤",
	jUp | jDown | jRight:         "├",
	jUp | jLeft | jRight:         "┴",
	jDown | jLeft | jRight:       "┬",
	jUp | jDown | jLeft | jRight: "┼",
}

// roundedJunctions is solid's table with just the four true two-arm outer
// corners swapped for rounded glyphs — matching the existing convention
// (namedTableStyle) that rounded's interior dividers are already
// solid-style, only its outer corners are rounded.
var roundedJunctions = func() [16]string {
	t := solidJunctions
	t[jDown|jRight] = "╭"
	t[jDown|jLeft] = "╮"
	t[jUp|jRight] = "╰"
	t[jUp|jLeft] = "╯"
	return t
}()

var heavyJunctions = [16]string{
	jUp:                          "┃",
	jDown:                        "┃",
	jUp | jDown:                  "┃",
	jLeft:                        "━",
	jRight:                       "━",
	jLeft | jRight:               "━",
	jUp | jLeft:                  "┛",
	jUp | jRight:                 "┗",
	jDown | jLeft:                "┓",
	jDown | jRight:               "┏",
	jUp | jDown | jLeft:          "┫",
	jUp | jDown | jRight:         "┣",
	jUp | jLeft | jRight:         "┻",
	jDown | jLeft | jRight:       "┳",
	jUp | jDown | jLeft | jRight: "╋",
}

var doubleJunctions = [16]string{
	jUp:                          "║",
	jDown:                        "║",
	jUp | jDown:                  "║",
	jLeft:                        "═",
	jRight:                       "═",
	jLeft | jRight:               "═",
	jUp | jLeft:                  "╝",
	jUp | jRight:                 "╚",
	jDown | jLeft:                "╗",
	jDown | jRight:               "╔",
	jUp | jDown | jLeft:          "╣",
	jUp | jDown | jRight:         "╠",
	jUp | jLeft | jRight:         "╩",
	jDown | jLeft | jRight:       "╦",
	jUp | jDown | jLeft | jRight: "╬",
}

// junctionGlyphTables maps a border-style name to its full junction set.
// Only styles with a real, complete box-drawing vocabulary of their own get
// an entry here. markdown/standard and any unnamed (literal-glyph) style
// have no junction glyphs of their own - junctionGlyph renders those as a
// blank space rather than borrowing a mismatched glyph from an unrelated
// style (see docs/TABLES.md).
var junctionGlyphTables = map[string][16]string{
	"solid":   solidJunctions,
	"rounded": roundedJunctions,
	"heavy":   heavyJunctions,
	"double":  doubleJunctions,
}

// namedJunctionProps maps an arm-presence mask to the table-level
// `border-*-junction` longhand that can supply a literal glyph override for
// that shape, wherever it occurs in the grid (not just the table's own
// outer corners - see border-*-corner for that narrower, position-specific
// case). Naming mirrors border-top-left-corner's own word order (location
// first, "junction"/"corner" last), matching real CSS's border-top-left-
// radius precedent. Two-arm straight-through masks (jUp|jDown, jLeft|
// jRight) and single-arm dead-ends are deliberately absent - those already
// just render as the fill character continuing, not a distinct junction
// shape worth its own override.
var namedJunctionProps = map[int]string{
	jDown | jRight:               "border-top-left-junction",
	jDown | jLeft:                "border-top-right-junction",
	jUp | jRight:                 "border-bottom-left-junction",
	jUp | jLeft:                  "border-bottom-right-junction",
	jDown | jLeft | jRight:       "border-top-junction",
	jUp | jLeft | jRight:         "border-bottom-junction",
	jUp | jDown | jRight:         "border-left-junction",
	jUp | jDown | jLeft:          "border-right-junction",
	jUp | jDown | jLeft | jRight: "border-center-junction",
}

// cellShapeProps maps an arm-presence mask to the property name a CELL's
// own declarations supply a literal glyph override through. Unlike the
// table-level split between border-*-corner (one true literal position)
// and border-*-junction (a shape-class default anywhere in the grid), a
// single cell only has 4 corners total - there's no interior,
// non-corner-shaped position belonging to just one cell - so "my own
// top-left corner" and "wherever a top-left-shaped vertex occurs among my
// own corners" are the same thing. This reuses border-*-corner's existing
// 4 property names for the corner shapes (already documented, already
// working under plain blocks/border-collapse:separate) rather than
// inventing 4 new "-corner-junction" names, plus border-*-junction's
// existing 5 non-diagonal names for the T-shapes and cross. No cell-level
// equivalent of the 4 diagonal border-*-junction properties is needed -
// see docs/TABLES.md.
var cellShapeProps = map[int]string{
	jDown | jRight:               "border-top-left-corner",
	jDown | jLeft:                "border-top-right-corner",
	jUp | jRight:                 "border-bottom-left-corner",
	jUp | jLeft:                  "border-bottom-right-corner",
	jDown | jLeft | jRight:       "border-top-junction",
	jUp | jLeft | jRight:         "border-bottom-junction",
	jUp | jDown | jRight:         "border-left-junction",
	jUp | jDown | jLeft:          "border-right-junction",
	jUp | jDown | jLeft | jRight: "border-center-junction",
}

// shapeOverride returns the literal glyph decls supplies for mask's shape
// via props (cellShapeProps or namedJunctionProps), or "" if none. Always a
// pure literal lookup (parseCSSString) - deliberately not
// resolveBoxBorders' blended style-fallback corners, so a style that
// didn't actually win the surrounding edges can never supply a mismatched
// corner glyph (e.g. a table's own border-style:rounded must not paint
// rounded corners around edges a cell's higher-precedence border-
// style:heavy actually won - see docs/TABLES.md).
func shapeOverride(decls map[string]string, props map[int]string, mask int) string {
	name, ok := props[mask]
	if !ok {
		return ""
	}
	return parseCSSString(decls[name])
}

// junctionGlyph returns the box-drawing character for a grid vertex with
// the given arm presence, in style's own glyph set. Returns "" when no arm
// is present at all, and " " when arms are present but style has no
// junction glyphs of its own (markdown/standard/a literal-glyph style) -
// a blank space signals "no junction configured" rather than silently
// importing an unrelated style's glyph.
func junctionGlyph(style string, up, down, left, right bool) string {
	mask := 0
	if up {
		mask |= jUp
	}
	if down {
		mask |= jDown
	}
	if left {
		mask |= jLeft
	}
	if right {
		mask |= jRight
	}
	if mask == 0 {
		return ""
	}
	table, ok := junctionGlyphTables[style]
	if !ok {
		return " "
	}
	return table[mask]
}

// cellFourEdgesFromDecls resolves a cell's own top/right/bottom/left
// edgeCandidates once (in that order) from its already-merged
// declarations, reusing resolveBoxBorders (the same primitive any other
// border box uses) for the actual glyph/color and edgeStyleName for the
// precedence/junction-table key. Takes decls directly (rather than
// re-merging from node/colDecls/colStart) since composeCollapsedGrid
// already computes each cell's merged decls once for its own cellDecls
// map, needed separately for shapeOverride lookups.
func cellFourEdgesFromDecls(decls map[string]string) [4]edgeCandidate {
	bl, br, bt, bb, _, _, _, _ := resolveBoxBorders(decls)
	return [4]edgeCandidate{
		{bt, edgeStyleName(decls, "border-top")},
		{br, edgeStyleName(decls, "border-right")},
		{bb, edgeStyleName(decls, "border-bottom")},
		{bl, edgeStyleName(decls, "border-left")},
	}
}

// resolveCellColEdges resolves cell's own four border edges against its
// column's <col>/<colgroup> declarations (if any) via real CSS 2.1
// §17.6.2.1 conflict resolution — same tiers resolveSegmentBorder already
// implements for every other border conflict (hidden wins, absent loses,
// then style precedence), with ties favoring the cell (real CSS's
// element-type precedence ranks a cell above a column). This replaces
// mergedCellDecls' flat "col is a fallback base, cell overrides outright"
// merge for border purposes specifically: that flat merge let a cell with
// any border declaration at all silently beat a <col>'s higher-precedence
// style (e.g. a <col>'s border-style: double should out-rank a cell's
// border-style: solid, since style precedence outranks element-type
// precedence — the flat merge never gave the column's style a chance to
// win that comparison at all). mergedCellDecls' own merge is left as-is for
// every other property (width, background-color, etc.), where "more
// specific wins outright" is the only conflict model that applies.
func (r *Engine) resolveCellColEdges(cell *tableCell, colDecls []map[string]string) [4]edgeCandidate {
	cellOwn := cellFourEdgesFromDecls(r.resolveDecls(cell.node))
	if cell.colStart >= len(colDecls) || len(colDecls[cell.colStart]) == 0 {
		return cellOwn
	}
	colOwn := cellFourEdgesFromDecls(colDecls[cell.colStart])
	var out [4]edgeCandidate
	for i := range out {
		out[i] = resolveSegmentBorder(colOwn[i], cellOwn[i])
	}
	return out
}

// renderTableCollapse renders n under `border-collapse: collapse`: a
// shared grid of horizontal/vertical lines between cells (and between the
// outermost cells and the table's own border), each segment resolved
// independently via resolveSegmentBorder from whichever cell(s)/table
// border it, and each vertex's junction glyph synthesized from
// junctionGlyph — real CSS 2.1 §17.6.2.1 conflict resolution, not a fixed
// preset. Grid topology, column-width estimation, and per-cell text
// wrapping are reused unchanged from table_render.go/table_separate.go's
// own shared helpers (resolveTableGrid, gridColumnConstraints,
// estimateColumnWidths/measureGridNaturalWidths, fillGridCellTokens,
// buildGridColumns, fillGridCellLines).
//
// Per real CSS: under collapse, the table's own padding has no effect (the
// table "does not have padding, though its cells do") — only margin
// (outside the whole grid) and the table's own border (merged into the
// grid via conflict resolution, not a separate wrapping frame) apply at
// the table level.
func (r *Engine) renderTableCollapse(n *html.Node, availWidth int, tableDecls map[string]string) (string, map[*html.Node]Rect) {
	colDecls := r.collectColDecls(n)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%" && !r.measuringNaturalWidth

	origAvailWidth := availWidth
	tableML, mlAuto := resolveMarginSide(tableDecls["margin-left"], availWidth)
	tableMR, mrAuto := resolveMarginSide(tableDecls["margin-right"], availWidth)
	availWidth = max(1, availWidth-tableML-tableMR)

	grid := r.resolveTableGrid(n)
	numCols := grid.numCols
	if numCols == 0 {
		return "", nil
	}

	// Column-width overhead: a rough estimate of how many columns the
	// eventual vertical gridlines will consume — exact activity (does this
	// boundary have a real border anywhere) isn't known until cell borders
	// are resolved below, so this uses the same representative-cell
	// approximation table_separate.go's own column allocation uses.
	colBorderW := r.separateColumnBorderOverhead(grid, colDecls, numCols)
	overhead := 0
	for _, w := range colBorderW {
		overhead += w
	}

	colsEst := r.gridColumnConstraints(grid, colDecls)
	estWidths := estimateColumnWidths(colsEst, availWidth-overhead, fullWidth)
	if estWidths == nil {
		measured := r.measureGridNaturalWidths(grid, colDecls, colsEst, 0)
		estWidths = sizeColumns(measured, availWidth-overhead, fullWidth)
	}
	fallbackCellWidth := max(1, (availWidth-overhead)/numCols)

	var captionText string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "caption" {
			captionWidth := max(1, availWidth-overhead)
			savedHint, savedHintSet := r.nestedTableWidth, r.nestedTableWidthSet
			r.nestedTableWidth, r.nestedTableWidthSet = fallbackCellWidth, true
			captionText = plainInlineText(stripANSI(r.renderInlineAcc(c, newInlineStyle(), captionWidth)))
			r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
			break
		}
	}

	r.fillGridCellTokens(grid, colDecls, estWidths, 0, fallbackCellWidth)
	cols := buildGridColumns(grid, numCols, 0)
	widths := sizeColumns(cols, availWidth-overhead, fullWidth)
	fillGridCellLines(grid, widths, 0)

	gridBox, positions := r.composeCollapsedGrid(grid, widths, colDecls, tableDecls)

	if mlAuto || mrAuto {
		remaining := origAvailWidth - gridBox.width - tableML - tableMR
		tableML, tableMR = splitAutoMargins(remaining, tableML, tableMR, mlAuto, mrAuto)
	}

	b := gridBox
	if captionText != "" {
		capLine := centerText(captionText, b.width)
		if tableDecls["caption-side"] == "bottom" {
			b.lines = append(b.lines, capLine)
		} else {
			b.lines = append([]string{capLine}, b.lines...)
			positions = mergePositions(nil, positions, 1, 0)
		}
		b.width = linesWidth(b.lines)
	}
	if tableML > 0 || tableMR > 0 {
		b = applyLineEdgesBox(b, strings.Repeat(" ", tableML), strings.Repeat(" ", tableMR))
		positions = mergePositions(nil, positions, 0, tableML)
	}
	return b.join() + "\n", positions
}

// composeCollapsedGrid builds the shared-grid content for
// border-collapse:collapse. Returned positions are relative to the grid's
// own (0,0) origin; the caller shifts them once when embedding (the same
// incremental convention used throughout the render package).
func (r *Engine) composeCollapsedGrid(g tableGrid, widths []int, colDecls []map[string]string, tableDecls map[string]string) (box, map[*html.Node]Rect) {
	numCols := g.numCols
	numRows := len(g.rows)
	if numRows == 0 || numCols == 0 {
		return box{lines: []string{""}, width: 0}, nil
	}
	cells := uniqueCells(g)

	cellEdges := make(map[*tableCell][4]edgeCandidate, len(cells))
	cellDecls := make(map[*tableCell]map[string]string, len(cells))
	for _, cell := range cells {
		decls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		cellDecls[cell] = decls
		cellEdges[cell] = r.resolveCellColEdges(cell, colDecls)
	}
	tbl, tbr, tbt, tbb, _, _, _, _ := resolveBoxBorders(tableDecls)
	tableEdges := [4]edgeCandidate{
		{tbt, edgeStyleName(tableDecls, "border-top")},
		{tbr, edgeStyleName(tableDecls, "border-right")},
		{tbb, edgeStyleName(tableDecls, "border-bottom")},
		{tbl, edgeStyleName(tableDecls, "border-left")},
	}

	// Horizontal segments: hSeg[rl][c] is the resolved border along the
	// grid-line above row rl's content (rl in 0..numRows), for column c.
	hSeg := make([][]edgeCandidate, numRows+1)
	for rl := 0; rl <= numRows; rl++ {
		hSeg[rl] = make([]edgeCandidate, numCols)
		for c := range numCols {
			var above, below *tableCell
			if rl > 0 {
				above = g.rows[rl-1][c]
			}
			if rl < numRows {
				below = g.rows[rl][c]
			}
			if above != nil && above == below {
				continue // rowspan interior - no divider through one cell's own box
			}
			var first, second edgeCandidate
			switch {
			case above == nil && below == nil:
				continue
			case above == nil:
				first, second = tableEdges[0], cellEdges[below][0]
			case below == nil:
				first, second = tableEdges[2], cellEdges[above][2]
			default:
				// Same-type (cell-vs-cell) tie: the earlier row wins,
				// confirmed against real browsers (see docs/TABLES.md) -
				// so above is passed second to win resolveSegmentBorder's
				// "ties favor second" rule.
				first, second = cellEdges[below][0], cellEdges[above][2]
			}
			hSeg[rl][c] = resolveSegmentBorder(first, second)
		}
	}

	// Vertical segments: vSeg[cl][r] is the resolved border along the
	// grid-line left of column cl's content (cl in 0..numCols), for row r.
	vSeg := make([][]edgeCandidate, numCols+1)
	for cl := 0; cl <= numCols; cl++ {
		vSeg[cl] = make([]edgeCandidate, numRows)
		for rr := range numRows {
			var left, right *tableCell
			if cl > 0 {
				left = g.rows[rr][cl-1]
			}
			if cl < numCols {
				right = g.rows[rr][cl]
			}
			if left != nil && left == right {
				continue // colspan interior
			}
			var first, second edgeCandidate
			switch {
			case left == nil && right == nil:
				continue
			case left == nil:
				first, second = tableEdges[3], cellEdges[right][3]
			case right == nil:
				first, second = tableEdges[1], cellEdges[left][1]
			default:
				// Same-type (cell-vs-cell) tie: the earlier (left) column
				// wins, confirmed against real browsers (see
				// docs/TABLES.md) - so left is passed second to win
				// resolveSegmentBorder's "ties favor second" rule.
				first, second = cellEdges[right][3], cellEdges[left][1]
			}
			vSeg[cl][rr] = resolveSegmentBorder(first, second)
		}
	}

	hasRow := make([]bool, numRows+1)
	for rl := 0; rl <= numRows; rl++ {
		for c := range numCols {
			if hSeg[rl][c].border.char != "" {
				hasRow[rl] = true
				break
			}
		}
	}
	hasCol := make([]bool, numCols+1)
	for cl := 0; cl <= numCols; cl++ {
		for rr := range numRows {
			if vSeg[cl][rr].border.char != "" {
				hasCol[cl] = true
				break
			}
		}
	}

	// Content row-height equalization: identical in shape to the original
	// (pre-border-collapse-support) row-height logic — content-only, no
	// border-line addition needed, since collapse-mode borders live on
	// their own separate gutter rows/columns rather than inside each
	// cell's own box the way separate mode's do.
	localHeight := make([]int, numRows)
	for _, cell := range cells {
		if cell.rowSpan == 1 {
			if h := len(cell.lines); h > localHeight[cell.rowStart] {
				localHeight[cell.rowStart] = h
			}
		}
	}
	for i := range localHeight {
		if localHeight[i] < 1 {
			localHeight[i] = 1
		}
	}
	for _, cell := range cells {
		if cell.rowSpan <= 1 {
			continue
		}
		need := len(cell.lines)
		have := 0
		for rr := cell.rowStart; rr < cell.rowStart+cell.rowSpan; rr++ {
			have += localHeight[rr]
		}
		if need > have {
			deficit := need - have
			base := deficit / cell.rowSpan
			rem := deficit % cell.rowSpan
			for i := range cell.rowSpan {
				add := base
				if i < rem {
					add++
				}
				localHeight[cell.rowStart+i] += add
			}
		}
	}
	mergedHeight := make(map[*tableCell]int, len(cells))
	for _, cell := range cells {
		h := 0
		for rr := cell.rowStart; rr < cell.rowStart+cell.rowSpan; rr++ {
			h += localHeight[rr]
		}
		mergedHeight[cell] = h
	}
	alignOffset := func(cell *tableCell) int {
		extra := mergedHeight[cell] - len(cell.lines)
		if extra <= 0 {
			return 0
		}
		switch cell.verticalAlign {
		case "bottom":
			return extra
		case "middle":
			return extra / 2
		default:
			return 0
		}
	}

	// Geometry: gutters (0 or 1 column/row wide, only if that boundary is
	// active anywhere) interleaved with content bands.
	colGutter := make([]int, numCols+1)
	for cl := 0; cl <= numCols; cl++ {
		if hasCol[cl] {
			colGutter[cl] = 1
		}
	}
	xOfGutter := make([]int, numCols+1)
	xOfContent := make([]int, numCols)
	x := 0
	for cl := 0; cl <= numCols; cl++ {
		xOfGutter[cl] = x
		x += colGutter[cl]
		if cl < numCols {
			xOfContent[cl] = x
			x += widths[cl]
		}
	}
	totalWidth := x

	rowGutter := make([]int, numRows+1)
	for rl := 0; rl <= numRows; rl++ {
		if hasRow[rl] {
			rowGutter[rl] = 1
		}
	}
	yOfGutter := make([]int, numRows+1)
	yOfContent := make([]int, numRows)
	y := 0
	for rl := 0; rl <= numRows; rl++ {
		yOfGutter[rl] = y
		y += rowGutter[rl]
		if rl < numRows {
			yOfContent[rl] = y
			y += localHeight[rl]
		}
	}
	totalHeight := y

	lines := make([]string, totalHeight)
	for i := range lines {
		lines[i] = strings.Repeat(" ", totalWidth)
	}

	// Junction/segment glyph at vertex (rl, cl).
	vertexGlyph := func(rl, cl int) string {
		up, down, left, right := false, false, false, false
		var arms []edgeCandidate
		if rl > 0 && vSeg[cl][rl-1].border.char != "" {
			up = true
			arms = append(arms, vSeg[cl][rl-1])
		}
		if rl < numRows && vSeg[cl][rl].border.char != "" {
			down = true
			arms = append(arms, vSeg[cl][rl])
		}
		if cl > 0 && hSeg[rl][cl-1].border.char != "" {
			left = true
			arms = append(arms, hSeg[rl][cl-1])
		}
		if cl < numCols && hSeg[rl][cl].border.char != "" {
			right = true
			arms = append(arms, hSeg[rl][cl])
		}
		if len(arms) == 0 {
			return ""
		}
		winner := arms[0]
		for _, a := range arms[1:] {
			if stylePrecedence[a.style] > stylePrecedence[winner.style] {
				winner = a
			}
		}
		mask := 0
		if up {
			mask |= jUp
		}
		if down {
			mask |= jDown
		}
		if left {
			mask |= jLeft
		}
		if right {
			mask |= jRight
		}
		glyph := ""
		// Tier 1: a cell's own override, at its own corner - most specific,
		// matching "cell beats table" for edges. Up to 4 cells can share an
		// interior vertex (the quadrants around it); gathered in row-major
		// order (above-left, above-right, below-left, below-right), which
		// already matches "earlier wins" (verified against real browsers
		// for edges - reused here, not reinvented), so the first non-empty
		// override found wins. De-duplicated by pointer via a linear scan
		// (at most 4 candidates, not worth a map allocation per vertex)
		// since a colspan/rowspan cell can occupy two quadrants at once -
		// and gated on (rl, cl) actually being one of that cell's own 4
		// corner coordinates: occupying a quadrant isn't enough on its own,
		// since a spanning cell also touches interior points along its own
		// edges (e.g. a colspan-2 cell's own bottom edge, at the column
		// boundary it spans over) that aren't corners of its box at all,
		// and its override must not leak onto those.
		var quadrants [4]*tableCell
		if rl > 0 && cl > 0 {
			quadrants[0] = g.rows[rl-1][cl-1]
		}
		if rl > 0 && cl < numCols {
			quadrants[1] = g.rows[rl-1][cl]
		}
		if rl < numRows && cl > 0 {
			quadrants[2] = g.rows[rl][cl-1]
		}
		if rl < numRows && cl < numCols {
			quadrants[3] = g.rows[rl][cl]
		}
	quadrantLoop:
		for i, cell := range quadrants {
			if cell == nil {
				continue
			}
			for _, prior := range quadrants[:i] {
				if prior == cell {
					continue quadrantLoop
				}
			}
			ownCorner := (rl == cell.rowStart || rl == cell.rowStart+cell.rowSpan) &&
				(cl == cell.colStart || cl == cell.colStart+cell.colSpan)
			if !ownCorner {
				continue
			}
			if v := shapeOverride(cellDecls[cell], cellShapeProps, mask); v != "" {
				glyph = v
				break
			}
		}
		// Tier 2: border-*-corner, only at the table's own true 4 outer
		// corners, gated on the mask actually being that 2-arm corner
		// shape (never a dead-end/absent at these positions).
		if glyph == "" {
			switch {
			case rl == 0 && cl == 0 && mask == jDown|jRight:
				glyph = shapeOverride(tableDecls, cellShapeProps, mask)
			case rl == 0 && cl == numCols && mask == jDown|jLeft:
				glyph = shapeOverride(tableDecls, cellShapeProps, mask)
			case rl == numRows && cl == 0 && mask == jUp|jRight:
				glyph = shapeOverride(tableDecls, cellShapeProps, mask)
			case rl == numRows && cl == numCols && mask == jUp|jLeft:
				glyph = shapeOverride(tableDecls, cellShapeProps, mask)
			}
		}
		// Tier 3: border-*-junction, a shape-class default wherever this
		// exact arm combination occurs anywhere in the grid.
		if glyph == "" {
			glyph = shapeOverride(tableDecls, namedJunctionProps, mask)
		}
		// Tier 4: computed from whichever style actually won the arms.
		if glyph == "" {
			glyph = junctionGlyph(winner.style, up, down, left, right)
		}
		return makePainter(winner.border.color, r.profile)(glyph)
	}

	// Draw horizontal gutter rows.
	for rl := 0; rl <= numRows; rl++ {
		if !hasRow[rl] {
			continue
		}
		row := yOfGutter[rl]
		line := lines[row]
		for cl := 0; cl <= numCols; cl++ {
			if colGutter[cl] > 0 {
				line = spliceColumns(line, xOfGutter[cl], 1, vertexGlyph(rl, cl))
			}
			if cl < numCols {
				seg := hSeg[rl][cl]
				content := strings.Repeat(" ", widths[cl])
				if seg.border.char != "" {
					content = makePainter(seg.border.color, r.profile)(strings.Repeat(seg.border.char, widths[cl]))
				}
				line = spliceColumns(line, xOfContent[cl], widths[cl], content)
			}
		}
		lines[row] = line
	}

	var positions map[*html.Node]Rect
	consumed := make(map[*tableCell]int, len(cells))
	for rowIdx := range numRows {
		for lineIdx := range localHeight[rowIdx] {
			row := yOfContent[rowIdx] + lineIdx
			line := lines[row]
			for c := range numCols {
				interior := c > 0 && g.rows[rowIdx][c-1] != nil && g.rows[rowIdx][c-1] == g.rows[rowIdx][c]
				if colGutter[c] > 0 && !interior {
					vseg := vSeg[c][rowIdx]
					content := " "
					if vseg.border.char != "" {
						content = makePainter(vseg.border.color, r.profile)(vseg.border.char)
					}
					line = spliceColumns(line, xOfGutter[c], 1, content)
				}
				cell := g.rows[rowIdx][c]
				if cell == nil || cell.colStart != c {
					continue // short row, or mid-colspan continuation
				}
				spanEnd := cell.colStart + cell.colSpan
				spanWidth := xOfGutter[min(spanEnd, numCols)] - xOfContent[c]
				if spanEnd >= numCols {
					spanWidth = totalWidth - xOfContent[c]
				}
				pl, pr, cw := clampCellPadding(spanWidth, cell.paddingLeft, cell.paddingRight)
				absPos := consumed[cell] + lineIdx
				var text string
				if idx := absPos - alignOffset(cell); idx >= 0 && idx < len(cell.lines) {
					text = cell.lines[idx]
				}
				rendered := cell.cellStyle.render(alignLines(text, cell.textAlign, cw), r.profile)
				if pl > 0 {
					rendered = strings.Repeat(" ", pl) + rendered
				}
				if pr > 0 {
					rendered += strings.Repeat(" ", pr)
				}
				line = spliceColumns(line, xOfContent[c], spanWidth, rendered)
			}
			if colGutter[numCols] > 0 {
				vseg := vSeg[numCols][rowIdx]
				content := " "
				if vseg.border.char != "" {
					content = makePainter(vseg.border.color, r.profile)(vseg.border.char)
				}
				line = spliceColumns(line, xOfGutter[numCols], 1, content)
			}
			lines[row] = line
		}

		seenThisRow := make(map[*tableCell]bool)
		for c := range numCols {
			cell := g.rows[rowIdx][c]
			if cell == nil || seenThisRow[cell] {
				continue
			}
			seenThisRow[cell] = true
			if cell.rowStart == rowIdx && len(cell.positions) > 0 {
				pl, _, _ := clampCellPadding(xOfGutter[min(cell.colStart+cell.colSpan, numCols)]-xOfContent[cell.colStart], cell.paddingLeft, cell.paddingRight)
				positions = mergePositions(positions, cell.positions, yOfContent[rowIdx]+alignOffset(cell), xOfContent[cell.colStart]+pl)
			}
			consumed[cell] += localHeight[rowIdx]
		}
	}

	return box{lines: lines, width: totalWidth}, positions
}
