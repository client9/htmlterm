package render

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/client9/htmlterm/internal/cssengine"
	"github.com/client9/htmlterm/internal/textcell"
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
// border-collapse:separate). resolveBoxBorders (block.go) is the only
// remaining consumer of this data (the legacy per-table shared-frame model
// this struct originally also served, via sep, header, and rowSep fields
// drawn by table_render.go's now-deleted renderTableBody, was retired in
// favor of
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
// glyph a named preset uses for one outer edge. Shared by
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

// borderStylePresets maps each of htmlterm's own named border-style values
// to the glyphs it draws. It is the render-side half of the border-style
// vocabulary; cssengine.BorderStyleNames (internal/cssengine/css.go) is the
// canonical copy of the name set itself, consulted while parsing the border
// and border-<edge> shorthands, well before any of these glyphs come into
// it. The two must carry exactly the same keys, checked automatically by
// TestNamedTableStyleMatchesBorderStyleVocabulary rather than left to drift
// apart the way a comment-only "kept in sync by hand" note would allow.
// table_collapse.go's junction-glyph tables are a separate case still kept in
// sync by hand, not derived from this map, since real collapsed-border
// junctions need the full T/cross combinations a tableStyle doesn't carry.
var borderStylePresets = map[string]tableStyle{
	"solid": {
		top:    &hBorder{"┌", "─", "┐"},
		bottom: &hBorder{"└", "─", "┘"},
		left:   "│", right: "│",
	},
	"rounded": {
		top:    &hBorder{"╭", "─", "╮"},
		bottom: &hBorder{"╰", "─", "╯"},
		left:   "│", right: "│",
	},
	"heavy": {
		top:    &hBorder{"┏", "━", "┓"},
		bottom: &hBorder{"┗", "━", "┛"},
		left:   "┃", right: "┃",
	},
	"double": {
		top:    &hBorder{"╔", "═", "╗"},
		bottom: &hBorder{"╚", "═", "╝"},
		left:   "║", right: "║",
	},
	"markdown": {left: "|", right: "|"},
	"standard": {},
	"hidden":   {},
	"none":     {},
}

// namedTableStyle returns the preset for a given border-style value, used by
// resolveBoxBorders (block.go) for any border box: block elements, and
// per-cell borders under border-collapse:separate.
func namedTableStyle(name string) (tableStyle, bool) {
	ts, ok := borderStylePresets[name]
	return ts, ok
}

