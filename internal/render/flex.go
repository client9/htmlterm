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
// main-axis basis in *column* direction: an explicit min-height - including
// percentage, resolved against containerHeight - if set, else 1. Row direction uses
// flexMainAxisFloor instead, which implements the spec's automatic minimum
// size; there is no vertical equivalent to implement here, since text can't be
// forced into fewer lines, so column-direction content taller than its floor
// simply overflows past it - the same graceful degradation already used for the
// flex-shrink:0 case.
func flexShrinkFloor(decls map[string]string, axisSize int) int {
	if v, ok := resolveCSSSize(decls["min-height"], axisSize); ok && v > 1 {
		return v
	}
	return 1
}

// flexMainAxisFloor resolves how far flex-shrink may take a row-direction item
// below its flex base size: CSS's *automatic minimum size* for flex items
// (Flexbox §4.5, defined in CSS Sizing 3 §5.1), which is the real reason a
// browser almost never shows an over-shrunk flex item overflowing its own box.
// min-width's initial value is `auto`, not 0, and on a flex item in the main
// axis that resolves to the content-based minimum - so flex-shrink simply
// cannot take an item below the width its content needs.
//
// The two opt-outs are the spec's own, and are why `min-width: 0` and
// `overflow: hidden` are the two standard fixes for a flex item that refuses to
// shrink - they are the same lever pulled two ways:
//
//   - An explicitly declared min-width wins outright, including a declared 0
//     (which parseFlexSizeVal, via resolveFlexCSSSize, can now tell apart from
//     unset - the floor is still one cell, since no box here renders in zero
//     columns).
//   - The automatic minimum applies only "on a flex item whose overflow is
//     visible in the main axis"; for any other overflow-x it computes to 0.
//     Such an item already clips or scrolls itself in renderBlockContentBox
//     (the resolved main size reaches it as a synthetic width - see
//     renderFlexItemBoxSized), so letting it shrink freely is exactly what the
//     author asked for.
//
// The content-based minimum is the item's min-content width, measured by
// rendering it at a cap of one column: every breakable point breaks, so what's
// left is the widest unbreakable run plus the item's own chrome, which is
// min-content. It is then clamped by the specified size suggestion (a definite
// width) and by the definite maximum main size, per Sizing 3 §5.1's "in all
// cases, the size is clamped by the maximum main size if it's definite" - which
// is what still lets `width: 3` on a long word shrink to 3 and overflow, as it
// does in a browser.
//
// Only called for a line that actually overflows (see layoutFlexLine), since
// the min-content measurement is a second trial render per item.
func (r *Engine) flexMainAxisFloor(it flexItem, innerW int) int {
	if v, ok := resolveFlexCSSSize(it.decls["min-width"], innerW); ok {
		return max(1, v)
	}
	if !isVisibleOverflow(it.decls["overflow-x"]) {
		return 1
	}
	floor := r.measureMinContentWidth(it)
	if w, ok := resolveCSSSize(it.decls["width"], innerW); ok && w < floor {
		floor = w
	}
	if m, ok := resolveCSSSize(it.decls["max-width"], innerW); ok && m < floor {
		floor = m
	}
	return max(1, floor)
}

// measureMinContentWidth measures an item's min-content width - the *content
// size suggestion* half of the automatic minimum size above - by rendering it
// at a cap of one column, so every breakable point breaks and what's left is
// the widest unbreakable run plus the item's own chrome.
//
// The item's own width/min-width/max-width are stripped for the measurement.
// Intrinsic sizes are computed from content, ignoring the box's specified size:
// leaving `width: 10` in place would just hand that 10 back as the "min-content"
// answer, floor the item at its own declared width, and make flex-shrink inert
// for exactly the items most likely to need it. The specified width is then
// applied by the caller as a separate clamp, which is how the spec composes the
// two (Sizing 3 §5.1: the smaller of the specified size suggestion and the
// content size suggestion).
// Memoized per node (Engine.minContentCache): the answer doesn't depend on the
// width the item was offered, and the measurement is a full trial render that
// recurses into nested flex containers, so re-deriving it per caller is what
// turns a nested `flex: 1` layout exponential. See the cache's comment in
// engine.go.
func (r *Engine) measureMinContentWidth(it flexItem) int {
	if w, ok := r.minContentCache[it.node]; ok {
		return w
	}
	probe := it
	probe.decls = maps.Clone(it.decls)
	delete(probe.decls, "width")
	delete(probe.decls, "min-width")
	delete(probe.decls, "max-width")
	w := r.measureNaturalWidth(probe, 1)
	if r.minContentCache != nil {
		r.minContentCache[it.node] = w
	}
	return w
}

