package render

import (
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/client9/htmlterm/internal/cssengine"
	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// This file implements htmlterm's CSS flexbox subset: display:flex/
// inline-flex, flex-direction:row|row-reverse|column|column-reverse,
// justify-content, align-items, align-self, order, gap/row-gap/column-gap,
// flex-grow/flex-shrink/flex-basis, and, in row direction only,
// flex-wrap (including wrap-reverse), align-content, and auto margins on
// either axis. A run of loose text in a container is an anonymous item.
//
// Main-axis size is resolved before grow and shrink distribute any leftover
// space. In row direction that leftover is always available, since width is
// always definite. In column direction it is only available once the
// container has an explicit CSS height, this engine's only notion of a
// column container's main-axis size. An auto main-axis margin absorbs
// leftover space, overriding justify-content.
//
// See CSS.md's "Flexbox" section for the exact supported subset and its
// documented non-goals: column-direction wrap and baseline alignment.

// flexItem is one direct element child of a flex container considered for
// layout, together with the sizing inputs the main-axis pass needs.
type flexItem struct {
	node   *html.Node
	decls  map[string]string
	grow   float64
	shrink float64
	order  int
}

// itemAlign resolves align-self's fallback to the container's align-items.
// Both "auto", the property's real default, and unset mean "defer to the
// container", matching CSS's own align-self semantics. The result is
// normalized by normalizeAlignKeyword, so every caller switching on it sees
// the flexbox-era spelling of whatever the author wrote.
func itemAlign(it flexItem, containerAlign string) string {
	if v := it.decls["align-self"]; v != "" && v != "auto" {
		return normalizeAlignKeyword(v)
	}
	return normalizeAlignKeyword(containerAlign)
}

// normalizeAlignKeyword folds the CSS Box Alignment module's newer keyword
// spellings onto the original flexbox ones this file switches on. In a flex
// container `start`/`self-start` and `end`/`self-end` mean what
// `flex-start`/`flex-end` do. `normal` behaves as each property's own
// default: stretch for align-items/align-self, flex-start for
// justify-content/align-content. That is what the unset value already
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
// column-reverse. Reversing the item sequence, which reverseFlexItems does,
// gets only half the job done. A reverse direction also moves the
// main-*start* edge to the right for row-reverse or the bottom for
// column-reverse, so the default justify-content:flex-start packs the
// already-reversed sequence against that far edge, not against the left or
// top one this file's layout passes work from.
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

// reverseFlexItems returns items in reverse order, for column-reverse. It is
// applied after order-based sorting, matching CSS's own "order determines
// position, then the reverse direction flips the whole sequence" behavior. A
// column container is always single-line here, since column-direction
// wrapping is a documented non-goal, so reversing the whole sequence and
// reversing "the line" are the same thing.
//
// Row direction must not use this: with flex-wrap it would reverse the
// sequence line breaking runs over, not just the placement within each line.
// See reverseLineGroups.
func reverseFlexItems(items []flexItem) []flexItem {
	out := make([]flexItem, len(items))
	for i, it := range items {
		out[len(items)-1-i] = it
	}
	return out
}

// reverseLineGroups flips the item order within each already-broken flex line,
// for flex-direction: row-reverse. Items are collected into lines in
// order-modified *document* order, since §9.3 "Main Size Determination"
// collects into lines before §9.5 places anything. The reversed direction only
// moves where along the main axis each line's items land. It does not change
// which line they land on.
//
// Reversing the whole item sequence before line breaking instead, which is what
// this used to do, changes line membership: five equal items over two lines came
// out as 5/4/3 then 2/1, where a browser gives 1/2 then 3/4 then 5, each line
// then painted right-to-left. Both the grouping and the contents were wrong.
// Groups are reversed in place, since each index belongs to exactly one line
// and nothing has read them yet.
func reverseLineGroups(lines [][]int) {
	for _, group := range lines {
		slices.Reverse(group)
	}
}

// isFlexDisplay reports whether display is one of the flex container values.
func isFlexDisplay(display string) bool {
	return display == "flex" || display == "inline-flex"
}

// parseFlexDirection resolves flex-direction into (isColumn, reverse). Only
// the four real keywords are recognized. Anything else, including an unset or
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

// pctHeightDecl reports whether decls carries a percentage height or
// min-height, the only case layoutFlex has to clone decls to normalize. Every
// other container, which is nearly all of them, keeps the caller's map.
func pctHeightDecl(decls map[string]string) bool {
	return strings.HasSuffix(strings.TrimSpace(decls["height"]), "%") ||
		strings.HasSuffix(strings.TrimSpace(decls["min-height"]), "%")
}

// layoutFlex dispatches to layoutFlexRow or layoutFlexColumn per decls'
// flex-direction. It is the single call site every renderFlexContentBox,
// renderInlineFlexContent, and measureNaturalWidth entry point shares, so
// direction parsing can't drift between them.
func (r *Engine) layoutFlex(n *html.Node, decls map[string]string, innerW int) (box, map[*html.Node]Rect) {
	if r.measuringNaturalWidth && innerW > measureBlockWidthCap {
		// renderInlineFlexContent calls layoutFlex directly, bypassing
		// renderFlexContentBox's own clamp above. Guard here too, since
		// this is the single shared entry point (see doc comment).
		innerW = measureBlockWidthCap
	}
	// A flex container is the containing block its items resolve percentage
	// heights against, and it is definite on the same terms as any other box:
	// only a declared height, never a min-height.
	//
	// The container's own percentage height and min-height are resolved to
	// absolutes here, once, and written back into a cloned decls. Everything
	// downstream then reads a definite row count and never re-resolves. That
	// is not just an optimization. r.cbHeight is about to become this
	// container's own height, and layoutFlexColumn re-reads decls["height"]
	// several times during layout, so leaving a percentage in place would have
	// it resolve against itself: `height: 50%` in a 20-row containing block
	// would come out 10 on the first read and 5 on the next.
	//
	// renderFlexItemBoxSized separately injects each item's resolved main size
	// as a synthetic absolute height, which renderBlockContentBox then makes
	// that item's own containing block. This handles the other direction: an
	// item with no flex-resolved height of its own still sees its container's.
	if pctHeightDecl(decls) {
		resolved := maps.Clone(decls)
		if h, ok := resolveCSSHeight(decls["height"], r.cbHeight); ok && h > 0 {
			resolved["height"] = strconv.Itoa(h)
		} else if decls["height"] != "" {
			delete(resolved, "height")
		}
		if h, ok := resolveCSSHeight(decls["min-height"], r.cbHeight); ok && h > 0 {
			resolved["min-height"] = strconv.Itoa(h)
		} else if decls["min-height"] != "" {
			delete(resolved, "min-height")
		}
		decls = resolved
	}
	outerCBHeight := r.cbHeight
	if h, ok := r.resolveFlexContainerHeight(decls); ok {
		r.cbHeight = h
	} else {
		r.cbHeight = indefiniteMainSize
	}
	defer func() { r.cbHeight = outerCBHeight }()
	isColumn, reverse := parseFlexDirection(decls)
	if isColumn {
		return r.layoutFlexColumn(n, decls, innerW, reverse)
	}
	return r.layoutFlexRow(n, decls, innerW, reverse)
}

// parseFlexGrow parses flex-grow. The default is 0, and negative values are
// invalid per spec and treated as unset. Parsing goes through
// cssengine.ParseNumber rather than strconv, so only real CSS <number> syntax
// is accepted. A non-finite value like "NaN" would otherwise sail through the
// `f < 0` check, since NaN compares false against everything, and then poison
// the total weight every share is computed against, silently zeroing out
// well-formed siblings.
func parseFlexGrow(decls map[string]string) float64 {
	f, ok := cssengine.ParseNumber(decls["flex-grow"])
	if !ok || f < 0 {
		return 0
	}
	return f
}

// parseFlexShrink parses flex-shrink. The default is 1 per spec, and negative
// values are invalid per spec and treated as unset, falling back to that same
// default. See parseFlexGrow on the parser choice.
func parseFlexShrink(decls map[string]string) float64 {
	f, ok := cssengine.ParseNumber(decls["flex-shrink"])
	if !ok || f < 0 {
		return 1
	}
	return f
}

// flexMainAxisHeightFloor resolves how far flex-shrink may take a
// column-direction item below its flex base size. It is the vertical
// counterpart of flexMainAxisFloor and applies the same rule: CSS's
// *automatic minimum size* (Flexbox §4.5), which is `min-height`'s own initial
// value of `auto` resolving to the content-based minimum on a flex item's main
// axis. An item is never shrunk below the height its content needs.
//
// The two opt-outs are flexMainAxisFloor's, read on the vertical axis:
//
//   - An explicitly declared min-height wins outright, plus the item's own
//     vertical chrome, since flex sizes are outer while min-height is
//     content-box here. See clampFlexMainHeight.
//   - A non-visible overflow-y computes it to 0, since such an item clips or
//     scrolls itself. Unlike the row axis, this is also exactly when block.go
//     will truncate the item to the height flex hands it, so it is the one
//     case where shrinking below the content does anything at all.
//
// The content-based minimum needs no min-content probe the way the row axis
// does. An item's content height at its used cross size *is* its minimum,
// since text can't be reflowed into fewer lines, and contentHeight is that
// height already measured. It is then clamped by a definite height and by
// max-height, per Sizing 3 §5.1's "clamped by the specified size suggestion"
// and "clamped by the maximum main size if it's definite". That is what still
// lets an item that asked to be short overflow rather than floor the container
// at its content.
//
// This is why the floor can't just be 1, which is what it used to be absent a
// min-height. take() would report absorbing rows the item then refuses to give
// up, because block.go pads to a synthetic height but never truncates to one
// without an explicit overflow. So the deficit was recorded as placed while
// the item went on painting its full content. The items that genuinely could
// shrink were under-shrunk by exactly that much, and the container overflowed
// its own declared height.
//
// contentHeight is the item's already-rendered height. That is its content
// height only when the item sized itself (renderH == 0). An item rendered at a
// declared flex-basis needs a second, override-free render to find out how tall
// its content actually is, which is why this can cost a trial render and is only
// called for a container that's actually overflowing.
func (r *Engine) flexMainAxisHeightFloor(it, renderIt flexItem, width, contentHeight, renderH, axisSize int) int {
	chrome := blockVerticalChrome(it.decls)
	if v, ok := resolveFlexCSSSize(it.decls["min-height"], axisSize); ok {
		return max(1, v+chrome)
	}
	if !isVisibleOverflow(it.decls["overflow-y"]) {
		return 1
	}
	floor := contentHeight
	if renderH > 0 {
		floor = r.measureContentHeight(renderIt, width)
	}
	if h, ok := resolveFlexCSSSize(it.decls["height"], axisSize); ok && h+chrome < floor {
		floor = h + chrome
	}
	if m, ok := resolveFlexCSSSize(it.decls["max-height"], axisSize); ok && m+chrome < floor {
		floor = m + chrome
	}
	return max(1, floor)
}

// measureContentHeight renders it at width with no main-axis size override,
// only to learn how many rows its content comes to, then discards the render.
// It is the vertical counterpart of measureNaturalWidth, used for the same
// reason: this engine has no way to compute a box's height short of laying it
// out. r.quoteDepth is saved and restored around the call so the trial leaves
// no state behind for the real render, since content: open-quote/close-quote
// mutates it live (see block.go). measureNaturalWidth does the same.
func (r *Engine) measureContentHeight(it flexItem, width int) int {
	saved := r.quoteDepth
	b, _ := r.renderFlexItemBoxHeight(it, width, 0)
	r.quoteDepth = saved
	return len(b.lines)
}

// flexMainAxisFloor resolves how far flex-shrink may take a row-direction item
// below its flex base size: CSS's *automatic minimum size* for flex items
// (Flexbox §4.5, defined in CSS Sizing 3 §5.1). That is the real reason a
// browser almost never shows an over-shrunk flex item overflowing its own box.
// min-width's initial value is `auto`, not 0, and on a flex item in the main
// axis that resolves to the content-based minimum, so flex-shrink cannot take
// an item below the width its content needs.
//
// The two opt-outs are the spec's own, and are why `min-width: 0` and
// `overflow: hidden` are the two standard fixes for a flex item that refuses to
// shrink. They are the same lever pulled two ways:
//
//   - An explicitly declared min-width wins outright, including a declared 0,
//     which parseFlexSizeVal, via resolveFlexCSSSize, can now tell apart from
//     unset. The floor is still one cell, since no box here renders in zero
//     columns.
//   - The automatic minimum applies only "on a flex item whose overflow is
//     visible in the main axis". For any other overflow-x it computes to 0.
//     Such an item already clips or scrolls itself in renderBlockContentBox,
//     where the resolved main size reaches it as a synthetic width (see
//     renderFlexItemBoxSized), so letting it shrink freely is what the author
//     asked for.
//
// The content-based minimum is the item's min-content width, measured by
// rendering it at a cap of one column. Every breakable point breaks, so what's
// left is the widest unbreakable run plus the item's own chrome, which is
// min-content. It is then clamped by the specified size suggestion, a definite
// width, and by the definite maximum main size, per Sizing 3 §5.1's "in all
// cases, the size is clamped by the maximum main size if it's definite". That
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
	// Both clamps read a declared zero as a real length via
	// resolveFlexCSSSize, for the same reason min-width's own opt-out above
	// does: `width: 0` is a specified size suggestion of nothing and
	// `max-width: 0` a definite maximum of nothing, and treating either as
	// absent would leave the item floored at its content after the author
	// asked for the opposite.
	floor := r.measureMinContentWidth(it)
	if w, ok := resolveFlexCSSSize(it.decls["width"], innerW); ok && w < floor {
		floor = w
	}
	if m, ok := resolveFlexCSSSize(it.decls["max-width"], innerW); ok && m < floor {
		floor = m
	}
	return max(1, floor)
}

// measureMinContentWidth measures an item's min-content width, the *content
// size suggestion* half of the automatic minimum size above, by rendering it
// at a cap of one column, so every breakable point breaks and what's left is
// the widest unbreakable run plus the item's own chrome.
//
// The item's own width/min-width/max-width are stripped for the measurement.
// Intrinsic sizes are computed from content, ignoring the box's specified size.
// Leaving `width: 10` in place would hand that 10 back as the "min-content"
// answer, floor the item at its own declared width, and make flex-shrink inert
// for the items most likely to need it. The specified width is then applied by
// the caller as a separate clamp, which is how the spec composes the two.
// Sizing 3 §5.1 takes the smaller of the specified size suggestion and the
// content size suggestion.
//
// Memoized per node in Engine.minContentCache. The answer doesn't depend on the
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
// `visible`, the initial value, so unset counts. Every other value — hidden,
// clip, scroll, auto — makes the box its own clipping or scrolling context,
// which is what disables the automatic minimum size above.
//
// Comparing lowercase spellings is safe. Keyword values are folded once in the
// cascade (cssengine's keywordcase.go), so an author's `overflow-X: HIDDEN`
// arrives here already lowercased. Before that folding existed this predicate
// read an uppercase value as non-visible while renderFlexContentBox's own
// `== "hidden"` check did not, which put such an item in the one combination
// this file works to avoid: no automatic minimum *and* no clip.
func isVisibleOverflow(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "visible"
}

