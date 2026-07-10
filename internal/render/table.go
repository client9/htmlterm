package render

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
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
	color  string   // ANSI color fallback for any edge without its own override below

	// Per-edge color overrides (border-top-color etc., mirroring block
	// elements' border-*-color). Empty means "use color" - internal lines
	// (header/rowSep/sep) have no per-edge override and always use color
	// directly, since there's no CSS property that targets just them.
	topColor    string
	rightColor  string
	bottomColor string
	leftColor   string
}

// colorOrFallback returns specific if set, else fallback - resolves a
// per-edge border-*-color override against the table's uniform border-color.
func colorOrFallback(specific, fallback string) string {
	if specific != "" {
		return specific
	}
	return fallback
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

// namedTableStyle returns the preset for a given border-style value.
func namedTableStyle(name string) (tableStyle, bool) {
	switch name {
	case "solid":
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
//	border-style: solid | rounded | thick | double | markdown | standard | hidden | none
//	border-top/-right/-bottom/-left: same literal-glyph + shorthand grammar as
//	  block elements (see resolveBorderEdgeChar) - a quoted string is a
//	  literal character, an unquoted value is the standard CSS
//	  <style>/<style> <color>/<width> <style> <color> shorthand, where
//	  <style> is a border-style preset name. An edge resolving to "" (an
//	  explicit none/hidden style) removes that whole line, corners included
//	  - for border-left/border-right this also clears the corresponding
//	  corner/junction glyph on every horizontal line, matching the
//	  "no left/right frame at all" meaning "none" has always had here.
//	border-top/-right/-bottom/-left-color: per-edge color override
//	border-top-mid/border-bottom-mid: T-junction character where a column
//	  separator meets the outer top/bottom border
//	border-left-mid/border-right-mid: T-junction character where an internal
//	  (header or row) separator meets the left/right edge - header and
//	  rowSep always share this glyph, so one property covers both
//	border-center: cross-junction character at internal column/row
//	  intersections - same header/rowSep sharing as border-*-mid above
//	border-top-left-corner/border-top-right-corner/border-bottom-left-corner/
//	  border-bottom-right-corner: outer corner character override, same
//	  literal-only model as the identically-named block element properties
//	border-columns: none                (removes column separator)
//	border-rows: solid                  (enables row separators)
//	border-header: none                 (removes header separator)
//	border-color: <color>               (fallback for any edge without its own override)
func applyTableCSSToStyle(ts tableStyle, decls map[string]string) tableStyle {
	if val := decls["border-style"]; val != "" {
		if ns, ok := namedTableStyle(val); ok {
			ts = ns
		}
	}

	topChar, topPresent := resolveBorderEdgeChar(decls["border-top"], edgeGlyphTop)
	if topPresent {
		switch {
		case topChar == "":
			ts.top = nil
		case ts.top != nil:
			ts.top.fill = topChar
		default:
			ts.top = &hBorder{fill: topChar}
		}
	}
	bottomChar, bottomPresent := resolveBorderEdgeChar(decls["border-bottom"], edgeGlyphBottom)
	if bottomPresent {
		switch {
		case bottomChar == "":
			ts.bottom = nil
		case ts.bottom != nil:
			ts.bottom.fill = bottomChar
		default:
			ts.bottom = &hBorder{fill: bottomChar}
		}
	}
	leftChar, leftPresent := resolveBorderEdgeChar(decls["border-left"], edgeGlyphLeft)
	if leftPresent {
		ts.left = leftChar
		if leftChar == "" {
			for _, b := range []*hBorder{ts.top, ts.header, ts.rowSep, ts.bottom} {
				if b != nil {
					b.left = ""
				}
			}
		}
	}
	rightChar, rightPresent := resolveBorderEdgeChar(decls["border-right"], edgeGlyphRight)
	if rightPresent {
		ts.right = rightChar
		if rightChar == "" {
			for _, b := range []*hBorder{ts.top, ts.header, ts.rowSep, ts.bottom} {
				if b != nil {
					b.right = ""
				}
			}
		}
	}

	// Internal separator lines reuse the outer top border's own fill
	// character rather than getting an independent property: every
	// built-in preset already keeps these identical, and a table with two
	// different dash styles in one frame isn't a real use case.
	if ts.top != nil {
		if ts.header != nil {
			ts.header.fill = ts.top.fill
		}
		if ts.rowSep != nil {
			ts.rowSep.fill = ts.top.fill
		}
	}

	if val := decls["border-top-color"]; val != "" {
		ts.topColor = val
	}
	if val := decls["border-right-color"]; val != "" {
		ts.rightColor = val
	}
	if val := decls["border-bottom-color"]; val != "" {
		ts.bottomColor = val
	}
	if val := decls["border-left-color"]; val != "" {
		ts.leftColor = val
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

	if v := parseCSSString(decls["border-top-mid"]); v != "" && ts.top != nil {
		ts.top.mid = v
	}
	if v := parseCSSString(decls["border-bottom-mid"]); v != "" && ts.bottom != nil {
		ts.bottom.mid = v
	}
	if v := parseCSSString(decls["border-left-mid"]); v != "" {
		if ts.header != nil {
			ts.header.left = v
		}
		if ts.rowSep != nil {
			ts.rowSep.left = v
		}
	}
	if v := parseCSSString(decls["border-right-mid"]); v != "" {
		if ts.header != nil {
			ts.header.right = v
		}
		if ts.rowSep != nil {
			ts.rowSep.right = v
		}
	}
	if v := parseCSSString(decls["border-center"]); v != "" {
		if ts.header != nil {
			ts.header.mid = v
		}
		if ts.rowSep != nil {
			ts.rowSep.mid = v
		}
	}
	if v := parseCSSString(decls["border-top-left-corner"]); v != "" && ts.top != nil {
		ts.top.left = v
	}
	if v := parseCSSString(decls["border-top-right-corner"]); v != "" && ts.top != nil {
		ts.top.right = v
	}
	if v := parseCSSString(decls["border-bottom-left-corner"]); v != "" && ts.bottom != nil {
		ts.bottom.left = v
	}
	if v := parseCSSString(decls["border-bottom-right-corner"]); v != "" && ts.bottom != nil {
		ts.bottom.right = v
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
