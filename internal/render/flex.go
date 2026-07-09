package render

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// This file implements htmlterm's CSS flexbox subset: display:flex/
// inline-flex, flex-direction:row|row-reverse|column|column-reverse,
// justify-content, align-items, align-self, order, gap/row-gap/column-gap,
// and flex-grow (flex-basis resolves the item's starting main-axis size
// before growth is distributed). See CSS.md's "Flexbox" section for the
// exact supported subset and its documented non-goals (flex-wrap,
// align-content, flex-shrink, baseline alignment).

// flexItem is one direct element child of a flex container considered for
// layout, together with the sizing inputs the main-axis pass needs.
type flexItem struct {
	node  *html.Node
	decls map[string]string
	grow  float64
	order int
}

// itemAlign resolves align-self's fallback to the container's align-items:
// "auto" (the property's real default) and unset both mean "defer to the
// container," matching CSS's own align-self semantics.
func itemAlign(it flexItem, containerAlign string) string {
	if v := it.decls["align-self"]; v != "" && v != "auto" {
		return v
	}
	return containerAlign
}

// reverseFlexItems returns items in reverse order, for row-reverse/
// column-reverse — applied after order-based sorting, matching CSS's own
// "order determines position, then the reverse direction flips the whole
// sequence" behavior.
func reverseFlexItems(items []flexItem) []flexItem {
	out := make([]flexItem, len(items))
	for i, it := range items {
		out[len(items)-1-i] = it
	}
	return out
}

// isFlexDisplay reports whether display is one of the flex container values.
func isFlexDisplay(display string) bool {
	return display == "flex" || display == "inline-flex"
}

// parseFlexDirection resolves flex-direction into (isColumn, reverse).
// row-reverse/column-reverse are recognized by their "-reverse" suffix;
// anything else (including an unset/invalid value) falls back to row,
// matching flex-direction's own default.
func parseFlexDirection(decls map[string]string) (isColumn, reverse bool) {
	direction := decls["flex-direction"]
	return strings.HasPrefix(direction, "column"), strings.HasSuffix(direction, "-reverse")
}

// layoutFlex dispatches to layoutFlexRow/layoutFlexColumn per decls'
// flex-direction — the single call site every renderFlexContentBox/
// renderInlineFlexContent/measureNaturalWidth entry point shares, so
// direction parsing can't drift between them.
func (r *Engine) layoutFlex(n *html.Node, decls map[string]string, innerW int) (box, map[*html.Node]Rect) {
	isColumn, reverse := parseFlexDirection(decls)
	if isColumn {
		return r.layoutFlexColumn(n, decls, innerW, reverse)
	}
	return r.layoutFlexRow(n, decls, innerW, reverse)
}

// parseFlexGrow parses flex-grow (default 0; negative values are invalid per
// spec and treated as unset).
func parseFlexGrow(decls map[string]string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(decls["flex-grow"]), 64)
	if err != nil || f < 0 {
		return 0
	}
	return f
}

// parseOrder parses the CSS order property (default 0; invalid values fall
// back to 0 rather than erroring).
func parseOrder(decls map[string]string) int {
	n, err := strconv.Atoi(strings.TrimSpace(decls["order"]))
	if err != nil {
		return 0
	}
	return n
}

// parseGapLen parses row-gap/column-gap as an absolute rune count. Percentage
// gaps are not supported (parseSizeVal's pct return is discarded), matching
// this engine's existing padding/border sizing model.
func parseGapLen(v string) int {
	abs, _, ok := parseSizeVal(v)
	if !ok {
		return 0
	}
	return abs
}

// collectFlexItems gathers n's direct element children that participate in
// flex layout: text nodes directly inside a flex container are not rendered
// (wrap loose text in a <span> to include it — see CSS.md), and any child
// with display:none is skipped, matching normal flow. Items are stable-sorted
// by the CSS order property (default 0), preserving document order among
// ties — row-reverse/column-reverse (reverseFlexItems) is applied on top of
// this order-sorted sequence by the caller, matching CSS's own layering of
// the two.
func (r *Engine) collectFlexItems(n *html.Node) []flexItem {
	var items []flexItem
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || isSkippedContentElement(c.Data) {
			continue
		}
		decls := r.resolveDecls(c)
		if decls["display"] == "none" {
			continue
		}
		items = append(items, flexItem{node: c, decls: decls, grow: parseFlexGrow(decls), order: parseOrder(decls)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].order < items[j].order })
	return items
}