// colConstraints holds horizontal sizing constraints for one table column.
//
// The percentDelta/minPercentDelta/maxPercentDelta fields carry
// cellBoxSizingDelta's adjustment for a percent-based constraint, which
// can't be applied immediately the way fixed/minWidth/maxWidth's is: a
// percentage isn't resolved to an absolute char count until sizeColumns (or
// effectiveMinMax for the min/max pair) multiplies it against contentWidth,
// so the box-sizing adjustment has to ride along and get added at that same
// point instead. See cellConstraints and cellBoxSizingDelta.
//
// calc/minCalc/maxCalc hold a raw calc()/min()/max()/clamp() declaration
// verbatim, for the same reason percent can't be resolved immediately: it
// needs contentWidth, which isn't known until sizeColumns/effectiveMinMax
// run. They exist as a separate string field rather than folding into
// fixed or percent because a calc() result isn't decomposable into "exactly
// one of an absolute count or a fraction" the way this struct's other
// fields assume — `calc(50% + 4)` is both at once, resolved as a whole
// against contentWidth via resolveLength, not as independent fixed and
// percent contributions. See docs/proposals/CSS_MATH.md's table.go caveat
// for why this couldn't just reuse the fixed/percent split. calcDelta/
// minCalcDelta/maxCalcDelta are cellBoxSizingDelta's adjustment for a calc
// value, added after resolveLength resolves it, mirroring percentDelta.
type colConstraints struct {
	natural         int     // max content width (runes) across all rows
	fixed           int     // exact char width from width= attribute or CSS width (0 = not set), already box-sizing-adjusted
	percent         float64 // CSS width as fraction of contentWidth (0 = not set; overrides fixed)
	percentDelta    int     // box-sizing adjustment for percent, added after percent resolves to an absolute width
	calc            string  // raw calc()/min()/max()/clamp() CSS width declaration (empty = not set; overrides fixed and percent)
	calcDelta       int     // box-sizing adjustment for calc, added after it resolves to an absolute width
	minWidth        int     // CSS min-width in chars (0 = none), already box-sizing-adjusted
	minPercent      float64 // CSS min-width as fraction (0 = none)
	minPercentDelta int     // box-sizing adjustment for minPercent, added after it resolves
	minCalc         string  // raw calc()/min()/max()/clamp() CSS min-width declaration (empty = none; overrides minWidth and minPercent)
	minCalcDelta    int     // box-sizing adjustment for minCalc, added after it resolves
	maxWidth        int     // CSS max-width in chars (0 = none), already box-sizing-adjusted
	maxPercent      float64 // CSS max-width as fraction (0 = none)
	maxPercentDelta int     // box-sizing adjustment for maxPercent, added after it resolves
	maxCalc         string  // raw calc()/min()/max()/clamp() CSS max-width declaration (empty = none; overrides maxWidth and maxPercent)
	maxCalcDelta    int     // box-sizing adjustment for maxCalc, added after it resolves
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

// cellBoxSizingDelta returns how much to adjust one of a cell's declared
// width/min-width/max-width values so that, combined with the padding
// subtraction fillGridCellTokens and composeSeparateGrid already do
// unconditionally downstream, the resolved column slot width ends up
// meaning what the cell's own box-sizing says the declared value means. A
// `<th>`/`<td>`'s box-sizing was, until this, never read at all
// (docs/TABLES.md, COMPATIBILITY.md); this is the one place that adds it.
//
// content-box (real CSS's own initial value): the declared value is pure
// content, with padding and border both meant to add on top of it. Padding
// is added back here so the later, unconditional padding subtraction nets
// back out to exactly the declared content size; border needs no
// adjustment, since it was never subtracted from a column's own slot width
// to begin with (border is tracked as separate "column border overhead",
// added on top after sizeColumns runs; see separateColumnBorderOverhead and
// collapsedBorders.colOverhead).
//
// border-box (the UA default): the declared value already includes
// padding, which the unconditional downstream subtraction already assumes
// with no adjustment needed here. It also includes border, which the
// downstream "column border overhead" lane assumes is never part of a
// column's own slot width, so borderW is subtracted here to compensate:
// once that overhead is added back on top after sizeColumns, the total
// nets back out to the declared value.
//
// borderW must be 0 under border-collapse:collapse. A collapsed border
// segment is shared, table-wide grid-line state resolved by conflict
// resolution (table_collapse.go), not any single cell's own border box, so
// there is no per-cell border width to subtract there; a border-box cell
// under collapse degrades to "padding only", the same adjustment
// content-box already gets under separate mode's own border term being
// zero. Callers pass the cell's own left+right border character width
// (resolveBoxBorders) under border-collapse:separate, where each cell owns
// a real border box, and 0 under collapse.
func cellBoxSizingDelta(decls map[string]string, padding, borderW int) int {
	if isBorderBox(decls) {
		return -borderW
	}
	return padding
}

// cellConstraints extracts layout constraints from a <th> or <td> node's
// already-resolved declarations, adjusted by cellBoxSizingDelta so a
// declared width/min-width/max-width means what its own box-sizing says
// regardless of which of this file's two passes, up-front estimation or the
// final per-cell pass, calls this. The legacy HTML width attribute is
// deliberately not consulted: in real-world markup (especially HTML email)
// it's almost always a pixel value, and there's no reliable way to convert
// pixels to terminal columns. Treating it as a char count, as this engine
// used to) forces columns to absurd widths. Use CSS width (e.g. "10ch") for
// an unambiguous fixed character width.
//
// borderW is the cell's own left+right border character width under
// border-collapse:separate, and must be 0 under border-collapse:collapse;
// see cellBoxSizingDelta. fixed/minWidth/maxWidth, already absolute at this
// point, are adjusted immediately. A percent value can't be: it isn't
// resolved to an absolute char count until sizeColumns or effectiveMinMax
// multiplies it against contentWidth, so its delta is carried in
// percentDelta/minPercentDelta/maxPercentDelta and applied there instead.
func (r *Engine) cellConstraints(decls map[string]string, borderW int) colConstraints {
	var c colConstraints
	padding := parsePaddingLen(decls["padding-left"]) + parsePaddingLen(decls["padding-right"])
	delta := cellBoxSizingDelta(decls, padding, borderW)
	if v, ok := decls["width"]; ok {
		v = strings.TrimSpace(v)
		if cssengine.IsMathFunctionToken(v) {
			c.calc, c.calcDelta = v, delta
		} else if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.percent = pct
				c.percentDelta = delta
			} else {
				c.fixed = max(1, abs+delta)
			}
		}
	}
	if v, ok := decls["min-width"]; ok {
		v = strings.TrimSpace(v)
		if cssengine.IsMathFunctionToken(v) {
			c.minCalc, c.minCalcDelta = v, delta
		} else if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.minPercent = pct
				c.minPercentDelta = delta
			} else {
				c.minWidth = max(0, abs+delta)
			}
		}
	}
	if v, ok := decls["max-width"]; ok {
		v = strings.TrimSpace(v)
		if cssengine.IsMathFunctionToken(v) {
			c.maxCalc, c.maxCalcDelta = v, delta
		} else if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				c.maxPercent = pct
				c.maxPercentDelta = delta
			} else {
				c.maxWidth = max(1, abs+delta)
			}
		}
	}
	return c
}