// indefiniteMainSize is the axisSize a caller passes to say that percentages
// have nothing to resolve against on this axis, which resolveFlexCSSSize
// reports as no length at all.
//
// Only column direction's main axis is ever indefinite here. A row's width is
// however wide the caller says to render at, and a column container's cross
// axis is that same width, but a column container's *height* is definite only
// when it declares one (see resolveFlexContainerHeight). Passing a plain 0 for
// that case is what this constant exists to prevent: 0 is a perfectly good
// percentage basis arithmetically, so `max-height: 50%` came back as a definite
// maximum of zero rows rather than as an absent one, and froze the item at one
// row instead of leaving it unbounded. See layoutFlexColumn's pctBasis.
const indefiniteMainSize = -1

// resolveFlexCSSSize is resolveCSSSize (block.go) over parseFlexSizeVal, so a
// declared zero reads as a real length rather than as an absent one. See
// parseFlexSizeVal. Used where the difference between "0" and unset carries
// meaning, which in flex layout is min-width's opt-out from the automatic
// minimum size.
//
// A percentage against indefiniteMainSize is not a length. CSS resolves a
// percentage size against an indefinite basis to `auto` (for a preferred size)
// or `none` (for a maximum), so reporting false, and leaving every caller to
// fall through to whatever it does for an unset value, is exactly that rule.
func resolveFlexCSSSize(s string, axisSize int) (int, bool) {
	abs, pct, ok := parseFlexSizeVal(s)
	if !ok {
		return 0, false
	}
	if pct > 0 {
		if axisSize < 0 {
			return 0, false
		}
		return int(pct * float64(axisSize)), true
	}
	return abs, true
}

// flexGrowCeiling resolves how far flex-grow may increase an item's main-axis
// size: an explicit max-width in row direction or max-height in column
// direction, including a percentage resolved against axisSize, if set. It
// returns 0 to mean uncapped.
//
// Resolved through resolveFlexCSSSize, so a declared zero is a real maximum
// rather than an absent one. That is the same distinction min-width already
// needs (see parseFlexSizeVal), read from the other end. `max-width: 0` asks
// for an item that doesn't grow at all; reading it as "no ceiling" let such an
// item absorb the whole line. The ceiling is still floored at one cell, since
// no box here renders in zero columns, which is also what keeps 0 usable as
// this function's "uncapped" sentinel.
func flexGrowCeiling(decls map[string]string, key string, axisSize int) int {
	if v, ok := resolveFlexCSSSize(decls[key], axisSize); ok {
		return max(1, v)
	}
	return 0
}

// flexGrowHeightCeiling is flexGrowCeiling on the vertical axis, where the
// ceiling and the size it caps are not measured in the same box. Flex layout
// resolves outer sizes, border rules and padding rows included, while
// max-height is content-box in this engine, so the item's own vertical chrome
// is added back before the ceiling is compared against a main size. That is the
// same conversion clampFlexMainHeight, flexMainAxisHeightFloor, and
// layoutFlexLine's stretch cap all make. Without it here, a bordered item
// stopped growing exactly its own border and padding rows short of its
// max-height. Row direction needs no counterpart, since width is a border-box
// size already.
func flexGrowHeightCeiling(decls map[string]string, axisSize int) int {
	if v, ok := resolveFlexCSSSize(decls["max-height"], axisSize); ok {
		return max(1, v+blockVerticalChrome(decls))
	}
	return 0
}

// clampFlexWidth clamps a resolved horizontal flex size to the item's own
// min-width and max-width. In row direction that produces the spec's
// "hypothetical main size", the size line-breaking and grow/shrink both start
// from. In column direction it bounds the cross size. The maximum is applied
// first and the minimum second, so a minimum larger than the maximum wins,
// matching CSS's own min/max resolution order. clampFlexMainHeight is the
// vertical counterpart, kept separate because min-height/max-height are
// content-box here while flex sizes are outer.
//
// Without this, an item's declared min/max still reached the item's *own*
// render, since renderBlockContentBox resolves them in
// resolveWidthConstraints, but not the flex layout around it. The box then
// painted at a different width than the column flex layout had allotted it:
// every following item on the line drifted, and the recorded Rects, which is
// what click hit-testing reads, pointed at the wrong columns.
//
// A declared zero is a real bound on both ends, hence resolveFlexCSSSize
// rather than resolveCSSSize. `max-width: 0` asks for an item that can't grow,
// and reading it as an absent maximum would let that item take the whole line.
// The result is still floored at one cell, since no box here renders in zero
// columns.
func clampFlexWidth(decls map[string]string, size, axisSize int) int {
	if v, ok := resolveFlexCSSSize(decls["max-width"], axisSize); ok && size > v {
		size = v
	}
	if v, ok := resolveFlexCSSSize(decls["min-width"], axisSize); ok && size < v {
		size = v
	}
	return max(1, size)
}

