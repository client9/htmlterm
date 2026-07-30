package render

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// hBorder describes one horizontal outer-edge rule (top or bottom) for a
// generic border box. An empty fill means the border is omitted entirely
// (same as nil *hBorder).
type hBorder struct {
	left  string // leftmost (corner) character
	fill  string // repeated fill character
	right string // rightmost (corner) character
}

// tableStyle controls the outer border characters a named preset supplies
// for a generic border box (block element, or a single cell under
// border-collapse:separate) — resolveBoxBorders (block.go) is the only
// remaining consumer of this data (the legacy per-table shared-frame model
// this struct originally also served — sep/header/rowSep fields, drawn by
// table_render.go's now-deleted renderTableBody — was retired in favor of
// real per-cell borders; see table_separate.go/table_collapse.go, and
// docs/TABLES.md).
type tableStyle struct {
	top    *hBorder // outer top border (nil = omit)
	bottom *hBorder // outer bottom border (nil = omit)
	left   string   // left edge ("" = none)
	right  string   // right edge ("" = none)
	color  string   // ANSI color fallback for any edge without its own override below
}

// edgeGlyphTop, edgeGlyphBottom, edgeGlyphLeft, edgeGlyphRight extract the
// glyph a named preset uses for one outer edge - shared by
// resolveBorderEdgeChar's callers in both block.go (block border edges) and
// table.go (table border edges), since both resolve against the same
// tableStyle preset table.
func edgeGlyphTop(ts tableStyle) string {
	if ts.top != nil {
		return ts.top.fill
	}
	return ""
}

func edgeGlyphBottom(ts tableStyle) string {
	if ts.bottom != nil {
		return ts.bottom.fill
	}
	return ""
}

func edgeGlyphLeft(ts tableStyle) string  { return ts.left }
func edgeGlyphRight(ts tableStyle) string { return ts.right }

// namedTableStyle returns the preset for a given border-style value —
// htmlterm's own whole-box vocabulary (not real CSS's per-edge line-style
// keywords), used by resolveBoxBorders (block.go) for any border box:
// block elements, and per-cell borders under border-collapse:separate.
// table_collapse.go's junction-glyph tables are built from the same
// characters directly (kept in sync by hand, not derived from this
// function), since real collapsed-border junctions need the full T/cross
// combinations this struct no longer carries.
func namedTableStyle(name string) (tableStyle, bool) {
	switch name {
	case "solid":
		return tableStyle{
			top:    &hBorder{"┌", "─", "┐"},
			bottom: &hBorder{"└", "─", "┘"},
			left:   "│", right: "│",
		}, true
	case "rounded":
		return tableStyle{
			top:    &hBorder{"╭", "─", "╮"},
			bottom: &hBorder{"╰", "─", "╯"},
			left:   "│", right: "│",
		}, true
	case "heavy":
		return tableStyle{
			top:    &hBorder{"┏", "━", "┓"},
			bottom: &hBorder{"┗", "━", "┛"},
			left:   "┃", right: "┃",
		}, true
	case "double":
		return tableStyle{
			top:    &hBorder{"╔", "═", "╗"},
			bottom: &hBorder{"╚", "═", "╝"},
			left:   "║", right: "║",
		}, true
	case "markdown":
		return tableStyle{left: "|", right: "|"}, true
	case "standard", "hidden", "none":
		return tableStyle{}, true
	}
	return tableStyle{}, false
}

// colConstraints holds horizontal sizing constraints for one table column.
type colConstraints struct {
	natural    int     // max content width (runes) across all rows
	fixed      int     // exact char width from width= attribute or CSS width (0 = not set)
	percent    float64 // CSS width as fraction of contentWidth (0 = not set; overrides fixed)
	minWidth   int     // CSS min-width in chars (0 = none)
	minPercent float64 // CSS min-width as fraction (0 = none)
	maxWidth   int     // CSS max-width in chars (0 = none)
	maxPercent float64 // CSS max-width as fraction (0 = none)
}

// parseSizeVal parses a CSS/HTML size string: bare integer, Nch, or N%.
// Returns (abs rune count, percent 0.0–1.0, ok). Exactly one of abs>0 or pct>0 is set on success.
func parseSizeVal(s string) (abs int, pct float64, ok bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err == nil && f > 0 {
			return 0, f / 100.0, true
		}
		return
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "ch"))
	if err == nil && n > 0 {
		return n, 0, true
	}
	return
}

// cellConstraints extracts layout constraints from a <th> or <td> node's
// already-resolved declarations. The legacy HTML width attribute is
// deliberately not consulted: in real-world markup (especially HTML email)
// it's almost always a pixel value, and there's no reliable way to convert
// pixels to terminal columns — treating it as a char count (as this engine
// used to) forces columns to absurd widths. Use CSS width (e.g. "10ch") for
// an unambiguous fixed character width.
func (r *Engine) cellConstraints(decls map[string]string) colConstraints {
	var c colConstraints
	if v, ok := decls["width"]; ok {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.percent = pct
				c.fixed = 0
			} else {
				c.fixed = abs
			}
		}
	}
	if v, ok := decls["min-width"]; ok {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.minPercent = pct
			} else {
				c.minWidth = abs
			}
		}
	}
	if v, ok := decls["max-width"]; ok {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.maxPercent = pct
			} else {
				c.maxWidth = abs
			}
		}
	}
	return c
}