// renderFlexItemBox renders one flex item's own box at the given resolved
// width, dispatching to a nested flex layout when the item is itself a flex
// container (display:flex/inline-flex nests normally) or the ordinary block
// box model otherwise — flex items are blockified regardless of their own
// declared display, matching real CSS.
func (r *Engine) renderFlexItemBox(it flexItem, width int) (box, map[*html.Node]Rect) {
	if isFlexDisplay(it.decls["display"]) {
		return r.renderFlexContentBox(it.node, it.decls, width)
	}
	return r.renderBlockContentBox(it.node, it.decls, width)
}

// measureNaturalWidth renders it at a generous width cap purely to measure
// its intrinsic content width (for flex-basis:auto/width:auto items), then
// discards the render — r.quoteDepth is saved/restored around the call so
// this trial render leaves no state behind for the real render that follows.
// Safe to call twice per item: counters are pre-resolved once per document
// into r.counterMap (see counter.go) before any rendering starts, so this
// isn't a double-increment risk, and r.liveScrollOffsets/r.liveContentOffsets
// are plain map[node]value assignments the item's real render overwrites
// unconditionally afterward.
func (r *Engine) measureNaturalWidth(it flexItem, cap int) int {
	saved := r.quoteDepth
	var b box
	if isFlexDisplay(it.decls["display"]) {
		// Unlike renderFlexItemBox's real (final) render, measurement must
		// not go through renderFlexContentBox: that function always fills
		// its result to the full width it's given (matching a block-level
		// flex container's normal CSS behavior), which would make every
		// width:auto nested flex container measure as "however wide cap
		// is" instead of its own natural content width. layoutFlex is the
		// unpadded layout pass underneath it.
		b, _ = r.layoutFlex(it.node, it.decls, cap)
	} else {
		b, _ = r.renderBlockContentBox(it.node, it.decls, cap)
	}
	r.quoteDepth = saved
	return b.width
}

// resolveMainBasis resolves a row-direction flex item's starting main-axis
// (horizontal) size: flex-basis (if set and not auto) takes priority, then
// width, then the item's own measured natural content width.
func (r *Engine) resolveMainBasis(it flexItem, innerW int) int {
	if v := it.decls["flex-basis"]; v != "" && v != "auto" {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				return max(1, int(pct*float64(innerW)))
			}
			return max(1, abs)
		}
	}
	if v := it.decls["width"]; v != "" {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				return max(1, int(pct*float64(innerW)))
			}
			return max(1, abs)
		}
	}
	return max(1, r.measureNaturalWidth(it, innerW))
}

// resolveCrossWidth resolves a column-direction flex item's cross-axis
// (horizontal) width when align-items isn't stretch: width if set, else the
// item's own measured natural content width, both capped to innerW.
func (r *Engine) resolveCrossWidth(it flexItem, innerW int) int {
	if v := it.decls["width"]; v != "" {
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				return max(1, min(innerW, int(pct*float64(innerW))))
			}
			return max(1, min(innerW, abs))
		}
	}
	return max(1, min(innerW, r.measureNaturalWidth(it, innerW)))
}

