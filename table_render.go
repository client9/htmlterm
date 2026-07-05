package htmlterm

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

type tableCell struct {
	text          string
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
}

// collectColDecls scans direct <colgroup> children of a <table> node and returns
// a slice of declaration maps, one entry per column position (expanded by span).
// A <colgroup> with <col> children uses per-col decls (col overrides colgroup base).
// A <colgroup> with no <col> children applies its own decls across its span.
func (r *Renderer) collectColDecls(table *html.Node) []map[string]string {
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
			// Also handle width HTML attribute on <col>.
			if colDecls == nil {
				colDecls = map[string]string{}
			}
			if _, hasW := colDecls["width"]; !hasW {
				if w := nodeAttr(col, "width"); w != "" {
					colDecls = copyMap(colDecls)
					colDecls["width"] = w
				}
			}
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

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// preScanTableColumns walks a table's rows gathering column count and the
// CSS/HTML width constraints (fixed/percent/min/max) for each column, without
// rendering any cell content. Constraints derive purely from declarations, so
// this can run before cell text is computed — letting column widths be
// estimated up front and handed to cell content as its wrap width, instead of
// wrapping cell content once at a guessed width and then again at the real
// column width (which produces mismatched, broken word-wrap for two-pass
// content like a nested <p>).
func (r *Renderer) preScanTableColumns(n *html.Node, colDecls []map[string]string) (numCols int, cols []colConstraints) {
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
					dc := r.cellConstraints(td, tdDecls)
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
func (r *Renderer) renderTable(n *html.Node, availWidth int) string {
	var headers []tableCell
	var rows [][]tableCell
	var captionText string

	colDecls := r.collectColDecls(n)
	tableDecls := r.resolveDecls(n)
	ts := applyTableCSSToStyle(namedTableStyleDefault(), tableDecls)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%"

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
	}
	// Fallback budget when a precise per-column estimate isn't available
	// (e.g. multiple unconstrained flex columns): an even split, same as
	// before — used only for the nested-table-width hint, not for wrapping
	// plain cell text (which still uses the raw availWidth as before).
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
					cellText := plainInlineText(r.renderInlineAcc(td, inlineStyle{}, cellBudget))
					r.nestedTableWidth, r.nestedTableWidthSet = savedHint, savedHintSet
					if tdDecls["visibility"] == "hidden" {
						// Blank the content but keep its line structure (e.g. a <br>
						// inside the cell) so the cell still occupies the same space;
						// blankVisibleContent preserves "\n" and blanks everything else.
						cellText = blankVisibleContent(cellText)
					}
					cells = append(cells, tableCell{
						text:          cellText,
						textAlign:     tdDecls["text-align"],
						cellStyle:     extractInlineStyle(tdDecls),
						constraints:   r.cellConstraints(td, tdDecls),
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
		return ""
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
	if captionText != "" && captionSide != "bottom" {
		// Center caption over the table width (default: top), including padding.
		out.WriteString(centerText(captionText, tableW) + "\n")
	}
	out.WriteString(drawHBorder(borderWidths, ts.top, ts.color, r.profile))
	for i := 0; i < tablePT; i++ {
		out.WriteString(blankBoxRow(widths, numCols, ts, r.profile, tablePL, tablePR))
	}
	if len(headers) > 0 {
		out.WriteString(renderTableRow(headers, widths, numCols, ts, r.profile, tablePL, tablePR))
		out.WriteString(drawHBorder(borderWidths, ts.header, ts.color, r.profile))
	}
	for i, row := range rows {
		if i > 0 {
			out.WriteString(drawHBorder(borderWidths, ts.rowSep, ts.color, r.profile))
		}
		out.WriteString(renderTableRow(row, widths, numCols, ts, r.profile, tablePL, tablePR))
	}
	for i := 0; i < tablePB; i++ {
		out.WriteString(blankBoxRow(widths, numCols, ts, r.profile, tablePL, tablePR))
	}
	out.WriteString(drawHBorder(borderWidths, ts.bottom, ts.color, r.profile))
	if captionText != "" && captionSide == "bottom" {
		out.WriteString(centerText(captionText, tableW) + "\n")
	}
	return wrapTableMargin(out.String(), tableML, tableMR)
}

// wrapTableMargin applies the <table> element's own margin-left/right as
// blank space outside its fully-rendered text (padding is applied inside the
// border box itself, in the main body of renderTable, since CSS padding sits
// between the border and the content, not outside the border like margin).
//
// Right-side fill (margin-right) uses U+00A0 (non-breaking space) instead of
// a plain space. When this table is nested inside another table's cell, the
// outer table computes its cell text via plainInlineText, which right-trims
// plain trailing spaces — that would silently delete this table's own right
// margin on every level of nesting. A trailing U+00A0 survives that trim;
// Render converts it back to a plain space before returning the final string.
// nbsp is U+00A0 (non-breaking space). See wrapTableMargin for why it's used
// instead of a plain space for a table's own right margin.
const nbsp = "\u00A0"

func wrapTableMargin(s string, ml, mr int) string {
	if ml > 0 || mr > 0 {
		s = applyLineEdges(s, strings.Repeat(" ", ml), strings.Repeat(nbsp, mr))
	}
	return s
}

func namedTableStyleDefault() tableStyle {
	ts, _ := namedTableStyle("normal")
	return ts
}

func buildTableColumns(headers []tableCell, rows [][]tableCell, numCols int) []colConstraints {
	cols := make([]colConstraints, numCols)
	for i := 0; i < numCols; i++ {
		if i < len(headers) {
			cols[i] = headers[i].constraints
			cols[i].natural = maxVisibleLineWidth(headers[i].text) + headers[i].paddingLeft + headers[i].paddingRight
		}
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= numCols {
				break
			}
			if w := maxVisibleLineWidth(c.text) + c.paddingLeft + c.paddingRight; w > cols[i].natural {
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
		for _, line := range strings.Split(strings.TrimRight(cells[i].text, "\n"), "\n") {
			if cells[i].noWrap {
				cells[i].lines = append(cells[i].lines, truncateToWidth(line, contentW, cells[i].textOverflow))
			} else {
				cells[i].lines = append(cells[i].lines, wordWrapANSI(line, contentW, "break-word")...)
			}
		}
		if len(cells[i].lines) == 0 {
			cells[i].lines = []string{""}
		}
		if pt := cells[i].paddingTop; pt > 0 {
			blank := make([]string, pt, pt+len(cells[i].lines))
			cells[i].lines = append(blank, cells[i].lines...)
		}
		if pb := cells[i].paddingBottom; pb > 0 {
			cells[i].lines = append(cells[i].lines, make([]string, pb)...)
		}
	}
}

func renderTableRow(cells []tableCell, widths []int, numCols int, ts tableStyle, p colorprofile.Profile, boxPL, boxPR int) string {
	height := 1
	for i := 0; i < numCols && i < len(cells); i++ {
		if h := len(cells[i].lines); h > height {
			height = h
		}
	}
	paint := makePainter(ts.color, p)
	var sb strings.Builder
	for lineIdx := 0; lineIdx < height; lineIdx++ {
		sb.WriteString(paint(ts.left))
		if boxPL > 0 {
			sb.WriteString(strings.Repeat(" ", boxPL))
		}
		for i := 0; i < numCols; i++ {
			if i > 0 {
				sb.WriteString(paint(ts.sep))
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
		sb.WriteString(paint(ts.right))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// blankBoxRow draws one bordered but content-free row (left border + blank
// interior + right border), used to render padding-top/padding-bottom on a
// <table>: unlike margin, CSS padding sits inside the border box, so these
// rows must still carry the left/right border characters.
func blankBoxRow(widths []int, numCols int, ts tableStyle, p colorprofile.Profile, boxPL, boxPR int) string {
	paint := makePainter(ts.color, p)
	interior := sum(widths) + (numCols-1)*runeLen(ts.sep) + boxPL + boxPR
	return paint(ts.left) + strings.Repeat(" ", interior) + paint(ts.right) + "\n"
}