// sizeColumns computes final column widths. contentWidth is the space available
// for cell content (terminal width minus all border/separator overhead).
// Percentage columns are resolved to absolute widths first; fixed columns are
// immune to the expand/shrink pass. Flexible columns start at their natural
// width (clamped by min/max). Extra space is distributed only to flexible
// columns; overage is reclaimed from flexible and percentage columns.
func sizeColumns(cols []colConstraints, contentWidth int, fullWidth bool) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		switch {
		case c.percent > 0:
			widths[i] = max(1, int(c.percent*float64(contentWidth)))
		case c.fixed > 0:
			widths[i] = c.fixed
		default:
			w := c.natural
			minW, maxW := effectiveMinMax(c, contentWidth)
			if minW > 0 && w < minW {
				w = minW
			}
			if maxW > 0 && w > maxW {
				w = maxW
			}
			widths[i] = w
		}
	}

	total := sum(widths)

	isConstrained := func(c colConstraints) bool { return c.fixed > 0 || c.percent > 0 }

	switch {
	case fullWidth && total < contentWidth:
		// Distribute extra space across uncapped flex columns, weighted by
		// each column's own (already natural/min/max-clamped) width - so an
		// empty column (weight 0) gets none of it and a content-heavy column
		// gets most of it, approximating CSS auto-table-layout's
		// proportional-to-preferred-width distribution instead of splitting
		// evenly across every flex column regardless of content. Falls back
		// to an even split only when every flex column's weight is 0 (e.g.
		// every cell in every flex column is empty), to avoid dividing by
		// zero. If a column would exceed its maxWidth we cap it and carry the
		// overflow back; each outer iteration saturates at least one column,
		// so the loop runs at most numCols times total.
		for extra := contentWidth - total; extra > 0; {
			var flex []int
			weightSum := 0
			for i, c := range cols {
				if isConstrained(c) {
					continue
				}
				_, maxW := effectiveMinMax(c, contentWidth)
				if maxW > 0 && widths[i] >= maxW {
					continue
				}
				flex = append(flex, i)
				weightSum += widths[i]
			}
			if len(flex) == 0 {
				break
			}
			remaining := extra
			extra = 0
			assigned := 0
			for k, i := range flex {
				var add int
				if k == len(flex)-1 {
					add = remaining - assigned
				} else if weightSum == 0 {
					add = remaining / len(flex)
					assigned += add
				} else {
					add = remaining * widths[i] / weightSum
					assigned += add
				}
				_, maxW := effectiveMinMax(cols[i], contentWidth)
				if maxW > 0 && widths[i]+add > maxW {
					extra += (widths[i] + add) - maxW
					widths[i] = maxW
				} else {
					widths[i] += add
				}
			}
		}

	case total > contentWidth:
		// Shrink flexible and percentage columns one unit at a time from the
		// widest, respecting effective min (floor of 1).
		for overage := total - contentWidth; overage > 0; {
			best, bestIdx := -1, -1
			for i, c := range cols {
				if c.fixed > 0 {
					continue
				}
				minW, _ := effectiveMinMax(c, contentWidth)
				floor := minW
				if floor < 1 {
					floor = 1
				}
				if widths[i] > floor && widths[i] > best {
					best, bestIdx = widths[i], i
				}
			}
			if bestIdx < 0 {
				break
			}
			widths[bestIdx]--
			overage--
		}
	}

	return widths
}

// effectiveMinMax returns the resolved min and max widths for a column,
// combining absolute and percentage constraints. max=0 means uncapped.
func effectiveMinMax(c colConstraints, contentWidth int) (minW, maxW int) {
	minW = c.minWidth
	if c.minPercent > 0 {
		mp := int(c.minPercent * float64(contentWidth))
		if mp > minW {
			minW = mp
		}
	}
	maxW = c.maxWidth
	if c.maxPercent > 0 {
		mp := int(c.maxPercent * float64(contentWidth))
		if maxW == 0 || mp < maxW {
			maxW = mp
		}
	}
	return
}

// drawHRule builds a horizontal line: left + (fill×segments[0]) + junction +
// (fill×segments[1]) + ... + right, all colored with color. Returns "" when
// fill is empty or "none". No trailing newline is added.
func drawHRule(segments []int, fill, color, left, junction, right string, p colorprofile.Profile) string {
	if fill == "" || fill == "none" || len(segments) == 0 {
		return ""
	}
	paint := makePainter(color, p)
	var sb strings.Builder
	sb.WriteString(paint(left))
	for i, w := range segments {
		sb.WriteString(paint(strings.Repeat(fill, w)))
		if i < len(segments)-1 {
			sb.WriteString(paint(junction))
		}
	}
	sb.WriteString(paint(right))
	return sb.String()
}

// makePainter returns a function that applies a border color if set.
func makePainter(cssColor string, p colorprofile.Profile) func(string) string {
	c := parseCSSColor(cssColor)
	if c == nil {
		return func(s string) string { return s }
	}
	converted := p.Convert(c)
	if converted == nil {
		return func(s string) string { return s }
	}
	st := ansi.Style{}.ForegroundColor(converted)
	return func(s string) string {
		if s == "" {
			return ""
		}
		return st.Styled(s)
	}
}

// textOverflowSuffix maps a CSS text-overflow value to the truncation suffix.
func textOverflowSuffix(val string) string {
	switch val {
	case "clip", "":
		return ""
	case "ellipsis":
		return "…"
	default:
		// Custom string value — strip surrounding quotes.
		return sanitizeTerminalText(strings.Trim(val, `"'`), false)
	}
}

func sum(ints []int) int {
	n := 0
	for _, v := range ints {
		n += v
	}
	return n
}

// centerText centers s within a field of width w, padding with spaces.
// If s is wider than w it is returned as-is.
func centerText(s string, w int) string {
	runes := []rune(s)
	n := len(runes)
	if n >= w {
		return s
	}
	total := w - n
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
