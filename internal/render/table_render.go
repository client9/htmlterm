package render

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
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
	// to cells[i].lines[0] (row 0, col 0) — i.e. after padding-top has been
	// folded in below, but before this cell is placed into a row/table. See
	// renderTableBody for how these get shifted into the table's own
	// coordinate space.
	positions map[*html.Node]Rect
	// colSpan/rowSpan (always >=1) and rowStart/colStart (this cell's own
	// anchor position in the grid) come from resolveTableGrid; a cell with
	// colSpan==rowSpan==1 behaves exactly as every cell did before colspan/
	// rowspan support existed.
	colSpan, rowSpan   int
	rowStart, colStart int
}

// tableGrid is the resolved row/column structure of a <table>, accounting
// for colspan/rowspan. rows[r][c] points to the *tableCell occupying that
// slot — the same pointer appears at every (row,col) a spanning cell covers,
// and nil means no cell occupies that slot (a short row). headerRow is the
// index of the single row treated as the table header (-1 if none),
// matching the existing thead/first-all-<th>-row rules — this engine only
// ever recognizes one header row, so a header-row cell's rowspan is clamped
// to 1 in resolveTableGrid (it can't merge into data rows).
type tableGrid struct {
	numCols   int
	rows      [][]*tableCell
	headerRow int
}

// collectColDecls scans direct <colgroup> children of a <table> node and returns
// a slice of declaration maps, one entry per column position (expanded by span).
// A <colgroup> with <col> children uses per-col decls (col overrides colgroup base).
// A <colgroup> with no <col> children applies its own decls across its span.
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