// distributeJustify resolves justify-content's leftover-main-axis-space
// distribution into a leading pad (before the first item) and n-1 extra
// per-gap amounts (added on top of the base column-gap between items).
// leftover <= 0 (no free space, or the row already overflows) always yields
// no extra spacing — matching this engine's no-shrink flex-grow pass, which
// already consumes all leftover space itself whenever any item can grow.
func distributeJustify(justify string, leftover, n int) (leadPad int, gaps []int) {
	if n > 1 {
		gaps = make([]int, n-1)
	}
	if leftover <= 0 {
		return 0, gaps
	}
	switch justify {
	case "flex-end":
		leadPad = leftover
	case "center":
		leadPad = leftover / 2
	case "space-between":
		if n < 2 {
			return 0, gaps
		}
		base := leftover / (n - 1)
		rem := leftover % (n - 1)
		for i := range gaps {
			gaps[i] = base
			if i < rem {
				gaps[i]++
			}
		}
	case "space-around":
		unit := leftover / n
		rem := leftover % n
		leadPad = unit/2 + rem/2
		for i := range gaps {
			gaps[i] = unit
		}
	}
	return leadPad, gaps
}

// crossOffset resolves align-items' vertical offset (row direction) for one
// item within the row's tallest item height. "stretch" (default) and
// "flex-start" both place content flush at the top — this engine has no way
// to stretch text content itself, so "stretch" is approximated by padding
// the item's box with blank lines up to height, same visual result as a
// real stretched box with blank interior.
func crossOffset(align string, height, itemHeight int) int {
	extra := height - itemHeight
	if extra <= 0 {
		return 0
	}
	switch align {
	case "center":
		return extra / 2
	case "flex-end":
		return extra
	default:
		return 0
	}
}

// padBoxVertical pads b to exactly height lines, inserting topOffset blank
// lines before its content (for align-items:center/flex-end) and filling any
// remaining rows after it with blank lines of b's own width.
func padBoxVertical(b box, height, topOffset int) box {
	if len(b.lines) >= height && topOffset == 0 {
		return b
	}
	blank := strings.Repeat(" ", b.width)
	lines := make([]string, 0, height)
	for range topOffset {
		lines = append(lines, blank)
	}
	lines = append(lines, b.lines...)
	for len(lines) < height {
		lines = append(lines, blank)
	}
	return box{lines: lines, width: b.width}
}

// layoutFlexRow lays out items left to right (or right to left when reverse
// is set, for flex-direction:row-reverse): main axis (flex-grow/
// justify-content) is horizontal, cross axis (align-items/align-self) is
// vertical.
func (r *Engine) layoutFlexRow(n *html.Node, decls map[string]string, innerW int, reverse bool) (box, map[*html.Node]Rect) {
	items := r.collectFlexItems(n)
	if reverse {
		items = reverseFlexItems(items)
	}
	if len(items) == 0 {
		return box{lines: []string{""}}, nil
	}
	gap := parseGapLen(decls["column-gap"])
	totalGap := gap * (len(items) - 1)
	availForItems := max(0, innerW-totalGap)

	widths := make([]int, len(items))
	totalGrow := 0.0
	for i, it := range items {
		widths[i] = r.resolveMainBasis(it, innerW)
		totalGrow += it.grow
	}
	leftover := availForItems - sum(widths)
	if leftover > 0 && totalGrow > 0 {
		assigned := 0
		lastGrowIdx := -1
		for i, it := range items {
			if it.grow <= 0 {
				continue
			}
			lastGrowIdx = i
			share := int(float64(leftover)*it.grow/totalGrow + 0.5)
			widths[i] += share
			assigned += share
		}
		widths[lastGrowIdx] += leftover - assigned
		leftover = 0
	}

	itemBoxes := make([]box, len(items))
	itemPositions := make([]map[*html.Node]Rect, len(items))
	height := 1
	for i, it := range items {
		w := max(1, widths[i])
		b, pos := r.renderFlexItemBox(it, w)
		// An item with no CSS width of its own doesn't auto-fill the
		// available width the way a plain block box would (renderBlockContentBox
		// only pads to its available width when something — an explicit
		// width, text-align, or a border — requires it), so this main-axis
		// pass's resolved width must be enforced explicitly here, or a grown
		// item's extra space would collapse back to its own natural width.
		b = alignLinesBox(b, it.decls["text-align"], w)
		itemBoxes[i] = b
		itemPositions[i] = pos
		height = max(height, len(b.lines))
	}

	align := decls["align-items"]
	leadPad, extraGaps := distributeJustify(decls["justify-content"], leftover, len(items))

	rowLines := make([]string, height)
	if leadPad > 0 {
		blank := strings.Repeat(" ", leadPad)
		for i := range rowLines {
			rowLines[i] = blank
		}
	}
	positions := map[*html.Node]Rect{}
	colStart := leadPad
	for i, it := range items {
		offset := crossOffset(itemAlign(it, align), height, len(itemBoxes[i].lines))
		padded := padBoxVertical(itemBoxes[i], height, offset)
		for li := range rowLines {
			rowLines[li] += padded.lines[li]
		}
		positions[it.node] = Rect{Row: offset, Col: colStart, Width: widths[i], Height: len(itemBoxes[i].lines)}
		if len(itemPositions[i]) > 0 {
			positions = mergePositions(positions, itemPositions[i], offset, colStart)
		}
		colStart += widths[i]
		if i < len(items)-1 {
			g := gap + extraGaps[i]
			colStart += g
			if g > 0 {
				blank := strings.Repeat(" ", g)
				for li := range rowLines {
					rowLines[li] += blank
				}
			}
		}
	}
	return box{lines: rowLines, width: linesWidth(rowLines)}, positions
}

