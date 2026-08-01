package render

import (
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

type tableCell struct {
	node          *html.Node // the <td>/<th> element itself
	tokens        []wrapToken
	textAlign     string
	cellStyle     inlineStyle
	constraints   colConstraints
	textOverflow  string
	noWrap        bool
	paddingLeft   int
	paddingRight  int
	paddingTop    int
	paddingBottom int
	verticalAlign string
	lines         []string
	// positions holds this cell's own trackable descendants' Rects, relative
	// to cells[i].lines[0], row 0 and col 0, meaning after padding-top has
	// been folded in below but before this cell is placed into a row or
	// table. See renderTableBody for how these get shifted into the table's
	// own coordinate space.
	positions map[*html.Node]Rect
	// colSpan/rowSpan (always >=1) and rowStart/colStart (this cell's own
	// anchor position in the grid) come from resolveTableGrid; a cell with
	// colSpan==rowSpan==1 behaves exactly as every cell did before colspan/
	// rowspan support existed.
	colSpan, rowSpan   int
	rowStart, colStart int
}

// tableGrid is the resolved row and column structure of a <table>,
// accounting for colspan and rowspan. rows[r][c] points to the *tableCell
// occupying that slot: the same pointer appears at every (row,col) a spanning
// cell covers, and nil means no cell occupies that slot, as in a short row.
//
// headerRow is the index of the single row treated as the table header, or -1
// if none, matching the existing thead and first-all-<th>-row rules. It is
// used only to pick a representative cell per column for border-overhead
// sizing (see separateColumnBorderOverhead). A header cell's own rowspan is
// otherwise resolved exactly like any other cell's, including merging into
// data rows.
type tableGrid struct {
	numCols   int
	rows      [][]*tableCell
	headerRow int
}

// collectColDecls scans direct <colgroup> children of a <table> node and
// returns a slice of declaration maps, one entry per column position, expanded
// by span. A <colgroup> with <col> children uses per-col decls, where col
// overrides the colgroup base. A <colgroup> with no <col> children applies its
// own decls across its span.
func (r *Engine) collectColDecls(table *html.Node) []map[string]string {
	var result []map[string]string
	for cg := table.FirstChild; cg != nil; cg = cg.NextSibling {
		if cg.Type != html.ElementNode || cg.Data != "colgroup" {
			continue
		}
		cgDecls := r.directDecls(cg)
		hasColChildren := false
		for col := cg.FirstChild; col != nil; col = col.NextSibling {
			if col.Type == html.ElementNode && col.Data == "col" {
				hasColChildren = true
				break
			}
		}
		if !hasColChildren {
			// <colgroup span="N"> with no <col> children.
			span := 1
			if s, err := strconv.Atoi(nodeAttr(cg, "span")); err == nil && s > 1 {
				span = s
			}
			for i := 0; i < span; i++ {
				result = append(result, cgDecls)
			}
			continue
		}
		// <colgroup> with <col> children.
		for col := cg.FirstChild; col != nil; col = col.NextSibling {
			if col.Type != html.ElementNode || col.Data != "col" {
				continue
			}
			span := 1
			if s, err := strconv.Atoi(nodeAttr(col, "span")); err == nil && s > 1 {
				span = s
			}
			colDecls := r.directDecls(col)
			// Merge: colgroup is the base, col overrides.
			merged := colDecls
			if len(cgDecls) > 0 {
				merged = make(map[string]string, len(cgDecls)+len(colDecls))
				for k, v := range cgDecls {
					merged[k] = v
				}
				for k, v := range colDecls {
					merged[k] = v
				}
			}
			for i := 0; i < span; i++ {
				result = append(result, merged)
			}
		}
	}
	return result
}

// parseSpanAttr parses a colspan or rowspan attribute value. Unparsable, or
// less than 1, resolves to 1, the HTML-spec default, and the result is clamped
// to max, matching the HTML spec's own limits of 1000 for colspan and 65534
// for rowspan.
func parseSpanAttr(raw string, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return 1
	}
	if n > max {
		return max
	}
	return n
}

