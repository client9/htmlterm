package render

import (
	"strings"

	"golang.org/x/net/html"
)

// renderTableSeparate renders n under `border-collapse: separate`: every
// <td>/<th> gets its own independently bordered box, built from the same
// generic block-border primitives any other element uses
// (resolveBoxBorders/applyBlockBordersBox/drawBlockHBorder/
// clampCellPadding/parsePaddingLen — see docs/TABLES.md), with
// border-spacing as the gap between adjacent cell boxes and between the
// table's own border and its outermost cells. This is the opposite of the
// legacy renderTable model: that draws one shared frame/divider from a
// tableStyle preset, with cells never reading their own border CSS at all.
//
// Grid topology, column-width estimation, and per-cell text wrapping are
// reused completely unchanged from the legacy path (resolveTableGrid,
// collectColDecls, gridColumnConstraints, estimateColumnWidths/
// measureGridNaturalWidths, fillGridCellTokens, buildGridColumns,
// fillGridCellLines) — only what happens once a cell's content lines are
// ready (box it in its own border/padding, assemble rows/grid with
// border-spacing gaps, wrap the table's own border/padding/margin around
// the result) is new.
//
// Purely additive and opt-in: renderTable only calls into this when
// tableDecls["border-collapse"] == "separate"; unset or "collapse" both
// keep the legacy path completely untouched.
func (r *Engine) renderTableSeparate(n *html.Node, availWidth int, tableDecls map[string]string) (string, map[*html.Node]Rect) {
	colDecls := r.collectColDecls(n)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%" && !r.measuringNaturalWidth

	spacingX := parsePaddingLen(tableDecls["border-spacing-x"])
	spacingY := parsePaddingLen(tableDecls["border-spacing-y"])

	origAvailWidth := availWidth
	tableML, mlAuto := resolveMarginSide(tableDecls["margin-left"], availWidth)
	tableMR, mrAuto := resolveMarginSide(tableDecls["margin-right"], availWidth)
	tablePL := parsePaddingLen(tableDecls["padding-left"])
	tablePR := parsePaddingLen(tableDecls["padding-right"])
	tablePT := parsePaddingLen(tableDecls["padding-top"])
	tablePB := parsePaddingLen(tableDecls["padding-bottom"])
	tableBl, tableBr, tableBt, tableBb, tlCorner, trCorner, blCorner, brCorner := resolveBoxBorders(tableDecls)
	availWidth = max(1, availWidth-tableML-tableMR-tablePL-tablePR-runeLen(tableBl.char)-runeLen(tableBr.char))

	grid := r.resolveTableGrid(n)
	numCols := grid.numCols
	if numCols == 0 {
		return "", nil
	}

	colBorderW := r.separateColumnBorderOverhead(grid, colDecls, numCols)
	overhead := (numCols + 1) * spacingX
	for _, w := range colBorderW {
		overhead += w
	}

	colsEst := r.gridColumnConstraints(grid, colDecls)
	estWidths := estimateColumnWidths(colsEst, availWidth-overhead, fullWidth)
	if estWidths == nil {
		// CSS constraints alone weren't enough to estimate (two or more
		// unconstrained flex columns) - measure each cell's real natural
		// width up front, exactly as the legacy path does. spacingX is
		// passed where the legacy path passes sepW: both represent "the
		// width reclaimed by a spanning cell at each interior column
		// boundary it crosses" - in separate mode that's the border-spacing
		// gap, since a spanning cell's own border box absorbs it directly
		// rather than any shared divider character.
		measured := r.measureGridNaturalWidths(grid, colDecls, colsEst, spacingX)
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

	r.fillGridCellTokens(grid, colDecls, estWidths, spacingX, fallbackCellWidth)

	cols := buildGridColumns(grid, numCols, spacingX)
	widths := sizeColumns(cols, availWidth-overhead, fullWidth)
	fillGridCellLines(grid, widths, spacingX)

	gridBox, positions := r.composeSeparateGrid(grid, widths, colDecls, colBorderW, spacingX, spacingY)

	// Wrap the table's own border/padding/margin around the assembled grid,
	// exactly as any other block element would — resolveBoxBorders et al.
	// are pure decls-driven functions with no dependency on how the content
	// inside them was produced. Order mirrors renderBlockContentBox's own
	// documented sequence: vertical padding -> horizontal padding -> left/
	// right border -> top/bottom rule -> caption -> margin.
	b := gridBox
	if tablePT > 0 || tablePB > 0 {
		blank := strings.Repeat(" ", b.width)
		if tablePT > 0 {
			padded := make([]string, 0, tablePT+len(b.lines))
			for range tablePT {
				padded = append(padded, blank)
			}
			b.lines = append(padded, b.lines...)
		}
		for range tablePB {
			b.lines = append(b.lines, blank)
		}
		b = box{lines: b.lines, width: linesWidth(b.lines)}
	}
	if tablePL > 0 || tablePR > 0 {
		b = applyLineEdgesBox(b, strings.Repeat(" ", tablePL), strings.Repeat(" ", tablePR))
	}
	rowShift := tablePT
	colShift := tablePL
	if tableBl.char != "" || tableBr.char != "" {
		b = applyBlockBordersBox(b, tableBl, tableBr, r.profile)
		colShift += runeLen(tableBl.char)
	}
	topRuleDrawn := false
	if top := drawBlockHBorder(tableBt.char, tableBt.color, tlCorner, trCorner, b.width, r.profile); top != "" {
		b.lines = append([]string{top}, b.lines...)
		b.width = linesWidth(b.lines)
		topRuleDrawn = true
	}
	if bot := drawBlockHBorder(tableBb.char, tableBb.color, blCorner, brCorner, b.width, r.profile); bot != "" {
		b.lines = append(b.lines, bot)
		b.width = linesWidth(b.lines)
	}
	if topRuleDrawn {
		rowShift++
	}
	positions = mergePositions(nil, positions, rowShift, colShift)

	if mlAuto || mrAuto {
		remaining := origAvailWidth - b.width - tableML - tableMR
		tableML, tableMR = splitAutoMargins(remaining, tableML, tableMR, mlAuto, mrAuto)
	}

	captionSide := tableDecls["caption-side"]
	if captionText != "" {
		capLine := centerText(captionText, b.width)
		if captionSide == "bottom" {
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

// separateColumnBorderOverhead resolves, per column, the border character
// width (left+right) contributed by a single representative cell in that
// column — the column's header cell if the table has one, else the first
// row's cell occupying that column. Used only to size column-width
// allocation up front; each cell still renders its own actual border
// independently (see composeSeparateGrid), so a column whose cells set
// inconsistent border widths may not perfectly align — a documented Phase-1
// simplification (see docs/TABLES.md) rather than something resolved
// generally here.
func (r *Engine) separateColumnBorderOverhead(g tableGrid, colDecls []map[string]string, numCols int) []int {
	out := make([]int, numCols)
	for c := range numCols {
		var rep *tableCell
		if g.headerRow >= 0 && g.headerRow < len(g.rows) {
			rep = g.rows[g.headerRow][c]
		}
		if rep == nil {
			for _, row := range g.rows {
				if row[c] != nil {
					rep = row[c]
					break
				}
			}
		}
		if rep == nil {
			continue
		}
		decls := r.mergedCellDecls(rep.node, colDecls, rep.colStart)
		bl, br, _, _, _, _, _, _ := resolveBoxBorders(decls)
		out[c] = runeLen(bl.char) + runeLen(br.char)
	}
	return out
}

// cellBorders caches one cell's resolved border/corner declarations —
// resolveBoxBorders is a pure function of a decls map, so this is computed
// once per unique cell and reused across the height-equalization pass and
// the final box-building pass in composeSeparateGrid.
type cellSeparateBorders struct {
	bl, br, bt, bb                         blockBorder
	tlCorner, trCorner, blCorner, brCorner string
}

// composeSeparateGrid builds the grid-of-independently-bordered-cells
// content for border-collapse:separate — the counterpart to the legacy
// path's renderTableBody, but composing each cell as its own complete box
// (content, then padding, then border — the same primitives any other
// block element uses) and splicing those boxes onto a blank canvas at
// their resolved (row, col) offset via spliceColumns, rather than
// interleaving one shared frame's characters directly into each output
// line. Returned positions are relative to the grid's own (0,0) origin;
// the caller shifts them once when embedding, the same incremental
// convention used throughout the render package (see wraptoken.go's
// mergePositions doc comment).
func (r *Engine) composeSeparateGrid(g tableGrid, widths []int, colDecls []map[string]string, colBorderW []int, spacingX, spacingY int) (box, map[*html.Node]Rect) {
	numCols := g.numCols
	numRows := len(g.rows)
	if numRows == 0 || numCols == 0 {
		return box{lines: []string{""}, width: 0}, nil
	}
	cells := uniqueCells(g)

	borders := make(map[*tableCell]cellSeparateBorders, len(cells))
	for _, cell := range cells {
		decls := r.mergedCellDecls(cell.node, colDecls, cell.colStart)
		bl, br, bt, bb, tlC, trC, blC, brC := resolveBoxBorders(decls)
		borders[cell] = cellSeparateBorders{bl, br, bt, bb, tlC, trC, blC, brC}
	}

	// Row-height resolution: a single deficit-growing pass covers both
	// rowSpan==1 and rowSpan>1 cells uniformly (unlike the legacy path's two
	// separate passes, which only needs a second pass for rowSpan>1 since it
	// never has to account for a cell's own border lines — here every cell's
	// "need" includes however many border-top/border-bottom lines its own
	// resolved border contributes, since two cells in the same row may have
	// different border presence and the row must be tall enough for the
	// tallest one).
	localHeight := make([]int, numRows)
	for i := range localHeight {
		localHeight[i] = 1
	}
	for _, cell := range cells {
		cb := borders[cell]
		need := len(cell.lines)
		if cb.bt.char != "" {
			need++
		}
		if cb.bb.char != "" {
			need++
		}
		have := (cell.rowSpan - 1) * spacingY
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

	mergedContentHeight := make(map[*tableCell]int, len(cells))
	for _, cell := range cells {
		cb := borders[cell]
		h := (cell.rowSpan - 1) * spacingY
		for rr := cell.rowStart; rr < cell.rowStart+cell.rowSpan; rr++ {
			h += localHeight[rr]
		}
		if cb.bt.char != "" {
			h--
		}
		if cb.bb.char != "" {
			h--
		}
		mergedContentHeight[cell] = h
	}
	alignOffset := func(cell *tableCell) int {
		extra := mergedContentHeight[cell] - len(cell.lines)
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

	// rowGroupTop[r] is the output-line offset row-group r's content starts
	// at, including the outer border-spacing gap before the first group and
	// between every pair of groups (matching real CSS: border-spacing also
	// separates the outermost cells from the table's own border).
	rowGroupTop := make([]int, numRows+1)
	rowGroupTop[0] = spacingY
	for r := range numRows {
		rowGroupTop[r+1] = rowGroupTop[r] + localHeight[r] + spacingY
	}
	totalHeight := rowGroupTop[numRows]

	colGroupLeft := make([]int, numCols+1)
	colGroupLeft[0] = spacingX
	for c := range numCols {
		colGroupLeft[c+1] = colGroupLeft[c] + widths[c] + colBorderW[c] + spacingX
	}
	totalWidth := colGroupLeft[numCols]

	lines := make([]string, totalHeight)
	for i := range lines {
		lines[i] = strings.Repeat(" ", totalWidth)
	}
	var positions map[*html.Node]Rect

	for _, cell := range cells {
		cb := borders[cell]
		spanW := (cell.colSpan - 1) * spacingX
		for i := cell.colStart; i < cell.colStart+cell.colSpan && i < len(widths); i++ {
			spanW += widths[i]
		}
		pl, pr, cw := clampCellPadding(spanW, cell.paddingLeft, cell.paddingRight)

		blank := cell.cellStyle.render(strings.Repeat(" ", cw), r.profile)
		off := alignOffset(cell)
		trailing := mergedContentHeight[cell] - len(cell.lines) - off
		var contentLines []string
		for range off {
			contentLines = append(contentLines, blank)
		}
		for _, line := range cell.lines {
			contentLines = append(contentLines, cell.cellStyle.render(alignLines(line, cell.textAlign, cw), r.profile))
		}
		for range trailing {
			contentLines = append(contentLines, blank)
		}

		cbox := box{lines: contentLines, width: linesWidth(contentLines)}
		if pl > 0 || pr > 0 {
			cbox = applyLineEdgesBox(cbox, strings.Repeat(" ", pl), strings.Repeat(" ", pr))
		}
		if cb.bl.char != "" || cb.br.char != "" {
			cbox = applyBlockBordersBox(cbox, cb.bl, cb.br, r.profile)
		}
		topDrawn := false
		if top := drawBlockHBorder(cb.bt.char, cb.bt.color, cb.tlCorner, cb.trCorner, cbox.width, r.profile); top != "" {
			cbox.lines = append([]string{top}, cbox.lines...)
			cbox.width = linesWidth(cbox.lines)
			topDrawn = true
		}
		if bot := drawBlockHBorder(cb.bb.char, cb.bb.color, cb.blCorner, cb.brCorner, cbox.width, r.profile); bot != "" {
			cbox.lines = append(cbox.lines, bot)
			cbox.width = linesWidth(cbox.lines)
		}

		// Allocated slot width may differ slightly from this cell's own
		// actual box width (e.g. a colspan cell, or a column whose
		// representative border width — used for the up-front width
		// allocation — differs from this specific cell's own border) — pad
		// or truncate to the slot so the grid stays a strict rectangle. See
		// separateColumnBorderOverhead's own doc comment.
		slotWidth := colGroupLeft[cell.colStart+cell.colSpan] - colGroupLeft[cell.colStart] - spacingX
		if cbox.width < slotWidth {
			cbox = padLinesToWidthBox(cbox, slotWidth)
		} else if cbox.width > slotWidth {
			for i, line := range cbox.lines {
				cbox.lines[i] = truncateToWidth(line, slotWidth, "")
			}
			cbox.width = slotWidth
		}

		row := rowGroupTop[cell.rowStart]
		col := colGroupLeft[cell.colStart]
		for i, line := range cbox.lines {
			lines[row+i] = spliceColumns(lines[row+i], col, cbox.width, line)
		}

		if len(cell.positions) > 0 {
			contentRow := row + off
			if topDrawn {
				contentRow++
			}
			contentCol := col + pl
			if cb.bl.char != "" {
				contentCol += runeLen(cb.bl.char)
			}
			positions = mergePositions(positions, cell.positions, contentRow, contentCol)
		}
	}

	return box{lines: lines, width: totalWidth}, positions
}
