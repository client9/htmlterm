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

// renderTable renders a <table> node using the custom table engine.
func (r *Renderer) renderTable(n *html.Node) string {
	var headers []tableCell
	var rows [][]tableCell
	var captionText string

	colDecls := r.collectColDecls(n)

	var collect func(*html.Node)
	collect = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "caption":
				if captionText == "" {
					captionText = plainInlineText(stripANSI(r.renderInlineAcc(c, inlineStyle{}, r.width)))
				}
			case "thead", "tbody", "tfoot":
				collect(c)
			case "tr":
				var cells []tableCell
				isHeader := false
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.Data != "th" && td.Data != "td") {
						continue
					}
					if td.Data == "th" {
						isHeader = true
					}
					tdDecls := r.resolveDecls(td)
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
					pl, pr, pt, pb := 0, 0, 0, 0
					if v := tdDecls["padding-left"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pl = abs
						}
					}
					if v := tdDecls["padding-right"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pr = abs
						}
					}
					if v := tdDecls["padding-top"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pt = abs
						}
					}
					if v := tdDecls["padding-bottom"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pb = abs
						}
					}
					cellText := plainInlineText(stripANSI(r.renderInlineAcc(td, inlineStyle{}, r.width)))
					if tdDecls["visibility"] == "hidden" {
						cellText = ""
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
	collect(n)

	numCols := len(headers)
	if numCols == 0 && len(rows) > 0 {
		numCols = len(rows[0])
	}
	if numCols == 0 {
		return ""
	}

	tableDecls := r.resolveDecls(n)
	ts := applyTableCSSToStyle(namedTableStyleDefault(), tableDecls)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%"
	cols := buildTableColumns(headers, rows, numCols)

	sepW := runeLen(ts.sep)
	overhead := runeLen(ts.left) + (numCols-1)*sepW + runeLen(ts.right)
	widths := sizeColumns(cols, r.width-overhead, fullWidth)
	fillTableCellLines(headers, widths, numCols)
	for i := range rows {
		fillTableCellLines(rows[i], widths, numCols)
	}

	captionSide := tableDecls["caption-side"]
	var out strings.Builder
	if captionText != "" && captionSide != "bottom" {
		// Center caption over the table width (default: top).
		tableW := sum(widths) + overhead
		out.WriteString(centerText(captionText, tableW) + "\n")
	}
	out.WriteString(drawHBorder(widths, ts.top, ts.color, r.profile))
	if len(headers) > 0 {
		out.WriteString(renderTableRow(headers, widths, numCols, ts, r.profile))
		out.WriteString(drawHBorder(widths, ts.header, ts.color, r.profile))
	}
	for i, row := range rows {
		if i > 0 {
			out.WriteString(drawHBorder(widths, ts.rowSep, ts.color, r.profile))
		}
		out.WriteString(renderTableRow(row, widths, numCols, ts, r.profile))
	}
	out.WriteString(drawHBorder(widths, ts.bottom, ts.color, r.profile))
	if captionText != "" && captionSide == "bottom" {
		tableW := sum(widths) + overhead
		out.WriteString(centerText(captionText, tableW) + "\n")
	}
	return out.String()
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
			cols[i].natural = runeLen(headers[i].text) + headers[i].paddingLeft + headers[i].paddingRight
		}
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= numCols {
				break
			}
			if w := runeLen(c.text) + c.paddingLeft + c.paddingRight; w > cols[i].natural {
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
		pl := cells[i].paddingLeft
		pr := cells[i].paddingRight
		contentW := widths[i] - pl - pr
		if contentW < 1 {
			contentW = 1
		}
		if cells[i].noWrap {
			cells[i].lines = []string{truncateToWidth(cells[i].text, contentW, cells[i].textOverflow)}
		} else {
			cells[i].lines = wrapToWidth(cells[i].text, contentW)
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

func renderTableRow(cells []tableCell, widths []int, numCols int, ts tableStyle, p colorprofile.Profile) string {
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
			contentW := widths[i] - c.paddingLeft - c.paddingRight
			if contentW < 1 {
				contentW = 1
			}
			rendered := c.cellStyle.render(alignLines(line, c.textAlign, contentW), p)
			if c.paddingLeft > 0 {
				rendered = strings.Repeat(" ", c.paddingLeft) + rendered
			}
			if c.paddingRight > 0 {
				rendered += strings.Repeat(" ", c.paddingRight)
			}
			sb.WriteString(rendered)
		}
		sb.WriteString(paint(ts.right))
		sb.WriteByte('\n')
	}
	return sb.String()
}
