package render

import (
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
	"github.com/client9/htmlterm/internal/textcell"
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
// container," matching CSS's own align-self semantics. The result is
// normalized (normalizeAlignKeyword), so every caller switching on it sees the
// flexbox-era spelling of whatever the author wrote.
func itemAlign(it flexItem, containerAlign string) string {
	if v := it.decls["align-self"]; v != "" && v != "auto" {
		return normalizeAlignKeyword(v)
	}
	return normalizeAlignKeyword(containerAlign)
}

// normalizeAlignKeyword folds the CSS Box Alignment module's newer keyword
// spellings onto the original flexbox ones this file switches on. In a flex
// container `start`/`self-start` and `end`/`self-end` mean exactly what
// `flex-start`/`flex-end` do, and `normal` behaves as each property's own
// default (stretch for align-items/align-self, flex-start for
// justify-content/align-content) - which is what the unset value already
// resolves to here, so it normalizes to the empty string. Anything else,
// including an unrecognized value, is returned unchanged for the caller's own
// switch to fall through on.
//
// The physical `left`/`right` keywords are deliberately not mapped: their
// meaning depends on the container's direction rather than on its main-start
// edge, which is the one thing every caller here reasons in terms of.
func normalizeAlignKeyword(v string) string {
	switch v {
	case "start", "self-start":
		return "flex-start"
	case "end", "self-end":
		return "flex-end"
	case "normal":
		return ""
	}
	return v
}

// reverseJustify flips justify-content's start/end sense for row-reverse/
// column-reverse. Reversing the item sequence (reverseFlexItems) only gets
// half the job done: a reverse direction also moves the main-*start* edge to
// the right (row-reverse) or the bottom (column-reverse), so the default
// justify-content:flex-start packs the already-reversed sequence against that
// far edge, not against the left/top one this file's layout passes work from.
// Every value that distributes space symmetrically is its own mirror image
// and passes through unchanged; anything unrecognized (including unset) is
// flex-start, whose mirror is flex-end.
func reverseJustify(justify string) string {
	switch normalizeAlignKeyword(justify) {
	case "flex-end":
		return "flex-start"
	case "center", "space-between", "space-around", "space-evenly":
		return justify
	default:
		return "flex-end"
	}
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

// parseFlexDirection resolves flex-direction into (isColumn, reverse). Only
// the four real keywords are recognized; anything else, including an unset or
// misspelled value, falls back to row, matching flex-direction's own default.
// Matching on a "column" prefix and a "-reverse" suffix instead would let a
// typo like "colum-reverse" silently take effect as a partly-honored
// direction, which is worse than ignoring it: an invalid declaration should
// leave the property at its initial value, not at half of what was written.
func parseFlexDirection(decls map[string]string) (isColumn, reverse bool) {
	switch decls["flex-direction"] {
	case "row-reverse":
		return false, true
	case "column":
		return true, false
	case "column-reverse":
		return true, true
	}
	return false, false
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
// spec and treated as unset). Parsing goes through cssengine.ParseNumber
// rather than strconv, so only real CSS <number> syntax is accepted: a
// non-finite value like "NaN" would otherwise sail through the `f < 0` check
// (NaN compares false against everything) and then poison the total weight
// every share is computed against, silently zeroing out well-formed siblings.
func parseFlexGrow(decls map[string]string) float64 {
	f, ok := cssengine.ParseNumber(decls["flex-grow"])
	if !ok || f < 0 {
		return 0
	}
	return f
}

// parseFlexShrink parses flex-shrink (default 1, per spec; negative values
// are invalid per spec and treated as unset, falling back to the same
// default). See parseFlexGrow on the parser choice.
func parseFlexShrink(decls map[string]string) float64 {
	f, ok := cssengine.ParseNumber(decls["flex-shrink"])
	if !ok || f < 0 {
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

// flexGrowCeiling resolves how far flex-grow may increase an item's main-axis
// size: an explicit max-width (row direction) or max-height (column
// direction) - including percentage, resolved against axisSize - if set, else
// 0 meaning uncapped.
func flexGrowCeiling(decls map[string]string, key string, axisSize int) int {
	if v, ok := resolveCSSSize(decls[key], axisSize); ok && v > 0 {
		return v
	}
	return 0
}

// clampFlexBasis clamps a resolved flex base size to the item's own minimum
// and maximum in the same axis, producing the spec's "hypothetical main size"
// - the size line-breaking and grow/shrink both start from. maxKey is applied
// first and minKey second, so a minimum larger than the maximum wins, matching
// CSS's own min/max resolution order.
//
// Without this, an item's declared min/max still reached the item's *own*
// render (renderBlockContentBox resolves them in resolveWidthConstraints) but
// not the flex layout around it, so the box painted at a different width than
// the column flex layout had allotted it: every following item on the line
// drifted, and the recorded Rects - what click hit-testing reads - pointed at
// the wrong columns.
func clampFlexBasis(decls map[string]string, minKey, maxKey string, size, axisSize int) int {
	if v, ok := resolveCSSSize(decls[maxKey], axisSize); ok && v > 0 && size > v {
		size = v
	}
	if v, ok := resolveCSSSize(decls[minKey], axisSize); ok && size < v {
		size = v
	}
	return max(1, size)
}

// proportionalShares splits total into one share per group member, weighted
// by weights[i] (members with a non-positive weight get nothing), using the
// largest-remainder method: each member first takes the floor of its exact
// share, then the units that flooring left unassigned go one apiece to the
// members with the largest fractional parts, earlier members winning ties.
// The returned slice is indexed by position within group, not by item index.
//
// The two properties that matter here are that shares sum to exactly total
// and that none is ever negative. Rounding each member's share independently
// (round-half-up) and handing leftover-assigned to the last member - what
// distributeFlexGrow/distributeFlexShrink used to do - guaranteed neither: the
// per-member round-ups accumulate, so that last member could be handed a
// *negative* remainder of up to half a unit per member, ending up narrower
// than its own flex-basis or outright negative. Twenty items at basis 2
// sharing 10 leftover columns left the last one at -7, overflowing the
// container by 9 columns.
func proportionalShares(weights []float64, group []int, total int) []int {
	shares := make([]int, len(group))
	if total <= 0 {
		return shares
	}
	totalWeight := 0.0
	for _, i := range group {
		if weights[i] > 0 {
			totalWeight += weights[i]
		}
	}
	if totalWeight <= 0 {
		return shares
	}
	type remainder struct {
		pos  int
		frac float64
	}
	var remainders []remainder
	assigned := 0
	for gi, i := range group {
		if weights[i] <= 0 {
			continue
		}
		exact := float64(total) * weights[i] / totalWeight
		base := int(exact) // exact is never negative, so this floors
		shares[gi] = base
		assigned += base
		remainders = append(remainders, remainder{gi, exact - float64(base)})
	}
	sort.SliceStable(remainders, func(a, b int) bool { return remainders[a].frac > remainders[b].frac })
	for k := 0; k < total-assigned && k < len(remainders); k++ {
		shares[remainders[k].pos]++
	}
	return shares
}

// distributeFlexSpace hands out total units of main-axis space across group,
// split by proportionalShares' weighting, offering each member its share via
// take(i, units) - which applies whatever it can and returns how many units
// the member actually absorbed, fewer than offered once that member runs into
// its own limit. A member that couldn't take its whole share is frozen at its
// limit and the units it left behind are re-split across the members that can
// still move, repeating until either all of total is placed or nothing more
// can move. Returns the units that could not be placed anywhere.
//
// The repeat is the point: the spec resolves flexible lengths exactly this way
// ("freeze, recompute the free space, repeat"), and a single pass gets it
// wrong whenever any item hits a limit. Three items of 10 with shrink 1 and a
// deficit of 6, the last floored at min-width:9, used to end up 25 wide - the
// unit the floored item couldn't give back was simply dropped rather than
// taken from the two items that still had room.
func distributeFlexSpace(weights []float64, group []int, total int, take func(i, units int) int) int {
	active := group
	for total > 0 && len(active) > 0 {
		placed := 0
		var next []int
		for gi, share := range proportionalShares(weights, active, total) {
			i := active[gi]
			if share <= 0 {
				if weights[i] > 0 {
					next = append(next, i)
				}
				continue
			}
			took := take(i, share)
			placed += took
			if took == share {
				// Absorbed everything offered, so not (yet) at its limit -
				// still eligible for whatever the frozen members leave behind.
				next = append(next, i)
			}
		}
		if placed == 0 {
			// Every remaining member is frozen at its limit.
			break
		}
		total -= placed
		active = next
	}
	return total
}

// distributeFlexGrow adds each group member's proportional share of leftover
// to sizes[i] (i ranges over group, indices into the full item-count-sized
// sizes/grows/ceilings arrays - matching widths/mt/mb's own convention),
// weighted by grows[i] - shared by row direction's per-line width
// distribution and column direction's height distribution. ceilings, if
// non-nil, caps sizes[i]'s growth at ceilings[i] (0 meaning uncapped) - real
// CSS's max-width/max-height on a flex item, redistributed to the items that
// can still grow (see distributeFlexSpace). Returns however much of leftover
// wasn't actually absorbed (0 unless every growable member hit its ceiling
// before leftover was exhausted, or no member can grow at all).
func distributeFlexGrow(sizes []int, grows []float64, group []int, leftover int, ceilings []int) int {
	return distributeFlexSpace(grows, group, leftover, func(i, units int) int {
		if ceilings != nil && ceilings[i] > 0 {
			if maxAdd := ceilings[i] - sizes[i]; units > maxAdd {
				units = max(0, maxAdd)
			}
		}
		sizes[i] += units
		return units
	})
}

// distributeFlexShrink subtracts each group member's proportional share of
// deficit from sizes[i], weighted by shrinks[i]*sizes[i] (the real spec's
// "scaled flex shrink factor"), floored at floors[i] - shared by row
// direction's width distribution and column direction's height distribution.
// Returns however much of deficit wasn't actually absorbed (0 unless every
// shrinkable member hit its floor before deficit was exhausted, or no member
// can shrink at all). Floored members' unabsorbed share is redistributed to
// the members that can still shrink, and shares come from proportionalShares,
// for the same reasons distributeFlexGrow's do - see distributeFlexSpace.
//
// The scaled shrink factors are computed once, from each member's incoming
// (flex base) size rather than its current one, matching the spec's own use of
// the inner flex base size across every round of the loop.
func distributeFlexShrink(sizes []int, shrinks []float64, floors []int, group []int, deficit int) int {
	scaled := make([]float64, len(sizes))
	for _, i := range group {
		if shrinks[i] <= 0 {
			continue
		}
		scaled[i] = shrinks[i] * float64(sizes[i])
	}
	return distributeFlexSpace(scaled, group, deficit, func(i, units int) int {
		if maxReduce := sizes[i] - floors[i]; units > maxReduce {
			units = max(0, maxReduce)
		}
		sizes[i] -= units
		return units
	})
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
	items := r.appendFlexItems(nil, n)
	sort.SliceStable(items, func(i, j int) bool { return items[i].order < items[j].order })
	return items
}

// appendFlexItems is collectFlexItems' walk over one element's children,
// recursing through any display:contents child so that child's own children
// become flex items of the container instead. A display:contents element
// generates no box at all (see render.go's and inline.go's "contents" cases),
// which in a flex container means its children are the items - without the
// recursion the wrapper itself became a single blockified item and its
// children stacked inside it as ordinary blocks. Their decls come from
// r.resolveDecls, which walks real ancestors, so anything inherited through
// the display:contents wrapper still reaches them.
func (r *Engine) appendFlexItems(items []flexItem, n *html.Node) []flexItem {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || isSkippedContentElement(c.Data) {
			continue
		}
		decls := r.resolveDecls(c)
		if decls["display"] == "none" || r.outOfFlow[c] {
			continue
		}
		if decls["display"] == "contents" {
			items = r.appendFlexItems(items, c)
			continue
		}
		items = append(items, flexItem{node: c, decls: decls, grow: parseFlexGrow(decls), shrink: parseFlexShrink(decls), order: parseOrder(decls)})
	}
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
// width, then the item's own measured natural content width - the result
// clamped by the item's own min-width/max-width (clampFlexBasis). The natural
// -width path is measured through renderBlockContentBox, which resolves those
// two itself, so clamping is a no-op there and only really bites on the
// flex-basis/width paths.
func (r *Engine) resolveMainBasis(it flexItem, innerW int) int {
	basis := 0
	if v := it.decls["flex-basis"]; v != "" && v != "auto" {
		basis = resolveFlexAxisSize(v, innerW)
	}
	if basis == 0 {
		if v := it.decls["width"]; v != "" {
			basis = resolveFlexAxisSize(v, innerW)
		}
	}
	if basis == 0 {
		basis = r.measureNaturalWidth(it, innerW)
	}
	return clampFlexBasis(it.decls, "min-width", "max-width", basis, innerW)
}

// resolveFlexAxisSize resolves one absolute-or-percentage CSS length against
// axisSize, returning 0 when the value isn't a length at all (so callers can
// fall through to their next source). A resolved length is floored at 1: this
// engine can't render a zero-width/zero-height box, so flex-basis:0 - what the
// common `flex: 1` shorthand expands to - starts at one cell rather than none.
func resolveFlexAxisSize(v string, axisSize int) int {
	abs, pct, ok := parseSizeVal(v)
	if !ok {
		return 0
	}
	if pct > 0 {
		return max(1, int(pct*float64(axisSize)))
	}
	return max(1, abs)
}

// resolveCrossWidth resolves a column-direction flex item's cross-axis
// (horizontal) width when align-items isn't stretch: width if set, else the
// item's own measured natural content width, capped to innerW and then clamped
// by the item's own min-width/max-width - so a min-width larger than the
// container still wins and overflows, matching CSS (and matching what the
// item's own render will do to itself regardless).
func (r *Engine) resolveCrossWidth(it flexItem, innerW int) int {
	w := 0
	if v := it.decls["width"]; v != "" {
		w = resolveFlexAxisSize(v, innerW)
	}
	if w == 0 {
		w = r.measureNaturalWidth(it, innerW)
	}
	return clampFlexBasis(it.decls, "min-width", "max-width", min(innerW, w), innerW)
}

// splitEvenly divides total into n parts that differ by at most one, earlier
// parts taking the remainder. Used for every even-spacing justify-content /
// align-content value, so none of them can dump a rounding remainder on a
// single edge.
func splitEvenly(total, n int) []int {
	parts := make([]int, n)
	if n <= 0 || total <= 0 {
		return parts
	}
	base, rem := total/n, total%n
	for i := range parts {
		parts[i] = base
		if i < rem {
			parts[i]++
		}
	}
	return parts
}

// distributeJustify resolves justify-content's leftover-main-axis-space
// distribution into a leading pad (before the first item) and n-1 extra
// per-gap amounts (added on top of the base column-gap between items).
// leftover <= 0 (no free space, or the line already overflows) always yields
// no extra spacing - which is the usual case whenever any item can grow, since
// flex-grow consumes the leftover first.
//
// distributeAlignContent shares this: align-content distributes leftover
// cross-axis space across a wrapped container's lines exactly the way
// justify-content distributes leftover main-axis space across one line's
// items, so they are one function rather than two that have to be kept in
// step. An unrecognized value (including unset, and align-content's
// "stretch" - see distributeAlignContent) falls through to no distribution,
// which is flex-start's behavior.
func distributeJustify(justify string, leftover, n int) (leadPad int, gaps []int) {
	if n > 1 {
		gaps = make([]int, n-1)
	}
	if leftover <= 0 {
		return 0, gaps
	}
	switch normalizeAlignKeyword(justify) {
	case "flex-end":
		leadPad = leftover
	case "center":
		leadPad = leftover / 2
	case "space-between":
		// All of it between the items, none at the edges.
		if n < 2 {
			return 0, gaps
		}
		copy(gaps, splitEvenly(leftover, n-1))
	case "space-around":
		// Each item gets an equal share centered on it, so the edge pads are
		// half a share and the pad between two items is two halves. Splitting
		// into 2n half-shares and recombining keeps the rounding remainder
		// spread across the line instead of landing on one edge.
		halves := splitEvenly(leftover, 2*n)
		leadPad = halves[0]
		for i := range gaps {
			gaps[i] = halves[2*i+1] + halves[2*i+2]
		}
	case "space-evenly":
		// n+1 equal pads: before the first item, between each pair, and after
		// the last.
		units := splitEvenly(leftover, n+1)
		leadPad = units[0]
		for i := range gaps {
			gaps[i] = units[i+1]
		}
	}
	return leadPad, gaps
}

// distributeAlignContent resolves align-content's leftover cross-axis space
// across a wrapped row container's lines into a leading blank-row count
// (before the first line) and n-1 extra row-gap amounts (added on top of the
// base row-gap between lines). Same distribution as distributeJustify, just
// applied to whole lines on the cross (vertical) axis rather than to items on
// the main (horizontal) axis - see there for the value handling.
//
// "stretch" (align-content's real default) has no cell-grid equivalent for
// growing each line's own items taller than their content, so it falls through
// to flex-start's no-distribution behavior rather than half implementing
// per-item vertical growth - see CSS.md.
func distributeAlignContent(align string, leftover, n int) (leadRows int, gaps []int) {
	return distributeJustify(align, leftover, n)
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
//
// minHeight forces the line's cross size (normally its tallest item's outer
// height) up to at least that many rows - a single-line container's line takes
// the container's own inner cross size, so align-items/align-self resolve
// against the container's declared height rather than against the content.
// 0 means "whatever the items come to", the multi-line case, where each line
// sizes to its own content and align-content places the lines instead.
func (r *Engine) layoutFlexLine(items []flexItem, widths, mt, mb []int, mlAuto, mrAuto []bool, group []int, innerW, gap, minHeight int, justify string, decls map[string]string) (box, int, map[*html.Node]Rect) {
	totalGap := gap * (len(group) - 1)
	availForItems := max(0, innerW-totalGap)

	sumW := 0
	for _, i := range group {
		sumW += widths[i]
	}
	grows := make([]float64, len(items))
	shrinks := make([]float64, len(items))
	floors := make([]int, len(items))
	ceilings := make([]int, len(items))
	for _, i := range group {
		grows[i] = items[i].grow
		shrinks[i] = items[i].shrink
		floors[i] = flexShrinkFloor(items[i].decls, "min-width", innerW)
		ceilings[i] = flexGrowCeiling(items[i].decls, "max-width", innerW)
	}
	leftover := availForItems - sumW
	switch {
	case leftover > 0:
		// Capped per item at its own max-width, the mirror of column
		// direction's max-height ceiling. Any leftover the ceilings prevented
		// from being absorbed flows through to justify-content below rather
		// than being silently dropped.
		leftover = distributeFlexGrow(widths, grows, group, leftover, ceilings)
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
	height := max(1, minHeight)
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
		leadPad, extraGaps = distributeJustify(justify, leftover, len(group))
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
		// Column bookkeeping tracks the box's *painted* width, not the width
		// the main-axis pass allotted it. alignLinesBox above pads a box up to
		// its allotment but never truncates one that exceeds it - content that
		// can't fit (an unbreakable word wider than the item's shrink floor)
		// overflows by design. Advancing by the allotment instead would leave
		// every later item on the line with a Rect that's short by the
		// overflow, pointing hit-testing at columns another item is painting.
		painted := itemBoxes[i].width
		positions[it.node] = Rect{Row: offset, Col: colStart, Width: painted, Height: len(itemBoxes[i].lines)}
		if len(itemPositions[i]) > 0 {
			positions = mergePositions(positions, itemPositions[i], offset, colStart)
		}
		colStart += painted
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
	justify := decls["justify-content"]
	if reverse {
		justify = reverseJustify(justify)
	}

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

	// A single-line container (flex-wrap: nowrap) has exactly one line, and
	// that line's cross size is the container's own inner cross size - so an
	// explicit height on the container is what align-items/align-self resolve
	// against, which is what makes align-items:center vertically center a
	// fixed-height row. align-content is inoperative in that case (real CSS
	// says so outright), and the multi-line case instead sizes each line to
	// its own content and lets align-content place the lines, so the height
	// only reaches layoutFlexLine here.
	lineMinHeight := 0
	if !wrap {
		if h, ok := resolveFlexContainerHeight(decls); ok {
			lineMinHeight = h
		}
	}

	type lineResult struct {
		b   box
		h   int
		pos map[*html.Node]Rect
	}
	results := make([]lineResult, len(lineGroups))
	for li, group := range lineGroups {
		b, h, pos := r.layoutFlexLine(items, widths, mt, mb, mlAuto, mrAuto, group, innerW, gap, lineMinHeight, justify, decls)
		results[li] = lineResult{b, h, pos}
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
	// crossSized[i] marks an item whose cross-axis width this pass resolved
	// itself (align-items/align-self other than stretch) rather than handing
	// it the container's full width. Only those get that width enforced on
	// their rendered box below; a stretched item is left at whatever it
	// rendered to, so an inline-flex container still shrinks to fit its
	// content instead of padding out to its caller's wrap bound.
	crossSized := make([]bool, len(items))
	basisH := make([]int, len(items))
	for i, it := range items {
		mt[i] = parseMargin(it.decls["margin-top"])
		mb[i] = parseMargin(it.decls["margin-bottom"])
		itAlign := itemAlign(it, align)
		w := innerW
		if itAlign != "" && itAlign != "stretch" {
			w = r.resolveCrossWidth(it, innerW)
			crossSized[i] = true
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
			ceilings[i] = flexGrowCeiling(it.decls, "max-height", containerHeight)
		}
		switch {
		case leftover > 0:
			// Mirrors layoutFlexLine's flex-grow branch, distributing into
			// height instead of width - capped per item at its own
			// max-height (ceilings), the mirror of row direction's max-width
			// ceiling. Any leftover the ceilings prevented from being
			// absorbed flows through to justify-content/the trailing pad
			// below, instead of being silently dropped.
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
	justify := decls["justify-content"]
	if reverse {
		justify = reverseJustify(justify)
	}
	leadRows, extraGaps := distributeJustify(justify, leftover, len(items))

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
		// A cross-axis width this pass resolved itself has to be enforced on
		// the box, the same way layoutFlexLine enforces the main-axis width in
		// row direction: renderBlockContentBox only pads to its available
		// width when something (an explicit width, text-align, a border)
		// requires it, so an item sized by min-width would otherwise collapse
		// back to its natural width and be aligned as if it were that narrow.
		b := boxes[i]
		if crossSized[i] {
			b = alignLinesBox(b, it.decls["text-align"], widths[i])
		}
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
		inner := max(1, totalW-ml-textcell.Width(bl.char)-pl-pr-textcell.Width(br.char)-mr)
		hBorderWidth = textcell.Width(bl.char) + pl + inner + pr + textcell.Width(br.char)
		hasExplicitWidth = true
	}
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		ml, mr = splitAutoMargins(remaining, ml, mr, mlAuto, mrAuto)
	}

	avail := hBorderWidth - textcell.Width(bl.char) - textcell.Width(br.char)
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
			positions = mergePositions(nil, positions, 0, textcell.Width(bl.char))
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
	if r.liveContentOffsetsX == nil {
		r.liveContentOffsetsX = map[*html.Node]int{}
	}
	r.liveContentOffsetsX[n] = pl + textcell.Width(bl.char) + ml
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