// resolveTableGrid walks a table's rows once, building its full grid
// topology — column count, per-cell anchor position, and colspan and rowspan
// occupancy — without touching CSS or cell content. Every other pass, whether
// CSS-constraint gathering, natural-width measurement, or final rendering,
// iterates this one resolved grid instead of separately re-deriving column
// indices from the DOM, which keeps colspan and rowspan bookkeeping
// consistent across all of them.
//
// rowspan="0", meaning "span to the end", is resolved against the whole
// table's remaining rows rather than being scoped per thead, tbody, or tfoot
// section as the spec specifies. That is a documented simplification for a
// rare case.
func (r *Engine) resolveTableGrid(n *html.Node) tableGrid {
	type trInfo struct {
		node    *html.Node
		inTHead bool
		inTFoot bool
	}
	var trs []trInfo
	var walkTR func(*html.Node, bool, bool)
	walkTR = func(n *html.Node, inTHead, inTFoot bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "thead":
				walkTR(c, true, false)
			case "tbody":
				walkTR(c, false, false)
			case "tfoot":
				walkTR(c, false, true)
			case "tr":
				if r.resolveDecls(c)["display"] == "none" {
					continue
				}
				trs = append(trs, trInfo{c, inTHead, inTFoot})
			}
		}
	}
	walkTR(n, false, false)

	numRows := len(trs)
	g := tableGrid{rows: make([][]*tableCell, numRows), headerRow: -1}

	type carryEntry struct {
		remaining int
		cell      *tableCell
	}
	var carry []carryEntry

	ensureCols := func(need int) {
		for need > g.numCols {
			g.numCols++
			for rr := range g.rows {
				if g.rows[rr] != nil {
					g.rows[rr] = append(g.rows[rr], nil)
				}
			}
			carry = append(carry, carryEntry{})
		}
	}

	for ri, tr := range trs {
		if g.rows[ri] == nil {
			g.rows[ri] = make([]*tableCell, g.numCols)
		}
		for c := 0; c < g.numCols; c++ {
			if carry[c].remaining > 0 {
				g.rows[ri][c] = carry[c].cell
				carry[c].remaining--
			}
		}

		// Header-row detection: any row inside <thead> is a header candidate.
		// Without <thead>, the first all-<th> row anywhere outside <tfoot>
		// is an implicit header. Only the first candidate encountered becomes
		// THE header row, used solely as a representative-cell lookup for
		// column border-overhead sizing (see separateColumnBorderOverhead).
		allTH := true
		for td := tr.node.FirstChild; td != nil; td = td.NextSibling {
			if td.Type == html.ElementNode && td.Data == "td" {
				allTH = false
				break
			}
		}
		isHeaderRow := tr.inTHead || (!tr.inTHead && !tr.inTFoot && allTH)
		if isHeaderRow && g.headerRow < 0 {
			g.headerRow = ri
		}

		ci := 0
		for td := tr.node.FirstChild; td != nil; td = td.NextSibling {
			if td.Type != html.ElementNode || (td.Data != "th" && td.Data != "td") {
				continue
			}
			if r.resolveDecls(td)["display"] == "none" {
				continue
			}
			for ci < g.numCols && g.rows[ri][ci] != nil {
				ci++
			}
			colSpan := parseSpanAttr(nodeAttr(td, "colspan"), 1000)
			var rowSpan int
			if raw := strings.TrimSpace(nodeAttr(td, "rowspan")); raw == "0" {
				rowSpan = numRows - ri
			} else {
				rowSpan = parseSpanAttr(raw, 65534)
			}
			if rowSpan < 1 {
				rowSpan = 1
			}
			// A rowspan can never reach past the table's actual last row -
			// unlike columns (which ensureCols grows to fit any colspan),
			// rows are fixed at the real <tr> count, so an over-large
			// explicit value (e.g. a typo'd rowspan="100" in a 2-row table)
			// must be clamped here, the same way a real browser silently
			// clamps it, rather than left to blow past every rows-indexed
			// slice later on.
			if maxSpan := numRows - ri; rowSpan > maxSpan {
				rowSpan = maxSpan
			}
			ensureCols(ci + colSpan)
			cell := &tableCell{node: td, colSpan: colSpan, rowSpan: rowSpan, rowStart: ri, colStart: ci}
			for cc := ci; cc < ci+colSpan; cc++ {
				g.rows[ri][cc] = cell
			}
			if rowSpan > 1 {
				for cc := ci; cc < ci+colSpan; cc++ {
					carry[cc] = carryEntry{remaining: rowSpan - 1, cell: cell}
				}
			}
			ci += colSpan
		}
	}
	return g
}