// parseSpanAttr parses a colspan/rowspan attribute value: unparsable or <1
// resolves to 1 (the HTML-spec default), clamped to max (matching the HTML
// spec's own limits: 1000 for colspan, 65534 for rowspan).
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
// topology (column count, per-cell anchor position, colspan/rowspan
// occupancy) without touching CSS or cell content. Every other pass
// (CSS-constraint gathering, natural-width measurement, final rendering)
// iterates this one resolved grid instead of separately re-deriving column
// indices from the DOM — keeping colspan/rowspan bookkeeping consistent
// across all of them.
//
// rowspan="0" ("span to the end") is resolved against the whole table's
// remaining rows, not scoped per thead/tbody/tfoot section as the spec
// technically specifies — a documented simplification for a rare case.
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

		// Header-row detection mirrors the original single-header-row rule
		// exactly: any row inside <thead> is a header candidate; without
		// <thead>, the first all-<th> row anywhere (not in <tfoot>) is an
		// implicit header. Only the first candidate encountered becomes THE
		// header row. Determined via a tag-name-only lookahead before cell
		// placement, so a rowspan starting in that row can be clamped to 1
		// (a header can't merge into data rows).
		allTH := true
		for td := tr.node.FirstChild; td != nil; td = td.NextSibling {
			if td.Type == html.ElementNode && td.Data == "td" {
				allTH = false
				break
			}
		}
		isHeaderRow := tr.inTHead || (!tr.inTHead && !tr.inTFoot && allTH)
		becomesHeader := isHeaderRow && g.headerRow < 0
		if becomesHeader {
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
			if becomesHeader && rowSpan > 1 {
				rowSpan = 1
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
// (rowStart, colStart) order — its first appearance scanning row-major,
// top-to-bottom/left-to-right is always its own anchor, since a cell never
// appears before its own rowStart/colStart.
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

// mergedCellDecls resolves td's own CSS/inline declarations, with the
// <col>/<colgroup> declarations at column ci (if any) merged in as a
// lower-priority base. For a spanning cell, ci is its leftmost (colStart)
// column — a single spanning cell can only take one column's <col> style,
// so the leftmost one wins; a rare, low-stakes simplification when
// <colgroup>/<col> and colspan are combined.
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

// mergeColConstraints folds dc into dst, keeping the first fixed/percent
// value seen and the tightest min/max — the same merge rule used whether the
// source is a single unconstrained pass or many rows' worth of cells.
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
// collectively have at least enough natural width for its content: if the
// sum of the columns it spans — plus the interior separators it reclaims,
// since no separator is drawn through a spanned cell — falls short of the
// cell's own measured natural width, the shortfall is added to those
// columns' natural field (evenly, remainder to the first columns).
// Approximates CSS2.1's fallback for spanning cells whose columns don't
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

// gridColumnConstraints gathers CSS/HTML width constraints (fixed/percent/
// min/max) per column from a resolved grid, without rendering any cell
// content — only from colSpan==1 cells, since a spanning cell's constraints
// don't map to a single column unambiguously (its content instead feeds into
// distributeSpanDeficit once natural widths are known).
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
// unbounded width, purely to measure how wide it would be if never forced to
// wrap — then discards that trial render. measuringNaturalWidth suppresses
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

// measureTableWidth computes a <table>'s own natural (shrink-to-fit) width -
// its final rendered box width if never constrained - without producing any
// actual rendered content. It mirrors renderTable's own column-sizing math
// (grid topology, CSS constraints, and, when needed, content measurement)
// but stops there: it never calls fillGridCellTokens/renderTableBody, so it
// never commits to a full "final" render of this table's cells.
//
// This exists specifically for inline.go's nested-<table> handling under
// measuringNaturalWidth (see measureCellNaturalWidth): without it, measuring
// an ancestor's natural width would mean fully rendering every descendant
// table too (via a real renderTable call) only to discard the string and
// keep just its width - and since that descendant table's own renderTable
// call would, if it also needed a measurement pass, do the same thing one
// level further down, the redundant "measure AND fully render" work
// compounds at every nesting level, making measurement exponential in
// nesting depth instead of linear. Real-world deeply-nested table markup
// (e.g. HTML email templates) hits exactly this - see the regression test
// covering it.
func (r *Engine) measureTableWidth(n *html.Node) int {
	tableDecls := r.resolveDecls(n)
	colDecls := r.collectColDecls(n)
	grid := r.resolveTableGrid(n)
	if grid.numCols == 0 {
		return 0
	}
	ts := applyTableCSSToStyle(namedTableStyleDefault(), tableDecls)
	sepW := runeLen(ts.sep)
	overhead := runeLen(ts.left) + (grid.numCols-1)*sepW + runeLen(ts.right)
	colsEst := r.gridColumnConstraints(grid, colDecls)
	measured := r.measureGridNaturalWidths(grid, colDecls, colsEst, sepW)
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
// couldn't produce a usable estimate from CSS constraints alone (two or more
// unconstrained flex columns). It renders each cell once (via
// measureCellNaturalWidth) to measure its content instead of only reading
// declared constraints; colSpan==1 cells contribute directly to their own
// column, colSpan>1 cells are folded in afterward via distributeSpanDeficit.
// Returns a copy of cols with natural filled in; does not mutate the input.
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
// before any cell content is rendered, using only CSS/HTML constraints (no
// natural content width, which isn't known yet). This exactly matches the
// real final sizeColumns pass whenever every column is either fixed/percent
// constrained, or there is at most one unconstrained ("flex") column — in
// both cases the real pass's use of natural width ends up not mattering: a
// percent/fixed column ignores natural entirely, and a lone flex column
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
		// its own natural width when the table is fullWidth (forced
		// expansion). Without fullWidth its final width is driven by natural
		// content width, which isn't known yet — don't guess.
		return nil
	}
	return sizeColumns(cols, contentWidth, fullWidth)
}

// fillGridCellTokens renders every unique cell's content once, using its
// span-aware final budget (estWidths summed across the columns it spans,
// plus the interior separators it reclaims since no separator is drawn
// through a spanned cell) as the wrap width — so nested content (e.g. a
// nested <table>) is only ever rendered at its real final budget, never
// prematurely committed at a wrong one. Also resolves each cell's other
// CSS-derived fields (padding, text-align, etc).
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

		// A leading/trailing brk (e.g. from a first/last block child's
		// margin, or a nested table rendered via pushBox) is only ever
		// meaningful as structural separation between siblings — at either
		// edge of the cell's own content it's not real content and would
		// otherwise wrap to a spurious blank line at the top/bottom of the
		// cell (table cells don't collapse margins with their content the
		// way a block container does).
		cellTokens = cellTokens[leadingBreaks(cellTokens):]
		for len(cellTokens) > 0 && cellTokens[len(cellTokens)-1].brk {
			cellTokens = cellTokens[:len(cellTokens)-1]
		}
		if tdDecls["visibility"] == "hidden" {
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
	var captionText string

	colDecls := r.collectColDecls(n)
	tableDecls := r.resolveDecls(n)
	if tableDecls["border-collapse"] == "separate" {
		// Opt-in only: per-cell independent borders + border-spacing gaps,
		// reusing the same generic block-border machinery any other element
		// uses instead of this file's tableStyle preset model — see
		// table_separate.go and docs/TABLES.md. Unset or "collapse" both
		// fall through to the unchanged legacy path below.
		return r.renderTableSeparate(n, availWidth, tableDecls)
	}
	ts := applyTableCSSToStyle(namedTableStyleDefault(), tableDecls)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%" && !r.measuringNaturalWidth

	// margin-left/right and padding-left/right/top/bottom on the <table>
	// itself: margin is blank space outside the rendered table block,
	// padding is blank space inside it (between the margin and the table's
	// own border/cell content). Percentages resolve against the width
	// available before either is subtracted, matching renderBlockContent.
	// A margin side of "auto" resolves to 0 here (isAuto tracks it) and is
	// filled in later, once the table's final rendered width is known — see
	// the splitAutoMargins call below.
	origAvailWidth := availWidth
	tableML, mlAuto := resolveMarginSide(tableDecls["margin-left"], availWidth)
	tableMR, mrAuto := resolveMarginSide(tableDecls["margin-right"], availWidth)
	tablePL := parsePaddingLen(tableDecls["padding-left"])
	tablePR := parsePaddingLen(tableDecls["padding-right"])
	tablePT := parsePaddingLen(tableDecls["padding-top"])
	tablePB := parsePaddingLen(tableDecls["padding-bottom"])
	availWidth = max(1, availWidth-tableML-tableMR-tablePL-tablePR)

	grid := r.resolveTableGrid(n)
	numCols := grid.numCols
	if numCols == 0 {
		return "", nil
	}
	sepW := runeLen(ts.sep)
	overhead := runeLen(ts.left) + (numCols-1)*sepW + runeLen(ts.right)

	// Estimate final column widths up front (from CSS constraints alone, no
	// cell content) so cell content that wraps itself (e.g. a nested <p> or
	// table) can be given its real final width as a wrap budget, rather than
	// wrapping once at a guess and again later at the true column width.
	colsEst := r.gridColumnConstraints(grid, colDecls)
	estWidths := estimateColumnWidths(colsEst, availWidth-overhead, fullWidth)
	if estWidths == nil {
		// CSS constraints alone weren't enough to estimate (two or more
		// unconstrained flex columns) - measure each cell's real natural
		// width up front instead of guessing.
		measured := r.measureGridNaturalWidths(grid, colDecls, colsEst, sepW)
		estWidths = sizeColumns(measured, availWidth-overhead, fullWidth)
	}
	fallbackCellWidth := max(1, (availWidth-overhead)/numCols)

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

	r.fillGridCellTokens(grid, colDecls, estWidths, sepW, fallbackCellWidth)

	cols := buildGridColumns(grid, numCols, sepW)
	widths := sizeColumns(cols, availWidth-overhead, fullWidth)
	fillGridCellLines(grid, widths, sepW)

	// Horizontal border lines (┌───┐ etc.) need to span the padding area too,
	// so padding reads as being inside the border, not merged with the
	// outer margin. Extend the first/last column's share of the line with
	// the border's own fill character; this only affects the horizontal
	// rule, not actual column content widths.
	borderWidths := widths
	if tablePL > 0 || tablePR > 0 {
		borderWidths = append([]int(nil), widths...)
		borderWidths[0] += tablePL
		borderWidths[len(borderWidths)-1] += tablePR
	}

	// tableW is the table's own final rendered width (border box, including
	// its padding but not its margin) — used both to center the caption and,
	// below, to resolve any margin-left/right: auto into concrete blank space.
	tableW := sum(widths) + overhead + tablePL + tablePR
	if mlAuto || mrAuto {
		remaining := origAvailWidth - tableW - tableML - tableMR
		tableML, tableMR = splitAutoMargins(remaining, tableML, tableMR, mlAuto, mrAuto)
	}

	captionSide := tableDecls["caption-side"]
	var out strings.Builder
	// rowOffset tracks how many lines have been written to out so far, so
	// each row's own (0-based) position map can be shifted into the table's
	// coordinate space as it's appended — the same incremental
	// shift-and-merge every other box-producing call site uses (see
	// wraptoken.go's mergePositions doc comment), just driven by line counts
	// instead of a box's width/height since out is a plain strings.Builder,
	// not a box.
	var positions map[*html.Node]Rect
	rowOffset := 0
	if captionText != "" && captionSide != "bottom" {
		// Center caption over the table width (default: top), including padding.
		out.WriteString(centerText(captionText, tableW) + "\n")
		rowOffset++
	}
	out.WriteString(drawHBorder(borderWidths, ts.top, colorOrFallback(ts.topColor, ts.color), r.profile))
	rowOffset++
	for i := 0; i < tablePT; i++ {
		out.WriteString(blankBoxRow(widths, numCols, ts, r.profile, tablePL, tablePR))
		rowOffset++
	}

	bodyStr, bodyPos := r.renderTableBody(grid, widths, borderWidths, ts, tablePL, tablePR)
	out.WriteString(bodyStr)
	positions = mergePositions(positions, bodyPos, rowOffset, 0)
	rowOffset += strings.Count(bodyStr, "\n")

	for i := 0; i < tablePB; i++ {
		out.WriteString(blankBoxRow(widths, numCols, ts, r.profile, tablePL, tablePR))
		rowOffset++
	}
	out.WriteString(drawHBorder(borderWidths, ts.bottom, colorOrFallback(ts.bottomColor, ts.color), r.profile))
	if captionText != "" && captionSide == "bottom" {
		out.WriteString(centerText(captionText, tableW) + "\n")
	}
	if len(positions) > 0 && tableML > 0 {
		positions = mergePositions(nil, positions, 0, tableML)
	}
	return wrapTableMargin(out.String(), tableML, tableMR), positions
}

// nbsp is U+00A0 (non-breaking space). A real &nbsp; HTML entity decodes to
// this rune and survives rendering as a distinct character
// (normalizeWhiteSpace and plainInlineText only touch plain ASCII space);
// Render's final pass normalizes it to a plain space in the returned string,
// since terminals don't distinguish breaking from non-breaking spaces.
const nbsp = " "

// wrapTableMargin applies the <table> element's own margin-left/right as
// blank space outside its fully-rendered text (padding is applied inside the
// border box itself, in the main body of renderTable, since CSS padding sits
// between the border and the content, not outside the border like margin).
// Plain spaces are safe on both sides: when this table is nested inside
// another table's cell, the outer table embeds it as a box token (see
// renderInlineAccTokens), never as flattened text passed through
// plainInlineText's trailing-space trim — so nothing here needs protecting
// from trimming the way it did before cells were token-based.
func wrapTableMargin(s string, ml, mr int) string {
	if ml > 0 || mr > 0 {
		s = applyLineEdges(s, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
	}
	return s
}

func namedTableStyleDefault() tableStyle {
	ts, _ := namedTableStyle("solid")
	return ts
}

// buildGridColumns rebuilds each column's constraints from its cells' final,
// already-rendered tokens (natural width) and per-cell CSS constraints —
// colSpan==1 cells contribute directly to their own column; colSpan>1 cells
// are folded in afterward via distributeSpanDeficit.
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
			// Not wrapped, so a plain trailing space from source text (e.g.
			// "<td>hi </td>") won't get naturally dropped by tokenization the
			// way wrapping does — trim it explicitly, per line, matching
			// plainInlineText's historical role here. naturalWidthCap (see
			// wraptoken.go) keeps this the token-domain equivalent of the old
			// flatten-to-string path — only structural brk/box breaks start a
			// new line, no width-driven wrapping — while still going through
			// wordWrapTokens so cell.tokens' own node positions are produced
			// instead of discarded.
			b, positions = wordWrapTokens(cell.tokens, naturalWidthCap, "", 0)
			for j, line := range b.lines {
				b.lines[j] = strings.TrimRight(line, " ")
			}
		} else {
			b, positions = wordWrapTokens(cell.tokens, contentW, "break-word", 0)
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
			cell.lines = append(cell.lines, truncateToWidth(line, contentW, suffix))
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

// renderTableBody renders every row of a resolved grid — header separator
// and (span-aware) interior row separators included — as one continuous
// block. widths are the final per-column content widths (used to size each
// cell's own content); borderWidths are the same widths with the table's own
// left/right padding folded into the first/last entries (used only for the
// header/row-separator rule lines, matching how the outer top/bottom
// borders already extend into that padding).
//
// A rowspan cell's content flows continuously through every row it spans
// (tracked via `consumed`, each cell's own running count of lines already
// emitted in earlier rows of its span) and the interior separator between
// two rows it spans is drawn with that column's segment blanked instead of
// ruled (see drawRowSepWithSpans) — together giving the cell a single,
// visually unbroken box rather than one broken up by every row boundary it
// crosses. A colspan cell's own row similarly draws no interior separator
// between the columns it spans, and its content is only emitted once, at its
// leftmost (anchor) column, across the combined width of every column it
// spans.
func (r *Engine) renderTableBody(g tableGrid, widths, borderWidths []int, ts tableStyle, boxPL, boxPR int) (string, map[*html.Node]Rect) {
	numCols := g.numCols
	numRows := len(g.rows)
	if numRows == 0 {
		return "", nil
	}
	sepW := runeLen(ts.sep)
	cells := uniqueCells(g)

	// Row-height resolution, two passes: first, every rowSpan==1 cell sets
	// its own row's local height directly (today's behavior, unchanged).
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
	// Second, a rowSpan>1 cell that needs more lines than its spanned rows
	// already provide grows those rows (evenly, remainder to the first) to
	// fit - processed in rowStart order (uniqueCells' own order) so later,
	// overlapping spans see already-adjusted heights from earlier ones.
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
			for i := 0; i < cell.rowSpan; i++ {
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
	// alignOffset generalizes the old per-row vertical-align offset across a
	// cell's whole merged block (mergedHeight) instead of just its own row -
	// for a rowSpan==1 cell mergedHeight equals localHeight[rowStart], so
	// this degenerates to exactly the old per-row behavior.
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
	spanContentWidth := func(cell *tableCell) int {
		w := (cell.colSpan - 1) * sepW
		for i := cell.colStart; i < cell.colStart+cell.colSpan && i < len(widths); i++ {
			w += widths[i]
		}
		return w
	}

	paintLeft := makePainter(colorOrFallback(ts.leftColor, ts.color), r.profile)
	paintSep := makePainter(ts.color, r.profile)
	paintRight := makePainter(colorOrFallback(ts.rightColor, ts.color), r.profile)

	var out strings.Builder
	var positions map[*html.Node]Rect
	consumed := make(map[*tableCell]int, len(cells))
	rowOffset := 0

	for row := 0; row < numRows; row++ {
		h := localHeight[row]
		for lineIdx := 0; lineIdx < h; lineIdx++ {
			var sb strings.Builder
			sb.WriteString(paintLeft(ts.left))
			if boxPL > 0 {
				sb.WriteString(strings.Repeat(" ", boxPL))
			}
			for c := 0; c < numCols; c++ {
				cell := g.rows[row][c]
				if c > 0 {
					prev := g.rows[row][c-1]
					if !(cell != nil && prev == cell) {
						sb.WriteString(paintSep(ts.sep))
					}
				}
				if cell != nil && cell.colStart != c {
					continue // mid-colspan continuation: already drawn at its anchor column
				}
				if cell == nil {
					// Short row: no cell at this column at all - blank fill
					// of the column's own width, same as a zero-value cell.
					var w int
					if c < len(widths) {
						w = widths[c]
					}
					sb.WriteString(strings.Repeat(" ", w))
					continue
				}
				contentW := spanContentWidth(cell)
				pl, pr, cw := clampCellPadding(contentW, cell.paddingLeft, cell.paddingRight)
				absPos := consumed[cell] + lineIdx
				var line string
				if idx := absPos - alignOffset(cell); idx >= 0 && idx < len(cell.lines) {
					line = cell.lines[idx]
				}
				rendered := cell.cellStyle.render(alignLines(line, cell.textAlign, cw), r.profile)
				if pl > 0 {
					rendered = strings.Repeat(" ", pl) + rendered
				}
				if pr > 0 {
					rendered += strings.Repeat(" ", pr)
				}
				sb.WriteString(rendered)
			}
			if boxPR > 0 {
				sb.WriteString(strings.Repeat(" ", boxPR))
			}
			sb.WriteString(paintRight(ts.right))
			out.WriteString(sb.String())
			out.WriteString("\n")
		}

		// Position merge, only for cells anchored at this row (their content
		// starts here; a rowspan continuation was already merged when its
		// anchor row was processed).
		colStart := runeLen(ts.left) + boxPL
		for c := 0; c < numCols; c++ {
			if c > 0 {
				colStart += runeLen(ts.sep)
			}
			cell := g.rows[row][c]
			if cell != nil && cell.rowStart == row && cell.colStart == c && len(cell.positions) > 0 {
				pl, _, _ := clampCellPadding(spanContentWidth(cell), cell.paddingLeft, cell.paddingRight)
				positions = mergePositions(positions, cell.positions, rowOffset+alignOffset(cell), colStart+pl)
			}
			if c < len(widths) {
				colStart += widths[c]
			}
		}

		seenThisRow := make(map[*tableCell]bool)
		for c := 0; c < numCols; c++ {
			cell := g.rows[row][c]
			if cell == nil || seenThisRow[cell] {
				continue
			}
			seenThisRow[cell] = true
			consumed[cell] += h
		}
		rowOffset += h

		if row == g.headerRow {
			out.WriteString(drawHBorder(borderWidths, ts.header, ts.color, r.profile))
			rowOffset++
		} else if row < numRows-1 {
			out.WriteString(drawRowSepWithSpans(borderWidths, g.rows[row], g.rows[row+1], ts.rowSep, ts.color, r.profile))
			rowOffset++
		}
	}
	return out.String(), positions
}

// blankBoxRow draws one bordered but content-free row (left border + blank
// interior + right border), used to render padding-top/padding-bottom on a
// <table>: unlike margin, CSS padding sits inside the border box, so these
// rows must still carry the left/right border characters.
func blankBoxRow(widths []int, numCols int, ts tableStyle, p colorprofile.Profile, boxPL, boxPR int) string {
	paintLeft := makePainter(colorOrFallback(ts.leftColor, ts.color), p)
	paintRight := makePainter(colorOrFallback(ts.rightColor, ts.color), p)
	interior := sum(widths) + (numCols-1)*runeLen(ts.sep) + boxPL + boxPR
	return paintLeft(ts.left) + strings.Repeat(" ", interior) + paintRight(ts.right) + "\n"
}
