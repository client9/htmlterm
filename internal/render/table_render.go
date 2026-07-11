package render

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

type tableCell struct {
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
	// renderTableRow/renderTable for how these get shifted into the table's
	// own coordinate space.
	positions map[*html.Node]Rect
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

// preScanTableColumns walks a table's rows gathering column count and the
// CSS/HTML width constraints (fixed/percent/min/max) for each column, without
// rendering any cell content. Constraints derive purely from declarations, so
// this can run before cell text is computed — letting column widths be
// estimated up front and handed to cell content as its wrap width, instead of
// wrapping cell content once at a guessed width and then again at the real
// column width (which produces mismatched, broken word-wrap for two-pass
// content like a nested <p>).
func (r *Engine) preScanTableColumns(n *html.Node, colDecls []map[string]string) (numCols int, cols []colConstraints) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "thead", "tbody", "tfoot":
				walk(c)
			case "tr":
				if r.resolveDecls(c)["display"] == "none" {
					continue
				}
				ci := 0
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.Data != "th" && td.Data != "td") {
						continue
					}
					tdDecls := r.resolveDecls(td)
					if tdDecls["display"] == "none" {
						continue
					}
					if ci < len(colDecls) && len(colDecls[ci]) > 0 {
						merged := make(map[string]string, len(colDecls[ci])+len(tdDecls))
						for k, v := range colDecls[ci] {
							merged[k] = v
						}
						for k, v := range tdDecls {
							merged[k] = v
						}
						tdDecls = merged
					}
					if ci >= len(cols) {
						cols = append(cols, make([]colConstraints, ci+1-len(cols))...)
					}
					dc := r.cellConstraints(tdDecls)
					if cols[ci].fixed == 0 && dc.fixed > 0 {
						cols[ci].fixed = dc.fixed
					}
					if cols[ci].percent == 0 && dc.percent > 0 {
						cols[ci].percent = dc.percent
					}
					if dc.minWidth > cols[ci].minWidth {
						cols[ci].minWidth = dc.minWidth
					}
					if dc.minPercent > cols[ci].minPercent {
						cols[ci].minPercent = dc.minPercent
					}
					if dc.maxWidth > 0 && (cols[ci].maxWidth == 0 || dc.maxWidth < cols[ci].maxWidth) {
						cols[ci].maxWidth = dc.maxWidth
					}
					if dc.maxPercent > 0 && (cols[ci].maxPercent == 0 || dc.maxPercent < cols[ci].maxPercent) {
						cols[ci].maxPercent = dc.maxPercent
					}
					ci++
				}
				if ci > numCols {
					numCols = ci
				}
			}
		}
	}
	walk(n)
	return numCols, cols
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
	tokens := r.renderInlineAccTokens(td, inlineStyle{}, naturalWidthCap)
	r.measuringNaturalWidth = savedMeasuring
	r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
	return tokensNaturalWidth(tokens)
}

// measureNaturalColumnWidths fills in cols' natural field with each column's
// real unwrapped content width, for the case where estimateColumnWidths
// couldn't produce a usable estimate from CSS constraints alone (two or more
// unconstrained flex columns). It mirrors preScanTableColumns's walk and
// <col>-decl merging, but additionally renders each cell once (via
// measureCellNaturalWidth) to measure its content instead of only reading
// declared constraints. Returns a copy of cols with natural filled in; does
// not mutate the input.
func (r *Engine) measureNaturalColumnWidths(n *html.Node, colDecls []map[string]string, cols []colConstraints) []colConstraints {
	out := append([]colConstraints(nil), cols...)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "thead", "tbody", "tfoot":
				walk(c)
			case "tr":
				if r.resolveDecls(c)["display"] == "none" {
					continue
				}
				ci := 0
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.Data != "th" && td.Data != "td") {
						continue
					}
					tdDecls := r.resolveDecls(td)
					if tdDecls["display"] == "none" {
						continue
					}
					if ci < len(colDecls) && len(colDecls[ci]) > 0 {
						merged := make(map[string]string, len(colDecls[ci])+len(tdDecls))
						for k, v := range colDecls[ci] {
							merged[k] = v
						}
						for k, v := range tdDecls {
							merged[k] = v
						}
						tdDecls = merged
					}
					if ci >= len(out) {
						out = append(out, make([]colConstraints, ci+1-len(out))...)
					}
					pl := parsePaddingLen(tdDecls["padding-left"])
					pr := parsePaddingLen(tdDecls["padding-right"])
					w := r.measureCellNaturalWidth(td) + pl + pr
					if w > out[ci].natural {
						out[ci].natural = w
					}
					ci++
				}
			}
		}
	}
	walk(n)
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
// affect the real split, so this returns nil and callers fall back to the
// old (occasionally double-wrapped) behavior for that rarer case.
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