// uniqueCells returns every distinct *tableCell in the grid exactly once, in
// (rowStart, colStart) order. Its first appearance scanning row-major, top to
// bottom and left to right, is always its own anchor, since a cell never
// appears before its own rowStart and colStart.
func uniqueCells(g tableGrid) []*tableCell {
	var out []*tableCell
	seen := make(map[*tableCell]bool)
	for _, row := range g.rows {
		for _, c := range row {
			if c == nil || seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// mergedCellDecls resolves td's own CSS and inline declarations, with any
// <col> or <colgroup> declarations at column ci merged in as a
// lower-priority base. For a spanning cell, ci is its leftmost colStart
// column: a single spanning cell can only take one column's <col> style, so
// the leftmost one wins. That is a rare, low-stakes simplification, reached
// only when <colgroup> or <col> and colspan are combined.
func (r *Engine) mergedCellDecls(td *html.Node, colDecls []map[string]string, ci int) map[string]string {
	tdDecls := r.resolveDecls(td)
	if ci < len(colDecls) && len(colDecls[ci]) > 0 {
		merged := make(map[string]string, len(colDecls[ci])+len(tdDecls))
		for k, v := range colDecls[ci] {
			merged[k] = v
		}
		for k, v := range tdDecls {
			merged[k] = v
		}
		return merged
	}
	return tdDecls
}

// mergeColConstraints folds dc into dst, keeping the first fixed or percent
// value seen and the tightest min and max. It is the same merge rule whether
// the source is a single unconstrained pass or many rows' worth of cells.
func mergeColConstraints(dst *colConstraints, dc colConstraints) {
	if dst.fixed == 0 && dc.fixed > 0 {
		dst.fixed = dc.fixed
	}
	if dst.percent == 0 && dc.percent > 0 {
		dst.percent = dc.percent
	}
	if dc.minWidth > dst.minWidth {
		dst.minWidth = dc.minWidth
	}
	if dc.minPercent > dst.minPercent {
		dst.minPercent = dc.minPercent
	}
	if dc.maxWidth > 0 && (dst.maxWidth == 0 || dc.maxWidth < dst.maxWidth) {
		dst.maxWidth = dc.maxWidth
	}
	if dc.maxPercent > 0 && (dst.maxPercent == 0 || dc.maxPercent < dst.maxPercent) {
		dst.maxPercent = dc.maxPercent
	}
}

// distributeSpanDeficit ensures a spanning cell's spanned columns
// collectively have at least enough natural width for its content. If the
// sum of the columns it spans, plus the interior separators it reclaims since
// no separator is drawn through a spanned cell, falls short of the cell's own
// measured natural width, the shortfall is added to those columns' natural
// field, spread evenly with the remainder going to the first columns. This
// approximates CSS2.1's fallback for spanning cells whose columns don't
// otherwise have enough room.
func distributeSpanDeficit(cols []colConstraints, colStart, colSpan, sepW, cellNatural int) {
	if colSpan <= 1 {
		return
	}
	spanned := (colSpan - 1) * sepW
	for i := 0; i < colSpan; i++ {
		spanned += cols[colStart+i].natural
	}
	if cellNatural <= spanned {
		return
	}
	deficit := cellNatural - spanned
	base := deficit / colSpan
	rem := deficit % colSpan
	for i := 0; i < colSpan; i++ {
		add := base
		if i < rem {
			add++
		}
		cols[colStart+i].natural += add
	}
}

// gridColumnConstraints gathers per-column CSS and HTML width constraints —
// fixed, percent, min, and max — from a resolved grid, without rendering any
// cell content. It reads only colSpan==1 cells, since a spanning cell's
// constraints don't map to a single column unambiguously. Such a cell's
// content instead feeds into distributeSpanDeficit once natural widths are
// known.
func (r *Engine) gridColumnConstraints(g tableGrid, colDecls []map[string]string) []colConstraints {
	cols := make([]colConstraints, g.numCols)
	for _, cell := range uniqueCells(g) {
		if cell.colSpan != 1 {
			continue
		}
		tdDecls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		mergeColConstraints(&cols[cell.colStart], r.cellConstraints(tdDecls))
	}
	return cols
}

// measureCellNaturalWidth renders td's content once, at an effectively
// unbounded width, only to measure how wide it would be if never forced to
// wrap, then discards that trial render. measuringNaturalWidth suppresses
// width:100% expansion in any nested <table> so its own shrink-to-fit natural
// width is measured instead of it stretching to fill the huge trial budget.
func (r *Engine) measureCellNaturalWidth(td *html.Node) int {
	savedMeasuring := r.measuringNaturalWidth
	savedHint, savedHintSet := r.nestedTableWidth, r.nestedTableWidthSet
	r.measuringNaturalWidth = true
	r.nestedTableWidth, r.nestedTableWidthSet = naturalWidthCap, true
	tokens := r.renderInlineAccTokens(td, newInlineStyle(), naturalWidthCap)
	r.measuringNaturalWidth = savedMeasuring
	r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
	return tokensNaturalWidth(tokens)
}

// measureTableWidth computes a <table>'s own natural, shrink-to-fit width,
// meaning its final rendered box width if never constrained, without producing
// any rendered content. It mirrors renderTable's own column-sizing math —
// grid topology, CSS constraints, and, when needed, content measurement — but
// stops there. It never calls fillGridCellTokens or renderTableBody, so it
// never commits to a full "final" render of this table's cells.
//
// This exists specifically for inline.go's nested-<table> handling under
// measuringNaturalWidth (see measureCellNaturalWidth). Without it, measuring
// an ancestor's natural width would mean fully rendering every descendant
// table too, via a real renderTable call, only to discard the string and
// keep just its width. And since that descendant table's own renderTable
// call would, if it also needed a measurement pass, do the same thing one
// level further down, the redundant "measure AND fully render" work
// compounds at every nesting level, making measurement exponential in
// nesting depth instead of linear. Real-world deeply-nested table markup,
// such as HTML email templates, hits exactly this; see the regression test
// covering it.
func (r *Engine) measureTableWidth(n *html.Node) int {
	tableDecls := r.resolveDecls(n)
	colDecls := r.collectColDecls(n)
	grid := r.resolveTableGrid(n)
	if grid.numCols == 0 {
		return 0
	}
	// Border overhead, resolved per the mode that applies. Collapse's count is
	// one column per *active grid line*, since adjacent cells share theirs and
	// the table's own border becomes the outermost two. That is a different
	// number from separate's per-cell left-plus-right sum, in both directions;
	// see renderTableCollapse. Neither mode's count depends on column widths,
	// so both are exact here rather than estimates, and this stays a
	// measurement: resolving declarations over an already-built grid is what
	// this function was doing anyway, not the full cell render it exists to
	// avoid.
	spacingX := parseSpacingLen(tableDecls["border-spacing-x"])
	overhead := (grid.numCols + 1) * spacingX
	if tableDecls["border-collapse"] == "collapse" {
		overhead += r.resolveCollapsedBorders(grid, colDecls, tableDecls).colOverhead()
	} else {
		for _, w := range r.separateColumnBorderOverhead(grid, colDecls, grid.numCols) {
			overhead += w
		}
		tbl, tbr, _, _, _, _, _, _ := resolveBoxBorders(tableDecls)
		if tbl.char != "" {
			overhead += textcell.Width(tbl.char)
		}
		if tbr.char != "" {
			overhead += textcell.Width(tbr.char)
		}
	}
	colsEst := r.gridColumnConstraints(grid, colDecls)
	measured := r.measureGridNaturalWidths(grid, colDecls, colsEst, spacingX)
	widths := sizeColumns(measured, naturalWidthCap, false)

	total := overhead
	for _, w := range widths {
		total += w
	}
	tablePL := parsePaddingLen(tableDecls["padding-left"])
	tablePR := parsePaddingLen(tableDecls["padding-right"])
	tableML, _ := resolveMarginSide(tableDecls["margin-left"], naturalWidthCap)
	tableMR, _ := resolveMarginSide(tableDecls["margin-right"], naturalWidthCap)
	return total + tablePL + tablePR + tableML + tableMR
}

// measureGridNaturalWidths fills in cols' natural field with each column's
// real unwrapped content width, for the case where estimateColumnWidths
// couldn't produce a usable estimate from CSS constraints alone, meaning two
// or more unconstrained flex columns. It renders each cell once, via
// measureCellNaturalWidth, to measure its content instead of only reading
// declared constraints. colSpan==1 cells contribute directly to their own
// column, and colSpan>1 cells are folded in afterward via
// distributeSpanDeficit. Returns a copy of cols with natural filled in, and
// does not mutate the input.
func (r *Engine) measureGridNaturalWidths(g tableGrid, colDecls []map[string]string, cols []colConstraints, sepW int) []colConstraints {
	out := append([]colConstraints(nil), cols...)
	cells := uniqueCells(g)
	for _, cell := range cells {
		if cell.colSpan != 1 {
			continue
		}
		tdDecls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		pl := parsePaddingLen(tdDecls["padding-left"])
		pr := parsePaddingLen(tdDecls["padding-right"])
		if w := r.measureCellNaturalWidth(cell.node) + pl + pr; w > out[cell.colStart].natural {
			out[cell.colStart].natural = w
		}
	}
	for _, cell := range cells {
		if cell.colSpan <= 1 {
			continue
		}
		tdDecls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		pl := parsePaddingLen(tdDecls["padding-left"])
		pr := parsePaddingLen(tdDecls["padding-right"])
		w := r.measureCellNaturalWidth(cell.node) + pl + pr
		distributeSpanDeficit(out, cell.colStart, cell.colSpan, sepW, w)
	}
	return out
}

// estimateColumnWidths computes a best-effort final column width per column
// before any cell content is rendered, using only CSS and HTML constraints,
// with no natural content width, which isn't known yet. This exactly matches
// the real final sizeColumns pass whenever every column is either fixed or
// percent constrained, or there is at most one unconstrained "flex" column.
// In both cases the real pass's use of natural width ends up not mattering: a
// percent or fixed column ignores natural entirely, and a lone flex column
// always absorbs whatever space is left over regardless of its own natural
// width. With two or more unconstrained flex columns, natural width does
// affect the real split, so this returns nil and callers fall back to a
// genuine measurement pass for that rarer case.
func estimateColumnWidths(cols []colConstraints, contentWidth int, fullWidth bool) []int {
	flexCount := 0
	for _, c := range cols {
		if c.fixed == 0 && c.percent == 0 {
			flexCount++
		}
	}
	if flexCount > 1 {
		return nil
	}
	if flexCount == 1 && !fullWidth {
		// A lone flex column only absorbs the remaining space regardless of
		// its own natural width when the table is fullWidth, meaning forced
		// expansion. Without fullWidth its final width is driven by natural
		// content width, which isn't known yet, so don't guess.
		return nil
	}
	return sizeColumns(cols, contentWidth, fullWidth)
}

// fillGridCellTokens renders every unique cell's content once, using its
// span-aware final budget as the wrap width. That budget is estWidths summed
// across the columns the cell spans, plus the interior separators it reclaims
// since no separator is drawn through a spanned cell. Nested content, such as
// a nested <table>, is therefore only ever rendered at its real final budget,
// never prematurely committed at a wrong one. This also resolves each cell's
// other CSS-derived fields, including padding and text-align.
func (r *Engine) fillGridCellTokens(g tableGrid, colDecls []map[string]string, estWidths []int, sepW, fallbackCellWidth int) {
	for _, cell := range uniqueCells(g) {
		tdDecls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		pl := parsePaddingLen(tdDecls["padding-left"])
		pr := parsePaddingLen(tdDecls["padding-right"])
		pt := parsePaddingLen(tdDecls["padding-top"])
		pb := parsePaddingLen(tdDecls["padding-bottom"])

		end := cell.colStart + cell.colSpan
		var cellBudget int
		if end <= len(estWidths) {
			sumW := (cell.colSpan - 1) * sepW
			for i := cell.colStart; i < end; i++ {
				sumW += estWidths[i]
			}
			cellBudget = max(1, sumW-pl-pr)
		} else {
			sumW := fallbackCellWidth*cell.colSpan + (cell.colSpan-1)*sepW
			cellBudget = max(1, sumW-pl-pr)
		}

		savedHint, savedHintSet := r.nestedTableWidth, r.nestedTableWidthSet
		r.nestedTableWidth, r.nestedTableWidthSet = cellBudget, true
		cellTokens := r.renderInlineAccTokens(cell.node, newInlineStyle(), cellBudget)
		r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet

		// A leading or trailing brk, from a first or last block child's
		// margin or a nested table rendered via pushBox, is only ever
		// meaningful as structural separation between siblings. At either
		// edge of the cell's own content it isn't real content and would
		// otherwise wrap to a spurious blank line at the top or bottom of
		// the cell, since table cells don't collapse margins with their
		// content the way a block container does.
		cellTokens = cellTokens[leadingBreaks(cellTokens):]
		for len(cellTokens) > 0 && cellTokens[len(cellTokens)-1].brk {
			cellTokens = cellTokens[:len(cellTokens)-1]
		}
		if isHiddenVisibility(tdDecls["visibility"]) {
			// Blank the content but keep its line/box structure (e.g. a
			// <br> or nested block inside the cell) so the cell still
			// occupies the same space.
			cellTokens = blankVisibleContentTokens(cellTokens)
		}

		cell.tokens = cellTokens
		cell.textAlign = tdDecls["text-align"]
		cell.cellStyle = extractInlineStyle(tdDecls)
		cell.constraints = r.cellConstraints(tdDecls)
		cell.textOverflow = textOverflowSuffix(tdDecls["text-overflow"])
		cell.noWrap = tdDecls["white-space"] == "nowrap"
		cell.paddingLeft = pl
		cell.paddingRight = pr
		cell.paddingTop = pt
		cell.paddingBottom = pb
		cell.verticalAlign = tdDecls["vertical-align"]
	}
}

// renderTable renders a <table> node using the custom table engine.
// availWidth is the width available at the table's rendering context (the
// full renderer width at the top level, or the containing cell's content
// width for a nested table).
func (r *Engine) renderTable(n *html.Node, availWidth int) (string, map[*html.Node]Rect) {
	tableDecls := r.resolveDecls(n)
	if tableDecls["border-collapse"] == "collapse" {
		return r.renderTableCollapse(n, availWidth, tableDecls)
	}
	// Real CSS's border-collapse initial value is "separate", so unset and
	// explicit "separate" are the same thing, not two branches. Separate's
	// own model (table_separate.go) is already spec-correct for the
	// borderless-by-default case, since it has no fallback border logic
	// of its own. See docs/TABLES.md.
	return r.renderTableSeparate(n, availWidth, tableDecls)
}

// nbsp is U+00A0, the non-breaking space. A real &nbsp; HTML entity decodes
// to this rune and survives rendering as a distinct character, since
// normalizeWhiteSpace and plainInlineText only touch plain ASCII space.
// Render's final pass normalizes it to a plain space in the returned string,
// since terminals don't distinguish breaking from non-breaking spaces.
const nbsp = " "

// buildGridColumns rebuilds each column's constraints from its cells' final,
// already-rendered tokens, which give natural width, and from per-cell CSS
// constraints. colSpan==1 cells contribute directly to their own column, and
// colSpan>1 cells are folded in afterward via distributeSpanDeficit.
func buildGridColumns(g tableGrid, numCols, sepW int) []colConstraints {
	cols := make([]colConstraints, numCols)
	cells := uniqueCells(g)
	for _, cell := range cells {
		if cell.colSpan != 1 {
			continue
		}
		mergeColConstraints(&cols[cell.colStart], cell.constraints)
		if w := tokensNaturalWidth(cell.tokens) + cell.paddingLeft + cell.paddingRight; w > cols[cell.colStart].natural {
			cols[cell.colStart].natural = w
		}
	}
	for _, cell := range cells {
		if cell.colSpan <= 1 {
			continue
		}
		w := tokensNaturalWidth(cell.tokens) + cell.paddingLeft + cell.paddingRight
		distributeSpanDeficit(cols, cell.colStart, cell.colSpan, sepW, w)
	}
	return cols
}

// fillGridCellLines wraps every unique cell's tokens into final lines, using
// its span-aware content width: the sum of the final widths of every column
// it spans, plus the interior separators it reclaims (no separator is drawn
// through a spanned cell).
func fillGridCellLines(g tableGrid, widths []int, sepW int) {
	for _, cell := range uniqueCells(g) {
		end := cell.colStart + cell.colSpan
		spanW := (cell.colSpan - 1) * sepW
		for i := cell.colStart; i < end && i < len(widths); i++ {
			spanW += widths[i]
		}
		_, _, contentW := clampCellPadding(spanW, cell.paddingLeft, cell.paddingRight)
		var b box
		var positions map[*html.Node]Rect
		if cell.noWrap {
			// Not wrapped, so a plain trailing space from source text, as in
			// "<td>hi </td>", won't get dropped by tokenization the way
			// wrapping does. Trim it explicitly, per line, matching
			// plainInlineText's historical role here. naturalWidthCap (see
			// wraptoken.go) keeps this the token-domain equivalent of the old
			// flatten-to-string path, where only structural brk and box breaks
			// start a new line and no width-driven wrapping happens, while
			// still going through wordWrapTokens so cell.tokens' own node
			// positions are produced instead of discarded.
			b, positions = wordWrapTokens(cell.tokens, naturalWidthCap, "", 0, false)
			for j, line := range b.lines {
				b.lines[j] = strings.TrimRight(line, " ")
			}
		} else {
			b, positions = wordWrapTokens(cell.tokens, contentW, "break-word", 0, false)
		}
		// text-overflow only applies in nowrap mode (CSS.md: "Ignored when
		// white-space: normal"); a wrapped line that still overflows contentW
		// despite word-wrap (an unbreakable embedded box, e.g.) is clipped
		// with no marker, just to keep the column boundary intact.
		suffix := ""
		if cell.noWrap {
			suffix = cell.textOverflow
		}
		cell.lines = nil
		for _, line := range b.lines {
			cell.lines = append(cell.lines, textcell.TruncateToWidth(line, contentW, suffix))
		}
		if len(cell.lines) == 0 {
			cell.lines = []string{""}
		} else {
			cell.positions = positions
		}
		if pt := cell.paddingTop; pt > 0 {
			blank := make([]string, pt, pt+len(cell.lines))
			cell.lines = append(blank, cell.lines...)
			if len(cell.positions) > 0 {
				cell.positions = mergePositions(nil, cell.positions, pt, 0)
			}
		}
		if pb := cell.paddingBottom; pb > 0 {
			cell.lines = append(cell.lines, make([]string, pb)...)
		}
	}
}