// explicitColumnWidth resolves a column's own declared width — calc(),
// percent, or fixed, tried in that order since they're mutually exclusive
// by construction (cellConstraints only ever sets one) — against
// contentWidth. ok is false when no width was declared at all, or a calc()
// failed to resolve; contentWidth is always definite here, so the only
// realistic way that happens is a malformed declaration, and the caller's
// fallback is the same either way: natural-width sizing, as if no width had
// been declared.
func explicitColumnWidth(c colConstraints, contentWidth int) (int, bool) {
	switch {
	case c.calc != "":
		if v, ok := resolveLength(c.calc, contentWidth, true); ok {
			return max(1, v+c.calcDelta), true
		}
		return 0, false
	case c.percent > 0:
		return max(1, int(c.percent*float64(contentWidth))+c.percentDelta), true
	case c.fixed > 0:
		return c.fixed, true
	default:
		return 0, false
	}
}

// sizeColumns computes final column widths. contentWidth is the space available
// for cell content (terminal width minus all border/separator overhead).
// Percentage and calc() columns are resolved to absolute widths first; fixed
// columns are immune to the expand/shrink pass. Flexible columns start at
// their natural width (clamped by min/max). Extra space is distributed only
// to flexible columns; overage is reclaimed from flexible, percentage, and
// calc() columns.
func sizeColumns(cols []colConstraints, contentWidth int, fullWidth bool) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		if w, ok := explicitColumnWidth(c, contentWidth); ok {
			widths[i] = w
			continue
		}
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

	total := sum(widths)

	isConstrained := func(c colConstraints) bool {
		_, ok := explicitColumnWidth(c, contentWidth)
		return ok
	}

	switch {
	case fullWidth && total < contentWidth:
		// Distribute extra space across uncapped flex columns, weighted by
		// each column's own width, already clamped by natural, min, and max, so an
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
// combining absolute, percentage, and calc() constraints. max=0 means
// uncapped. A calc() that fails to resolve (contentWidth is always definite
// here, so only a malformed declaration triggers this) contributes nothing,
// the same as an unset min-width/max-width.
func effectiveMinMax(c colConstraints, contentWidth int) (minW, maxW int) {
	minW = c.minWidth
	if c.minCalc != "" {
		if v, ok := resolveLength(c.minCalc, contentWidth, true); ok {
			// Floored at 0 for the same reason the minPercent branch below
			// is: see its own comment.
			if mp := max(0, v+c.minCalcDelta); mp > minW {
				minW = mp
			}
		}
	} else if c.minPercent > 0 {
		// Floored at 0, minWidth's own "no constraint" sentinel: a
		// box-sizing delta that would push this below 0 asks for a minimum
		// content-box shrunk past its own chrome, which is already
		// trivially satisfied by any width, the same as no minimum at all.
		mp := max(0, int(c.minPercent*float64(contentWidth))+c.minPercentDelta)
		if mp > minW {
			minW = mp
		}
	}
	maxW = c.maxWidth
	if c.maxCalc != "" {
		if v, ok := resolveLength(c.maxCalc, contentWidth, true); ok {
			// Floored at 1 for the same reason the maxPercent branch below
			// is: see its own comment.
			mp := max(1, v+c.maxCalcDelta)
			if maxW == 0 || mp < maxW {
				maxW = mp
			}
		}
	} else if c.maxPercent > 0 {
		// Floored at 1, not 0: 0 is maxWidth's own "uncapped" sentinel, and
		// an author who declared a real percent maximum still gets one
		// clamped as tight as this engine can render, one cell, rather than
		// having a box-sizing delta silently erase the cap.
		mp := max(1, int(c.maxPercent*float64(contentWidth))+c.maxPercentDelta)
		if maxW == 0 || mp < maxW {
			maxW = mp
		}
	}
	return
}

// drawHRule builds a horizontal line: left + (fill tiled to segments[0]
// columns) + junction + (fill tiled to segments[1] columns) + ... + right,
// all colored with color. Returns "" when fill is empty or "none". No
// trailing newline is added.
func drawHRule(segments []int, fill, color, left, junction, right string, p colorprofile.Profile) string {
	if fill == "" || fill == "none" || len(segments) == 0 {
		return ""
	}
	paint := makePainter(color, p)
	var sb strings.Builder
	sb.WriteString(paint(left))
	for i, w := range segments {
		sb.WriteString(paint(tileFillToWidth(fill, w)))
		if i < len(segments)-1 {
			sb.WriteString(paint(junction))
		}
	}
	sb.WriteString(paint(right))
	return sb.String()
}

// tileFillToWidth repeats fill until it reaches exactly width columns. fill
// is usually a single character, and `strings.Repeat(fill, width)` would be
// enough on its own, but border-top and border-bottom also accept a
// multi-character literal (`border-top: "=-"`), documented in CSS.md as
// repeating to make a patterned rule. Repeating a multi-column fill by count
// rather than by column overshoots the box's own width, since a segment's
// column budget and its rune count are different numbers for anything wider
// than one column. The last, partial repetition is truncated to the exact
// column that remains and padded with spaces if the fill's own grapheme
// boundaries don't land on it, matching the "pad plain text to fixed width
// first" rule every other fixed-width row in this package follows.
func tileFillToWidth(fill string, width int) string {
	if width <= 0 {
		return ""
	}
	unit := textcell.Width(fill)
	if unit <= 0 {
		return strings.Repeat(" ", width)
	}
	full := width / unit
	var b strings.Builder
	b.WriteString(strings.Repeat(fill, full))
	if remaining := width - full*unit; remaining > 0 {
		prefix := textcell.VisiblePrefix(fill, remaining)
		b.WriteString(prefix)
		b.WriteString(strings.Repeat(" ", remaining-textcell.Width(prefix)))
	}
	return b.String()
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
		// Custom string value: strip surrounding quotes.
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