// renderTable renders a <table> node using the custom table engine.
// availWidth is the width available at the table's rendering context (the
// full renderer width at the top level, or the containing cell's content
// width for a nested table).
func (r *Engine) renderTable(n *html.Node, availWidth int) (string, map[*html.Node]Rect) {
	var headers []tableCell
	var rows [][]tableCell
	var captionText string

	colDecls := r.collectColDecls(n)
	tableDecls := r.resolveDecls(n)
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

	// Estimate final column widths up front (from CSS constraints alone, no
	// cell content) so cell content that wraps itself (e.g. a nested <p> or
	// table) can be given its real final width as a wrap budget, rather than
	// wrapping once at a guess and again later at the true column width.
	numColsEst, colsEst := r.preScanTableColumns(n, colDecls)
	var estWidths []int
	overheadEst := 0
	if numColsEst > 0 {
		overheadEst = runeLen(ts.left) + (numColsEst-1)*runeLen(ts.sep) + runeLen(ts.right)
		estWidths = estimateColumnWidths(colsEst, availWidth-overheadEst, fullWidth)
		if estWidths == nil {
			// CSS constraints alone weren't enough to estimate (two or more
			// unconstrained flex columns) - measure each cell's real natural
			// width up front instead of guessing, so nested content (e.g. a
			// nested <table>) is only ever rendered at its real final budget,
			// never prematurely committed at a wrong one.
			measured := r.measureNaturalColumnWidths(n, colDecls, colsEst)
			estWidths = sizeColumns(measured, availWidth-overheadEst, fullWidth)
		}
	}
	// Fallback budget for the caption hint below, when no columns were found
	// in the pre-scan at all: an even split.
	fallbackCellWidth := availWidth
	if numColsEst > 0 {
		fallbackCellWidth = max(1, (availWidth-overheadEst)/numColsEst)
	}

	var collect func(*html.Node, bool, bool)
	collect = func(n *html.Node, inTHead, inTFoot bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "caption":
				if captionText == "" {
					captionWidth := availWidth
					if numColsEst > 0 {
						captionWidth = max(1, availWidth-overheadEst)
					}
					savedHint, savedHintSet := r.nestedTableWidth, r.nestedTableWidthSet
					r.nestedTableWidth, r.nestedTableWidthSet = fallbackCellWidth, true
					captionText = plainInlineText(stripANSI(r.renderInlineAcc(c, inlineStyle{}, captionWidth)))
					r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
				}
			case "thead":
				collect(c, true, false)
			case "tbody":
				collect(c, false, false)
			case "tfoot":
				collect(c, false, true)
			case "tr":
				trDecls := r.resolveDecls(c)
				if trDecls["display"] == "none" {
					continue
				}
				var cells []tableCell
				// A row is a header row when it lives inside <thead>. Without
				// <thead>, the first all-<th> row in <tbody> acts as an implicit
				// header. Rows inside <tfoot> are never promoted to headers.
				isHeader := inTHead
				if !inTHead && !inTFoot {
					allTH := true
					for td := c.FirstChild; td != nil; td = td.NextSibling {
						if td.Type == html.ElementNode && td.Data == "td" {
							allTH = false
							break
						}
					}
					if allTH && len(headers) == 0 {
						isHeader = true
					}
				}
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.Data != "th" && td.Data != "td") {
						continue
					}
					tdDecls := r.resolveDecls(td)
					if tdDecls["display"] == "none" {
						continue
					}
					// Merge col-level declarations as a lower-priority base.
					ci := len(cells)
					if ci < len(colDecls) && len(colDecls[ci]) > 0 {
						merged := make(map[string]string, len(colDecls[ci])+len(tdDecls))
						for k, v := range colDecls[ci] {
							merged[k] = v
						}
						for k, v := range tdDecls {
							merged[k] = v // cell overrides col
						}
						tdDecls = merged
					}
					pl := parsePaddingLen(tdDecls["padding-left"])
					pr := parsePaddingLen(tdDecls["padding-right"])
					pt := parsePaddingLen(tdDecls["padding-top"])
					pb := parsePaddingLen(tdDecls["padding-bottom"])
					cellBudget := availWidth
					if ci < len(estWidths) {
						cellBudget = max(1, estWidths[ci]-pl-pr)
					} else if numColsEst > 0 {
						cellBudget = max(1, fallbackCellWidth-pl-pr)
					}
					savedHint, savedHintSet := r.nestedTableWidth, r.nestedTableWidthSet
					r.nestedTableWidth, r.nestedTableWidthSet = cellBudget, true
					cellTokens := r.renderInlineAccTokens(td, inlineStyle{}, cellBudget)
					r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
					// A trailing brk (e.g. from a block child or nested table
					// rendered via pushBox) is only ever meaningful as
					// structural separation between siblings — trailing, it's
					// not real content and would otherwise wrap to a spurious
					// blank line at the bottom of the cell.
					for len(cellTokens) > 0 && cellTokens[len(cellTokens)-1].brk {
						cellTokens = cellTokens[:len(cellTokens)-1]
					}
					if tdDecls["visibility"] == "hidden" {
						// Blank the content but keep its line/box structure (e.g. a
						// <br> or nested block inside the cell) so the cell still
						// occupies the same space.
						cellTokens = blankVisibleContentTokens(cellTokens)
					}
					cells = append(cells, tableCell{
						tokens:        cellTokens,
						textAlign:     tdDecls["text-align"],
						cellStyle:     extractInlineStyle(tdDecls),
						constraints:   r.cellConstraints(tdDecls),
						textOverflow:  textOverflowSuffix(tdDecls["text-overflow"]),
						noWrap:        tdDecls["white-space"] == "nowrap",
						paddingLeft:   pl,
						paddingRight:  pr,
						paddingTop:    pt,
						paddingBottom: pb,
						verticalAlign: tdDecls["vertical-align"],
					})
				}
				if len(cells) == 0 {
					continue
				}
				if isHeader && len(headers) == 0 {
					headers = cells
				} else {
					rows = append(rows, cells)
				}
			}
		}
	}
	collect(n, false, false)

	numCols := len(headers)
	if numCols == 0 && len(rows) > 0 {
		numCols = len(rows[0])
	}
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	if numCols == 0 {
		return "", nil
	}

	cols := buildTableColumns(headers, rows, numCols)

	sepW := runeLen(ts.sep)
	overhead := runeLen(ts.left) + (numCols-1)*sepW + runeLen(ts.right)
	widths := sizeColumns(cols, availWidth-overhead, fullWidth)
	fillTableCellLines(headers, widths, numCols)
	for i := range rows {
		fillTableCellLines(rows[i], widths, numCols)
	}

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
	if len(headers) > 0 {
		rowStr, rowPos := renderTableRow(headers, widths, numCols, ts, r.profile, tablePL, tablePR)
		out.WriteString(rowStr)
		positions = mergePositions(positions, rowPos, rowOffset, 0)
		rowOffset += strings.Count(rowStr, "\n")
		out.WriteString(drawHBorder(borderWidths, ts.header, ts.color, r.profile))
		rowOffset++
	}
	for i, row := range rows {
		if i > 0 {
			out.WriteString(drawHBorder(borderWidths, ts.rowSep, ts.color, r.profile))
			rowOffset++
		}
		rowStr, rowPos := renderTableRow(row, widths, numCols, ts, r.profile, tablePL, tablePR)
		out.WriteString(rowStr)
		positions = mergePositions(positions, rowPos, rowOffset, 0)
		rowOffset += strings.Count(rowStr, "\n")
	}
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
const nbsp = "\u00A0"

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

