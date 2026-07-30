package htmlterm_test

import "testing"

func TestFlexRow(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row lays out items side by side with no grow", width: 20, html: `<div style="display:flex;width:100%"><div>a</div><div>bb</div><div>ccc</div></div>`, want: "abbccc              \n"},
		{name: "flex-grow distributes leftover space by weight", width: 21, html: `<div style="display:flex;width:100%"><div style="flex-grow:1">a</div><div style="flex-grow:2">b</div></div>`, want: "a      b             \n"},
		{name: "flex shorthand N sets equal grow with zero basis", width: 10, html: `<div style="display:flex;width:100%"><div style="flex:1">a</div><div style="flex:1">b</div></div>`, want: "a    b    \n"},
		{name: "many equally-growing items still total exactly the container width", width: 21, html: `<div style="display:flex;width:100%"><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div><div style="flex-grow:1">x</div></div>`, want: "x  x  x  x  x  x x x \n"},
		{name: "flex-basis percent sets starting main size before grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:30%">a</div><div style="flex-grow:1">b</div></div>`, want: "a     b             \n"},
		{name: "width longhand sets item main size when flex-basis is unset", width: 12, html: `<div style="display:flex;width:100%"><div style="width:4">a</div><div style="flex-grow:1">b</div></div>`, want: "a   b       \n"},
		{name: "items shrink by default when they overflow the container", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:8">aaaaaaaa</div><div style="flex-basis:8">bbbbbbbb</div></div>`, want: "aaaaaaaabbbbbbbb\n"},
		{name: "flex-shrink:0 opts an item out of shrinking, forcing overflow", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:4;flex-shrink:0">a</div><div style="flex-basis:4;flex-shrink:0">b</div><div style="flex-basis:4;flex-shrink:0">c</div></div>`, want: "a   b   c   \n"},
		{name: "justify-content space-between spreads items across the row", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-between"><div>a</div><div>b</div><div>c</div></div>`, want: "a         b        c\n"},
		{name: "justify-content center centers items with no grow", width: 10, html: `<div style="display:flex;width:100%;justify-content:center"><div>ab</div></div>`, want: "    ab    \n"},
		{name: "justify-content flex-end pushes items to the end", width: 10, html: `<div style="display:flex;width:100%;justify-content:flex-end"><div>ab</div></div>`, want: "        ab\n"},
		// 17 free columns over 3 items: each item's share is centered on it, so
		// the edges get a half-share (3 and 2, the rounding remainder spread
		// rather than piled onto one edge) and each between-gap gets two.
		{name: "justify-content space-around distributes half-shares at the edges", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-around"><div>a</div><div>b</div><div>c</div></div>`, want: "   a      b      c  \n"},
		{name: "justify-content space-evenly makes every gap, edges included, equal", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-evenly"><div>a</div><div>b</div><div>c</div></div>`, want: "     a    b    c    \n"},
		{name: "justify-content start is a synonym for flex-start", width: 10, html: `<div style="display:flex;width:100%;justify-content:start"><div>ab</div></div>`, want: "ab        \n"},
		{name: "justify-content end is a synonym for flex-end", width: 10, html: `<div style="display:flex;width:100%;justify-content:end"><div>ab</div></div>`, want: "        ab\n"},
		{name: "justify-content normal behaves as the unset default", width: 10, html: `<div style="display:flex;width:100%;justify-content:normal"><div>ab</div></div>`, want: "ab        \n"},
		{name: "column-gap inserts space between items", width: 10, html: `<div style="display:flex;width:100%;gap:2"><div>a</div><div>b</div></div>`, want: "a  b      \n"},
		{name: "align-items center centers shorter items on the cross axis", width: 10, html: `<div style="display:flex;width:100%;align-items:center"><div>a<br>b<br>c</div><div>x</div></div>`, want: "a         \nbx        \nc         \n"},
		{name: "align-items flex-end aligns items to the row's bottom", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-end"><div>a<br>b</div><div>x</div></div>`, want: "a         \nbx        \n"},
		{name: "text-only children between flex items are not laid out as items", width: 10, html: `<div style="display:flex;width:100%">loose text<div>a</div></div>`, want: "a         \n"},
		{name: "display:none child is excluded from layout", width: 10, html: `<div style="display:flex;width:100%"><div style="display:none">a</div><div>b</div></div>`, want: "b         \n"},
	})
}

// TestFlexDisplayContentsChild covers a display:contents child of a flex
// container: it generates no box of its own, so its children become the
// container's flex items. It used to be blockified into one item instead,
// stacking its children vertically inside it.
func TestFlexDisplayContentsChild(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a display:contents child's children become flex items", width: 20, html: `<div style="display:flex;width:100%"><div style="display:contents"><div>a</div><div>b</div></div><div>c</div></div>`, want: "abc                 \n"},
		{name: "nested display:contents wrappers flatten all the way down", width: 20, html: `<div style="display:flex;width:100%"><div style="display:contents"><div style="display:contents"><div>a</div></div><div>b</div></div></div>`, want: "ab                  \n"},
		{name: "hoisted items take part in the container's own order sequence", width: 20, html: `<div style="display:flex;width:100%"><div style="display:contents"><div style="order:2">a</div></div><div>b</div></div>`, want: "ba                  \n"},
		{name: "style inherited through the display:contents wrapper still reaches its items", width: 20, html: `<div style="display:flex;width:100%"><div style="display:contents;text-transform:uppercase"><div>a</div><div>b</div></div></div>`, want: "AB                  \n"},
	})
}

func TestFlexColumn(t *testing.T) {
	runCases(t, []renderCase{
		{name: "column stacks items top to bottom", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div>a</div><div>bbb</div></div>`, want: "a         \nbbb       \n"},
		{name: "row-gap inserts a blank line between items", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;gap:1"><div>a</div><div>b</div></div>`, want: "a         \n          \nb         \n"},
		{name: "align-items center centers narrower items horizontally", width: 11, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:center"><div>a</div><div>bbbbb</div></div>`, want: "     a     \n   bbbbb   \n"},
		{name: "align-items flex-end right-aligns items", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:flex-end"><div>a</div><div>bbb</div></div>`, want: "         a\n       bbb\n"},
		{name: "align-items center honors an item's own explicit width over its natural content width", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:center"><div style="width:6">a</div></div>`, want: "   a        \n"},
		{name: "align-items stretch (default) fills the full width", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div>a</div></div>`, want: "a         \n"},
	})
}

func TestFlexColumnMainAxis(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-basis sets an item's starting height, padding with blank lines", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:3">a</div><div>b</div></div>`, want: "a         \n          \n          \nb         \n"},
		{name: "flex-grow distributes leftover height once the container has an explicit height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:6"><div style="flex-grow:1">a</div><div style="flex-grow:1">b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n"},
		{name: "flex-shrink shrinks items proportionally when content overflows an explicit height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:3"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div></div>`, want: "a         \nb         \n          \n"},
		{name: "justify-content:center distributes leftover height when no item grows", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:5;justify-content:center"><div>a</div></div>`, want: "          \n          \na         \n          \n          \n"},
		{name: "flex-basis percent resolves against the container's explicit height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:10"><div style="flex-basis:50%">a</div><div>b</div></div>`, want: "a         \n          \n          \n          \n          \nb         \n          \n          \n          \n          \n"},
		{name: "no explicit height and no flex-basis keeps the original flex-start stacking behavior", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-grow:1">a</div><div>b</div></div>`, want: "a         \nb         \n"},
		{name: "flex-grow on a nested flex-container child grows via the column height override", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:6"><div style="display:flex;flex-grow:1"><span>a</span></div><div>b</div></div>`, want: "a         \n          \n          \n          \n          \nb         \n"},
		{name: "flex-grow never grows an item past its own max-height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:10"><div style="flex-grow:1;max-height:3">a</div><div>b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n          \n          \n          \n          \n"},
		{name: "flex-basis never renders an item shorter than its own min-height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:2;min-height:10">a</div></div>`, want: "a         \n          \n          \n          \n          \n          \n          \n          \n          \n          \n"},
		{
			name:  "a re-rendered (flex-grow-adjusted) item's quote nesting depth isn't double-counted",
			css:   `.opener::before { content: open-quote; } q { quotes: '"' '"' "'" "'" '<' '>'; }`,
			html:  `<div style="display:flex;flex-direction:column;width:100%;height:4"><div class="opener" style="flex-grow:1">x</div></div><q>y</q>`,
			width: 10,
			want:  "“x        \n          \n          \n          \n'y'",
		},
	})
}

func TestFlexContainerBoxModel(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border and padding wrap flex row content", width: 12, html: `<div style="display:flex;width:100%;border-style:solid;padding:1"><div>a</div><div>b</div></div>`, want: "┌──────────┐\n│          │\n│ ab       │\n│          │\n└──────────┘\n"},
		{name: "margin-top separates a flex container from siblings", width: 10, html: `<p>before</p><div style="display:flex;margin-top:1"><div>x</div></div>`, want: "before\n\nx         \n"},
		{name: "explicit width on the flex container itself is honored", width: 20, html: `<div style="display:flex;width:8;border-style:solid"><div>a</div></div>`, want: "┌──────┐\n│a     │\n└──────┘\n"},
	})
}

func TestInlineFlex(t *testing.T) {
	runCases(t, []renderCase{
		{name: "inline-flex renders as one atomic unit within inline flow", width: 40, html: `<p>before <span style="display:inline-flex;gap:1"><span>a</span><span>b</span></span></p>`, want: "before a b\n\n"},
		{name: "inline-flex column direction stacks items", width: 40, html: `<span style="display:inline-flex;flex-direction:column"><span>a</span><span>b</span></span>`, want: "a\nb"},
	})
}

// TestFlexReverseAndOrder also covers a reverse direction moving the
// main-*start* edge, not just flipping the item sequence: with row-reverse the
// main axis starts at the right, so items pack against the right edge under
// the default justify-content, and justify-content's own start/end values are
// mirrored to match.
func TestFlexReverseAndOrder(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row-reverse lays items out right to left, packed against the right edge", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div>a</div><div>b</div><div>c</div></div>`, want: "       cba\n"},
		{name: "row-reverse justify-content:flex-end packs against the left edge", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%;justify-content:flex-end"><div>a</div><div>b</div><div>c</div></div>`, want: "cba       \n"},
		{name: "row-reverse justify-content:center is its own mirror image", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%;justify-content:center"><div>a</div><div>b</div></div>`, want: "    ba    \n"},
		{name: "column-reverse stacks items bottom to top", width: 5, html: `<div style="display:flex;flex-direction:column-reverse;width:100%"><div>a</div><div>b</div></div>`, want: "b    \na    \n"},
		{name: "column-reverse packs items against the bottom of a taller container", width: 5, html: `<div style="display:flex;flex-direction:column-reverse;width:100%;height:4"><div>a</div><div>b</div></div>`, want: "     \n     \nb    \na    \n"},
		{name: "order re-sequences items ascending, ties keep document order", width: 10, html: `<div style="display:flex;width:100%"><div style="order:2">a</div><div style="order:1">b</div><div>c</div></div>`, want: "cba       \n"},
		{name: "order is resolved before row-reverse flips the sequence", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div style="order:2">a</div><div style="order:1">b</div></div>`, want: "        ab\n"},
	})
}

func TestFlexAlignSelf(t *testing.T) {
	runCases(t, []renderCase{
		{name: "align-self overrides align-items on the cross axis in a row", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-start"><div>a<br>b</div><div style="align-self:flex-end">x</div></div>`, want: "a         \nbx        \n"},
		{name: "align-self overrides align-items on the cross axis in a column", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:flex-start"><div>a</div><div style="align-self:center">bbb</div></div>`, want: "a         \n   bbb    \n"},
		{name: "align-items start is a synonym for flex-start", width: 10, html: `<div style="display:flex;width:100%;align-items:start"><div>a<br>b</div><div>x</div></div>`, want: "ax        \nb         \n"},
		{name: "align-self self-end is a synonym for flex-end", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-start"><div>a<br>b</div><div style="align-self:self-end">x</div></div>`, want: "a         \nbx        \n"},
		{name: "align-self:auto defers to the container's align-items", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-end"><div>a<br>b</div><div style="align-self:auto">x</div></div>`, want: "a         \nbx        \n"},
	})
}

func TestFlexItemMargin(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row margin-left/right shrinks available space and shifts the next item", width: 20, html: `<div style="display:flex;width:100%;gap:1"><div style="margin-left:2;margin-right:1">a</div><div>b</div></div>`, want: "  a  b              \n"},
		{name: "row margin-left combines correctly with flex-grow on a later item", width: 20, html: `<div style="display:flex;width:100%"><div style="margin-left:2">a</div><div style="flex-grow:1">b</div></div>`, want: "  ab                \n"},
		{name: "row margin-top/margin-bottom widen the row and shift align-items:center", width: 20, html: `<div style="display:flex;width:100%;align-items:center"><div style="margin-top:1">a</div><div style="margin-bottom:1">b</div></div>`, want: " b                  \na                   \n"},
		{name: "column margin-top/margin-bottom sum with row-gap, not collapsed", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;gap:1"><div style="margin-bottom:1">a</div><div style="margin-top:1">b</div></div>`, want: "a         \n          \n          \n          \nb         \n"},
		{name: "column last item's margin-bottom still renders, not collapsed away", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="margin-bottom:2">a</div></div>after`, want: "a         \n          \n          \nafter"},
		{name: "column margin-left/right under align-items:center", width: 20, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:center"><div style="margin-left:2;margin-right:2">x</div></div>`, want: "         x          \n"},
		{name: "column margin-left/right under the default stretch", width: 20, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="margin-left:2;margin-right:2">x</div></div>`, want: "  x                 \n"},
		// Physical margins stay physical under a reverse direction: "a" is
		// laid out last (so it paints rightmost) and its margin-right is still
		// on its right, past the right edge of the packed sequence.
		{name: "row-reverse keeps each item's own margin, not mirrored", width: 20, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div style="margin-right:2">a</div><div>b</div></div>`, want: "                ba  \n"},
	})
}

func TestFlexMarginAuto(t *testing.T) {
	runCases(t, []renderCase{
		{name: "margin-left:auto pushes a single item to the far end of the row", width: 20, html: `<div style="display:flex;width:100%"><div style="margin-left:auto">a</div></div>`, want: "                   a\n"},
		{name: "margin-right:auto on the first of two items pushes the second to the far end", width: 10, html: `<div style="display:flex;width:100%"><div style="margin-right:auto">a</div><div>b</div></div>`, want: "a        b\n"},
		{name: "margin:auto on both sides centers a single item", width: 10, html: `<div style="display:flex;width:100%"><div style="margin-left:auto;margin-right:auto">a</div></div>`, want: "     a    \n"},
		{name: "margin:auto overrides justify-content entirely for the line", width: 10, html: `<div style="display:flex;width:100%;justify-content:center"><div style="margin-right:auto">a</div><div>b</div></div>`, want: "a        b\n"},
		{name: "flex-grow already consuming all leftover space leaves nothing for margin:auto", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-grow:1">a</div><div style="margin-left:auto">b</div></div>`, want: "a                  b\n"},
	})
}

func TestFlexShrink(t *testing.T) {
	runCases(t, []renderCase{
		{name: "equal flex-shrink weights shrink items proportionally by basis to fit the row", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a  b  c   \n"},
		{name: "a larger flex-shrink weight absorbs more of the overflow", width: 8, html: `<div style="display:flex;width:100%"><div style="flex-basis:6;flex-shrink:3">a</div><div style="flex-basis:6">b</div></div>`, want: "a  b    \n"},
		{name: "flex-shrink:0 opts an item out of shrinking entirely", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:6;flex-shrink:0">a</div><div style="flex-basis:6">b</div></div>`, want: "a     b   \n"},
		{name: "min-width floors how far flex-shrink can reduce an item", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:6;min-width:5"></div><div style="flex-basis:6">bbbbbb</div></div>`, want: "     bbbbbb\n"},
		{name: "items that already fit are never shrunk", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div></div>`, want: "a   b               \n"},
		// "NaN" parses as a float in Go but is not a CSS <number>. Were it
		// accepted, it would compare false against every bound (so it would
		// pass for a valid weight) and then make the total weight NaN, leaving
		// the well-formed sibling with nothing.
		{name: "a non-CSS numeric flex-grow is ignored without affecting its siblings", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-grow:NaN">a</div><div style="flex-grow:1">b</div></div>`, want: "ab                  \n"},
		{name: "flex shorthand's <grow> <shrink> <basis> form shrinks items proportionally", width: 10, html: `<div style="display:flex;width:100%"><div style="flex:0 1 8">a</div><div style="flex:0 1 8">b</div></div>`, want: "a    b    \n"},
		// Items 1 and 2 absorb the whole 6-column deficit between them because
		// item 3 is floored at min-width:7 and can only give back 1 of the 2
		// columns it was asked for — that unusable column is re-split onto the
		// two that can still shrink, so the row totals exactly 18 rather than
		// overflowing by one.
		{name: "a floored item's share of the deficit moves to the items that can still shrink", width: 18, html: `<div style="display:flex;width:100%"><div style="flex-basis:8">a</div><div style="flex-basis:8">b</div><div style="flex-basis:8;min-width:7">c</div></div>`, want: "a    b     c      \n"},
	})
}

// TestFlexBasisMinMaxClamp covers the flex base size being clamped by the
// item's own min/max in the same axis before line-breaking and grow/shrink -
// the spec's "hypothetical main size". renderBlockContentBox resolves those
// two for the item's own render regardless, so without the clamp here the box
// painted at a different width than flex layout had allotted it and every
// later item on the line drifted.
func TestFlexBasisMinMaxClamp(t *testing.T) {
	runCases(t, []renderCase{
		{name: "min-width raises an item's basis above its own width", width: 20, html: `<div style="display:flex;width:100%"><div style="width:3;min-width:9">aa</div><div>bb</div></div>`, want: "aa       bb         \n"},
		{name: "max-width lowers an item's basis below its own flex-basis", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:12;max-width:4">aa</div><div>bb</div></div>`, want: "aa  bb              \n"},
		{name: "max-width caps how far flex-grow can grow an item", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-grow:1;max-width:5">a</div><div style="flex-grow:1">b</div></div>`, want: "a    b              \n"},
		// The capped item's unusable share of the leftover moves to the item
		// that can still grow, so nothing is left for justify-content to place
		// — a leading pad here would mean the row came up short.
		{name: "a capped item's share of the leftover moves to the items that can still grow", width: 20, html: `<div style="display:flex;width:100%;justify-content:flex-end"><div style="flex-grow:1;max-width:5">a</div><div style="flex-grow:1">b</div></div>`, want: "a    b              \n"},
		{name: "column cross-axis min-width widens the item's own box", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:flex-end"><div style="min-width:6">a</div></div>`, want: "      a     \n"},
		{name: "column cross-axis max-width narrows the item's own box", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:flex-end"><div style="width:8;max-width:4">a b</div></div>`, want: "        a b \n"},
	})
}

func TestFlexWrap(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-wrap:wrap moves overflowing items to a second line", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a   b     \nc         \n"},
		{name: "gap sets both column-gap and row-gap, including between wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;gap:1"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a    b    \n          \nc         \n"},
		{name: "nowrap (default) keeps items on one line, shrinking to fit rather than wrapping", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a  b  c   \n"},
		{name: "the flex-flow shorthand sets flex-direction and flex-wrap together", width: 10, html: `<div style="display:flex;flex-flow:row wrap;width:100%"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \nb         \n"},
		{name: "an item too wide for the container still gets its own line rather than being dropped", width: 6, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="flex-basis:8">aaaaaaaa</div><div style="flex-basis:4">b</div></div>`, want: "aaaaaaaa\nb     \n"},
	})
}

func TestFlexAlignContent(t *testing.T) {
	runCases(t, []renderCase{
		{name: "align-content:flex-end pushes wrapped lines to the container's bottom", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:flex-end"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \n          \n          \na         \nb         \n"},
		{name: "align-content:center splits leftover space evenly above and below wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:center"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \na         \nb         \n          \n          \n"},
		{name: "align-content:space-between puts all leftover space between wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:space-between"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \n          \n          \nb         \n"},
		{name: "align-content:space-evenly spreads wrapped lines with equal gaps, edges included", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:space-evenly"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \na         \n          \nb         \n          \n"},
		{name: "align-content has no effect on a nowrap container, which still fills its height", width: 10, html: `<div style="display:flex;width:100%;height:5;align-content:center"><div>a</div></div>`, want: "a         \n          \n          \n          \n          \n"},
		{name: "align-content applies to a wrapping container even at a single line", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:4;align-content:flex-end"><div>a</div></div>`, want: "          \n          \n          \na         \n"},
	})
}

// TestFlexRowContainerHeight covers a single-line (nowrap) row container's own
// explicit height: the line takes the container's inner cross size, so the
// container fills that height and align-items/align-self resolve against it
// rather than against the tallest item. Before, an explicit height on a
// single-line row container was ignored outright.
func TestFlexRowContainerHeight(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a single-line row fills its container's explicit height", width: 10, html: `<div style="display:flex;width:100%;height:3"><div>a</div></div>`, want: "a         \n          \n          \n"},
		{name: "align-items:center centers an item in a fixed-height row", width: 10, html: `<div style="display:flex;width:100%;height:5;align-items:center"><div>a</div></div>`, want: "          \n          \na         \n          \n          \n"},
		{name: "align-items:flex-end drops an item to the bottom of a fixed-height row", width: 10, html: `<div style="display:flex;width:100%;height:5;align-items:flex-end"><div>a</div></div>`, want: "          \n          \n          \n          \na         \n"},
		{name: "content taller than the container's height overflows rather than clipping", width: 10, html: `<div style="display:flex;width:100%;height:1"><div>a<br>b</div></div>`, want: "a         \nb         \n"},
		{name: "no explicit height keeps content-driven cross sizing", width: 10, html: `<div style="display:flex;width:100%;align-items:center"><div>a</div></div>`, want: "a         \n"},
	})
}

func TestFlexNesting(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a nested flex row inside a flex column stretches to the column's full width by default", width: 20, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="display:flex;gap:1"><div>a</div><div>b</div></div><div>c</div></div>`, want: "a b                 \nc                   \n"},
		{name: "flex-grow on a nested flex container measures its own natural width, not the outer container's", width: 20, html: `<div style="display:flex;width:100%"><div style="display:flex;flex-direction:column;flex-grow:1"><div>x</div><div>y</div></div><div>z</div></div>`, want: "x                  z\ny                   \n"},
	})
}
