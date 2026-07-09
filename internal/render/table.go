package render

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// hBorder describes one horizontal separator line drawn between rows.
// An empty fill means the border is omitted entirely (same as nil *hBorder).
type hBorder struct {
	left  string // leftmost character
	fill  string // repeated fill character
	mid   string // column junction character
	right string // rightmost character
}

// tableStyle controls every border character in a rendered table.
type tableStyle struct {
	top    *hBorder // outer top border (nil = omit)
	header *hBorder // header/data separator (nil = omit)
	rowSep *hBorder // between data rows (nil = omit)
	bottom *hBorder // outer bottom border (nil = omit)
	left   string   // left edge of each data row ("" = none)
	sep    string   // column separator in data rows ("" = none)
	right  string   // right edge of each data row ("" = none)
	color  string   // ANSI color applied to all border characters
}

// namedTableStyle returns the preset for a given border-style value.
func namedTableStyle(name string) (tableStyle, bool) {
	switch name {
	case "normal":
		return tableStyle{
			top:    &hBorder{"┌", "─", "┬", "┐"},
			header: &hBorder{"├", "─", "┼", "┤"},
			bottom: &hBorder{"└", "─", "┴", "┘"},
			left:   "│", sep: "│", right: "│",
		}, true
	case "rounded":
		return tableStyle{
			top:    &hBorder{"╭", "─", "┬", "╮"},
			header: &hBorder{"├", "─", "┼", "┤"},
			bottom: &hBorder{"╰", "─", "┴", "╯"},
			left:   "│", sep: "│", right: "│",
		}, true
	case "thick":
		return tableStyle{
			top:    &hBorder{"┏", "━", "┳", "┓"},
			header: &hBorder{"┣", "━", "╋", "┫"},
			bottom: &hBorder{"┗", "━", "┻", "┛"},
			left:   "┃", sep: "┃", right: "┃",
		}, true
	case "double":
		return tableStyle{
			top:    &hBorder{"╔", "═", "╦", "╗"},
			header: &hBorder{"╠", "═", "╬", "╣"},
			bottom: &hBorder{"╚", "═", "╩", "╝"},
			left:   "║", sep: "║", right: "║",
		}, true
	case "markdown":
		return tableStyle{
			header: &hBorder{"|", "-", "|", "|"},
			left:   "|", sep: "|", right: "|",
		}, true
	case "standard":
		// No outer frame, no column separators; header underlined with ─.
		// Columns separated by a single space.
		return tableStyle{
			header: &hBorder{"", "─", " ", ""},
			sep:    " ",
		}, true
	case "hidden", "none":
		return tableStyle{sep: " "}, true
	}
	return tableStyle{}, false
}

// applyTableCSSToStyle applies border-* CSS declarations from a <table> element
// to ts, returning the modified style. Supported properties:
//
//	border-style: normal | rounded | thick | double | markdown | standard | hidden | none
//	border-top/bottom/left/right: none  (disables that outer edge)
//	border-columns: none                (removes column separator)
//	border-rows: solid                  (enables row separators)
//	border-header: none                 (removes header separator)
//	border-color: <color>
func applyTableCSSToStyle(ts tableStyle, decls map[string]string) tableStyle {
	if val := decls["border-style"]; val != "" {
		if ns, ok := namedTableStyle(val); ok {
			ts = ns
		}
	}
	if decls["border-top"] == "none" {
		ts.top = nil
	}
	if decls["border-bottom"] == "none" {
		ts.bottom = nil
	}
	if decls["border-left"] == "none" {
		ts.left = ""
		for _, b := range []*hBorder{ts.top, ts.header, ts.rowSep, ts.bottom} {
			if b != nil {
				b.left = ""
			}
		}
	}
	if decls["border-right"] == "none" {
		ts.right = ""
		for _, b := range []*hBorder{ts.top, ts.header, ts.rowSep, ts.bottom} {
			if b != nil {
				b.right = ""
			}
		}
	}
	if decls["border-columns"] == "none" {
		ts.sep = ""
	}
	if val := decls["border-rows"]; val != "" {
		if val == "none" {
			ts.rowSep = nil
		} else if ts.rowSep == nil {
			ts.rowSep = &hBorder{"├", "─", "┼", "┤"}
		}
	}
	if decls["border-header"] == "none" {
		ts.header = nil
	}
	if val := decls["border-color"]; val != "" {
		ts.color = val
	}
	return ts
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

// cellConstraints extracts layout constraints from a <th> or <td> node using
// already-resolved declarations. The node is still needed for the HTML width attribute.
func (r *Engine) cellConstraints(n *html.Node, decls map[string]string) colConstraints {
	var c colConstraints
	// HTML width attribute: always an absolute char count.
	if w, err := strconv.Atoi(nodeAttr(n, "width")); err == nil && w > 0 {
		c.fixed = w
	}
	// CSS width: may be absolute or percentage (overrides HTML attribute).
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
		// Distribute extra space across uncapped flex columns using integer
		// division. If a column would exceed its maxWidth we cap it and carry
		// the overflow back; each outer iteration saturates at least one column,
		// so the loop runs at most numCols times total.
		for extra := contentWidth - total; extra > 0; {
			var flex []int
			for i, c := range cols {
				if isConstrained(c) {
					continue
				}
				_, maxW := effectiveMinMax(c, contentWidth)
				if maxW > 0 && widths[i] >= maxW {
					continue
				}
				flex = append(flex, i)
			}
			if len(flex) == 0 {
				break
			}
			base := extra / len(flex)
			rem := extra % len(flex)
			extra = 0
			for k, i := range flex {
				add := base
				if k < rem {
					add++
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

// drawHBorder renders one horizontal separator row given the computed column
// widths. Returns "" if b is nil (border omitted).
func drawHBorder(widths []int, b *hBorder, color string, p colorprofile.Profile) string {
	if b == nil {
		return ""
	}
	return drawHRule(widths, b.fill, color, b.left, b.mid, b.right, p) + "\n"
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

// runeLen returns the number of runes in s (used for terminal-column overhead).
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// truncateToWidth truncates s to at most width runes. suffix is appended when
// content is clipped (use "" for clip/no indicator, "…" for ellipsis, etc.).
func truncateToWidth(s string, width int, suffix string) string {
	if width <= 0 {
		return ""
	}
	if ansiVisibleLen(s) <= width {
		return s
	}
	cut := width - ansiVisibleLen(suffix)
	if cut <= 0 {
		return visiblePrefixWithTrailingEscapes(s, width)
	}
	return visiblePrefixWithTrailingEscapes(s, cut) + suffix
}

// textOverflowSuffix maps a CSS text-overflow value to the truncation suffix.
func textOverflowSuffix(val string) string {
	switch val {
	case "clip":
		return ""
	case "ellipsis", "":
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