func buildTableColumns(headers []tableCell, rows [][]tableCell, numCols int) []colConstraints {
	cols := make([]colConstraints, numCols)
	for i := 0; i < numCols; i++ {
		if i < len(headers) {
			cols[i] = headers[i].constraints
			cols[i].natural = tokensNaturalWidth(headers[i].tokens) + headers[i].paddingLeft + headers[i].paddingRight
		}
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= numCols {
				break
			}
			if w := tokensNaturalWidth(c.tokens) + c.paddingLeft + c.paddingRight; w > cols[i].natural {
				cols[i].natural = w
			}
			dc := c.constraints
			if cols[i].fixed == 0 && dc.fixed > 0 {
				cols[i].fixed = dc.fixed
			}
			if cols[i].percent == 0 && dc.percent > 0 {
				cols[i].percent = dc.percent
			}
			if dc.minWidth > cols[i].minWidth {
				cols[i].minWidth = dc.minWidth
			}
			if dc.minPercent > cols[i].minPercent {
				cols[i].minPercent = dc.minPercent
			}
			if dc.maxWidth > 0 && (cols[i].maxWidth == 0 || dc.maxWidth < cols[i].maxWidth) {
				cols[i].maxWidth = dc.maxWidth
			}
			if dc.maxPercent > 0 && (cols[i].maxPercent == 0 || dc.maxPercent < cols[i].maxPercent) {
				cols[i].maxPercent = dc.maxPercent
			}
		}
	}
	return cols
}