// layoutFlexColumn lays out items top to bottom: cross axis (align-items) is
// horizontal (align-items/align-self). There is no main-axis (vertical)
// distribution pass in this v1 — flex-grow and justify-content only matter
// once a container has an explicit main-axis size to grow/distribute into,
// and this engine has no notion of an explicit flex-container height yet;
// items simply stack with row-gap between them (flex-start main-axis
// behavior). reverse stacks bottom to top, for flex-direction:
// column-reverse. See CSS.md.
func (r *Engine) layoutFlexColumn(n *html.Node, decls map[string]string, innerW int, reverse bool) (box, map[*html.Node]Rect) {
	items := r.collectFlexItems(n)
	if reverse {
		items = reverseFlexItems(items)
	}
	if len(items) == 0 {
		return box{lines: []string{""}}, nil
	}
	rowGap := parseGapLen(decls["row-gap"])
	align := decls["align-items"]

	var lines []string
	positions := map[*html.Node]Rect{}
	row := 0
	for i, it := range items {
		if i > 0 {
			for range rowGap {
				lines = append(lines, "")
				row++
			}
		}
		itAlign := itemAlign(it, align)
		w := innerW
		if itAlign != "" && itAlign != "stretch" {
			w = r.resolveCrossWidth(it, innerW)
		}
		b, pos := r.renderFlexItemBox(it, max(1, w))
		colOffset := 0
		switch itAlign {
		case "center":
			colOffset = max(0, (innerW-b.width)/2)
		case "flex-end":
			colOffset = max(0, innerW-b.width)
		}
		prefix := ""
		if colOffset > 0 {
			prefix = strings.Repeat(" ", colOffset)
		}
		for _, ln := range b.lines {
			lines = append(lines, prefix+ln)
		}
		positions[it.node] = Rect{Row: row, Col: colOffset, Width: b.width, Height: len(b.lines)}
		if len(pos) > 0 {
			positions = mergePositions(positions, pos, row, colOffset)
		}
		row += len(b.lines)
	}
	return box{lines: lines, width: linesWidth(lines)}, positions
}

