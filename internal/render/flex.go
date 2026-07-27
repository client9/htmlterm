package render

import (
	"maps"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// This file implements htmlterm's CSS flexbox subset: display:flex/
// inline-flex, flex-direction:row|row-reverse|column|column-reverse,
// justify-content, align-items, align-self, order, gap/row-gap/column-gap,
// flex-grow/flex-shrink/flex-basis (main-axis size resolution, resolved
// before grow/shrink distribute any leftover space - in row direction the
// leftover is always available since width is always definite; in column
// direction it's only available once the container has an explicit CSS
// height, this engine's only notion of a column container's main-axis
// size), and (row direction only) flex-wrap/align-content/
// margin-left/margin-right:auto (main-axis space absorption, overriding
// justify-content). See CSS.md's "Flexbox" section for the exact supported
// subset and its documented non-goals (column-direction wrap, baseline
// alignment, cross-axis margin:auto).

// flexItem is one direct element child of a flex container considered for
// layout, together with the sizing inputs the main-axis pass needs.
type flexItem struct {
	node   *html.Node
	decls  map[string]string
	grow   float64
	shrink float64
	order  int
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
	if r.measuringNaturalWidth && innerW > measureBlockWidthCap {
		// renderInlineFlexContent calls layoutFlex directly, bypassing
		// renderFlexContentBox's own clamp above - guard here too, since
		// this is the single shared entry point (see doc comment).
		innerW = measureBlockWidthCap
	}
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

// parseFlexShrink parses flex-shrink (default 1, per spec; negative values
// are invalid per spec and treated as unset, falling back to the same
// default).
func parseFlexShrink(decls map[string]string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(decls["flex-shrink"]), 64)
	if err != nil || f < 0 {
		return 1
	}
	return f
}

// flexShrinkFloor resolves how far a flex item may shrink below its resolved
// main-axis basis: an explicit min-width (row direction) or min-height
// (column direction) - including percentage, resolved against axisSize - if
// set, else 1. This engine has no min-content measurement to use as the real
// spec's automatic minimum, so content that can't actually fit at the shrunk
// size (an unbreakable word wider than its floor in row direction; any
// content taller than its floor in column direction, since text can't be
// forced into fewer lines) simply overflows past it, the same graceful
// degradation already used for the flex-shrink:0 case.
func flexShrinkFloor(decls map[string]string, key string, axisSize int) int {
	if v, ok := resolveCSSSize(decls[key], axisSize); ok && v > 1 {
		return v
	}
	return 1
}

// flexGrowCeilingHeight resolves how far flex-grow may increase a
// column-direction item's height: an explicit max-height (including
// percentage, resolved against the container's own definite height) if set,
// else 0 meaning uncapped. Row direction has no equivalent max-width ceiling
// on flex-grow - a separate, pre-existing gap (see CSS.md's Flexbox
// section) - so this is column-only.
func flexGrowCeilingHeight(decls map[string]string, containerHeight int) int {
	if h, ok := resolveCSSSize(decls["max-height"], containerHeight); ok && h > 0 {
		return h
	}
	return 0
}

// distributeFlexGrow adds each group member's proportional share of leftover
// to sizes[i] (i ranges over group, indices into the full item-count-sized
// sizes/grows/ceilings arrays - matching widths/mt/mb's own convention),
// weighted by grows[i], the last growable member absorbing the rounding
// remainder - shared by row direction's per-line width distribution and
// column direction's height distribution. ceilings, if non-nil, caps
// sizes[i]'s growth at ceilings[i] (0 meaning uncapped) - real CSS's
// max-width/max-height on a flex item. Returns however much of leftover
// wasn't actually absorbed (0 unless every growable member hit its ceiling
// before leftover was exhausted, or no member can grow at all).
func distributeFlexGrow(sizes []int, grows []float64, group []int, leftover int, ceilings []int) int {
	totalGrow := 0.0
	for _, i := range group {
		totalGrow += grows[i]
	}
	if totalGrow <= 0 {
		return leftover
	}
	assigned := 0
	lastGrowIdx := -1
	for _, i := range group {
		if grows[i] <= 0 {
			continue
		}
		lastGrowIdx = i
		share := int(float64(leftover)*grows[i]/totalGrow + 0.5)
		if ceilings != nil && ceilings[i] > 0 {
			if maxAdd := ceilings[i] - sizes[i]; share > maxAdd {
				share = max(0, maxAdd)
			}
		}
		sizes[i] += share
		assigned += share
	}
	extra := leftover - assigned
	if ceilings != nil && ceilings[lastGrowIdx] > 0 {
		if maxAdd := ceilings[lastGrowIdx] - sizes[lastGrowIdx]; extra > maxAdd {
			extra = max(0, maxAdd)
		}
	}
	sizes[lastGrowIdx] += extra
	assigned += extra
	return leftover - assigned
}

// distributeFlexShrink subtracts each group member's proportional share of
// deficit from sizes[i], weighted by shrinks[i]*sizes[i] (the real spec's
// "scaled flex shrink factor"), floored at floors[i], the last shrinkable
// member absorbing the rounding remainder - shared by row direction's width
// distribution and column direction's height distribution. Returns however
// much of deficit wasn't actually absorbed (0 unless every shrinkable member
// hit its floor before deficit was exhausted, or no member can shrink at
// all).
func distributeFlexShrink(sizes []int, shrinks []float64, floors []int, group []int, deficit int) int {
	scaled := make([]float64, len(sizes))
	totalScaled := 0.0
	for _, i := range group {
		if shrinks[i] <= 0 {
			continue
		}
		s := shrinks[i] * float64(sizes[i])
		scaled[i] = s
		totalScaled += s
	}
	if totalScaled <= 0 {
		return deficit
	}
	assigned := 0
	lastShrinkIdx := -1
	for _, i := range group {
		if scaled[i] <= 0 {
			continue
		}
		lastShrinkIdx = i
		reduce := int(float64(deficit)*scaled[i]/totalScaled + 0.5)
		if maxReduce := sizes[i] - floors[i]; reduce > maxReduce {
			reduce = max(0, maxReduce)
		}
		sizes[i] -= reduce
		assigned += reduce
	}
	extra := deficit - assigned
	if maxReduce := sizes[lastShrinkIdx] - floors[lastShrinkIdx]; extra > maxReduce {
		extra = max(0, maxReduce)
	}
	sizes[lastShrinkIdx] -= extra
	assigned += extra
	return deficit - assigned
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

// parseFlexWrap parses flex-wrap: "wrap" enables multi-line row layout;
// nowrap (the default, and any other/invalid value) keeps the single-line
// behavior row-direction flex layout has always had.
func parseFlexWrap(decls map[string]string) bool {
	return decls["flex-wrap"] == "wrap"
}

// resolveFlexContainerHeight resolves an explicit CSS height on a flex
// container to an absolute line count, for align-content's cross-axis
// distribution. Percentage heights are not resolved (this engine has no
// notion of a flex container's percentage-basis height), matching
// renderBlockContentBox's own height handling, which likewise only acts on
// parseSizeVal's absolute return.
func resolveFlexContainerHeight(decls map[string]string) (int, bool) {
	abs, _, ok := parseSizeVal(decls["height"])
	if !ok || abs <= 0 {
		return 0, false
	}
	return abs, true
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
		if decls["display"] == "none" || r.outOfFlow[c] {
			continue
		}
		items = append(items, flexItem{node: c, decls: decls, grow: parseFlexGrow(decls), shrink: parseFlexShrink(decls), order: parseOrder(decls)})
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
	return r.renderFlexItemBoxHeight(it, width, 0)
}

// renderFlexItemBoxHeight is renderFlexItemBox with an optional column-
// direction main-axis (vertical) size override: heightOverride > 0 injects a
// synthetic "height" declaration (taking priority over any "height" the item
// already declares - matching flex-basis's real-CSS precedence over the
// height property) before rendering, so the existing block box model's own
// height handling (block.go's fixed-height pad/clip logic) does the actual
// work for a non-flex item; heightOverride == 0 renders unmodified, identical
// to renderFlexItemBox. Used by layoutFlexColumn to resolve flex-basis and
// apply flex-grow/flex-shrink's height adjustments - see CSS.md's Flexbox
// section.
//
// decls is always cloned, even when heightOverride == 0: layoutFlexColumn
// can render the same item twice (a basis-measurement pass, then a second
// pass if flex-grow/flex-shrink changes its height), and block.go's own
// margin-top/margin-bottom collapse-through bookkeeping mutates its decls
// argument in place - without a clone here, that first render could leave a
// stale mutation in it.decls for the second render (or any later reader) to
// pick up. Row direction's layoutFlexLine renders every item exactly once,
// so this is strictly defensive there, not required by the row path itself.
func (r *Engine) renderFlexItemBoxHeight(it flexItem, width, heightOverride int) (box, map[*html.Node]Rect) {
	decls := maps.Clone(it.decls)
	if heightOverride > 0 {
		decls["height"] = strconv.Itoa(heightOverride)
		if ov := decls["overflow-y"]; ov == "scroll" || ov == "auto" {
			// This height is synthetic - it exists purely to drive
			// flex-basis/flex-grow/flex-shrink sizing, not a real declared
			// box height - so it must not activate this engine's
			// scrollbar-gutter reservation/live-scroll-offset tracking as an
			// unrelated side effect of flex layout math. An author's own
			// explicit "hidden"/"clip" clipping intent is left alone: that's
			// a legitimate combination with an explicit height, synthetic or
			// not.
			decls["overflow-y"] = "visible"
		}
	}
	if isFlexDisplay(decls["display"]) {
		b, pos := r.renderFlexContentBox(it.node, decls, width)
		if heightOverride > 0 && len(b.lines) < heightOverride {
			// Unlike renderBlockContentBox (block.go), renderFlexContentBox
			// has no notion of an explicit CSS height of its own - a nested
			// flex/inline-flex item's flex-basis/flex-grow/flex-shrink
			// height would otherwise have no effect at all. Pad here,
			// mirroring block.go's own "fixed height taller than content
			// pads with blank lines" behavior; a taller-than-heightOverride
			// result (the nested container already reached or exceeded it,
			// e.g. it's itself flex-direction:column with the same explicit
			// height) is left untouched, matching the graceful-overflow
			// convention used throughout this file.
			blank := strings.Repeat(" ", b.width)
			lines := append([]string(nil), b.lines...)
			for len(lines) < heightOverride {
				lines = append(lines, blank)
			}
			b = box{lines: lines, width: b.width}
		}
		return b, pos
	}
	return r.renderBlockContentBox(it.node, decls, width)
}

// parseColumnFlexBasis resolves a column-direction flex item's flex-basis to
// an absolute line count: 0 means unset/auto (or a percentage basis with no
// definite container height to resolve against - real CSS treats a
// percentage flex-basis as auto when the container's own main size is
// indefinite, which this engine only ever considers "definite" when an
// explicit CSS height is set on the container - see resolveFlexContainerHeight).
func parseColumnFlexBasis(decls map[string]string, containerHeight int, hasContainerHeight bool) int {
	v := decls["flex-basis"]
	if v == "" || v == "auto" {
		return 0
	}
	abs, pct, ok := parseSizeVal(v)
	if !ok {
		return 0
	}
	if pct > 0 {
		if !hasContainerHeight {
			return 0
		}
		return max(1, int(pct*float64(containerHeight)))
	}
	return max(1, abs)
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

// distributeAlignContent resolves align-content's leftover cross-axis space
// across a wrapped row container's lines into a leading blank-row count
// (before the first line) and n-1 extra row-gap amounts (added on top of the
// base row-gap between lines) - the same shape as distributeJustify, but
// operating on whole lines in the cross (vertical) axis rather than items in
// the main (horizontal) axis. leftover <= 0 (no explicit container height, or
// the wrapped lines already fill or exceed it) always yields no extra
// spacing. "stretch" (align-content's real default) has no cell-grid
// equivalent for growing each line's own items taller than their content, so
// it's approximated as flex-start (no distribution) rather than half
// implementing per-item vertical growth - see CSS.md.
func distributeAlignContent(align string, leftover, n int) (leadRows int, gaps []int) {
	if n > 1 {
		gaps = make([]int, n-1)
	}
	if leftover <= 0 {
		return 0, gaps
	}
	switch align {
	case "flex-end":
		leadRows = leftover
	case "center":
		leadRows = leftover / 2
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
		leadRows = unit/2 + rem/2
		for i := range gaps {
			gaps[i] = unit
		}
	}
	return leadRows, gaps
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

// breakFlexLines buckets item indices into flex lines for flex-wrap: wrap,
// using each item's already-resolved (pre-grow) main-axis width - real CSS's
// "hypothetical main size" used for line-breaking, before any flex-grow
// distribution (which happens per line, once a line's membership is fixed).
// An item that alone exceeds innerW still gets its own line rather than
// being dropped or splitting further - this engine has no notion of
// splitting a single item across lines, matching real CSS. nowrap (default)
// always returns a single line containing every item, matching row-direction
// flex layout's pre-flex-wrap behavior exactly.
func breakFlexLines(n int, widths []int, innerW, gap int, wrap bool) [][]int {
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	if !wrap {
		return [][]int{all}
	}
	var lines [][]int
	var cur []int
	running := 0
	for _, i := range all {
		w := widths[i]
		needed := w
		if len(cur) > 0 {
			needed = running + gap + w
		}
		if len(cur) > 0 && needed > innerW {
			lines = append(lines, cur)
			cur = []int{i}
			running = w
			continue
		}
		cur = append(cur, i)
		running = needed
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// layoutFlexLine lays out one flex line's worth of items (a subset of
// items/widths/mt/mb/mlAuto/mrAuto, named by group's indices into those
// slices): resolves flex-grow/flex-shrink within just this line, then any
// margin-left/margin-right:auto absorption, then justify-content/align-items
// exactly as row-direction flex layout always has for a single line. widths
// is mutated in place for group's own indices (each index belongs to exactly
// one line, so this is safe across repeated calls for other lines).
func (r *Engine) layoutFlexLine(items []flexItem, widths, mt, mb []int, mlAuto, mrAuto []bool, group []int, innerW, gap int, decls map[string]string) (box, int, map[*html.Node]Rect) {
	totalGap := gap * (len(group) - 1)
	availForItems := max(0, innerW-totalGap)

	sumW := 0
	for _, i := range group {
		sumW += widths[i]
	}
	grows := make([]float64, len(items))
	shrinks := make([]float64, len(items))
	floors := make([]int, len(items))
	for _, i := range group {
		grows[i] = items[i].grow
		shrinks[i] = items[i].shrink
		floors[i] = flexShrinkFloor(items[i].decls, "min-width", innerW)
	}
	leftover := availForItems - sumW
	switch {
	case leftover > 0:
		// Row direction has no max-width ceiling on flex-grow (a separate,
		// pre-existing gap - see CSS.md), so growth is always uncapped here.
		leftover = distributeFlexGrow(widths, grows, group, leftover, nil)
	case leftover < 0:
		// Overflow: shrink items proportionally to flex-shrink * basis width
		// (the real spec's "scaled flex shrink factor"), floored at
		// flexShrinkFloor.
		leftover = -distributeFlexShrink(widths, shrinks, floors, group, -leftover)
	}

	// margin-left/margin-right:auto absorbs whatever main-axis leftover
	// space remains after flex-grow/flex-shrink have already had their turn
	// (matching real CSS's ordering: flexible-length resolution happens
	// first, auto-margin alignment second), splitting it evenly across every
	// auto margin present anywhere in the line - and, when any exist,
	// overriding justify-content entirely for this line, same as real CSS.
	extraLeft := make([]int, len(group))
	extraRight := make([]int, len(group))
	marginsOverrideJustify := false
	if autoCount := 0; leftover > 0 {
		for _, i := range group {
			if mlAuto[i] {
				autoCount++
			}
			if mrAuto[i] {
				autoCount++
			}
		}
		if autoCount > 0 {
			share := leftover / autoCount
			rem := leftover % autoCount
			slot := 0
			for gi, i := range group {
				if mlAuto[i] {
					extraLeft[gi] = share
					if slot < rem {
						extraLeft[gi]++
					}
					slot++
				}
				if mrAuto[i] {
					extraRight[gi] = share
					if slot < rem {
						extraRight[gi]++
					}
					slot++
				}
			}
			leftover = 0
			marginsOverrideJustify = true
		}
	}

	itemBoxes := make(map[int]box, len(group))
	itemPositions := make(map[int]map[*html.Node]Rect, len(group))
	outerHeights := make(map[int]int, len(group))
	height := 1
	for _, i := range group {
		it := items[i]
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
		// The item's margin-top/margin-bottom widen its own cross-axis
		// footprint (its "outer" height) - align-items/align-self and the
		// line's own shared height are resolved against that outer height,
		// not the bare content height, matching real CSS (alignment
		// distributes free space around an item's margin box).
		outerHeights[i] = mt[i] + len(b.lines) + mb[i]
		height = max(height, outerHeights[i])
	}

	align := decls["align-items"]
	var leadPad int
	extraGaps := make([]int, max(0, len(group)-1))
	if !marginsOverrideJustify {
		leadPad, extraGaps = distributeJustify(decls["justify-content"], leftover, len(group))
	}

	rowLines := make([]string, height)
	if leadPad > 0 {
		blank := strings.Repeat(" ", leadPad)
		for i := range rowLines {
			rowLines[i] = blank
		}
	}
	positions := map[*html.Node]Rect{}
	colStart := leadPad
	for gi, i := range group {
		it := items[i]
		if extraLeft[gi] > 0 {
			blank := strings.Repeat(" ", extraLeft[gi])
			for li := range rowLines {
				rowLines[li] += blank
			}
			colStart += extraLeft[gi]
		}
		offset := mt[i] + crossOffset(itemAlign(it, align), height, outerHeights[i])
		padded := padBoxVertical(itemBoxes[i], height, offset)
		for li := range rowLines {
			rowLines[li] += padded.lines[li]
		}
		positions[it.node] = Rect{Row: offset, Col: colStart, Width: widths[i], Height: len(itemBoxes[i].lines)}
		if len(itemPositions[i]) > 0 {
			positions = mergePositions(positions, itemPositions[i], offset, colStart)
		}
		colStart += widths[i]
		if extraRight[gi] > 0 {
			blank := strings.Repeat(" ", extraRight[gi])
			for li := range rowLines {
				rowLines[li] += blank
			}
			colStart += extraRight[gi]
		}
		if gi < len(group)-1 {
			g := gap + extraGaps[gi]
			colStart += g
			if g > 0 {
				blank := strings.Repeat(" ", g)
				for li := range rowLines {
					rowLines[li] += blank
				}
			}
		}
	}
	return box{lines: rowLines, width: linesWidth(rowLines)}, height, positions
}

// layoutFlexRow lays out items left to right (or right to left when reverse
// is set, for flex-direction:row-reverse): main axis (flex-grow/
// justify-content) is horizontal, cross axis (align-items/align-self) is
// vertical. flex-wrap:wrap buckets items into multiple lines (breakFlexLines)
// stacked vertically with row-gap, and align-content distributes any extra
// cross-axis space across those lines when the container has an explicit
// height taller than their natural stack - see distributeAlignContent.
func (r *Engine) layoutFlexRow(n *html.Node, decls map[string]string, innerW int, reverse bool) (box, map[*html.Node]Rect) {
	items := r.collectFlexItems(n)
	if reverse {
		items = reverseFlexItems(items)
	}
	if len(items) == 0 {
		return box{lines: []string{""}}, nil
	}
	gap := parseGapLen(decls["column-gap"])
	rowGap := parseGapLen(decls["row-gap"])
	wrap := parseFlexWrap(decls)

	// Fixed (non-auto) margin-left/margin-right on a flex item need no
	// separate handling here: renderFlexItemBox dispatches to the ordinary
	// block box model (renderBlockContentBox), which already bakes
	// horizontal margin directly into an item's own returned box (and into
	// resolveMainBasis's natural-width measurement, which renders through
	// the same box model) regardless of flex context - adding it again here
	// would double it. margin-top/margin-bottom get no such treatment from
	// the block box model when the item's own top/bottom edge is open (no
	// border/padding/height): that's normally left for a block-flow caller
	// to apply externally via the collapse-through margin on decls (see
	// block.go), a convention flex layout doesn't participate in - flex
	// items never collapse margins with each other or the container at all
	// (real CSS: a flex formatting context doesn't collapse), so each
	// item's raw margin-top/margin-bottom is read directly here and applied
	// as its own cross-axis space (via outerHeight, below).
	//
	// margin-left/margin-right:auto is handled externally instead, in
	// layoutFlexLine, once each line's main-axis leftover space is known:
	// resolveMainBasis treats an auto side as contributing 0 to an item's
	// flex basis (matching real CSS), and the final render call below always
	// passes exactly the item's own resolved main-axis width as availWidth,
	// leaving renderBlockContentBox's own (unrelated) auto-margin-splitting
	// path nothing to distribute (remaining is always 0) - so it's safe to
	// leave decls untouched here and let layoutFlexLine own the real
	// distribution. margin-top/margin-bottom:auto is not supported (falls
	// back to parseMargin's 0) - see CSS.md's Flexbox "Not supported" list.
	mt := make([]int, len(items))
	mb := make([]int, len(items))
	mlAuto := make([]bool, len(items))
	mrAuto := make([]bool, len(items))
	widths := make([]int, len(items))
	for i, it := range items {
		mt[i] = parseMargin(it.decls["margin-top"])
		mb[i] = parseMargin(it.decls["margin-bottom"])
		mlAuto[i] = strings.TrimSpace(it.decls["margin-left"]) == "auto"
		mrAuto[i] = strings.TrimSpace(it.decls["margin-right"]) == "auto"
		widths[i] = r.resolveMainBasis(it, innerW)
	}

	lineGroups := breakFlexLines(len(items), widths, innerW, gap, wrap)
	type lineResult struct {
		b   box
		h   int
		pos map[*html.Node]Rect
	}
	results := make([]lineResult, len(lineGroups))
	for li, group := range lineGroups {
		b, h, pos := r.layoutFlexLine(items, widths, mt, mb, mlAuto, mrAuto, group, innerW, gap, decls)
		results[li] = lineResult{b, h, pos}
	}

	if len(results) == 1 {
		return results[0].b, results[0].pos
	}

	contentHeight := 0
	for i, res := range results {
		contentHeight += res.h
		if i > 0 {
			contentHeight += rowGap
		}
	}
	leadRows := 0
	wantHeight := 0
	extraGapsBetweenLines := make([]int, len(results)-1)
	if h, ok := resolveFlexContainerHeight(decls); ok && h > contentHeight {
		wantHeight = h
		leadRows, extraGapsBetweenLines = distributeAlignContent(decls["align-content"], wantHeight-contentHeight, len(results))
	}

	var allLines []string
	for range leadRows {
		allLines = append(allLines, "")
	}
	positions := map[*html.Node]Rect{}
	rowOffset := leadRows
	for i, res := range results {
		if len(res.pos) > 0 {
			positions = mergePositions(positions, res.pos, rowOffset, 0)
		}
		allLines = append(allLines, res.b.lines...)
		rowOffset += res.h
		if i < len(results)-1 {
			g := rowGap + extraGapsBetweenLines[i]
			for range g {
				allLines = append(allLines, "")
			}
			rowOffset += g
		}
	}
	// leadRows/extraGapsBetweenLines only place free space around/between
	// lines - flex-start/stretch (the default when align-content is unset or
	// "stretch"/"flex-start") and "center" (half the leftover, not all of it)
	// can each still leave the container short of its explicit height, so any
	// remainder is always flushed as trailing blank rows - this is what makes
	// the container actually reach wantHeight, not align-content's own
	// distribution.
	for len(allLines) < wantHeight {
		allLines = append(allLines, "")
	}
	return box{lines: allLines, width: linesWidth(allLines)}, positions
}

// layoutFlexColumn lays out items top to bottom: cross axis (align-items) is
// horizontal (align-items/align-self), main axis (flex-grow/flex-shrink/
// justify-content) is vertical. flex-basis sets an item's starting main-axis
// (line count) size the same way it sets width in row direction.
// flex-grow/flex-shrink/justify-content only have free space to work with
// once the container has a definite main-axis size — an explicit CSS
// `height` (resolveFlexContainerHeight) — since unlike row direction's width
// (always definite, it's however wide the caller says to render at), this
// engine has no other notion of a column flex container's main-axis size;
// with no explicit height and no item using flex-basis, items simply stack
// with row-gap between them exactly as if none of this machinery existed
// (flex-start main-axis behavior - the common case, and the only case
// before this file supported the main axis in column direction at all).
// reverse stacks bottom to top, for flex-direction: column-reverse. See
// CSS.md.
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
	containerHeight, hasContainerHeight := resolveFlexContainerHeight(decls)

	// margin-left/margin-right on a flex item need no separate handling
	// here - see layoutFlexRow's doc comment on the same point: the block
	// box model already bakes horizontal margin into an item's own returned
	// box regardless of flex direction. margin-top/margin-bottom get no such
	// treatment (a bare item's open top/bottom edge means the block box
	// model strips them into decls for a block-flow caller to apply, a
	// convention flex layout doesn't participate in), so they're read
	// directly here. Flex items never collapse margins with each other or
	// the container at all (real CSS: a flex formatting context doesn't
	// collapse) - each item's margin-bottom and the next item's margin-top
	// both apply in full, summed with row-gap, not collapsed via max the way
	// ordinary block flow would. margin: auto is not supported in column
	// direction (see CSS.md's Flexbox "Not supported" list).
	mt := make([]int, len(items))
	mb := make([]int, len(items))
	widths := make([]int, len(items))
	basisH := make([]int, len(items))
	for i, it := range items {
		mt[i] = parseMargin(it.decls["margin-top"])
		mb[i] = parseMargin(it.decls["margin-bottom"])
		itAlign := itemAlign(it, align)
		w := innerW
		if itAlign != "" && itAlign != "stretch" {
			w = r.resolveCrossWidth(it, innerW)
		}
		widths[i] = max(1, w)
		basisH[i] = parseColumnFlexBasis(it.decls, containerHeight, hasContainerHeight)
		if basisH[i] > 0 {
			// flex-basis's own height override must never end up shorter
			// than the item's own declared min-height - real CSS resolves
			// an item's used size to at least min-height regardless of
			// flex-basis. Without this, setting flex-basis on an item would
			// paradoxically defeat a min-height it would otherwise get for
			// free from block.go's ordinary (non-flex-basis) min-height
			// padding.
			if minH := flexShrinkFloor(it.decls, "min-height", containerHeight); minH > basisH[i] {
				basisH[i] = minH
			}
		}
	}

	// Render once per item at its resolved flex-basis (0 meaning "no
	// override" - identical to the item's own pre-existing height/natural
	// content behavior) to learn each item's hypothetical main size, exactly
	// mirroring row direction's flex-basis/measureNaturalWidth step.
	// quoteBefore[i] captures r.quoteDepth immediately before each item's own
	// render, so a later second render (flex-grow/flex-shrink adjusting its
	// height) can be replayed from the correct starting depth instead of
	// whatever the full basis pass over every item left it at - content:
	// open-quote/close-quote mutates r.quoteDepth live (see block.go), and
	// this file's other repeated-render site (measureNaturalWidth) already
	// saves/restores it for the same reason.
	heights := make([]int, len(items))
	boxes := make([]box, len(items))
	poses := make([]map[*html.Node]Rect, len(items))
	quoteBefore := make([]int, len(items))
	for i, it := range items {
		quoteBefore[i] = r.quoteDepth
		b, pos := r.renderFlexItemBoxHeight(it, widths[i], basisH[i])
		boxes[i], poses[i], heights[i] = b, pos, len(b.lines)
	}

	totalContentHeight := 0
	prevMb := 0
	for i := range items {
		gap := mt[i] + prevMb
		if i > 0 {
			gap += rowGap
		}
		totalContentHeight += gap + heights[i]
		prevMb = mb[i]
	}
	totalContentHeight += prevMb

	finalHeights := append([]int(nil), heights...)
	leftover := 0
	if hasContainerHeight {
		leftover = containerHeight - totalContentHeight
		allIdx := make([]int, len(items))
		grows := make([]float64, len(items))
		shrinks := make([]float64, len(items))
		floors := make([]int, len(items))
		ceilings := make([]int, len(items))
		for i, it := range items {
			allIdx[i] = i
			grows[i] = it.grow
			shrinks[i] = it.shrink
			floors[i] = flexShrinkFloor(it.decls, "min-height", containerHeight)
			ceilings[i] = flexGrowCeilingHeight(it.decls, containerHeight)
		}
		switch {
		case leftover > 0:
			// Mirrors layoutFlexLine's flex-grow branch, distributing into
			// height instead of width - capped per item at its own
			// max-height (ceilings), unlike row direction's flex-grow,
			// which has no max-width equivalent yet (see CSS.md). Any
			// leftover the ceilings prevented from being absorbed flows
			// through to justify-content/the trailing pad below, instead of
			// being silently dropped.
			leftover = distributeFlexGrow(finalHeights, grows, allIdx, leftover, ceilings)
		case leftover < 0:
			// Mirrors layoutFlexLine's flex-shrink branch.
			leftover = -distributeFlexShrink(finalHeights, shrinks, floors, allIdx, -leftover)
		}
	}

	// Only re-render items flex-grow/flex-shrink actually adjusted - every
	// other item's basis-step render (including the common case where
	// neither applies at all) is already the final box. Reset r.quoteDepth
	// to what it was immediately before this same item's basis render
	// (quoteBefore[i]) rather than leaving it at whatever the full basis
	// pass over every item left it at - see quoteBefore's own doc comment
	// above.
	for i, it := range items {
		if finalHeights[i] != heights[i] {
			r.quoteDepth = quoteBefore[i]
			boxes[i], poses[i] = r.renderFlexItemBoxHeight(it, widths[i], finalHeights[i])
		}
	}

	// justify-content distributes whatever main-axis leftover space remains
	// after flex-grow/flex-shrink - the same distributeJustify used for
	// row direction's horizontal leftover, just producing leading/between
	// blank lines instead of blank columns. leftover is only ever set away
	// from 0 above when hasContainerHeight is true, so without an explicit
	// container height this always resolves to distributeJustify(justify, 0,
	// n)'s own no-op case - safe to call unconditionally.
	leadRows, extraGaps := distributeJustify(decls["justify-content"], leftover, len(items))

	var lines []string
	for range leadRows {
		lines = append(lines, "")
	}
	positions := map[*html.Node]Rect{}
	row := leadRows
	prevMb = 0
	for i, it := range items {
		gapLines := mt[i] + prevMb
		if i > 0 {
			gapLines += rowGap + extraGaps[i-1]
		}
		for range gapLines {
			lines = append(lines, "")
			row++
		}
		b := boxes[i]
		colOffset := 0
		switch itemAlign(it, align) {
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
		if len(poses[i]) > 0 {
			positions = mergePositions(positions, poses[i], row, colOffset)
		}
		row += len(b.lines)
		prevMb = mb[i]
	}
	// The last item's own margin-bottom is never collapsed away (see above)
	// - flush it as trailing blank rows, unlike an ordinary block's last
	// child (which can collapse through its container's open bottom edge).
	for range prevMb {
		lines = append(lines, "")
	}
	// leadRows/extraGaps only place free space around/between items -
	// flex-start/justify-content values that don't consume all the leftover
	// (or flex-grow not applying to any item) can still leave the container
	// short of its explicit height, so any remainder is always flushed as
	// trailing blank rows, mirroring row direction's align-content handling.
	// containerHeight is 0 when hasContainerHeight is false, so this is
	// already a no-op without an explicit height - safe unconditionally.
	for len(lines) < containerHeight {
		lines = append(lines, "")
	}
	return box{lines: lines, width: linesWidth(lines)}, positions
}

// renderFlexContentBox renders a block-level (display:flex) flex container:
// the same margin/border/padding box model as renderBlockContentBox, wrapped
// around flex item layout instead of inline content flow. See CSS.md's
// Flexbox section for the supported subset.
func (r *Engine) renderFlexContentBox(n *html.Node, decls map[string]string, availWidth int) (box, map[*html.Node]Rect) {
	if r.measuringNaturalWidth && availWidth > measureBlockWidthCap {
		availWidth = measureBlockWidthCap
	}
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