// proportionalShares splits total into one share per group member, weighted
// by weights[i], where members with a non-positive weight get nothing. It uses
// the largest-remainder method: each member first takes the floor of its exact
// share, then the units that flooring left unassigned go one apiece to the
// members with the largest fractional parts, earlier members winning ties.
// The returned slice is indexed by position within group, not by item index.
//
// The two properties that matter here are that shares sum to exactly total
// and that none is ever negative. Rounding each member's share independently
// with round-half-up and handing leftover-assigned to the last member, which
// is what distributeFlexGrow and distributeFlexShrink used to do, guaranteed
// neither. The per-member round-ups accumulate, so that last member could be
// handed a *negative* remainder of up to half a unit per member, ending up
// narrower than its own flex-basis or outright negative. Twenty items at basis
// 2 sharing 10 leftover columns left the last one at -7, overflowing the
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
// take(i, units). take applies whatever it can and returns how many units the
// member actually absorbed, fewer than offered once that member runs into its
// own limit. A member that couldn't take its whole share is frozen at its
// limit and the units it left behind are re-split across the members that can
// still move, repeating until either all of total is placed or nothing more
// can move. Returns the units that could not be placed anywhere.
//
// The repeat is the point. The spec resolves flexible lengths this way:
// "freeze, recompute the free space, repeat". A single pass gets it wrong
// whenever any item hits a limit. Three items of 10 with shrink 1 and a
// deficit of 6, the last floored at min-width:9, used to end up 25 wide. The
// unit the floored item couldn't give back was dropped rather than taken from
// the two items that still had room.
//
// factors is the members' *raw* flex-grow/flex-shrink factors, used only for
// the spec's sub-1 clamp, §9.7 step 4b: "if the sum of the unfrozen flex
// items' flex factors is less than one, multiply the initial free space by
// this sum; if the magnitude of this value is less than the magnitude of the
// remaining free space, use this as the remaining free space". It is a
// separate argument from weights because the shrink path weights by the scaled
// flex shrink factor, shrink x base size, while the clamp is defined on the
// unscaled one. Without the clamp a lone flex-grow:0.5 item absorbed all of
// the free space rather than half of it, since proportionalShares normalizes
// by the total weight and so can't tell 0.5 from 5.
func distributeFlexSpace(weights, factors []float64, group []int, total int, take func(i, units int) int) int {
	active := group
	initial := total
	// absorbed[i] is what member i has taken across all rounds so far. The
	// spec recomputes each unfrozen item's *target* size from its flex base
	// every round, so the sub-1 clamp bounds a member's cumulative share, not
	// each round's increment. Subtracting what the still-active members
	// already hold is what turns this loop's incremental offers back into that
	// target-based bound. Without it, a clamped round that froze somebody
	// would re-offer the survivors their whole entitlement a second time.
	absorbed := make(map[int]int, len(group))
	for total > 0 && len(active) > 0 {
		// The clamp is recomputed every round against the still-unfrozen
		// members, since a frozen member's factor drops out of the sum, but
		// always applies to the *initial* free space, as the spec has it.
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
				// Absorbed everything offered, so not yet at its limit, and
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
// to sizes[i], weighted by grows[i]. It is shared by row direction's per-line
// width distribution and column direction's height distribution. i ranges over
// group, whose entries index into the full item-count-sized
// sizes/grows/ceilings arrays, matching widths/mt/mb's own convention.
//
// ceilings, if non-nil, caps sizes[i]'s growth at ceilings[i], where 0 means
// uncapped. That is real CSS's max-width/max-height on a flex item,
// redistributed to the items that can still grow (see distributeFlexSpace).
// Returns however much of leftover wasn't absorbed, which is 0 unless every
// growable member hit its ceiling before leftover was exhausted, or no member
// can grow at all.
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

// growFlexLine implements the grow half of CSS Flexbox §9.7's flexible-length
// resolution over one line, or in column direction over the container's single
// stack of items. Every unfrozen item's target main size is recomputed each
// round from its *flex base size*, not from its hypothetical main size. The
// hypothetical is then applied as the round's minimum, freezing any item that
// violated it and re-running the distribution over what's left.
//
// The distinction is the whole point of this function. The hypothetical main
// size is the flex base size already clamped by the item's used minimum, which
// in the main axis is `min-width: auto`, the automatic minimum size
// (flexMainAxisFloor). Seeding growth from it hands every item its own
// min-content width for free, before any weighting, and only splits what's
// left. Two `flex: 1` items in a 40-column container then come out 13/27
// rather than 20/20, sized by their content rather than by their equal flex
// factors. The spec instead has that minimum bite only as a step-4c violation:
// items that land above it keep their weighted share, and only the ones that
// land below it are frozen at the minimum, with the space they took redivided
// among the rest (§9.7 steps 3-4, "fix min/max violations, freeze, repeat").
//
// sizes is written for every member of group. bases/hypos/ceilings are indexed
// the same way (see distributeFlexGrow on the convention), and ceilings may be
// nil. Returns however much of the free space could not be placed, for
// justify-content to distribute.
func growFlexLine(sizes, bases, hypos, ceilings []int, grows []float64, group []int, avail int) int {
	frozen := make([]bool, len(sizes))
	for _, i := range group {
		sizes[i] = hypos[i]
		// §9.7 step 3, "size inflexible items". An item that can't grow at all,
		// or whose flex base size its own maximum already clamped *down* to the
		// hypothetical, is frozen there and takes no part in the distribution.
		frozen[i] = grows[i] <= 0 || bases[i] > hypos[i]
	}
	for {
		active := make([]int, 0, len(group))
		used := 0
		for _, i := range group {
			if frozen[i] {
				used += sizes[i]
				continue
			}
			sizes[i] = bases[i]
			used += bases[i]
			active = append(active, i)
		}
		free := avail - used
		if len(active) == 0 || free <= 0 {
			return max(0, free)
		}
		leftover := distributeFlexGrow(sizes, grows, active, free, ceilings)
		violated := false
		for _, i := range active {
			if sizes[i] < hypos[i] {
				sizes[i] = hypos[i]
				frozen[i] = true
				violated = true
			}
		}
		if !violated {
			return leftover
		}
	}
}

// distributeFlexShrink subtracts each group member's proportional share of
// deficit from sizes[i], weighted by shrinks[i]*bases[i], the real spec's
// "scaled flex shrink factor" scaled by the inner flex base size, and floored
// at floors[i]. It is shared by row direction's width distribution and column
// direction's height distribution.
//
// Returns however much of deficit wasn't absorbed, which is 0 unless every
// shrinkable member hit its floor before deficit was exhausted, or no member
// can shrink at all. Floored members' unabsorbed share is redistributed to
// the members that can still shrink, and shares come from proportionalShares,
// for the same reasons distributeFlexGrow's do. See distributeFlexSpace.
//
// The scaled shrink factors are computed once, from each member's flex base
// size rather than from its current or hypothetical one, matching the spec's
// own use of the inner flex base size across every round of the loop.
//
// There is no separate freeze pass for the spec's step-3 "size inflexible
// items" rule here, the way growFlexLine needs one. An item whose used minimum
// already raised it above its flex base size arrives with floors[i] equal to
// its incoming size, so take() can move it by nothing and distributeFlexSpace
// freezes it on the first round anyway.
func distributeFlexShrink(sizes []int, shrinks []float64, bases, floors []int, group []int, deficit int) int {
	scaled := make([]float64, len(sizes))
	for _, i := range group {
		if shrinks[i] <= 0 {
			continue
		}
		scaled[i] = shrinks[i] * float64(bases[i])
	}
	return distributeFlexSpace(scaled, shrinks, group, deficit, func(i, units int) int {
		if maxReduce := sizes[i] - floors[i]; units > maxReduce {
			units = max(0, maxReduce)
		}
		sizes[i] -= units
		return units
	})
}

// parseOrder parses the CSS order property. The default is 0, and invalid
// values fall back to 0 rather than erroring.
func parseOrder(decls map[string]string) int {
	n, err := strconv.Atoi(strings.TrimSpace(decls["order"]))
	if err != nil {
		return 0
	}
	return n
}

// parseGapLen parses row-gap/column-gap as an absolute rune count. basis is the
// container's own content size along the same axis the gap applies to: its
// content width for column-gap, its content height for row-gap. A percentage
// gap resolves against that (CSS Box Alignment §8.3), truncating like every
// other percentage in this engine.
//
// A basis of 0 means the size in that axis is indefinite, which for row-gap is
// a container with no declared height and for either axis is a container being
// measured rather than laid out. A percentage against an indefinite size
// resolves to zero, matching both the spec and what a browser paints. An
// absolute gap is unaffected either way.
func parseGapLen(v string, basis int) int {
	abs, pct, ok := parseSizeVal(v)
	if !ok {
		return 0
	}
	if pct > 0 {
		return int(pct * float64(basis))
	}
	return abs
}

// parseFlexWrap parses flex-wrap into two independent facts: whether the
// container is multi-line at all, and whether its cross axis points the other
// way. "nowrap", the default and also any other or invalid value, is neither.
//
// wrap-reverse reverses the cross-start and cross-end edges (§5.2), which is
// not the same shape of change row-reverse makes to the main axis. Line
// breaking is untouched, since it runs along the main axis: the same items land
// on the same lines. What changes is where those lines stack, last line first,
// and what "start" means to everything resolving on the cross axis. See
// reverseAlignItems and reverseAlignContent.
func parseFlexWrap(decls map[string]string) (wrap, crossReverse bool) {
	switch decls["flex-wrap"] {
	case "wrap":
		return true, false
	case "wrap-reverse":
		return true, true
	}
	return false, false
}

// reverseCrossAlign mirrors an align-items, align-self, or align-content value
// for a wrap-reverse container, whose cross-start edge is the bottom rather
// than the top. Only flex-start and flex-end name an edge; center, stretch,
// and the space-* values are symmetric about the middle and are their own
// mirror image. So is anything unrecognized, including unset, since both
// properties' own initial value is stretch.
//
// This is where the cross axis differs in shape from the main axis, and why it
// isn't reverseJustify: justify-content's unset default is flex-start, which
// does name an edge, so reverseJustify has to mirror the unset case too.
func reverseCrossAlign(align string) string {
	switch normalizeAlignKeyword(align) {
	case "flex-start":
		return "flex-end"
	case "flex-end":
		return "flex-start"
	}
	return align
}

// alignContentStretches reports whether align-content resolves to stretch, its
// initial value: lines share the container's leftover cross space equally by
// growing, rather than being packed somewhere within it (§8.4).
//
// Everything this file doesn't recognize resolves to stretch as well, matching
// real CSS, where an invalid declaration is dropped and leaves the property at
// its initial value. That includes `normal`, which normalizeAlignKeyword folds
// to unset for exactly this reason.
func alignContentStretches(align string) bool {
	switch normalizeAlignKeyword(align) {
	case "flex-start", "flex-end", "center", "space-between", "space-around", "space-evenly":
		return false
	}
	return true
}

// reverseCrossAxisDecls returns decls with its cross-axis alignment properties
// mirrored, for a wrap-reverse container. The clone is what layoutFlexLine and
// distributeAlignContent read, so nothing below them needs to know the axis was
// flipped: every value they see is already expressed in the direction they
// resolve in, which is top-down. Items declaring their own align-self are
// mirrored the same way, since align-self overrides align-items rather than
// being derived from it.
func reverseCrossAxisDecls(decls map[string]string, items []flexItem) (map[string]string, []flexItem) {
	out := maps.Clone(decls)
	for _, prop := range []string{"align-items", "align-content"} {
		if v := out[prop]; v != "" {
			out[prop] = reverseCrossAlign(v)
		}
	}
	flipped := make([]flexItem, len(items))
	copy(flipped, items)
	for i, it := range flipped {
		v := it.decls["align-self"]
		if v == "" || v == "auto" {
			// Deferring to align-items, which was already mirrored above.
			continue
		}
		d := maps.Clone(it.decls)
		d["align-self"] = reverseCrossAlign(v)
		flipped[i].decls = d
	}
	return out, flipped
}

// resolveFlexContainerHeight resolves an explicit CSS height on a flex
// container to an absolute line count, for align-content's cross-axis
// distribution and for column direction's main-axis size. A percentage
// resolves against the container's own containing block (Engine.cbHeight), and
// only when that basis is itself definite, which is the same rule
// renderBlockContentBox's height handling follows.
func (r *Engine) resolveFlexContainerHeight(decls map[string]string) (int, bool) {
	h, ok := resolveCSSHeight(decls["height"], r.cbHeight)
	if !ok || h <= 0 {
		return 0, false
	}
	return h, true
}

// resolveFlexContainerHeightFloor is how many rows a flex container's own
// content box has to fill at minimum: its explicit CSS height if it declares
// one, else its min-height. Real CSS makes a container's main size definite
// from either, and the "column flex container with min-height that its
// flex-grow children fill" pattern is as common as the explicit-height one. On
// the cross axis both alike give align-items and align-content a box taller
// than the content to align within.
//
// fromMin reports that the value came from min-height, which callers resolving
// the *main* axis must respect. A minimum can only raise a container's main
// size, never lower it, so content taller than a min-height must overflow
// rather than being handed to flex-shrink as a deficit. An explicit height
// legitimately does both.
func (r *Engine) resolveFlexContainerHeightFloor(decls map[string]string) (h int, ok, fromMin bool) {
	if h, ok := r.resolveFlexContainerHeight(decls); ok {
		return h, true, false
	}
	// A percentage min-height resolves against the same containing block a
	// percentage height would. It still doesn't make *this* container's main
	// size definite for anything inside it (see fromMin), but resolving it is
	// what lets "at least half the screen" work at all.
	mh, ok := resolveCSSHeight(decls["min-height"], r.cbHeight)
	if !ok || mh <= 0 {
		return 0, false, false
	}
	return mh, true, true
}

// collectFlexItems gathers n's children that participate in flex layout: its
// direct element children, plus one anonymous item per run of loose text
// between them (see appendFlexItems). Any child with display:none is skipped,
// matching normal flow. Items are stable-sorted by
// the CSS order property, default 0, preserving document order among ties.
// row-reverse and column-reverse, via reverseFlexItems, are applied on top of
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
// which in a flex container means its children are the items. Without the
// recursion the wrapper itself became a single blockified item and its
// children stacked inside it as ordinary blocks. Their decls come from
// r.resolveDecls, which walks real ancestors, so anything inherited through
// the display:contents wrapper still reaches them.
// A run of loose text between two element children becomes one anonymous flex
// item, per Flexbox §4: "each contiguous sequence of child text runs is
// wrapped in an anonymous block container flex item". Only an element child
// breaks a run. A comment doesn't, since it generates no box and browsers
// treat the text on either side of it as one run.
func (r *Engine) appendFlexItems(items []flexItem, n *html.Node) []flexItem {
	// Resolved once per container and only if a text run is actually found,
	// since it's a full cascade resolve and the overwhelmingly common flex
	// container has no loose text in it at all.
	var parentDecls map[string]string
	var run []*html.Node
	flushRun := func() {
		if len(run) == 0 {
			return
		}
		if parentDecls == nil {
			parentDecls = r.resolveDecls(n)
		}
		if it, ok := r.anonymousFlexItem(n, run, parentDecls); ok {
			items = append(items, it)
		}
		run = nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			run = append(run, c)
			continue
		}
		if c.Type != html.ElementNode {
			continue
		}
		flushRun()
		if isSkippedContentElement(c.Data) {
			continue
		}
		decls := r.resolveDecls(c)
		// visibility: collapse on a flex item is a *collapsed flex item*
		// (Flexbox §4.4), which the spec has behave as display: none rather
		// than as visibility: hidden, which is what the same declaration means
		// on any other element (see isHiddenVisibility). A browser keeps the
		// collapsed item's cross-size strut alive so the line doesn't get
		// shorter. This engine doesn't, since the row's cross size is its
		// tallest remaining item and reserving a strut for an item nobody can
		// see would leave an unexplained blank band. See COMPATIBILITY.md.
		if decls["display"] == "none" || decls["visibility"] == "collapse" || r.outOfFlow[c] {
			continue
		}
		if decls["display"] == "contents" {
			items = r.appendFlexItems(items, c)
			continue
		}
		items = append(items, flexItem{node: c, decls: decls, grow: parseFlexGrow(decls), shrink: parseFlexShrink(decls), order: parseOrder(decls)})
	}
	flushRun()
	return items
}

// anonymousFlexItem builds the anonymous flex item wrapping one run of loose
// text nodes inside a flex container, reporting false for a run that isn't
// rendered at all. parentDecls is the container's own resolved declarations,
// which the item inherits from.
//
// A whitespace-only run is dropped, per §4's own exception: "if the entire
// sequence of child text runs contains only white space it is instead not
// rendered". That is what keeps the newlines and indentation in
//
//	<div style="display:flex">
//	  <span>a</span>
//	  <span>b</span>
//	</div>
//
// from becoming three empty items with gaps between them, which is the reason
// the exception exists in the spec. It doesn't apply in a white-space mode
// that preserves spacing, where that whitespace is content the author asked
// for.
//
// The run is carried on a synthetic element node rather than rendered
// specially, so an anonymous item reaches every existing sizing path (flex
// base size, min-content measurement, stretching) as an ordinary item. Three
// properties of that node matter:
//
//   - Its computed style is seeded into the cascade's own per-node cache
//     rather than resolved through it. An anonymous box is unselectable in
//     CSS: it has no element for a selector to match, so its whole style is
//     what it inherits, which cssengine.InheritedDecls supplies. Seeding is
//     what makes that hold everywhere rather than only here, since a flex item
//     is re-resolved from its node deeper in the render (inline.go reads
//     white-space and text-transform off it). Left to the cascade, a `div > *`
//     rule would style a box the author has no way to name. It has no tag
//     name for the same reason, so nothing dispatches on it either.
//   - Its children are *copies* of the run's text nodes, not the nodes
//     themselves. Splicing the originals in would leave their NextSibling
//     chain pointing at the container's later children, and rendering the item
//     would walk straight out of the run and into its siblings.
//   - It is memoized per run in Engine.anonFlexNodes, so every pass over the
//     same container gets the same pointer. Measurement re-collects a
//     container's items repeatedly, and a fresh node each time would defeat
//     minContentCache and grow directCache without bound.
func (r *Engine) anonymousFlexItem(container *html.Node, run []*html.Node, parentDecls map[string]string) (flexItem, bool) {
	ws := parentDecls["white-space"]
	preserving := ws == "pre" || ws == "pre-wrap" || ws == "break-spaces"
	if !preserving {
		blank := true
		for _, t := range run {
			if strings.TrimFunc(t.Data, unicode.IsSpace) != "" {
				blank = false
				break
			}
		}
		if blank {
			return flexItem{}, false
		}
	}
	inherited := cssengine.InheritedDecls(parentDecls)
	node, ok := r.anonFlexNodes[run[0]]
	if !ok {
		node = &html.Node{Type: html.ElementNode, Parent: container}
		var prev *html.Node
		for i, t := range run {
			text := t.Data
			if !preserving {
				// The item is a block container, so its own leading and
				// trailing whitespace collapses away at its edges, exactly as
				// it would for text directly inside a <div>. Doing it here
				// rather than leaving it to the wrapper keeps the item's flex
				// base size measured from the text that will actually paint:
				// " a " is one column wider than "a".
				if i == 0 {
					text = strings.TrimLeftFunc(text, unicode.IsSpace)
				}
				if i == len(run)-1 {
					text = strings.TrimRightFunc(text, unicode.IsSpace)
				}
			}
			c := &html.Node{Type: html.TextNode, Data: text, Parent: node}
			if prev == nil {
				node.FirstChild = c
			} else {
				prev.NextSibling = c
				c.PrevSibling = prev
			}
			prev = c
		}
		node.LastChild = prev
		if r.anonFlexNodes != nil {
			r.anonFlexNodes[run[0]] = node
		}
		if r.directCache != nil {
			r.directCache[node] = maps.Clone(inherited)
		}
	}
	// No grow, shrink, order, or basis lookups: those are properties of the
	// item's own box, and an anonymous box has no declarations of its own. The
	// longhand defaults (grow 0, shrink 1, order 0) are what CSS gives it.
	return flexItem{node: node, decls: inherited, shrink: 1}, true
}

// renderFlexItemBox renders one row-direction flex item's own box at the
// main-axis width flex layout resolved for it. It dispatches to a nested flex
// layout when the item is itself a flex container, since display:flex and
// inline-flex nest normally, and to the ordinary block box model otherwise.
// Flex items are blockified regardless of their own declared display, matching
// real CSS.
//
// width is imposed as a synthetic declaration, not merely offered as the
// available width. An item's used main size is whatever flex layout resolved,
// its flex base size adjusted by flex-grow and flex-shrink, which per spec
// overrides its own width property outright. Passing it as available width
// only would let renderBlockContentBox re-resolve the item's declared width
// and win: a grown item would paint at its declared width with the growth
// left as trailing blanks, and a shrunk one would paint at full width and
// overflow the container, dragging every later item's recorded Rect with it.
// Re-resolving min-width/max-width inside the item's own render is harmless.
// resolveMainBasis already clamped the base size through clampFlexWidth, and
// flex-grow and flex-shrink respect the same two as their ceiling and floor,
// so the width handed here is already inside that range.
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
// main-axis size overrides. A widthOverride or heightOverride greater than 0
// injects a synthetic "width" or "height" declaration before rendering, taking
// priority over any the item already declares, matching flex-basis's real-CSS
// precedence over the width/height properties. The existing block box model's
// own sizing then does the work for a non-flex item, via block.go's width
// resolution and fixed-height pad/clip logic, and
// resolveFlexContainerHeight/resolveWidthConstraints do it for an item that is
// itself a flex container. 0 means "no override".
//
// Both overrides are *outer* box sizes, the item's whole painted extent with
// border characters and padding included. That's the size flex layout resolved
// and reserved space for, and the convention a declared width already follows
// everywhere else in this engine (see COMPATIBILITY.md). The width property
// means that already, but the height property is content-box here, so the
// vertical override subtracts the item's own top/bottom border and padding
// rows via blockVerticalChrome before injecting it. Without that subtraction a
// bordered item overshot its allotted rows by exactly its chrome and had the
// overflow silently clipped off the bottom of the line, so the stretched box's
// bottom border vanished.
//
// decls is always cloned, even with no override at all. layoutFlexColumn can
// render the same item twice, once for a basis measurement and again if
// flex-grow or flex-shrink changes its height, and block.go's own
// margin-top/margin-bottom collapse-through bookkeeping mutates its decls
// argument in place. Without a clone here, that first render could leave a
// stale mutation in it.decls for the second render, or any later reader, to
// pick up. Row direction's layoutFlexLine renders every item exactly once,
// so this is defensive there, not required by the row path itself.
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
			// for a bordered item is its own right border glyph. block.go's
			// clipping path knows the box model and truncates the *content* to
			// its inner width, leaving border and padding intact, and honors
			// text-overflow while it's there.
			//
			// Only reached when the author left overflow-x at visible. hidden
			// and clip mean this is already what they asked for, and scroll and
			// auto make the item its own horizontal viewport, whose windowing
			// must not be downgraded to a truncation.
			//
			// Clipping at all is a deliberate deviation from CSS, and a narrow
			// one. flexMainAxisFloor's automatic minimum size is the primary
			// defense, so this only fires where CSS itself would overflow: a
			// definite width narrower than min-content, a `min-width: 0`
			// opt-out, or `flex-shrink: 0` against a small declared width. This
			// engine's convention elsewhere is *not* to clip without an
			// explicit overflow. A plain bordered block whose unbreakable
			// content exceeds its width paints straight past its own border,
			// and a flex line whose items are all at their floors is likewise
			// left to overflow its container. What makes an item exceeding its
			// *allotment* different is that flex already positioned every later
			// item on the line from that number. Lines are assembled by
			// concatenation (see layoutFlexLine), so an item painting wider
			// than its allotment doesn't overlay its siblings the way a
			// browser's visible overflow would. It displaces them into columns
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
			// This height is synthetic. It exists only to drive
			// flex-basis/flex-grow/flex-shrink sizing, not as a real declared
			// box height, so it must not activate this engine's
			// scrollbar-gutter reservation or live-scroll-offset tracking as
			// an unrelated side effect of flex layout math. An author's own
			// explicit "hidden" or "clip" clipping intent is left alone:
			// that's a legitimate combination with an explicit height,
			// synthetic or not.
			decls["overflow-y"] = "visible"
		}
	}
	if b, pos, ok := r.renderFlexItemLeaf(it.node, decls, width); ok {
		if heightOverride > 0 && len(b.lines) < heightOverride {
			// Safety net only, for a leaf that sizes itself. For a nested flex
			// container the injected height above reaches its own layout through
			// resolveFlexContainerHeight, so layoutFlexColumn and layoutFlexRow
			// normally fill to exactly this many rows themselves. That is the
			// point of injecting it rather than padding from outside. Padding
			// here happens *after* renderFlexContentBox has drawn the box's
			// bottom rule, so the extra rows land below the border instead of
			// inside it, and the nested container's own children never see the
			// height their parent was allotted, leaving a flex-grow child of a
			// grown nested column container at its content height. This branch
			// is left in place for the cases layout can't fill, such as a nested
			// container with no items at all, and is the only vertical sizing a
			// <table> or list item gets, neither renderer having a notion of a
			// target height. A taller-than-heightOverride result is left
			// untouched, matching the graceful-overflow convention used
			// throughout this file.
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

// renderFlexItemLeaf renders a flex item whose box is not produced by the
// ordinary block box model, reporting false for every item that is. That is
// the common case, left to renderBlockContentBox.
//
// Flex items are blockified, but blockifying an element only changes its outer
// display type. A <table> item is still a table and a <ul> item is still a
// list, exactly as they are in normal flow. renderBlockContentBox has no notion
// of either. It walks an element's children as ordinary content, which turns a
// table into its cells' text run together and a list into its items with the
// markers missing. The same dispatch already exists at both of the other two
// places an element can turn up as somebody's box, root level (render.go) and
// nested inline (inline.go), and this is the third.
//
// <input>, <select>, <progress>, and <meter> are *not* here. Their synthesized
// content is handled inside renderBlockContentBox itself (see
// synthesizedControlText), so they keep the block box model's own border,
// padding, and width handling, which they need and a table or list renderer
// supplies for itself.
//
// width is the item's resolved main-axis size in row direction and its
// cross-axis size in column direction. Both renderers take it as an available
// width, which is the most either can use, since neither has a way to be told
// "paint exactly this wide". The caller pads via alignLinesBox or lets the item
// overflow, as it does for a block item that comes out a different size than
// allotted.
func (r *Engine) renderFlexItemLeaf(n *html.Node, decls map[string]string, width int) (box, map[*html.Node]Rect, bool) {
	switch {
	case isFlexDisplay(decls["display"]):
		b, pos := r.renderFlexContentBox(n, decls, width)
		return b, pos, true
	case n.Data == "table" && isTableLayoutDisplay(decls["display"]):
		content, pos := r.renderTable(n, width)
		return newBox(strings.TrimSuffix(content, "\n")), pos, true
	case n.Data == "ul" || n.Data == "ol" || n.Data == "menu":
		content, pos := r.renderList(n, n.Data == "ol", width)
		return newBox(strings.TrimSuffix(content, "\n")), pos, true
	}
	return box{}, nil, false
}

// blockVerticalChrome is how many rows of an ordinary block box's height are
// its own chrome rather than its content: a drawn top/bottom border rule plus
// padding-top/padding-bottom. Flex layout resolves outer sizes, while the
// height property is content-box in this engine, so this is the difference
// between the two. See renderFlexItemBoxSized.
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

// blockHorizontalChrome is how many columns of a box's total painted width are
// its own chrome rather than its content: horizontal margin, drawn left and
// right border characters, and left and right padding. It is
// blockVerticalChrome's
// counterpart, and the difference between a flex container's inner content
// width and the availWidth renderFlexContentBox has to be handed to paint that
// content at exactly that width. An `auto` margin contributes nothing, matching
// resolveMarginSide's own reading of it.
//
// Border characters are measured in terminal columns, not runes, since a
// border glyph can legitimately be double-width.
func blockHorizontalChrome(decls map[string]string, availWidth int) int {
	bl, br, _, _, _, _, _, _ := resolveBoxBorders(decls)
	ml, _ := resolveMarginSide(decls["margin-left"], availWidth)
	mr, _ := resolveMarginSide(decls["margin-right"], availWidth)
	return ml + mr + textcell.Width(bl.char) + textcell.Width(br.char) +
		parsePaddingLen(decls["padding-left"]) + parsePaddingLen(decls["padding-right"])
}

// parseColumnFlexBasis resolves a column-direction flex item's flex-basis to
// an absolute line count, reporting false when there is no declared base size
// to use: flex-basis unset, `auto`, `content`, or a percentage with no definite
// container height to resolve against. Real CSS treats a percentage flex-basis
// as auto when the container's own main size is indefinite, which this engine
// only ever considers "definite" when an explicit CSS height is set on the
// container (see resolveFlexContainerHeight). containerHeight is
// indefiniteMainSize in that case, which is what resolveFlexCSSSize reads to
// drop the percentage.
//
// A declared flex-basis of 0 is a definite base size of nothing, not an absent
// one, and is returned as 0 with ok true (see parseFlexSizeVal). The rule that
// no box can render in zero lines is imposed by the caller's own clamp against
// the item's minimum, not here. The base size is an arithmetic input to
// flex-grow, and flooring it at one line is what leaves `flex: 1` items a row
// off their true shares.
func parseColumnFlexBasis(decls map[string]string, containerHeight int) (int, bool) {
	v := decls["flex-basis"]
	if v == "" || v == "auto" {
		return 0, false
	}
	return resolveFlexCSSSize(v, containerHeight)
}

// clampFlexMainHeight clamps a column-direction main-axis size to the item's
// own min-height/max-height, producing the spec's hypothetical main size on the
// vertical axis. It is clampFlexWidth's counterpart for the one axis where the
// two sizes aren't measured in the same box. Flex layout resolves *outer*
// sizes, border rules and padding rows included, while min-height/max-height
// are content-box in this engine, so each bound gets the item's own vertical
// chrome added back before it's compared. Maximum first and minimum second, so
// a minimum larger than a maximum wins, CSS's own min/max resolution order.
func clampFlexMainHeight(decls map[string]string, size, containerHeight int) int {
	chrome := blockVerticalChrome(decls)
	if v, ok := resolveFlexCSSSize(decls["max-height"], containerHeight); ok && size > v+chrome {
		size = v + chrome
	}
	if v, ok := resolveFlexCSSSize(decls["min-height"], containerHeight); ok && size < v+chrome {
		size = v + chrome
	}
	return max(1, size)
}

// measureNaturalWidth renders it at a generous width cap only to measure its
// intrinsic content width, for flex-basis:auto and width:auto items, then
// discards the render. r.quoteDepth is saved and restored around the call so
// this trial render leaves no state behind for the real render that follows.
// Safe to call twice per item: counters are pre-resolved once per document
// into r.counterMap (see counter.go) before any rendering starts, so this
// isn't a double-increment risk, and r.liveScrollOffsets and
// r.liveContentOffsets are plain map[node]value assignments the item's real
// render overwrites unconditionally afterward.
func (r *Engine) measureNaturalWidth(it flexItem, cap int) int {
	saved := r.quoteDepth
	savedShrink := r.shrinkToFit
	// Shrink-to-fit for the duration of the trial render. Without it a flex
	// item that paints a rectangle, because of a border, text-align, or an
	// explicit height, measures as however wide cap is rather than as its own
	// content, so every such item on a line reports an identical basis and
	// flex-shrink's content-proportional split flattens to an even one. See
	// Engine.shrinkToFit.
	r.shrinkToFit = true
	// Measurement goes through renderFlexItemLeaf and renderBlockContentBox,
	// the same two entry points the real render uses, so an item measures as
	// the box it will actually paint. For a nested flex container that means
	// its own whole box, border characters and padding included, rather than
	// just the items inside it. For a <table> or list item it means the table
	// or list renderer's own extent rather than its content run together as
	// text, which is a different number entirely.
	//
	// The flex case used to call layoutFlex, the unpadded layout pass
	// underneath, because renderFlexContentBox fills its result to the full
	// width it's given, a block-level flex container's normal CSS behavior, and
	// so measured every width:auto container as "however wide cap is". That
	// reason is gone. renderFlexContentBox now narrows to its content's own
	// width under a shrink-to-fit measurement render, which is the state
	// measureNaturalWidth puts the engine in above. See Engine.shrinkToFit.
	b, _, ok := r.renderFlexItemLeaf(it.node, it.decls, cap)
	if !ok {
		b, _ = r.renderBlockContentBox(it.node, it.decls, cap)
	}
	r.quoteDepth = saved
	r.shrinkToFit = savedShrink
	return b.width
}

// resolveMainBasis resolves a row-direction flex item's two main-axis
// (horizontal) starting sizes, which the spec keeps deliberately apart:
//
//   - the **flex base size**: flex-basis, if set and not auto, takes priority,
//     then width, then the item's own measured natural content width. Nothing
//     clamps it. It is the size flex-grow and flex-shrink distribute *from*.
//   - the **hypothetical main size**: that base clamped by the item's used
//     minimum and maximum main size (§9.2), where min-width's used value on a
//     flex item is `auto`, the automatic minimum size (flexMainAxisFloor),
//     rather than zero. This is what flex-wrap breaks lines on and what decides
//     whether the line grows or shrinks.
//
// Collapsing the two, by growing from the hypothetical, is what makes `flex: 1`
// items come out sized by their content instead of by their flex factors. See
// growFlexLine, which applies the hypothetical as a violation floor instead.
//
// A measured base needs no automatic-minimum probe. measureNaturalWidth is a
// shrink-to-fit (fit-content) measurement, so it is already at or above
// min-content and the floor could never bite. That matters for more than
// tidiness: flexMainAxisFloor's probe is a second trial render per item.
func (r *Engine) resolveMainBasis(it flexItem, innerW int) (base, hypo int) {
	content := flexBasisIsContent(it.decls)
	declared := false
	if v := it.decls["flex-basis"]; v != "" && v != "auto" && !content {
		base, declared = resolveFlexCSSSize(v, innerW)
	}
	if !declared && !content {
		if v := it.decls["width"]; v != "" {
			base, declared = resolveFlexCSSSize(v, innerW)
		}
	}
	if !declared {
		// `content` and the intrinsic keywords measure the item's content with
		// its own width property out of the way. That is the whole difference
		// between them and `auto`, which consults width first, above, and would
		// otherwise have the measurement render honor it as well.
		probe := it
		if content {
			probe = itemIgnoringSizeDecl(it, "width")
		}
		if kind, ok := flexBasisIntrinsic(it.decls); ok {
			base = r.measureFlexIntrinsicWidth(probe, kind, innerW)
		} else {
			base = r.measureNaturalWidth(probe, innerW)
		}
		return base, clampFlexWidth(it.decls, base, innerW)
	}
	// Maximum first and minimum second, so a minimum larger than a maximum
	// wins. That is CSS's own min/max resolution order, and the same order
	// clampFlexWidth uses for the measured path above. flexMainAxisFloor is
	// already the *used* minimum: an explicitly declared min-width if there is
	// one, else the automatic minimum size, itself already clamped by any
	// definite maximum, per CSS Sizing 3 §5.1.
	hypo = base
	if v, ok := resolveFlexCSSSize(it.decls["max-width"], innerW); ok && hypo > v {
		hypo = v
	}
	if floor := r.flexMainAxisFloor(it, innerW); hypo < floor {
		hypo = floor
	}
	return base, max(1, hypo)
}

// flexBasisIsContent reports whether an item declares a flex-basis that sizes
// it from its content while ignoring its own width/height property: `content`
// itself, or one of the intrinsic sizing keywords, which are content-based in
// the same sense. CSS Sizing 3 §5.1 defines an intrinsic size as computed from
// content, ignoring the box's specified size. `auto` defers to the
// width/height property first and only falls back to content, so it is the one
// content-ish value that isn't here. The two differ exactly when the item
// declares a main size.
func flexBasisIsContent(decls map[string]string) bool {
	switch strings.TrimSpace(decls["flex-basis"]) {
	case "content", "min-content", "max-content", "fit-content":
		return true
	}
	return false
}

// flexBasisIntrinsic reports which intrinsic sizing keyword an item's
// flex-basis names, if any. They differ only in which measurement of the item's
// content they ask for; see measureFlexIntrinsicWidth.
func flexBasisIntrinsic(decls map[string]string) (string, bool) {
	switch v := strings.TrimSpace(decls["flex-basis"]); v {
	case "min-content", "max-content", "fit-content":
		return v, true
	}
	return "", false
}

// measureFlexIntrinsicWidth measures a row-direction item's content under one
// of the intrinsic sizing keywords, all three of which ignore the item's own
// width. The caller has already stripped it (see flexBasisIsContent).
//
//   - `min-content` is the widest unbreakable run plus the item's own chrome,
//     which is what flexMainAxisFloor's own floor measurement computes.
//   - `max-content` is the width the item would take if never forced to wrap,
//     measured at an effectively unbounded budget. measuringNaturalWidth is set
//     for the duration so a nested width:100% box measures its own shrink-to-fit
//     width instead of stretching to fill that budget, the same guard
//     measureCellNaturalWidth uses for the identical reason.
//   - `fit-content` is min(max-content, max(min-content, available)), which for
//     this engine is the ordinary natural-width measurement at the container's
//     own content width. measureNaturalWidth wraps to the budget it is given
//     and reports what the content came to.
func (r *Engine) measureFlexIntrinsicWidth(it flexItem, kind string, innerW int) int {
	switch kind {
	case "min-content":
		return r.measureMinContentWidth(it)
	case "max-content":
		saved := r.measuringNaturalWidth
		r.measuringNaturalWidth = true
		w := r.measureNaturalWidth(it, measureBlockWidthCap)
		r.measuringNaturalWidth = saved
		return w
	default:
		return r.measureNaturalWidth(it, innerW)
	}
}

// itemIgnoringSizeDecl returns it with one size declaration removed, over a
// cloned decls map so the item's own shared resolved declarations are left
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
// engine can't render a zero-width or zero-height box, so a declared zero comes
// back as one cell rather than none. Only the *cross* axis reads sizes through
// here, in resolveCrossWidth. The main-axis basis path goes through
// resolveFlexCSSSize, which deliberately does not floor, so flex-basis:0 stays
// a true zero as an arithmetic input to flex-grow. See resolveMainBasis.
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
// this engine a zero size and an absent one mean the same thing. Not here:
// flex-basis distinguishes "0", a definite base size of nothing, which is what
// makes flex-grow split the container's whole main size by weight, from unset
// or auto, which fall back to width and then to the item's natural content
// size. Reading a declared 0 as "not a length" sent resolveMainBasis and
// parseColumnFlexBasis down the auto path, so `flex: 1`, whose shorthand
// expansion is `1 1 0`, laid out as `1 1 auto`: two items with different
// content got content-proportional widths rather than equal ones.
//
// The unit is trimmed off before the zero check rather than matched against
// this engine's own vocabulary of "ch" and "%". This is the one place unit
// syntax this engine otherwise ignores has to be recognized, because a zero
// length is dimensionless in CSS. `0`, `0px`, `0em`, `0rem` and `0%` are all
// the same computed value, and the spec lets the unit be omitted on a zero
// <length> for that reason. The policy that leaves `width: 10px` inert is about
// there being no defensible px-to-column conversion, which says nothing about a
// value that needs no converting. Reading `0px` as "not a length" sent the
// common `flex: 1 1 0px` down the auto path and laid it out as `flex: 1 1
// auto`. Only the zero is exempt: a non-zero value with an unrecognized unit
// still fails here, as everywhere else. This also puts the parser back in
// step with cssengine's isCSSFlexBasisToken, which already accepted `0px` as
// the shorthand's <'flex-basis'> component and so expanded `flex: 1 1 0px`
// into a longhand this function then dropped.
func parseFlexSizeVal(s string) (abs int, pct float64, ok bool) {
	if abs, pct, ok := parseSizeVal(s); ok {
		return abs, pct, true
	}
	t := strings.TrimSpace(s)
	t = strings.TrimSuffix(t, "%")
	t = strings.TrimRightFunc(t, unicode.IsLetter)
	if f, err := strconv.ParseFloat(t, 64); err == nil && f == 0 {
		return 0, 0, true
	}
	return 0, 0, false
}

// resolveCrossWidth resolves a column-direction flex item's cross-axis
// (horizontal) width when align-items isn't stretch: width if set, else the
// item's own measured natural content width, capped to innerW and then clamped
// by the item's own min-width/max-width. A min-width larger than the container
// still wins and overflows, matching CSS, and matching what the item's own
// render will do to itself regardless.
func (r *Engine) resolveCrossWidth(it flexItem, innerW int) int {
	w := 0
	if v := it.decls["width"]; v != "" {
		w = resolveFlexAxisSize(v, innerW)
	}
	if w == 0 {
		w = r.measureNaturalWidth(it, innerW)
	}
	return clampFlexWidth(it.decls, min(innerW, w), innerW)
}

// stretchCrossWidth resolves a *stretched* column-direction item's cross-axis
// (horizontal) size when stretching isn't what decides it, reporting false when
// it is. That is the ordinary case, where the item takes the container's whole
// content width and nothing needs recording.
//
// `stretch` applies only to an auto cross size. §8.3: "if the cross size
// property of the flex item computes to auto... its used value is the length
// necessary to make the cross size of the item's margin box as close to the
// same size as the line as possible, while still respecting the constraints
// imposed by min-height/min-width/max-height/max-width". So:
//
//   - a definite `width` opts the item out of stretching entirely and is its
//     used cross size;
//   - otherwise the item does stretch to the container's content width, but
//     still clamped by its own min-width/max-width.
//
// Without this, every stretch-resolved item was recorded at the container's
// full inner width, including one that isn't stretched at all. A `width: 5`
// item in a 20-column container reported a 20-column Rect, so a click fifteen
// columns to its right hit-tested to the item rather than to the container. The
// item's own render always applied its width regardless; only flex layout's
// bookkeeping disagreed with what was painted.
func stretchCrossWidth(it flexItem, innerW int) (int, bool) {
	if v := it.decls["width"]; v != "" {
		if w := resolveFlexAxisSize(v, innerW); w > 0 {
			// Capped at the container like resolveCrossWidth's own definite
			// width, then clamped by the item's minimum, which can push it back
			// out past the container. That is CSS's "minimum wins", and what
			// the item's own render does to itself anyway.
			return clampFlexWidth(it.decls, min(innerW, w), innerW), true
		}
	}
	w := clampFlexWidth(it.decls, innerW, innerW)
	return w, w != innerW
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
// distribution into a leading pad, before the first item, and n-1 extra
// per-gap amounts, added on top of the base column-gap between items. A
// leftover of 0 or less, meaning no free space or a line that already
// overflows, always yields no extra spacing. That is the usual case whenever
// any item can grow, since flex-grow consumes the leftover first.
//
// Negative leftover deliberately packs flex-start rather than distributing a
// negative pad. That is CSS's own *safe* alignment: "if the size of the
// alignment subject overflows the alignment container, the alignment subject is
// aligned as if the alignment mode were start" (Box Alignment §4.2). Browsers
// default to unsafe alignment on this axis, so `center` there overflows equally
// off both edges and `flex-end` off the start edge. A leading pad is the only
// thing this function can express, and a negative one has nowhere to put the
// columns it would move off the container's start edge: they'd land in the
// container's own border or padding, or in a sibling. Losing content outright is
// what safe alignment exists to prevent, so safe is what this does. It also
// keeps the spec's own negative-free-space fallbacks correct for free, per
// §8.2, where space-between behaves as flex-start and space-around and
// space-evenly as center. Each of those would need the same unrepresentable
// negative shift.
//
// distributeAlignContent shares this. align-content distributes leftover
// cross-axis space across a wrapped container's lines the way justify-content
// distributes leftover main-axis space across one line's items, so they are one
// function rather than two that have to be kept in step. An unrecognized value,
// including unset and align-content's "stretch" (see distributeAlignContent),
// falls through to no distribution, which is flex-start's behavior.
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
		// Each item gets an equal share centered on it, so the two edge pads are
		// half a share each and the pad between two items is two halves. That is
		// n+1 slots weighted [0.5, 1, 1, ..., 1, 0.5], handed to the same
		// largest-remainder split flex-grow's own shares use: each slot takes the
		// floor of its exact size, then the units flooring left over go one
		// apiece to the slots with the largest fractional parts.
		//
		// Splitting into 2n half-shares and recombining them pairwise, which is
		// what this used to do, put the remainder wherever the *halves* fell
		// rather than where the real slots are, and splitEvenly hands its own
		// remainder to the earliest parts. Both surpluses therefore piled up at
		// the start of the line. Three items with four free columns came out as
		// a 1-column leading pad, a 2-column gap, a 1-column gap, and nothing at
		// all on the trailing edge, where space-evenly gives every slot 1 on the
		// same input. Weighting the real slots directly keeps the two edges equal
		// whenever the arithmetic allows it.
		weights := make([]float64, n+1)
		slots := make([]int, n+1)
		for i := range weights {
			weights[i] = 1
			slots[i] = i
		}
		weights[0], weights[n] = 0.5, 0.5
		shares := proportionalShares(weights, slots, leftover)
		leadPad = shares[0]
		for i := range gaps {
			gaps[i] = shares[i+1]
		}
	case "space-evenly":
		// n+1 equal pads, one before the first item, one between each pair, and
		// one after the last.
		units := splitEvenly(leftover, n+1)
		leadPad = units[0]
		for i := range gaps {
			gaps[i] = units[i+1]
		}
	}
	return leadPad, gaps
}

// distributeAlignContent resolves align-content's leftover cross-axis space
// across a wrapped row container's lines into a leading blank-row count, before
// the first line, and n-1 extra row-gap amounts, added on top of the base
// row-gap between lines. Same distribution as distributeJustify, applied to
// whole lines on the cross (vertical) axis rather than to items on the main
// (horizontal) axis. See there for the value handling.
//
// "stretch", align-content's initial value, is not one of the values this
// distributes: it grows the lines rather than packing them, and is resolved
// before this is reached (see layoutFlexRow). It falls through to
// flex-start's no-distribution behavior here, which is what places any
// remainder stretch couldn't grow into.
func distributeAlignContent(align string, leftover, n int) (leadRows int, gaps []int) {
	return distributeJustify(align, leftover, n)
}

// crossOffset resolves align-items' vertical offset in row direction for one
// item within the row's tallest item height. "stretch", the default, and
// "flex-start" both place content flush at the top. This engine has no way
// to stretch text content itself, so "stretch" is approximated by padding
// the item's box with blank lines up to height, the same visual result as a
// real stretched box with a blank interior.
//
// An item taller than the line gets no offset whatever the alignment, by the
// same safe-alignment reasoning distributeJustify's negative-leftover case
// rests on: a negative offset would move rows above the line's own first row,
// where another line or the container's own border already is.
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

// crossAxisOffset resolves how far down its line a row-direction flex item
// starts: its own margin-top plus whatever align-items or align-self asks for,
// except where an auto cross-axis margin takes the decision over.
//
// An auto margin on the cross axis absorbs that axis's free space, and §8.1
// gives it priority: "if a flex item has auto cross-axis margins, they absorb
// the free space in the cross axis, and align-self is ignored". Two auto
// margins split the space and center the item, which is the property's real
// use. One auto margin takes all of it and pushes the item to the other end,
// so margin-top: auto sits an item at the bottom of its line, the cross-axis
// twin of margin-left: auto pushing an item to the end of the row.
//
// This is the same claim on the same space justify-content's own auto-margin
// override handles on the main axis (see layoutFlexLine), and it is the same
// safe alignment as everywhere else here: with no free space to absorb, an
// auto margin resolves to zero rather than to a negative offset.
func crossAxisOffset(m flexMargins, align string, height, outerHeight int) int {
	if !m.topAuto && !m.bottomAuto {
		return m.top + crossOffset(align, height, outerHeight)
	}
	free := max(0, height-outerHeight)
	switch {
	case m.topAuto && m.bottomAuto:
		return free / 2
	case m.topAuto:
		return free
	default:
		// margin-bottom: auto alone, which pushes the item to cross-start.
		return 0
	}
}

// padBoxVertical pads b to exactly height lines, inserting topOffset blank
// lines before its content, for align-items:center and flex-end, and filling
// any remaining rows after it with blank lines of b's own width.
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
// using each item's already-resolved pre-grow main-axis width. That is real
// CSS's "hypothetical main size" used for line-breaking, before any flex-grow
// distribution, which happens per line once a line's membership is fixed.
// An item that alone exceeds innerW still gets its own line rather than
// being dropped or splitting further, since this engine has no notion of
// splitting a single item across lines, matching real CSS. nowrap, the
// default, always returns a single line containing every item, matching
// row-direction flex layout's pre-flex-wrap behavior exactly.
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

// measuringIntrinsicWidth reports whether the current render is a discardable
// measurement whose only product is a width: measureNaturalWidth's own
// shrink-to-fit probe, or one of the wider measuring passes
// (measureCellNaturalWidth, measureFlexIntrinsicWidth's max-content branch,
// the nested-<table> hint) that render at a deliberately huge budget.
//
// Row-direction flex layout consults it to suppress free-space distribution.
// A measurement render asks what the container's content comes to, and
// flex-grow, justify-content, and auto margins all exist to spend space the
// container was *given*, so under a measurement they answer the wrong
// question: they expand the line to fill whatever budget the probe offered,
// and renderFlexContentBox's shrink-to-fit narrowing then finds nothing to
// narrow. See layoutFlexLine.
//
// This is CSS's own distinction between a flex container's intrinsic main size
// and its used one, though not yet the whole of it. Flexbox §9.9 sums each
// item's *max-content contribution* through a chosen-flex-fraction step, while
// suppressing distribution here sums their hypothetical main sizes. The two
// agree for every item whose flex base size is content-derived, which is
// flex-basis:auto and width-sized items. They differ for flex-basis:0 items
// (`flex: 1`), where a hypothetical main size is the automatic minimum size,
// meaning min-content, and §9.9 would use the item's max-content contribution.
// Such a container measures narrower here than in a browser. See
// COMPATIBILITY.md's "A flex: 1 container measures narrower..." entry.
func (r *Engine) measuringIntrinsicWidth() bool {
	return r.shrinkToFit || r.measuringNaturalWidth
}

// flexMargins is one row-direction flex item's margins as flex layout needs
// them: its resolved cross-axis (vertical) lengths, plus which of its four
// sides were declared auto.
//
// An auto margin is a claim on leftover space rather than a length, so it
// contributes nothing to the item's own size on either axis and is resolved
// only once that axis's free space is known: on the main axis in
// layoutFlexLine, after flex-grow and flex-shrink, and on the cross axis in
// crossAxisOffset, against the line's own height. That is why the two auto
// flags have no length beside them and top/bottom read 0 when their own side
// is auto.
type flexMargins struct {
	top, bottom         int
	topAuto, bottomAuto bool
	leftAuto, rightAuto bool
}

// layoutFlexLine lays out one flex line's worth of items, a subset of
// items/widths/margins named by group's indices into those slices.
// It resolves flex-grow and flex-shrink within just this line, then any
// margin-left/margin-right:auto absorption, then justify-content and
// align-items exactly as row-direction flex layout always has for a single
// line. widths is mutated in place for group's own indices, which is safe
// across repeated calls for other lines since each index belongs to exactly
// one line.
//
// minHeight forces the line's cross size, normally its tallest item's outer
// height, up to at least that many rows. A single-line container's line takes
// the container's own inner cross size, so align-items and align-self resolve
// against the container's declared height rather than against the content.
// 0 means "whatever the items come to", the multi-line case, where each line
// sizes to its own content and align-content places the lines instead.
func (r *Engine) layoutFlexLine(items []flexItem, bases, widths []int, margins []flexMargins, group []int, innerW, gap, minHeight int, justify string, decls map[string]string) (box, int, map[*html.Node]Rect) {
	totalGap := gap * (len(group) - 1)
	availForItems := max(0, innerW-totalGap)

	// The grow-or-shrink decision is made on the sum of the line's
	// *hypothetical* main sizes (§9.7 step 1), which is what widths holds on
	// the way in. The distribution itself then works from each item's flex base
	// size, held in bases. See growFlexLine and resolveMainBasis.
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
	case leftover > 0 && r.measuringIntrinsicWidth():
		// A discardable measurement render (see measuringIntrinsicWidth). The
		// probe budget isn't space the container has to spend, it's the bound
		// the measurement is taken under, so there is no free space here to
		// grow into, justify, or hand to an auto margin. Dropping it to zero
		// leaves every item at its hypothetical main size and the line at the
		// sum of those, which is the width being measured, and lets
		// renderFlexContentBox's shrink-to-fit narrowing do its job.
		//
		// Without this the line filled the probe budget exactly, so that
		// narrowing never fired and a nested flex container measured as wide
		// as whatever cap it was probed at: as a flex item it then took its
		// parent's whole line, and in a table cell its whole column.
		leftover = 0
	case leftover > 0:
		// Grown from each item's flex base size and floored at its hypothetical
		// main size, the automatic minimum size applied as a §9.7 step-4c
		// violation rather than as a head start, capped per item at its own
		// max-width, the mirror of column direction's max-height ceiling. Any
		// leftover the ceilings prevented from being absorbed flows through to
		// justify-content below rather than being silently dropped.
		hypos := append([]int(nil), widths...)
		leftover = growFlexLine(widths, bases, hypos, ceilings, grows, group, availForItems)
	case leftover < 0:
		// Overflow: shrink items proportionally to flex-shrink * flex base
		// width, the real spec's "scaled flex shrink factor", floored at each
		// item's automatic minimum size. The floors are resolved here rather
		// than alongside grows and ceilings above because flexMainAxisFloor
		// measures min-content with a trial render, and a line that isn't
		// overflowing has no use for the answer.
		floors := make([]int, len(items))
		for _, i := range group {
			floors[i] = r.flexMainAxisFloor(items[i], innerW)
		}
		leftover = -distributeFlexShrink(widths, shrinks, bases, floors, group, -leftover)
	}

	// margin-left/margin-right:auto absorbs whatever main-axis leftover
	// space remains after flex-grow and flex-shrink have had their turn,
	// matching real CSS's ordering of flexible-length resolution first and
	// auto-margin alignment second. It splits that space evenly across every
	// auto margin present anywhere in the line and, when any exist, overrides
	// justify-content entirely for this line, same as real CSS.
	extraLeft := make([]int, len(group))
	extraRight := make([]int, len(group))
	marginsOverrideJustify := false
	if autoCount := 0; leftover > 0 {
		for _, i := range group {
			if margins[i].leftAuto {
				autoCount++
			}
			if margins[i].rightAuto {
				autoCount++
			}
		}
		if autoCount > 0 {
			share := leftover / autoCount
			rem := leftover % autoCount
			slot := 0
			for gi, i := range group {
				if margins[i].leftAuto {
					extraLeft[gi] = share
					if slot < rem {
						extraLeft[gi]++
					}
					slot++
				}
				if margins[i].rightAuto {
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
		// An item with no CSS width of its own doesn't auto-fill the available
		// width the way a plain block box would, since renderBlockContentBox
		// only pads to its available width when an explicit width, text-align,
		// or a border requires it. So this main-axis pass's resolved width must
		// be enforced explicitly here, or a grown item's extra space would
		// collapse back to its own natural width.
		b = alignLinesBox(b, it.decls["text-align"], w)
		itemBoxes[i] = b
		itemPositions[i] = pos
		// The item's margin-top/margin-bottom widen its own cross-axis
		// footprint, its "outer" height. align-items, align-self, and the
		// line's own shared height are resolved against that outer height
		// rather than the bare content height, matching real CSS, where
		// alignment distributes free space around an item's margin box.
		outerHeights[i] = margins[i].top + len(b.lines) + margins[i].bottom
		height = max(height, outerHeights[i])
	}

	// align-items: stretch, the default, means the item's own box grows to the
	// line's cross size, not merely that blank rows are parked underneath it.
	// A bordered or filled item has to be re-rendered at that height rather
	// than padded from outside, or its border closes at its content height and
	// leaves the stretch visible as a gap below the box. Only items that come
	// up short are re-rendered, and only when their own cross size is auto,
	// since an explicit height opts an item out of stretching per spec.
	// quoteDepth is replayed per item and restored afterward (see quoteBefore
	// above), since this is a second render of content the first pass already
	// walked.
	quoteAfter := r.quoteDepth
	for _, i := range group {
		it := items[i]
		if margins[i].topAuto || margins[i].bottomAuto {
			// An auto cross-axis margin opts the item out of stretching too
			// (§9.6): the free space it would have stretched into is the free
			// space the margin is claiming, and the two can't both have it.
			// crossAxisOffset places the unstretched item within the line.
			continue
		}
		if a := itemAlign(it, align); a != "" && a != "stretch" {
			continue
		}
		if h, ok := resolveCSSHeight(it.decls["height"], r.cbHeight); ok && h > 0 {
			continue
		}
		want := height - margins[i].top - margins[i].bottom
		// Stretching makes the item's cross size "as close to the line's as
		// possible, while still respecting the constraints imposed by
		// min-height/max-height" (§8.3), so an item's own max-height caps how
		// far it stretches. Without this the synthetic height injected by
		// renderFlexItemBoxSized overrode max-height, since block.go's height
		// takes priority over max-height by design, and a max-height:1 item on
		// a five-row line grew to all five. The cap is an outer size like want
		// itself, while max-height is content-box in this engine, so the item's
		// own border and padding rows are added back on. A percentage resolves
		// against the container's own height, r.cbHeight here, and is dropped
		// when that basis is indefinite.
		if m, ok := resolveCSSHeight(it.decls["max-height"], r.cbHeight); ok && m > 0 {
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
		outerHeights[i] = margins[i].top + len(b.lines) + margins[i].bottom
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
		offset := crossAxisOffset(margins[i], itemAlign(it, align), height, outerHeights[i])
		padded := padBoxVertical(itemBoxes[i], height, offset)
		for li := range rowLines {
			rowLines[li] += padded.lines[li]
		}
		// Column bookkeeping tracks the box's *painted* width, not the width the
		// main-axis pass allotted it. The synthetic overflow-x in
		// renderFlexItemBoxSized keeps the two equal for ordinary content, but
		// they can still legitimately diverge. An item whose box isn't produced
		// by the block box model, which renderFlexItemLeaf handles for a <table>
		// or a list, gets that width only as an available budget, since neither
		// renderer has a way to be told "paint exactly this wide", and a nested
		// flex container under a min-width larger than its allotment paints past
		// it too. Reading the width off the box keeps this honest either way:
		// whatever ends up on the line is what the next item's column is
		// measured from, so a Rect can't come to point at columns another item
		// is painting.
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

// layoutFlexRow lays out items left to right, or right to left when reverse
// is set for flex-direction:row-reverse. The main axis, carrying flex-grow and
// justify-content, is horizontal; the cross axis, carrying align-items and
// align-self, is vertical. flex-wrap:wrap buckets items into multiple lines via
// breakFlexLines, stacked vertically with row-gap, and align-content
// distributes any extra cross-axis space across those lines when the container
// has an explicit height taller than their natural stack. See
// distributeAlignContent.
func (r *Engine) layoutFlexRow(n *html.Node, decls map[string]string, innerW int, reverse bool) (box, map[*html.Node]Rect) {
	// Not reversed here for row-reverse: line breaking below runs over the
	// order-modified document order whichever direction the axis points, and
	// only each line's own placement flips. See reverseLineGroups.
	items := r.collectFlexItems(n)
	if len(items) == 0 {
		return r.emptyFlexBox(decls), nil
	}
	gap := parseGapLen(decls["column-gap"], innerW)
	// A percentage row-gap needs a definite container height, which only an
	// explicit height is: a min-height leaves the real height dependent on the
	// content, which is what these lines are still being laid out to
	// determine. Same rule percentage flex-basis follows in column direction.
	rowGapBasis, _ := r.resolveFlexContainerHeight(decls)
	rowGap := parseGapLen(decls["row-gap"], rowGapBasis)
	wrap, crossReverse := parseFlexWrap(decls)
	justify := decls["justify-content"]
	if reverse {
		justify = reverseJustify(justify)
	}
	// flex-wrap: wrap-reverse. Everything below lays out top-down, so the axis
	// is flipped once here, by mirroring the cross-axis alignment values, and
	// once more at the end, by stacking the finished lines in reverse. Nothing
	// in between has to know. justify-content is untouched: wrap-reverse turns
	// the cross axis around, not the main one.
	if crossReverse {
		decls, items = reverseCrossAxisDecls(decls, items)
	}

	// Fixed, non-auto margin-left/margin-right on a flex item needs no
	// separate handling here. renderFlexItemBox dispatches to the ordinary
	// block box model, renderBlockContentBox, which already bakes horizontal
	// margin directly into an item's own returned box regardless of flex
	// context, and into resolveMainBasis's natural-width measurement, which
	// renders through the same box model. Adding it again here would double it.
	//
	// margin-top/margin-bottom get no such treatment from the block box model
	// when the item's own top/bottom edge is open, meaning no border, padding,
	// or height. That is normally left for a block-flow caller to apply
	// externally via the collapse-through margin on decls (see block.go), a
	// convention flex layout doesn't participate in. Flex items never collapse
	// margins with each other or the container at all, since a flex formatting
	// context doesn't collapse in real CSS either, so each item's raw
	// margin-top/margin-bottom is read directly here and applied as its own
	// cross-axis space, via outerHeight below.
	//
	// margin-left/margin-right:auto is handled externally instead, in
	// layoutFlexLine, once each line's main-axis leftover space is known.
	// resolveMainBasis treats an auto side as contributing 0 to an item's
	// flex basis, matching real CSS, and the final render call below always
	// passes exactly the item's own resolved main-axis width as availWidth,
	// leaving renderBlockContentBox's own unrelated auto-margin-splitting
	// path nothing to distribute, since remaining is always 0. So it's safe to
	// leave decls untouched here and let layoutFlexLine own the real
	// distribution. margin-top/margin-bottom:auto is the cross-axis version of
	// the same idea and is resolved in crossAxisOffset, against the line's own
	// height rather than the row's leftover width.
	margins := make([]flexMargins, len(items))
	// bases[i] is the item's flex base size and widths[i] its hypothetical main
	// size, the two sizes §9.7 keeps apart. Line breaking below uses the
	// hypothetical, as the spec has it, while layoutFlexLine distributes from
	// the base. See resolveMainBasis.
	bases := make([]int, len(items))
	widths := make([]int, len(items))
	for i, it := range items {
		margins[i] = flexMargins{
			top:        resolveVerticalMargin(it.decls["margin-top"], innerW),
			bottom:     resolveVerticalMargin(it.decls["margin-bottom"], innerW),
			topAuto:    strings.TrimSpace(it.decls["margin-top"]) == "auto",
			bottomAuto: strings.TrimSpace(it.decls["margin-bottom"]) == "auto",
			leftAuto:   strings.TrimSpace(it.decls["margin-left"]) == "auto",
			rightAuto:  strings.TrimSpace(it.decls["margin-right"]) == "auto",
		}
		bases[i], widths[i] = r.resolveMainBasis(it, innerW)
	}

	lineGroups := breakFlexLines(len(items), widths, innerW, gap, wrap)
	if reverse {
		reverseLineGroups(lineGroups)
	}

	// A single-line container, flex-wrap: nowrap, has exactly one line, and
	// that line's cross size is the container's own inner cross size. So a
	// declared height on the container, or failing that its min-height (see
	// resolveFlexContainerHeightFloor), is what align-items and align-self
	// resolve against, which is what makes align-items:center vertically center
	// a fixed-height row. align-content is inoperative in that case, as real CSS
	// says outright, and the multi-line case instead sizes each line to its own
	// content and lets align-content place the lines, so the height only
	// reaches layoutFlexLine here.
	lineMinHeight := 0
	if !wrap {
		if h, ok, _ := r.resolveFlexContainerHeightFloor(decls); ok {
			lineMinHeight = h
		}
	}

	type lineResult struct {
		b   box
		h   int
		pos map[*html.Node]Rect
	}
	widthsBefore := slices.Clone(widths)
	quoteBefore := r.quoteDepth
	layout := func(lineMinHeights []int) ([]lineResult, int) {
		out := make([]lineResult, len(lineGroups))
		total := 0
		for li, group := range lineGroups {
			minH := lineMinHeight
			if lineMinHeights != nil {
				minH = lineMinHeights[li]
			}
			b, h, pos := r.layoutFlexLine(items, bases, widths, margins, group, innerW, gap, minH, justify, decls)
			out[li] = lineResult{b, h, pos}
			total += h
			if li > 0 {
				total += rowGap
			}
		}
		return out, total
	}
	results, contentHeight := layout(nil)

	// align-content: stretch, its initial value, shares the container's
	// leftover cross space equally across its lines by *growing* them, rather
	// than packing them somewhere inside it (§8.4). A line's cross size isn't
	// known until it's laid out, so this is a replay: the lines are laid out
	// once to measure, then again at the taller size. Growing a line grows the
	// items on it, which is the whole point and is already what layoutFlexLine
	// does with a forced cross size, so the second pass is the same call with
	// a different minHeight rather than any padding applied from outside.
	//
	// Two pieces of state have to be rewound first. layoutFlexLine resolves
	// flex-grow and flex-shrink into widths in place, so a replay against the
	// already-distributed widths would see no free space left to distribute.
	// And content: open-quote advances r.quoteDepth as it renders, so a replay
	// from where the first pass left off would continue the nesting instead of
	// repeating it. Both are restored, not merely saved.
	if h, ok, _ := r.resolveFlexContainerHeightFloor(decls); ok && wrap && h > contentHeight && alignContentStretches(decls["align-content"]) {
		shares := splitEvenly(h-contentHeight, len(results))
		lineMinHeights := make([]int, len(results))
		for li, res := range results {
			lineMinHeights[li] = res.h + shares[li]
		}
		copy(widths, widthsBefore)
		r.quoteDepth = quoteBefore
		results, contentHeight = layout(lineMinHeights)
	}

	if crossReverse {
		// The lines themselves are unchanged: wrap-reverse moves the
		// cross-start edge to the bottom of the container, so the first line
		// is the one that ends up lowest. Reversing here rather than at line
		// breaking is what keeps the items on each line the ones a browser
		// puts there, and each line's own item order untouched.
		slices.Reverse(results)
	}

	leadRows := 0
	wantHeight := 0
	extraGapsBetweenLines := make([]int, len(results)-1)
	// Anything left over after the stretch pass above, or all of it for the
	// packing values, which grow nothing. Stretch normally leaves nothing here:
	// it consumed the leftover by growing the lines themselves. It can still
	// come up short when a line refuses to grow, which a line whose items all
	// declare their own height does, and the trailing-blank flush at the end
	// covers that the same way it covers flex-start's.
	if h, ok, _ := r.resolveFlexContainerHeightFloor(decls); ok && h > contentHeight {
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
	// leadRows and extraGapsBetweenLines only place free space around and
	// between lines. flex-start and stretch, the default when align-content is
	// unset or set to "stretch" or "flex-start", and "center", which places
	// half the leftover rather than all of it, can each leave the container
	// short of its explicit height. Any remainder is flushed as trailing blank
	// rows, and that is what makes the container reach wantHeight, not
	// align-content's own distribution.
	for len(allLines) < wantHeight {
		allLines = append(allLines, "")
	}
	return box{lines: allLines, width: linesWidth(allLines)}, positions
}

// layoutFlexColumn lays out items top to bottom. The cross axis, carrying
// align-items and align-self, is horizontal; the main axis, carrying flex-grow,
// flex-shrink, and justify-content, is vertical. flex-basis sets an item's
// starting main-axis size, a line count, the same way it sets width in row
// direction.
//
// flex-grow, flex-shrink, and justify-content only have free space to work with
// once the container has a definite main-axis size: a declared CSS `height`, or
// failing that a `min-height` (resolveFlexContainerHeightFloor), which grows the
// container the same way but can never shrink it. Row direction's width is
// always definite, being however wide the caller says to render at, but this
// engine has no other notion of a column flex container's main-axis size. With
// no such height and no item using flex-basis, items stack with row-gap between
// them as if none of this machinery existed. That is flex-start main-axis
// behavior, the common case, and the only case before this file supported the
// main axis in column direction at all.
//
// reverse stacks bottom to top, for flex-direction: column-reverse. See CSS.md.
func (r *Engine) layoutFlexColumn(n *html.Node, decls map[string]string, innerW int, reverse bool) (box, map[*html.Node]Rect) {
	items := r.collectFlexItems(n)
	if reverse {
		items = reverseFlexItems(items)
	}
	if len(items) == 0 {
		return r.emptyFlexBox(decls), nil
	}
	// row-gap is the main axis here, and a percentage of it resolves against
	// the container's own explicit height for the same reason a percentage
	// flex-basis does: that height is the only definite main size a column
	// container has (see pctBasis below).
	rowGapBasis, _ := r.resolveFlexContainerHeight(decls)
	rowGap := parseGapLen(decls["row-gap"], rowGapBasis)
	align := decls["align-items"]
	// heightIsMinOnly marks a main size that came from min-height rather than
	// from an explicit height. It can raise the container, which flex-grow
	// fills, but must never put the items into flex-shrink, since a minimum
	// that content already exceeds is already satisfied. See
	// resolveFlexContainerHeightFloor.
	containerHeight, hasContainerHeight, heightIsMinOnly := r.resolveFlexContainerHeightFloor(decls)
	// Percentages need a *definite* main size to resolve against, which a
	// min-height isn't. The container's real main size is max(content,
	// min-height), so it isn't known until the content is laid out, and real CSS
	// treats a percentage against an indefinite basis as auto. Only an explicit
	// height feeds percentage resolution below, in parseColumnFlexBasis,
	// clampFlexMainHeight, flexMainAxisHeightFloor, and flexGrowHeightCeiling.
	// containerHeight itself is used for the row arithmetic, where a min-height
	// is a good target to grow into and pad out to.
	//
	// The indefinite case is carried as indefiniteMainSize rather than as a
	// plain 0. A zero basis resolves every percentage to zero, which is a
	// *definite* bound of nothing, so `max-height: 50%` on an item in a
	// min-height-only container froze it at one row and `flex-basis: 4;
	// max-height: 75%` rendered one row instead of four. Both should have
	// ignored the percentage outright, which is what a browser does.
	pctBasis := indefiniteMainSize
	if h, ok := r.resolveFlexContainerHeight(decls); ok {
		pctBasis = h
	}

	// margin-left/margin-right on a flex item needs no separate handling
	// here. See layoutFlexRow's doc comment on the same point: the block
	// box model already bakes horizontal margin into an item's own returned
	// box regardless of flex direction. margin-top/margin-bottom get no such
	// treatment, since a bare item's open top/bottom edge means the block box
	// model strips them into decls for a block-flow caller to apply, a
	// convention flex layout doesn't participate in, so they're read
	// directly here. Flex items never collapse margins with each other or
	// the container at all, since a flex formatting context doesn't collapse
	// in real CSS either. Each item's margin-bottom and the next item's
	// margin-top both apply in full, summed with row-gap, not collapsed via
	// max the way ordinary block flow would.
	//
	// An auto margin on either axis is resolved here rather than by the block
	// box model, the mirror of what layoutFlexLine does for row direction.
	// parseMargin returns 0 for "auto", so an auto vertical margin contributes
	// nothing to totalContentHeight below and is free to absorb leftover once
	// flex-grow and flex-shrink have run. The horizontal pair is read into
	// mlAuto/mrAuto and consumed at the cross-axis alignment switch.
	mt := make([]int, len(items))
	mb := make([]int, len(items))
	mtAuto := make([]bool, len(items))
	mbAuto := make([]bool, len(items))
	mlAuto := make([]bool, len(items))
	mrAuto := make([]bool, len(items))
	widths := make([]int, len(items))
	// crossSized[i] marks an item whose cross-axis width this pass resolved
	// itself, under an align-items or align-self other than stretch, rather
	// than handing it the container's full width. Only those get that width
	// enforced on their rendered box below. A stretched item is left at
	// whatever it rendered to, so an inline-flex container still shrinks to fit
	// its content instead of padding out to its caller's wrap bound.
	crossSized := make([]bool, len(items))
	// declBasis[i] is the item's declared flex base size in rows, 0 when
	// flex-basis is auto or content, or a percentage with no definite container
	// height to resolve it against. renderH[i] is the height override the
	// basis render below is given, which is that base clamped by the item's own
	// min-height/max-height, the hypothetical main size. They are the same two
	// sizes resolveMainBasis returns for row direction, and are kept apart for
	// the same reason: growth is measured from the base, and the hypothetical is
	// only a floor. 0 in renderH means "no override", so the item sizes itself
	// from its content.
	declBasis := make([]int, len(items))
	hasBasis := make([]bool, len(items))
	renderH := make([]int, len(items))
	for i, it := range items {
		mt[i] = resolveVerticalMargin(it.decls["margin-top"], innerW)
		mb[i] = resolveVerticalMargin(it.decls["margin-bottom"], innerW)
		mtAuto[i] = strings.TrimSpace(it.decls["margin-top"]) == "auto"
		mbAuto[i] = strings.TrimSpace(it.decls["margin-bottom"]) == "auto"
		mlAuto[i] = strings.TrimSpace(it.decls["margin-left"]) == "auto"
		mrAuto[i] = strings.TrimSpace(it.decls["margin-right"]) == "auto"
		itAlign := itemAlign(it, align)
		w := innerW
		if mlAuto[i] || mrAuto[i] {
			// An auto cross-axis margin opts the item out of stretching
			// (§9.6), exactly as it does on row direction's cross axis: the
			// free space it would have stretched into is the free space the
			// margin is claiming, and the two can't both have it. The item is
			// therefore sized to its own content and positioned by the
			// auto-margin split at the alignment switch below.
			w = r.resolveCrossWidth(it, innerW)
			crossSized[i] = true
		} else if itAlign != "" && itAlign != "stretch" {
			w = r.resolveCrossWidth(it, innerW)
			crossSized[i] = true
		} else if sw, sized := stretchCrossWidth(it, innerW); sized {
			w = sw
			crossSized[i] = true
		}
		widths[i] = max(1, w)
		declBasis[i], hasBasis[i] = parseColumnFlexBasis(it.decls, pctBasis)
		if hasBasis[i] {
			// The declared base is clamped by the item's own minimum and
			// maximum before it is rendered at all. Real CSS resolves an
			// item's used main size to at least min-height and at most
			// max-height regardless of flex-basis. Without the minimum, setting
			// flex-basis would defeat a min-height the item would otherwise get
			// for free from block.go's ordinary padding. Without the maximum,
			// the synthetic height renderFlexItemBoxHeight injects overrode
			// max-height, since block.go ranks an explicit height above it by
			// design, and a `flex-basis: 6; max-height: 1` item rendered six
			// rows tall.
			renderH[i] = clampFlexMainHeight(it.decls, declBasis[i], pctBasis)
		}
	}

	// Render once per item at its resolved flex-basis, where 0 means "no
	// override" and is identical to the item's own pre-existing height or
	// natural content behavior, to learn each item's hypothetical main size.
	// This mirrors row direction's flex-basis/measureNaturalWidth step.
	// quoteBefore[i] captures r.quoteDepth immediately before each item's own
	// render, so a later second render, when flex-grow or flex-shrink adjusts
	// its height, can be replayed from the correct starting depth instead of
	// whatever the full basis pass over every item left it at. content:
	// open-quote/close-quote mutates r.quoteDepth live (see block.go), and
	// this file's other repeated-render site, measureNaturalWidth, already
	// saves and restores it for the same reason.
	heights := make([]int, len(items))
	boxes := make([]box, len(items))
	poses := make([]map[*html.Node]Rect, len(items))
	quoteBefore := make([]int, len(items))
	// `flex-basis: content` sizes an item from its content while ignoring its
	// own main-size property, which in column direction is height.
	// parseColumnFlexBasis already returns 0 for it, meaning no synthetic height
	// override, but that alone would leave the item's own declared height to
	// size it through the ordinary block box model, which is the one thing the
	// keyword is asking not to happen. Stripping it here is what makes
	// `content` differ from `auto`. Rendered from renderItems throughout, so the
	// flex-grow/flex-shrink re-render below stays consistent with the basis
	// pass.
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
		b, pos := r.renderFlexItemBoxHeight(renderItems[i], widths[i], renderH[i])
		// The hypothetical main size is read back off the rendered box rather
		// than computed, so what flex layout reserves is always exactly what
		// the item paints. For a declared basis that's the already-clamped
		// renderH it was just handed. For a content-sized item it's whatever
		// its content came to, min-height padding included, which block.go
		// applies. A content-sized item's max-height is deliberately *not*
		// applied on top. block.go only clips to max-height under an explicit
		// overflow-y of hidden or clip, and this engine never truncates content
		// vertically without one, so honoring it here would reserve fewer rows
		// than the item goes on to paint.
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

	// heights[i] is the item's *hypothetical* main size: its flex base size
	// already raised by everything that can't be compressed away, namely its
	// own content, since text can't be forced into fewer lines, which is column
	// direction's stand-in for row direction's automatic minimum size, and its
	// min-height. baseH[i] is the flex base size itself, which is what flex-grow
	// distributes from. The two differ exactly when a declared flex-basis was
	// raised. See resolveMainBasis and growFlexLine for the row-direction
	// statement of the same rule.
	baseH := make([]int, len(items))
	for i := range items {
		baseH[i] = heights[i]
		if hasBasis[i] && declBasis[i] < heights[i] {
			baseH[i] = declBasis[i]
		}
	}

	finalHeights := append([]int(nil), heights...)
	leftover := 0
	if hasContainerHeight {
		leftover = containerHeight - totalContentHeight
		// Whatever of the container's height isn't the items themselves,
		// meaning row-gap and the items' own margins, isn't available to
		// distribute, so growFlexLine works against the height the items share.
		sumHeights := 0
		for _, h := range heights {
			sumHeights += h
		}
		availForItems := containerHeight - (totalContentHeight - sumHeights)
		allIdx := make([]int, len(items))
		grows := make([]float64, len(items))
		shrinks := make([]float64, len(items))
		ceilings := make([]int, len(items))
		for i, it := range items {
			allIdx[i] = i
			grows[i] = it.grow
			shrinks[i] = it.shrink
			ceilings[i] = flexGrowHeightCeiling(it.decls, pctBasis)
		}
		switch {
		case leftover > 0:
			// Mirrors layoutFlexLine's flex-grow branch, distributing into
			// height instead of width. Grown from each item's flex base size,
			// floored at its hypothetical main size, and capped per item at its
			// own max-height via ceilings, the mirror of row direction's
			// max-width ceiling. Any leftover the ceilings prevented from being
			// absorbed flows through to justify-content and the trailing pad
			// below, rather than being silently dropped.
			leftover = growFlexLine(finalHeights, baseH, heights, ceilings, grows, allIdx, availForItems)
		case leftover < 0 && !heightIsMinOnly:
			// Mirrors layoutFlexLine's flex-shrink branch, floors included. The
			// floors are resolved here rather than alongside grows and ceilings
			// above because measuring one can cost a trial render, and a
			// container that isn't overflowing has no use for the answer.
			// Skipped entirely for a min-height-derived main size: items taller
			// than the minimum have already satisfied it, and shrinking them to
			// fit would turn a floor into a ceiling.
			floors := make([]int, len(items))
			for i, it := range items {
				floors[i] = r.flexMainAxisHeightFloor(it, renderItems[i], widths[i], heights[i], renderH[i], pctBasis)
			}
			leftover = -distributeFlexShrink(finalHeights, shrinks, baseH, floors, allIdx, -leftover)
		}
	}

	// margin-top/margin-bottom: auto absorbs whatever main-axis leftover
	// remains once flex-grow and flex-shrink have had their turn, matching
	// real CSS's ordering of flexible-length resolution first and auto-margin
	// alignment second. This is layoutFlexLine's margin-left/margin-right:auto
	// pass with the axis swapped, and it follows the same two rules: the space
	// is split evenly across every auto margin in the container, and any auto
	// margin at all overrides justify-content outright, since both mechanisms
	// claim the same leftover and CSS gives it to the margins.
	//
	// An odd remainder favors the earliest auto margin in document order, top
	// before bottom within one item, which is what slot counts here.
	autoMarginsOverrideJustify := false
	if leftover > 0 {
		autoCount := 0
		for i := range items {
			if mtAuto[i] {
				autoCount++
			}
			if mbAuto[i] {
				autoCount++
			}
		}
		if autoCount > 0 {
			share, rem, slot := leftover/autoCount, leftover%autoCount, 0
			for i := range items {
				if mtAuto[i] {
					mt[i] += share
					if slot < rem {
						mt[i]++
					}
					slot++
				}
				if mbAuto[i] {
					mb[i] += share
					if slot < rem {
						mb[i]++
					}
					slot++
				}
			}
			leftover = 0
			autoMarginsOverrideJustify = true
		}
	}

	// Only re-render items flex-grow or flex-shrink adjusted. Every other
	// item's basis-step render, including the common case where neither
	// applies at all, is already the final box. Reset r.quoteDepth to what it
	// was immediately before this same item's basis render, quoteBefore[i],
	// rather than leaving it at whatever the full basis pass over every item
	// left it at. See quoteBefore's own doc comment above.
	for i := range items {
		if finalHeights[i] != heights[i] {
			r.quoteDepth = quoteBefore[i]
			boxes[i], poses[i] = r.renderFlexItemBoxHeight(renderItems[i], widths[i], finalHeights[i])
		}
	}

	// justify-content distributes whatever main-axis leftover space remains
	// after flex-grow and flex-shrink. It is the same distributeJustify used
	// for row direction's horizontal leftover, producing leading and in-between
	// blank lines instead of blank columns. leftover is only ever set away
	// from 0 above when hasContainerHeight is true, so without an explicit
	// container height this always resolves to distributeJustify(justify, 0,
	// n)'s own no-op case, and is safe to call unconditionally.
	justify := decls["justify-content"]
	if reverse {
		justify = reverseJustify(justify)
	}
	if autoMarginsOverrideJustify {
		// The auto margins above already consumed the leftover and set it to
		// 0, so distributeJustify is a no-op here whatever the value. Naming
		// the override explicitly keeps that from being an accident of
		// ordering if either side changes.
		justify = ""
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
		// row direction. renderBlockContentBox only pads to its available
		// width when an explicit width, text-align, or a border requires it,
		// so an item sized by min-width would otherwise collapse back to its
		// natural width and be aligned as if it were that narrow.
		b := boxes[i]
		if crossSized[i] {
			b = alignLinesBox(b, it.decls["text-align"], widths[i])
		}
		colOffset := 0
		switch {
		case mlAuto[i] || mrAuto[i]:
			// The cross-axis twin of the main-axis pass above (§8.1): an auto
			// margin absorbs the item's free space within the container's
			// content width and overrides align-items/align-self for this one
			// item. Both sides auto centers it, one side alone takes all the
			// space and pushes the item to the other end, so margin-left: auto
			// puts it against the container's right edge. With nothing left to
			// absorb it resolves to 0 rather than to a negative offset, like
			// every other alignment here.
			free := max(0, innerW-b.width)
			switch {
			case mlAuto[i] && mrAuto[i]:
				colOffset = free / 2
			case mlAuto[i]:
				colOffset = free
			}
		case itemAlign(it, align) == "center":
			colOffset = max(0, (innerW-b.width)/2)
		case itemAlign(it, align) == "flex-end":
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
		// though its painted box is left at whatever it rendered to. See
		// crossSized above: padding it out would stop an inline-flex container
		// shrinking to fit. The blank columns to its right are the item's, not
		// the container's, so its Rect has to claim them. Reporting the painted
		// width instead made a click in that region hit-test to the container.
		// widths[i] is innerW for a stretched item and the already-enforced
		// cross width for every other, so max() covers both, and leaves an item
		// that overflowed its allotment reporting what it painted.
		positions[it.node] = Rect{Row: row, Col: colOffset, Width: max(b.width, widths[i]), Height: len(b.lines)}
		if len(poses[i]) > 0 {
			positions = mergePositions(positions, poses[i], row, colOffset)
		}
		row += len(b.lines)
		prevMb = mb[i]
	}
	// The last item's own margin-bottom is never collapsed away, as above, so
	// flush it as trailing blank rows. An ordinary block's last child differs:
	// it can collapse through its container's open bottom edge.
	for range prevMb {
		lines = append(lines, "")
	}
	// leadRows and extraGaps only place free space around and between items.
	// justify-content values that don't consume all the leftover, or flex-grow
	// not applying to any item, can still leave the container short of its
	// explicit height, so any remainder is flushed as trailing blank rows,
	// mirroring row direction's align-content handling. containerHeight is 0
	// when hasContainerHeight is false, so this is already a no-op without an
	// explicit height, and is safe unconditionally.
	for len(lines) < containerHeight {
		lines = append(lines, "")
	}
	return box{lines: lines, width: linesWidth(lines)}, positions
}

// emptyFlexBox is the content box of a flex container with no items at all.
// There is nothing to lay out, but the container still has whatever main size
// it declared, an explicit height or failing that a min-height
// (resolveFlexContainerHeightFloor), so it reserves that many blank rows. The
// per-item distribution passes that normally do this are the same ones the
// caller skipped by returning early here.
//
// With no declared height it is one empty line, not none. That is every other
// empty box's content in this engine, and renderFlexContentBox drops it there
// if something is going to be drawn around it (see contentEmpty).
func (r *Engine) emptyFlexBox(decls map[string]string) box {
	if h, ok, _ := r.resolveFlexContainerHeightFloor(decls); ok && h > 0 {
		return box{lines: make([]string, h)}
	}
	return box{lines: []string{""}}
}

// renderFlexContentBox renders a block-level display:flex flex container. It
// uses the same margin/border/padding box model as renderBlockContentBox,
// wrapped around flex item layout instead of inline content flow. See CSS.md's
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
	// ovY, heightLines, gutterWidth, hasScrollbarGutter, capStartDrawn, and
	// capEndDrawn mirror renderBlockContentBox's identically-named locals
	// (block.go): ovY is read once here rather than repeatedly from decls
	// below. heightLines is resolved ahead of layoutFlex only so a scrollable
	// container's gutter can reserve its column before wrapping. Only an
	// explicit height counts, not max-height, the same "min-height/max-height
	// alone don't count" rule CSS.md's overflow entry already documents for
	// ordinary boxes. capStartDrawn and capEndDrawn are declared here, not
	// with := in the overflow-y switch below, so they survive that switch's
	// scope for the liveScrollViewport assignment near the end of this
	// function.
	ovY := decls["overflow-y"]
	heightLines := 0
	if h, ok := r.resolveFlexContainerHeight(decls); ok {
		heightLines = h
	}
	gutterWidth := 0
	if heightLines > 0 && ovY == "scroll" {
		if w := r.scrollbarGutterWidth(n, decls); avail-w >= 1 {
			gutterWidth = w
		}
	}
	hasScrollbarGutter := gutterWidth > 0
	var capStartDrawn, capEndDrawn bool
	var innerW int
	pl, pr, innerW = clampCellPadding(avail-gutterWidth, pl, pr)
	if innerW < 1 {
		innerW = 1
	}

	content, positions := r.layoutFlex(n, decls, innerW)
	// Captured before padLinesToWidthBox and the padding/border steps below turn
	// an empty line into real characters. That is the same point, and the same
	// reason, renderBlockContentBox captures its own. See the contentEmpty block
	// further down.
	contentEmpty := len(content.lines) == 1 && content.lines[0] == ""
	// Under a shrink-to-fit measurement render, report the laid-out content's
	// own width rather than the available width. This is the flex-container
	// mirror of renderBlockContentBox's identical narrowing, and what lets a
	// nested display:flex item measure as its items' natural extent. See
	// Engine.shrinkToFit.
	if r.shrinkToFit && !hasExplicitWidth && content.width < innerW {
		innerW = max(1, content.width)
		hBorderWidth = textcell.Width(bl.char) + pl + innerW + gutterWidth + pr + textcell.Width(br.char)
	}
	// overflow-x: hidden/clip truncates each line to the container's content
	// width, the same as renderBlockContentBox does for an ordinary block box
	// (block.go). A flex container is no exception to CSS.md's overflow-x
	// entry, but this used to be the one box model that never consulted it, so
	// an author's own `overflow: hidden` on a flex container did nothing and a
	// nested flex item that exceeded its allotment could only be trimmed from
	// outside, which cut its right border glyph off.
	//
	// Unlike block.go's, this is not gated on the container having an explicit
	// width. That gate exists because a plain block with no declared width
	// already fills its available width and so has nothing to clip, an
	// assumption a flex container breaks: items floored at their automatic
	// minimum size (flexMainAxisFloor) can overflow a container that declared
	// no width at all. Gating here would make `overflow: hidden` do nothing in
	// exactly the case it's reached for. See CSS.md's overflow entry, which
	// carries the same carve-out.
	//
	// Deliberately not extended to scroll/auto: a horizontally scrollable flex
	// container would need the live offset and gutter plumbing block.go has for
	// ordinary boxes (see docs/SCROLLING.md), which is a larger piece of work
	// than this.
	if ov := decls["overflow-x"]; ov == "hidden" || ov == "clip" {
		suffix := textOverflowSuffix(decls["text-overflow"])
		lines := make([]string, len(content.lines))
		for i, ln := range content.lines {
			lines[i] = textcell.TruncateToWidth(ln, innerW, suffix)
		}
		content = box{lines: lines, width: linesWidth(lines)}
	}
	// A block-level flex container fills its available width by default, same
	// as any other block box. Pad any leftover columns after the last item or
	// row, whether that is a row narrower than innerW or a column item narrower
	// than innerW under a non-stretch align-items, rather than leaving a
	// ragged-width box.
	content = padLinesToWidthBox(content, innerW)
	// overflow-y truncates or scrolls the container to its own declared
	// height, the vertical counterpart of the overflow-x block above and of
	// what renderBlockContentBox does for an ordinary block box (block.go).
	// Growing to a height, or to a min-height, is layout's job: layoutFlexRow
	// and layoutFlexColumn pad to it themselves, since the items have to be
	// able to distribute those rows among themselves. But nothing in layout
	// ever shortens the container, so a flex container whose content exceeded
	// its height ignored both the height and the author's overflow.
	switch ovY {
	case "hidden", "clip":
		// An explicit height wins over max-height, matching block.go's own
		// priority. heightLines already holds the explicit-height case;
		// max-height is the fallback only when there's no explicit height.
		limit := heightLines
		if limit == 0 {
			if m, ok := resolveCSSHeight(decls["max-height"], r.cbHeight); ok && m > 0 {
				limit = m
			}
		}
		if limit > 0 && len(content.lines) > limit {
			lines := content.lines[:limit]
			content = box{lines: lines, width: linesWidth(lines)}
		}
	case "scroll", "auto":
		// Unlike hidden/clip, max-height never substitutes for an explicit
		// height here: real scrolling needs a resolved height to scroll
		// within, and CSS.md's overflow entry already documents "min-height/
		// max-height alone don't count" for auto on an ordinary box.
		// applyVerticalScroll is shared with renderBlockContentBox (block.go);
		// row-direction and column-direction flex containers alike already
		// reduce to a flat content.lines by this point, the same as hidden/clip
		// already treats them uniformly above, so no direction-specific branch
		// is needed here either.
		if heightLines > 0 {
			var lines []string
			lines, positions, capStartDrawn, capEndDrawn = r.applyVerticalScroll(n, decls, content.lines, positions, heightLines, innerW, gutterWidth, hasScrollbarGutter)
			content = box{lines: lines, width: linesWidth(lines)}
		}
	}

	// Resolved here rather than where they're applied, because whether they
	// exist decides whether an empty container keeps its content row. See
	// renderBlockContentBox, which does the same in the same order.
	topRule := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth, r.profile)
	botRule := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth, r.profile)

	// A flex container with no items has zero content height, like any other
	// empty block box, so it gives up its one blank content line when
	// something else will occupy a row: an empty bordered container's two rules
	// meet, rather than sitting either side of a row nothing is in. The
	// declared-height guard is emptyFlexBox's own reservation coming back
	// through. Those rows were asked for, and layout skipped only the
	// distribution of them, not the request. See renderBlockContentBox for the
	// full statement of the rule.
	if _, hasDeclaredHeight, _ := r.resolveFlexContainerHeightFloor(decls); contentEmpty && !hasDeclaredHeight && (pt+pb > 0 || topRule != "" || botRule != "") {
		content = box{}
	}

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
	// No empty-container special case on either rule: an empty container has
	// already surrendered its content line above, so each rule simply becomes
	// one line here.
	topRuleDrawn := false
	if topRule != "" {
		content.lines = append([]string{topRule}, content.lines...)
		content.width = linesWidth(content.lines)
		topRuleDrawn = true
	}
	if botRule != "" {
		content.lines = append(content.lines, botRule)
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
	if isHiddenVisibility(decls["visibility"]) {
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
	colShift := pl + textcell.Width(bl.char) + ml
	r.liveContentOffsetsX[n] = colShift
	if heightLines > 0 && (ovY == "scroll" || ovY == "auto") {
		// See recordScrollViewport's own doc comment (block.go); it's shared
		// with renderBlockContentBox.
		r.recordScrollViewport(n, heightLines, rowShift, colShift, innerW, gutterWidth, capStartDrawn, capEndDrawn)
	}
	return content, positions
}

// renderInlineFlexContent renders a display:inline-flex element's whole box at
// availWidth, a wrap-context bound rather than a literal target width, the same
// convention inline-block already uses. It is returned as a plain string
// because, like inline-block, an inline-flex container is one atomic trackable
// unit. See inline.go's nested "inline-block" case for the identical,
// already-accepted rationale. Its own position is tracked by the caller boxing
// this string, but individual descendants inside it are not.
//
// The box comes from renderFlexContentBox, the same margin/border/padding model
// a block-level display:flex container uses, so an inline-flex container's own
// border, padding, margin, and overflow-x/overflow-y clipping all behave as
// they do everywhere else. This used to call layoutFlex directly, which is only
// the *content* half of that model, so every one of those was silently dropped:
// a bordered inline-flex painted no border at all, and CSS.md's promise that
// overflow-x/overflow-y clip a `flex`/`inline-flex` container held for one of
// the two.
//
// availWidth is only a bound. An inline-flex container is shrink-to-fit, since
// real CSS sizes it by fit-content rather than by its containing block, while
// renderFlexContentBox fills whatever width it is handed, which is correct for
// the block-level container it was written for. Narrowing the bound to the
// container's own natural outer extent first is what reconciles the two: the
// fill then stops at the content's own width. Handing the full bound over
// instead makes it a *definite* size that flex-grow and justify-content have
// free space to spread items across, so `display: inline-flex;
// justify-content: space-between` pushes its items to the far edges of the
// surrounding text's wrap bound rather than sitting compactly wherever it was
// placed.
//
// A declared width opts out of the narrowing entirely, since it is a definite
// size and renderFlexContentBox resolves it against the original bound. So do
// min-width and max-width, but only inside that resolution: both are applied to
// the *narrowed* width there, which is what makes a max-width smaller than the
// content shrink the container while one larger than it leaves a shrink-to-fit
// box alone rather than padding it out to the maximum.
func (r *Engine) renderInlineFlexContent(n *html.Node, decls map[string]string, availWidth int) string {
	if r.measuringNaturalWidth && availWidth > measureBlockWidthCap {
		availWidth = measureBlockWidthCap
	}
	if _, definite := resolveCSSSize(decls["width"], availWidth); !definite {
		// The natural extent is measured inside the container's own chrome and
		// the bound is narrowed outside it, so the two are the same box:
		// renderFlexContentBox subtracts that chrome straight back off to reach
		// the inner width the measurement was taken at.
		chrome := blockHorizontalChrome(decls, availWidth)
		budget := max(1, availWidth-chrome)
		if natural := r.inlineFlexNaturalWidth(n, decls, budget); natural > 0 && natural+chrome < availWidth {
			availWidth = natural + chrome
		}
	}
	b, _ := r.renderFlexContentBox(n, decls, availWidth)
	return b.join()
}

// inlineFlexNaturalWidth is an inline-flex container's shrink-to-fit main-axis
// extent: in row direction its items' own resolved base sizes plus the gaps
// between them, in column direction its widest single item. flex-grow and
// justify-content are deliberately not consulted, since both only distribute
// space that a definite container size makes available, and the point of a
// shrink-to-fit container is that it has none to hand out. flex-shrink isn't
// consulted either: a natural width wider than availWidth is capped by the
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
	// A basis of 0: a shrink-to-fit container's own width is what this is
	// measuring, so it isn't definite yet, and a percentage gap contributes
	// nothing to an intrinsic size contribution (CSS Box Alignment §8.3).
	total := parseGapLen(decls["column-gap"], 0) * (len(items) - 1)
	for _, it := range items {
		// The hypothetical main size, not the flex base size. Shrink-to-fit is
		// a content-based measurement, and an item's used minimum, its
		// automatic minimum size, is part of the room it genuinely needs.
		// A `flex: 1` item's flex base size of 0 is not.
		_, hypo := r.resolveMainBasis(it, availWidth)
		total += hypo
	}
	return total
}