func fillTableCellLines(cells []tableCell, widths []int, numCols int) {
	for i := range cells {
		if i >= numCols {
			break
		}
		_, _, contentW := clampCellPadding(widths[i], cells[i].paddingLeft, cells[i].paddingRight)
		var b box
		var positions map[*html.Node]Rect
		if cells[i].noWrap {
			// Not wrapped, so a plain trailing space from source text (e.g.
			// "<td>hi </td>") won't get naturally dropped by tokenization the
			// way wrapping does — trim it explicitly, per line, matching
			// plainInlineText's historical role here. naturalWidthCap (see
			// wraptoken.go) keeps this the token-domain equivalent of the old
			// flatten-to-string path — only structural brk/box breaks start a
			// new line, no width-driven wrapping — while still going through
			// wordWrapTokens so cells[i].tokens' own node positions are
			// produced instead of discarded.
			b, positions = wordWrapTokens(cells[i].tokens, naturalWidthCap, "", 0)
			for j, line := range b.lines {
				b.lines[j] = strings.TrimRight(line, " ")
			}
		} else {
			b, positions = wordWrapTokens(cells[i].tokens, contentW, "break-word", 0)
		}
		// text-overflow only applies in nowrap mode (CSS.md: "Ignored when
		// white-space: normal"); a wrapped line that still overflows contentW
		// despite word-wrap (an unbreakable embedded box, e.g.) is clipped
		// with no marker, just to keep the column boundary intact.
		suffix := ""
		if cells[i].noWrap {
			suffix = cells[i].textOverflow
		}
		for _, line := range b.lines {
			cells[i].lines = append(cells[i].lines, truncateToWidth(line, contentW, suffix))
		}
		if len(cells[i].lines) == 0 {
			cells[i].lines = []string{""}
		} else {
			cells[i].positions = positions
		}
		if pt := cells[i].paddingTop; pt > 0 {
			blank := make([]string, pt, pt+len(cells[i].lines))
			cells[i].lines = append(blank, cells[i].lines...)
			if len(cells[i].positions) > 0 {
				cells[i].positions = mergePositions(nil, cells[i].positions, pt, 0)
			}
		}
		if pb := cells[i].paddingBottom; pb > 0 {
			cells[i].lines = append(cells[i].lines, make([]string, pb)...)
		}
	}
}

func renderTableRow(cells []tableCell, widths []int, numCols int, ts tableStyle, p colorprofile.Profile, boxPL, boxPR int) (string, map[*html.Node]Rect) {
	height := 1
	for i := 0; i < numCols && i < len(cells); i++ {
		if h := len(cells[i].lines); h > height {
			height = h
		}
	}
	paintLeft := makePainter(colorOrFallback(ts.leftColor, ts.color), p)
	paintSep := makePainter(ts.color, p)
	paintRight := makePainter(colorOrFallback(ts.rightColor, ts.color), p)
	rowLines := make([]string, 0, height)
	for lineIdx := 0; lineIdx < height; lineIdx++ {
		var sb strings.Builder
		sb.WriteString(paintLeft(ts.left))
		if boxPL > 0 {
			sb.WriteString(strings.Repeat(" ", boxPL))
		}
		for i := 0; i < numCols; i++ {
			if i > 0 {
				sb.WriteString(paintSep(ts.sep))
			}
			var c tableCell
			if i < len(cells) {
				c = cells[i]
			}
			offset := 0
			switch c.verticalAlign {
			case "bottom":
				offset = height - len(c.lines)
			case "middle":
				offset = (height - len(c.lines)) / 2
			}
			var line string
			if ci := lineIdx - offset; ci >= 0 && ci < len(c.lines) {
				line = c.lines[ci]
			}
			pl, pr, contentW := clampCellPadding(widths[i], c.paddingLeft, c.paddingRight)
			rendered := c.cellStyle.render(alignLines(line, c.textAlign, contentW), p)
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
		rowLines = append(rowLines, sb.String())
	}

	// Column start offsets are row-invariant (every line has the same
	// left border/boxPL/separator/column-width layout), so this pass over
	// columns — computing where each cell's content begins and shifting its
	// own local position map by (verticalAlignOffset, contentCol) — only
	// needs to run once, not once per lineIdx.
	var positions map[*html.Node]Rect
	colStart := runeLen(ts.left) + boxPL
	for i := 0; i < numCols; i++ {
		if i > 0 {
			colStart += runeLen(ts.sep)
		}
		if i < len(cells) && len(cells[i].positions) > 0 {
			pl, _, _ := clampCellPadding(widths[i], cells[i].paddingLeft, cells[i].paddingRight)
			offset := 0
			switch cells[i].verticalAlign {
			case "bottom":
				offset = height - len(cells[i].lines)
			case "middle":
				offset = (height - len(cells[i].lines)) / 2
			}
			positions = mergePositions(positions, cells[i].positions, offset, colStart+pl)
		}
		if i < len(widths) {
			colStart += widths[i]
		}
	}
	return strings.Join(rowLines, "\n") + "\n", positions
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