// isVisibleOverflow reports whether a computed overflow-x/overflow-y value is
// `visible` - the initial value, so unset counts. Every other value (hidden,
// clip, scroll, auto) makes the box its own clipping/scrolling context, which
// is what disables the automatic minimum size above.
//
// Comparing lowercase spellings is safe: keyword values are folded once in the
// cascade (cssengine's keywordcase.go), so an author's `overflow-X: HIDDEN`
// arrives here already lowercased. Before that folding existed this predicate
// read an uppercase value as non-visible while renderFlexContentBox's own
// `== "hidden"` check did not, which put such an item in the one combination
// this file works to avoid: no automatic minimum *and* no clip.
func isVisibleOverflow(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "visible"
}

// resolveFlexCSSSize is resolveCSSSize (block.go) over parseFlexSizeVal, so a
// declared zero reads as a real length rather than as an absent one - see
// parseFlexSizeVal. Used where the difference between "0" and unset carries
// meaning, which in flex layout is min-width's opt-out from the automatic
// minimum size.
func resolveFlexCSSSize(s string, axisSize int) (int, bool) {
	abs, pct, ok := parseFlexSizeVal(s)
	if !ok {
		return 0, false
	}
	if pct > 0 {
		return int(pct * float64(axisSize)), true
	}
	return abs, true
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
//
// factors is the members' *raw* flex-grow/flex-shrink factors, used only for
// the spec's sub-1 clamp (§9.7 step 4b: "if the sum of the unfrozen flex
// items' flex factors is less than one, multiply the initial free space by
// this sum; if the magnitude of this value is less than the magnitude of the
// remaining free space, use this as the remaining free space"). It is a
// separate argument from weights because the shrink path weights by the scaled
// flex shrink factor (shrink x base size) while the clamp is defined on the
// unscaled one. Without the clamp a lone flex-grow:0.5 item absorbed all of
// the free space rather than half of it, since proportionalShares normalizes
// by the total weight and so can't tell 0.5 from 5.
func distributeFlexSpace(weights, factors []float64, group []int, total int, take func(i, units int) int) int {
	active := group
	initial := total
	// absorbed[i] is what member i has taken across all rounds so far. The
	// spec recomputes each unfrozen item's *target* size from its flex base
	// every round, so the sub-1 clamp bounds a member's cumulative share, not
	// each round's increment - subtracting what the still-active members
	// already hold is what turns this loop's incremental offers back into that
	// target-based bound. Without it, a clamped round that froze somebody
	// would re-offer the survivors their whole entitlement a second time.
	absorbed := make(map[int]int, len(group))
	for total > 0 && len(active) > 0 {
		// The clamp is recomputed every round against the still-unfrozen
		// members (a frozen member's factor drops out of the sum) but always
		// applies to the *initial* free space, as the spec has it.
		round := total
		if sum := sumFlexFactors(factors, active); sum < 1 {
			entitled := int(sum * float64(initial))
			for _, i := range active {
				entitled -= absorbed[i]
			}
			if entitled < round {
				round = entitled
			}
		}
		if round <= 0 {
			break
		}
		placed := 0
		var next []int
		for gi, share := range proportionalShares(weights, active, round) {
			i := active[gi]
			if share <= 0 {
				if weights[i] > 0 {
					next = append(next, i)
				}
				continue
			}
			took := take(i, share)
			placed += took
			absorbed[i] += took
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

// sumFlexFactors totals the positive flex factors across group, for
// distributeFlexSpace's sub-1 clamp.
func sumFlexFactors(factors []float64, group []int) float64 {
	sum := 0.0
	for _, i := range group {
		if factors[i] > 0 {
			sum += factors[i]
		}
	}
	return sum
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
	return distributeFlexSpace(grows, grows, group, leftover, func(i, units int) int {
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
	return distributeFlexSpace(scaled, shrinks, group, deficit, func(i, units int) int {
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
//
// "wrap-reverse" is accepted as an alias for "wrap": its cross-axis line order
// (last line first) isn't implemented, but line breaking itself is the whole
// point of the property, and treating it as nowrap instead made a container
// that should have wrapped overflow its width on a single line - the loudest
// possible failure for the part that does work here. See CSS.md.
func parseFlexWrap(decls map[string]string) bool {
	v := decls["flex-wrap"]
	return v == "wrap" || v == "wrap-reverse"
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

// renderFlexItemBox renders one row-direction flex item's own box at the
// main-axis width flex layout resolved for it, dispatching to a nested flex
// layout when the item is itself a flex container (display:flex/inline-flex
// nests normally) or the ordinary block box model otherwise — flex items are
// blockified regardless of their own declared display, matching real CSS.
//
// width is imposed as a synthetic declaration, not merely offered as the
// available width: an item's used main size is whatever flex layout resolved
// (its flex base size, adjusted by flex-grow/flex-shrink), which per spec
// overrides its own width property outright. Passing it as available width
// only would let renderBlockContentBox re-resolve the item's declared width
// and win - a grown item would paint at its declared width with the growth
// left as trailing blanks, and a shrunk one would paint at full width and
// overflow the container, dragging every later item's recorded Rect with it.
// Re-resolving min-width/max-width inside the item's own render is harmless:
// resolveMainBasis already clamped the base size through clampFlexBasis, and
// flex-grow/flex-shrink respect the same two as their ceiling/floor, so the
// width handed here is already inside that range.
func (r *Engine) renderFlexItemBox(it flexItem, width int) (box, map[*html.Node]Rect) {
	return r.renderFlexItemBoxSized(it, width, width, 0)
}

// renderFlexItemBoxHeight is renderFlexItemBox for column direction: width is
// the item's cross-axis size and is offered as available width only (an
// item's own width property legitimately wins over align-items: stretch,
// since stretch applies only to an auto cross size), while heightOverride > 0
// imposes the column-direction main-axis size the same way and for the same
// reason renderFlexItemBox imposes width.
func (r *Engine) renderFlexItemBoxHeight(it flexItem, width, heightOverride int) (box, map[*html.Node]Rect) {
	return r.renderFlexItemBoxSized(it, width, 0, heightOverride)
}

// renderFlexItemBoxSized renders one flex item at availWidth, with optional
// main-axis size overrides: widthOverride/heightOverride > 0 inject a
// synthetic "width"/"height" declaration (taking priority over any the item
// already declares - matching flex-basis's real-CSS precedence over the
// width/height properties) before rendering, so the existing block box model's
// own sizing (block.go's width resolution and fixed-height pad/clip logic)
// does the actual work for a non-flex item, and resolveFlexContainerHeight/
// resolveWidthConstraints do it for an item that is itself a flex container.
// 0 means "no override".
//
// Both overrides are *outer* box sizes - the item's whole painted extent,
// border characters and padding included - since that's the size flex layout
// resolved and reserved space for, and the convention a declared width already
// follows everywhere else in this engine (see COMPATIBILITY.md). The width
// property means exactly that already, but the height property is content-box
// here, so the vertical override subtracts the item's own top/bottom border
// and padding rows (blockVerticalChrome) before injecting it. Without that
// subtraction a bordered item overshot its allotted rows by exactly its chrome
// and had the overflow silently clipped off the bottom of the line - the
// stretched box's bottom border vanished.
//
// decls is always cloned, even with no override at all: layoutFlexColumn
// can render the same item twice (a basis-measurement pass, then a second
// pass if flex-grow/flex-shrink changes its height), and block.go's own
// margin-top/margin-bottom collapse-through bookkeeping mutates its decls
// argument in place - without a clone here, that first render could leave a
// stale mutation in it.decls for the second render (or any later reader) to
// pick up. Row direction's layoutFlexLine renders every item exactly once,
// so this is strictly defensive there, not required by the row path itself.
func (r *Engine) renderFlexItemBoxSized(it flexItem, availWidth, widthOverride, heightOverride int) (box, map[*html.Node]Rect) {
	width := availWidth
	decls := maps.Clone(it.decls)
	if widthOverride > 0 {
		decls["width"] = strconv.Itoa(widthOverride)
		if isVisibleOverflow(decls["overflow-x"]) {
			// Clip the item's content to the main size flex resolved for it, by
			// synthesizing the overflow-x declaration the situation implies
			// rather than trimming the painted box afterward. Trimming from
			// outside cuts whatever happens to sit in the last columns, which
			// for a bordered item is its own right border glyph; block.go's
			// clipping path knows the box model and truncates the *content* to
			// its inner width, leaving border and padding intact - and honors
			// text-overflow while it's there.
			//
			// Only reached when the author left overflow-x at visible: hidden/
			// clip means this is already what they asked for, and scroll/auto
			// makes the item its own horizontal viewport, whose windowing must
			// not be downgraded to a truncation.
			//
			// Clipping at all is a deliberate deviation from CSS, and a narrow
			// one - flexMainAxisFloor's automatic minimum size is the primary
			// defense, so this only fires where CSS itself would overflow (a
			// definite width narrower than min-content, a `min-width: 0`
			// opt-out, `flex-shrink: 0` against a small declared width). This
			// engine's convention elsewhere is *not* to clip without an
			// explicit overflow: a plain bordered block whose unbreakable
			// content exceeds its width paints straight past its own border,
			// and a flex line whose items are all at their floors is likewise
			// left to overflow its container. What makes an item exceeding its
			// *allotment* different is that flex already positioned every later
			// item on the line from that number - lines are assembled by
			// concatenation (see layoutFlexLine), so an item painting wider
			// than its allotment doesn't overlay its siblings the way a
			// browser's visible overflow would, it displaces them into columns
			// that belong to somebody else, and only on the rows that overflow.
			// That's not visible overflow, it's a desynchronized frame, and a
			// character grid has no way to express the former. See
			// COMPATIBILITY.md.
			decls["overflow-x"] = "hidden"
		}
	}
	if heightOverride > 0 {
		decls["height"] = strconv.Itoa(max(1, heightOverride-blockVerticalChrome(decls)))
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
			// Safety net only. The injected height above reaches a nested flex
			// container's own layout through resolveFlexContainerHeight, so
			// layoutFlexColumn/layoutFlexRow normally fill to exactly this many
			// rows themselves - which is the point of injecting it rather than
			// padding from outside: padding here happens *after*
			// renderFlexContentBox has drawn the box's bottom rule, so the extra
			// rows land below the border instead of inside it, and the nested
			// container's own children never see the height their parent was
			// allotted (a flex-grow child of a grown nested column container
			// stayed at its content height). This branch is left in place for
			// the cases layout can't fill - a nested container with no items at
			// all, say - where blank rows below the box still beat coming up
			// short of the row the parent reserved. A taller-than-heightOverride
			// result is left untouched, matching the graceful-overflow
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

// blockVerticalChrome is how many rows of an ordinary block box's height are
// its own chrome rather than its content: a drawn top/bottom border rule plus
// padding-top/padding-bottom. Flex layout resolves outer sizes, while the
// height property is content-box in this engine, so this is the difference
// between the two - see renderFlexItemBoxSized.
func blockVerticalChrome(decls map[string]string) int {
	_, _, bt, bb, _, _, _, _ := resolveBoxBorders(decls)
	rows := parsePaddingLen(decls["padding-top"]) + parsePaddingLen(decls["padding-bottom"])
	if bt.char != "" {
		rows++
	}
	if bb.char != "" {
		rows++
	}
	return rows
}

// parseColumnFlexBasis resolves a column-direction flex item's flex-basis to
// an absolute line count: 0 means unset/auto (or a percentage basis with no
// definite container height to resolve against - real CSS treats a
// percentage flex-basis as auto when the container's own main size is
// indefinite, which this engine only ever considers "definite" when an
// explicit CSS height is set on the container - see resolveFlexContainerHeight).
// A declared flex-basis of 0 is a definite base size, not an absent one, and
// resolves to one line - see parseFlexSizeVal.
func parseColumnFlexBasis(decls map[string]string, containerHeight int, hasContainerHeight bool) int {
	v := decls["flex-basis"]
	if v == "" || v == "auto" {
		return 0
	}
	abs, pct, ok := parseFlexSizeVal(v)
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
	savedShrink := r.shrinkToFit
	// Shrink-to-fit for the duration of the trial render: without it a flex
	// item that paints a rectangle (a border, text-align, an explicit height)
	// measures as however wide cap is rather than as its own content, so every
	// such item on a line reports an identical basis and flex-shrink's
	// content-proportional split flattens to an even one. See
	// Engine.shrinkToFit.
	r.shrinkToFit = true
	var b box
	if isFlexDisplay(it.decls["display"]) {
		// Measurement goes through renderFlexContentBox, the same entry point
		// the real render uses, so a nested flex container measures as its own
		// whole box - border characters and padding included - and not just as
		// the items inside it. Those columns are part of what the container
		// will paint, so leaving them out under-measures such an item by its
		// own chrome at every level of nesting, and flex then allots it fewer
		// columns than it needs.
		//
		// This used to call layoutFlex, the unpadded layout pass underneath,
		// because renderFlexContentBox fills its result to the full width it's
		// given (a block-level flex container's normal CSS behavior) and so
		// measured every width:auto container as "however wide cap is". That
		// reason is gone: renderFlexContentBox now narrows to its content's own
		// width under a shrink-to-fit measurement render, which is exactly the
		// state measureNaturalWidth puts the engine in above. See
		// Engine.shrinkToFit.
		b, _ = r.renderFlexContentBox(it.node, it.decls, cap)
	} else {
		b, _ = r.renderBlockContentBox(it.node, it.decls, cap)
	}
	r.quoteDepth = saved
	r.shrinkToFit = savedShrink
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
	content := flexBasisIsContent(it.decls)
	if v := it.decls["flex-basis"]; v != "" && v != "auto" && !content {
		basis = resolveFlexAxisSize(v, innerW)
	}
	if basis == 0 && !content {
		if v := it.decls["width"]; v != "" {
			basis = resolveFlexAxisSize(v, innerW)
		}
	}
	// A basis that came from a declared length, rather than from measuring the
	// item, is floored at the automatic minimum size: the spec clamps the flex
	// base size by the used min/max main size to get the *hypothetical* main
	// size (§9.2), and min-width's used value is `auto`, not 0 - so a declared
	// flex-basis smaller than the item's content is raised to fit it before
	// line-breaking and grow/shrink ever see it, not merely prevented from
	// shrinking further later. Skipped for a measured basis, which is
	// fit-content and so already at or above min-content: the floor could never
	// bite, and flexMainAxisFloor's probe is a second trial render per item.
	if basis > 0 {
		basis = max(basis, r.flexMainAxisFloor(it, innerW))
	}
	if basis == 0 {
		// `content` measures the item's content with its own width property out
		// of the way - that's the entire difference between it and `auto`,
		// which consults width first (above) and would otherwise have the
		// measurement render honor it as well.
		probe := it
		if content {
			probe = itemIgnoringSizeDecl(it, "width")
		}
		basis = r.measureNaturalWidth(probe, innerW)
	}
	return clampFlexBasis(it.decls, "min-width", "max-width", basis, innerW)
}

// flexBasisIsContent reports whether an item declares `flex-basis: content`,
// the keyword that sizes an item from its content while ignoring its own
// width/height property. `auto` defers to that property first and only falls
// back to content, so the two differ exactly when the item declares a main size.
func flexBasisIsContent(decls map[string]string) bool {
	return strings.TrimSpace(decls["flex-basis"]) == "content"
}

// itemIgnoringSizeDecl returns it with one size declaration removed, over a
// cloned decls map so the item's own (shared) resolved declarations are left
// alone. Used to render or measure an item as if it had not declared that size.
func itemIgnoringSizeDecl(it flexItem, key string) flexItem {
	out := it
	out.decls = maps.Clone(it.decls)
	delete(out.decls, key)
	return out
}

// resolveFlexAxisSize resolves one absolute-or-percentage CSS length against
// axisSize, returning 0 when the value isn't a length at all (so callers can
// fall through to their next source). A resolved length is floored at 1: this
// engine can't render a zero-width/zero-height box, so flex-basis:0 - what the
// common `flex: 1` shorthand expands to - starts at one cell rather than none.
func resolveFlexAxisSize(v string, axisSize int) int {
	abs, pct, ok := parseFlexSizeVal(v)
	if !ok {
		return 0
	}
	if pct > 0 {
		return max(1, int(pct*float64(axisSize)))
	}
	return max(1, abs)
}

// parseFlexSizeVal is parseSizeVal (table.go) extended to accept a *zero*
// length. parseSizeVal reports only n > 0 as a length, since everywhere else in
// this engine a zero size and an absent one mean the same thing - but not here:
// flex-basis distinguishes "0" (a definite base size of nothing, which is what
// makes flex-grow split the container's whole main size by weight) from unset/
// auto (fall back to width, then to the item's natural content size). Reading a
// declared 0 as "not a length" sent resolveMainBasis/parseColumnFlexBasis down
// the auto path, so `flex: 1` - whose shorthand expansion is `1 1 0` - laid out
// as `1 1 auto`: two items with different content got content-proportional
// widths rather than equal ones.
//
// The zero is still floored to one cell by each caller's own max(1, ...) (see
// resolveFlexAxisSize), so grow ratios stay slightly off for items with
// differing flex-grow weights - the documented tradeoff, since no box here can
// render in zero columns/lines. See CSS.md and COMPATIBILITY.md.
func parseFlexSizeVal(s string) (abs int, pct float64, ok bool) {
	if abs, pct, ok := parseSizeVal(s); ok {
		return abs, pct, true
	}
	t := strings.TrimSpace(s)
	t = strings.TrimSuffix(t, "%")
	t = strings.TrimSuffix(t, "ch")
	if f, err := strconv.ParseFloat(t, 64); err == nil && f == 0 {
		return 0, 0, true
	}
	return 0, 0, false
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
	ceilings := make([]int, len(items))
	for _, i := range group {
		grows[i] = items[i].grow
		shrinks[i] = items[i].shrink
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
		// (the real spec's "scaled flex shrink factor"), floored at each item's
		// automatic minimum size. The floors are resolved here rather than
		// alongside grows/ceilings above because flexMainAxisFloor measures
		// min-content with a trial render, and a line that isn't overflowing
		// has no use for the answer.
		floors := make([]int, len(items))
		for _, i := range group {
			floors[i] = r.flexMainAxisFloor(items[i], innerW)
		}
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

	align := decls["align-items"]
	itemBoxes := make(map[int]box, len(group))
	itemPositions := make(map[int]map[*html.Node]Rect, len(group))
	outerHeights := make(map[int]int, len(group))
	// quoteBefore[i] captures r.quoteDepth immediately before item i's own
	// render, so the stretch pass below can replay a re-rendered item from the
	// correct starting depth - content: open-quote/close-quote mutates
	// r.quoteDepth live (see block.go), and layoutFlexColumn's own
	// repeated-render site does the same for the same reason.
	quoteBefore := make(map[int]int, len(group))
	height := max(1, minHeight)
	for _, i := range group {
		it := items[i]
		w := max(1, widths[i])
		quoteBefore[i] = r.quoteDepth
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

	// align-items: stretch (the default) means the item's own box grows to the
	// line's cross size, not just that blank rows are parked underneath it -
	// so a bordered or filled item has to be re-rendered at that height rather
	// than padded from outside, or its border closes at its content height and
	// leaves the stretch visible as a gap below the box. Only items that
	// actually come up short are re-rendered, and only when their own cross
	// size is auto: an explicit height opts an item out of stretching, per
	// spec. quoteDepth is replayed per item and restored afterward (see
	// quoteBefore above), since this is a second render of content the first
	// pass already walked.
	quoteAfter := r.quoteDepth
	for _, i := range group {
		it := items[i]
		if a := itemAlign(it, align); a != "" && a != "stretch" {
			continue
		}
		if abs, _, ok := parseSizeVal(it.decls["height"]); ok && abs > 0 {
			continue
		}
		want := height - mt[i] - mb[i]
		// Stretching makes the item's cross size "as close to the line's as
		// possible, while still respecting the constraints imposed by
		// min-height/max-height" (§8.3) - so an item's own max-height caps how
		// far it stretches. Without this the synthetic height injected by
		// renderFlexItemBoxSized simply overrode max-height (block.go's height
		// takes priority over max-height, by design), and a max-height:1 item on
		// a five-row line grew to all five. The cap is an outer size like want
		// itself, while max-height is content-box in this engine, so the item's
		// own border/padding rows are added back on. Percentage max-heights
		// aren't resolved, matching this engine's height handling generally.
		if m, _, ok := parseSizeVal(it.decls["max-height"]); ok && m > 0 {
			want = min(want, m+blockVerticalChrome(it.decls))
		}
		if want <= len(itemBoxes[i].lines) {
			continue
		}
		w := max(1, widths[i])
		r.quoteDepth = quoteBefore[i]
		b, pos := r.renderFlexItemBoxSized(it, w, w, want)
		b = alignLinesBox(b, it.decls["text-align"], w)
		itemBoxes[i], itemPositions[i] = b, pos
		outerHeights[i] = mt[i] + len(b.lines) + mb[i]
	}
	r.quoteDepth = quoteAfter

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
		// Column bookkeeping tracks the box's *painted* width, not the width the
		// main-axis pass allotted it. The synthetic overflow-x in
		// renderFlexItemBoxSized keeps the two equal for ordinary content, but
		// they can still legitimately diverge - a flex-container item's basis is
		// measured by layoutFlex, which excludes the container's own border and
		// padding, so such an item can paint wider than its allotment. Reading
		// the width off the box is what keeps this honest either way: whatever
		// ends up on the line is what the next item's column is measured from,
		// so a Rect can't come to point at columns another item is painting.
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
			if minH := flexShrinkFloor(it.decls, containerHeight); minH > basisH[i] {
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
	// `flex-basis: content` sizes an item from its content while ignoring its
	// own main-size property - height, in column direction. parseColumnFlexBasis
	// already returns 0 for it (no synthetic height override), but that alone
	// would leave the item's own declared height to size it through the ordinary
	// block box model, which is the one thing the keyword is asking not to
	// happen. Stripping it here is what makes `content` differ from `auto`.
	// Rendered from renderItems throughout, so the flex-grow/flex-shrink
	// re-render below stays consistent with the basis pass.
	renderItems, cloned := items, false
	for i, it := range items {
		if !flexBasisIsContent(it.decls) || it.decls["height"] == "" {
			continue
		}
		if !cloned {
			renderItems, cloned = append([]flexItem(nil), items...), true
		}
		renderItems[i] = itemIgnoringSizeDecl(it, "height")
	}
	for i := range items {
		quoteBefore[i] = r.quoteDepth
		b, pos := r.renderFlexItemBoxHeight(renderItems[i], widths[i], basisH[i])
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
			floors[i] = flexShrinkFloor(it.decls, containerHeight)
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
	for i := range items {
		if finalHeights[i] != heights[i] {
			r.quoteDepth = quoteBefore[i]
			boxes[i], poses[i] = r.renderFlexItemBoxHeight(renderItems[i], widths[i], finalHeights[i])
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
		// A stretched item's cross size is the container's content width, even
		// though its painted box is left at whatever it rendered to (see
		// crossSized above - padding it out would stop an inline-flex container
		// shrinking to fit). The blank columns to its right are the item's, not
		// the container's, so its Rect has to claim them: reporting the painted
		// width instead made a click in that region hit-test to the container.
		// widths[i] is innerW for a stretched item and the already-enforced
		// cross width for every other, so max() covers both - and leaves an item
		// that overflowed its allotment reporting what it actually painted.
		positions[it.node] = Rect{Row: row, Col: colOffset, Width: max(b.width, widths[i]), Height: len(b.lines)}
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
	// Under a shrink-to-fit measurement render, report the laid-out content's
	// own width rather than the available width — the flex-container mirror of
	// renderBlockContentBox's identical narrowing, and what lets a nested
	// display:flex item measure as its items' natural extent. See
	// Engine.shrinkToFit.
	if r.shrinkToFit && !hasExplicitWidth && content.width < innerW {
		innerW = max(1, content.width)
		hBorderWidth = textcell.Width(bl.char) + pl + innerW + pr + textcell.Width(br.char)
	}
	// overflow-x: hidden/clip truncates each line to the container's content
	// width, the same as renderBlockContentBox does for an ordinary block box
	// (block.go) - a flex container is no exception to CSS.md's overflow-x
	// entry, but this used to be the one box model that never consulted it, so
	// an author's own `overflow: hidden` on a flex container did nothing and a
	// nested flex item that exceeded its allotment could only be trimmed from
	// outside, which cut its right border glyph off.
	//
	// Unlike block.go's, this is not gated on the container having an explicit
	// width. That gate exists because a plain block with no declared width
	// already fills its available width and so has nothing to clip - an
	// assumption a flex container breaks, since items floored at their
	// automatic minimum size (flexMainAxisFloor) can overflow a container that
	// declared no width at all. Gating here would make `overflow: hidden` do
	// nothing in exactly the case it's reached for. See CSS.md's overflow entry,
	// which carries the same carve-out.
	//
	// Deliberately not extended
	// to scroll/auto: a horizontally scrollable flex container would need the
	// live offset/gutter plumbing block.go has for ordinary boxes (see
	// docs/SCROLLING.md), which is a larger piece of work than this.
	if ov := decls["overflow-x"]; ov == "hidden" || ov == "clip" {
		suffix := textOverflowSuffix(decls["text-overflow"])
		lines := make([]string, len(content.lines))
		for i, ln := range content.lines {
			lines[i] = textcell.TruncateToWidth(ln, innerW, suffix)
		}
		content = box{lines: lines, width: linesWidth(lines)}
	}
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
// availWidth really is only a bound: an inline-flex container is shrink-to-fit
// (real CSS sizes it by fit-content, not by its containing block), so it lays
// out at its own natural extent whenever that's narrower. Handing availWidth
// straight through as the container's main size instead made it a *definite*
// size that flex-grow and justify-content then had free space to spread items
// across - `display: inline-flex; justify-content: space-between` pushed its
// items to the far edges of the surrounding text's wrap bound rather than
// sitting compactly wherever it was placed.
func (r *Engine) renderInlineFlexContent(n *html.Node, decls map[string]string, availWidth int) string {
	if r.measuringNaturalWidth && availWidth > measureBlockWidthCap {
		availWidth = measureBlockWidthCap
	}
	if w, constrained := resolveWidthConstraints(decls, availWidth, availWidth); constrained && w > 0 {
		// A declared width (or a min/max-width that bites) is a definite main
		// size: shrink-to-fit is what an *auto* size resolves to, and
		// justify-content/flex-grow do have that space to distribute.
		availWidth = w
	} else if natural := r.inlineFlexNaturalWidth(n, decls, availWidth); natural > 0 && natural < availWidth {
		availWidth = natural
	}
	b, _ := r.layoutFlex(n, decls, availWidth)
	return b.join()
}

// inlineFlexNaturalWidth is an inline-flex container's shrink-to-fit main-axis
// extent: in row direction its items' own resolved base sizes plus the gaps
// between them, in column direction its widest single item. flex-grow and
// justify-content are deliberately not consulted - both only distribute space
// that a definite container size makes available, and the whole point of a
// shrink-to-fit container is that it has none to hand out. flex-shrink isn't
// either: a natural width wider than availWidth is simply capped by the
// caller, which is what puts the items back into a shrinking line.
func (r *Engine) inlineFlexNaturalWidth(n *html.Node, decls map[string]string, availWidth int) int {
	items := r.collectFlexItems(n)
	if len(items) == 0 {
		return 0
	}
	if isColumn, _ := parseFlexDirection(decls); isColumn {
		widest := 0
		for _, it := range items {
			widest = max(widest, r.resolveCrossWidth(it, availWidth))
		}
		return widest
	}
	total := parseGapLen(decls["column-gap"]) * (len(items) - 1)
	for _, it := range items {
		total += r.resolveMainBasis(it, availWidth)
	}
	return total
}