// renderFlexContentBox renders a block-level (display:flex) flex container:
// the same margin/border/padding box model as renderBlockContentBox, wrapped
// around flex item layout instead of inline content flow. See CSS.md's
// Flexbox section for the supported subset.
func (r *Engine) renderFlexContentBox(n *html.Node, decls map[string]string, availWidth int) (box, map[*html.Node]Rect) {
	bl, br, bt, bb, tlCorner, trCorner, blCorner, brCorner := resolveBoxBorders(decls)
	ml, mlAuto := resolveMarginSide(decls["margin-left"], availWidth)
	mr, mrAuto := resolveMarginSide(decls["margin-right"], availWidth)
	pl := parsePaddingLen(decls["padding-left"])
	pr := parsePaddingLen(decls["padding-right"])
	pt := parsePaddingLen(decls["padding-top"])
	pb := parsePaddingLen(decls["padding-bottom"])
	hBorderWidth := availWidth - ml - mr

	hasExplicitWidth := false
	if totalW, constrained := resolveWidthConstraints(decls, availWidth, availWidth); constrained {
		inner := max(1, totalW-ml-runeLen(bl.char)-pl-pr-runeLen(br.char)-mr)
		hBorderWidth = runeLen(bl.char) + pl + inner + pr + runeLen(br.char)
		hasExplicitWidth = true
	}
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		ml, mr = splitAutoMargins(remaining, ml, mr, mlAuto, mrAuto)
	}

	avail := hBorderWidth - runeLen(bl.char) - runeLen(br.char)
	var innerW int
	pl, pr, innerW = clampCellPadding(avail, pl, pr)
	if innerW < 1 {
		innerW = 1
	}

	content, positions := r.layoutFlex(n, decls, innerW)
	// A block-level flex container fills its available width by default,
	// same as any other block box — pad any leftover columns after the last
	// item/row (a row narrower than innerW, or a column item narrower than
	// innerW under a non-stretch align-items) rather than leaving a
	// ragged-width box.
	content = padLinesToWidthBox(content, innerW)

	if pt > 0 || pb > 0 {
		blank := strings.Repeat(" ", innerW)
		lines := content.lines
		if pt > 0 {
			padded := make([]string, 0, pt+len(lines))
			for range pt {
				padded = append(padded, blank)
			}
			lines = append(padded, lines...)
			if len(positions) > 0 {
				positions = mergePositions(nil, positions, pt, 0)
			}
		}
		if pb > 0 {
			for range pb {
				lines = append(lines, blank)
			}
		}
		content = box{lines: lines, width: linesWidth(lines)}
	}
	if pl > 0 || pr > 0 {
		content = applyLineEdgesBox(content, strings.Repeat(" ", pl), strings.Repeat(" ", pr))
		if len(positions) > 0 {
			positions = mergePositions(nil, positions, 0, pl)
		}
	}
	if bl.char != "" || br.char != "" {
		content = applyBlockBordersBox(content, bl, br, r.profile)
		if len(positions) > 0 {
			positions = mergePositions(nil, positions, 0, runeLen(bl.char))
		}
	}
	isEmpty := len(content.lines) == 1 && content.lines[0] == ""
	topRuleDrawn := false
	if top := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth, r.profile); top != "" {
		if isEmpty {
			content.lines = []string{top}
		} else {
			content.lines = append([]string{top}, content.lines...)
		}
		content.width = linesWidth(content.lines)
		topRuleDrawn = true
	}
	if bot := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth, r.profile); bot != "" {
		if isEmpty {
			content.lines = []string{bot}
		} else {
			content.lines = append(content.lines, bot)
		}
		content.width = linesWidth(content.lines)
	}
	if topRuleDrawn && len(positions) > 0 {
		positions = mergePositions(nil, positions, 1, 0)
	}
	if ml > 0 || mr > 0 {
		content = applyLineEdgesBox(content, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
		if len(positions) > 0 {
			positions = mergePositions(nil, positions, 0, ml)
		}
	}
	if decls["visibility"] == "hidden" {
		content = blankVisibleContentBox(content)
	}
	if r.liveContentOffsets == nil {
		r.liveContentOffsets = map[*html.Node]int{}
	}
	rowShift := pt
	if topRuleDrawn {
		rowShift++
	}
	r.liveContentOffsets[n] = rowShift
	return content, positions
}

// renderInlineFlexContent renders a display:inline-flex element's row/column
// layout at availWidth (a wrap-context bound, not a literal target width —
// same convention inline-block already uses). Returned as a plain string:
// like inline-block, an inline-flex container is one atomic trackable unit
// (see inline.go's nested "inline-block" case for the identical, already-
// accepted rationale) — its own position is tracked by the caller boxing
// this string, but individual descendants inside it are not.
func (r *Engine) renderInlineFlexContent(n *html.Node, decls map[string]string, availWidth int) string {
	b, _ := r.layoutFlex(n, decls, availWidth)
	return b.join()
}
