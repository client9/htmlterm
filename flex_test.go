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
		// An inline-flex container is shrink-to-fit, so there's no free space
		// for justify-content or flex-grow to spread items across; both used to
		// treat the surrounding wrap bound as the container's own size and push
		// the items out to it.
		{name: "inline-flex is shrink-to-fit, leaving justify-content nothing to spread", width: 30, html: `<span style="display:inline-flex;justify-content:space-between"><span style="border-style:solid">a</span><span style="border-style:solid">b</span></span>`, want: "┌─┐┌─┐\n│a││b│\n└─┘└─┘"},
		{name: "flex-grow has no free space in a shrink-to-fit container", width: 30, html: `<span style="display:inline-flex"><span style="flex-grow:1;border-style:solid">a</span><span style="flex-grow:1;border-style:solid">b</span></span>`, want: "┌─┐┌─┐\n│a││b│\n└─┘└─┘"},
		{name: "an explicit width makes the main size definite again", width: 20, html: `<span style="display:inline-flex;width:10;justify-content:space-between"><span>a</span><span>b</span></span>z`, want: "a        bz"},
		{name: "items wider than the wrap bound still shrink to it", width: 10, html: `<span style="display:inline-flex"><span style="flex-basis:20">a</span><span style="flex-basis:20">b</span></span>`, want: "a    b    "},
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

// TestFlexPlaceShorthands covers the CSS Box Alignment place-* shorthands.
// Two-value forms are align-then-justify, the opposite order from most
// two-value shorthands.
func TestFlexPlaceShorthands(t *testing.T) {
	runCases(t, []renderCase{
		{name: "place-content one value sets justify-content too", width: 10, html: `<div style="display:flex;width:100%;place-content:center"><div>ab</div></div>`, want: "    ab    \n"},
		{name: "place-content two values are align-content then justify-content", width: 10, html: `<div style="display:flex;width:100%;place-content:flex-start flex-end"><div>ab</div></div>`, want: "        ab\n"},
		{name: "place-items sets align-items", width: 10, html: `<div style="display:flex;width:100%;place-items:center"><div>a<br>b<br>c</div><div>x</div></div>`, want: "a         \nbx        \nc         \n"},
		{name: "place-self sets align-self on one item", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-start"><div>a<br>b</div><div style="place-self:flex-end">x</div></div>`, want: "a         \nbx        \n"},
		{name: "place-content drives both axes of a wrapping container", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:8;place-content:flex-end"><div style="width:6">a</div><div style="width:6">b</div></div>`, want: "          \n          \n          \n          \n          \n          \n    a     \n    b     \n"},
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
// TestFlexFactorsBelowOne pins CSS Flexbox 1 section 9.7 step 4b: when the
// flex factors sum to less than one, only that fraction of the free space is
// distributed and the rest goes to justify-content. The shares used to be
// normalized by the total weight, so any positive factor consumed all of it.
// TestFlexShorthandBasisOnly pins the one-value `flex: <basis>` form. The
// shorthand's omitted-value defaults are 1/1, so such an item grows; it used
// to expand to flex-basis alone, leaving flex-grow at the longhand's own
// initial 0 and the item stuck at its basis.
// TestFlexAlignItemsStretchGrowsTheBox pins that align-items: stretch grows a
// short item's own box to the line's cross size rather than parking blank rows
// under it — the difference is invisible for plain text but not for anything
// that paints a rectangle, where the border used to close at the content
// height and leave the stretch showing as a gap below the box.
func TestFlexAlignItemsStretchGrowsTheBox(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a bordered item stretches to the tallest item on the line", width: 8, html: `<div style="display:flex;width:100%"><div style="border-style:solid">a</div><div style="border-style:solid">b<br>c<br>d</div></div>`, want: "┌─┐┌─┐  \n│a││b│  \n│ ││c│  \n│ ││d│  \n└─┘└─┘  \n"},
		{name: "a single-line container stretches items to its own height", width: 6, html: `<div style="display:flex;width:100%;height:5"><div style="border-style:solid">a</div></div>`, want: "┌─┐   \n│a│   \n│ │   \n│ │   \n└─┘   \n"},
		{name: "an explicit height opts an item out of stretching", width: 8, html: `<div style="display:flex;width:100%"><div style="border-style:solid;height:1">a</div><div style="border-style:solid">b<br>c<br>d</div></div>`, want: "┌─┐┌─┐  \n│a││b│  \n└─┘│c│  \n   │d│  \n   └─┘  \n"},
		{name: "align-items flex-start leaves a short item at its own height", width: 8, html: `<div style="display:flex;width:100%;align-items:flex-start"><div style="border-style:solid">a</div><div style="border-style:solid">b<br>c</div></div>`, want: "┌─┐┌─┐  \n│a││b│  \n└─┘│c│  \n   └─┘  \n"},
		{name: "align-self stretch overrides a centering container", width: 8, html: `<div style="display:flex;width:100%;align-items:center"><div style="border-style:solid;align-self:stretch">a</div><div>b<br>c<br>d</div></div>`, want: "┌─┐b    \n│a│c    \n└─┘d    \n"},
		{name: "plain text items are unaffected", width: 10, html: `<div style="display:flex;width:100%"><div>a<br>b</div><div>x</div></div>`, want: "ax        \nb         \n"},
		// Stretching sizes the item "as close to the line's cross size as
		// possible, while still respecting the constraints imposed by
		// min-height/max-height" (§8.3). The synthetic height flex injects used
		// to override max-height outright (block.go's height wins over
		// max-height, by design), so a capped item grew the whole line's height.
		{name: "max-height caps how far an item stretches", width: 20, html: `<div style="display:flex;width:100%"><div style="max-height:2;border-style:solid">a</div><div>x<br>y<br>z<br>w<br>v</div></div>`, want: "┌─┐x                \n│a│y                \n│ │z                \n└─┘w                \n   v                \n"},
		{name: "a max-height above the line's own height doesn't bite", width: 20, html: `<div style="display:flex;width:100%"><div style="max-height:9;border-style:solid">a</div><div>x<br>y<br>z<br>w<br>v</div></div>`, want: "┌─┐x                \n│a│y                \n│ │z                \n│ │w                \n└─┘v                \n"},
	})
}

// TestFlexBasisContent covers the `content` keyword, which sizes an item from
// its content while ignoring its own main-size property — the one thing that
// distinguishes it from `auto`, which consults width/height first. It used to
// fall through as an unrecognized value, landing on `auto`'s behavior by
// accident.
func TestFlexBasisContent(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row direction sizes from content, ignoring the item's width", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:content;width:2;border-style:solid">abcdefgh</div><div>r</div></div>`, want: "┌────────┐r         \n│abcdefgh│          \n└────────┘          \n"},
		{name: "flex-basis:auto still defers to that width", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:auto;width:4;border-style:solid">abcdefgh</div><div>r</div></div>`, want: "┌──┐r               \n│ab│                \n└──┘                \n"},
		{name: "column direction sizes from content, ignoring the item's height", width: 16, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:content;height:4;border-style:solid">a</div><div>z</div></div>`, want: "┌──────────────┐\n│a             │\n└──────────────┘\nz               \n"},
		{name: "without flex-basis that height still applies", width: 16, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="height:4;border-style:solid">a</div><div>z</div></div>`, want: "┌──────────────┐\n│a             │\n│              │\n│              │\n│              │\n└──────────────┘\nz               \n"},
	})
}

// TestFlexCollapsedItem pins Flexbox §4.4's collapsed flex item: on a flex
// item, visibility: collapse behaves as display: none, not as the
// visibility: hidden it means on any other element. The value used to do
// nothing at all anywhere — a collapsed item rendered in full.
func TestFlexCollapsedItem(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a collapsed flex item is dropped from layout", width: 20, html: `<div style="display:flex;width:100%"><div style="visibility:collapse">aaa</div><div>b</div></div>`, want: "b                   \n"},
		{name: "visibility:hidden still reserves the item's space", width: 20, html: `<div style="display:flex;width:100%"><div style="visibility:hidden">aaa</div><div>b</div></div>`, want: "   b                \n"},
		// A collapsed item is gone before flex-grow runs, so what it would have
		// taken goes to its siblings rather than sitting blank.
		{name: "a collapsed item's share goes to the items that remain", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;visibility:collapse">a</div><div style="flex:1;border-style:solid">b</div></div>`, want: "┌──────────────────┐\n│b                 │\n└──────────────────┘\n"},
		// Off a flex item the same declaration is plain `hidden`, per the
		// spec's "on other elements it computes to hidden".
		{name: "outside a flex container collapse behaves as hidden", width: 20, html: `<div>plain <span style="visibility:collapse">xxx</span> tail</div>`, want: "plain    tail\n"},
	})
}

// TestFlexZeroMaxSize pins that a declared maximum of zero is a real bound, not
// an absent one — the other end of the same distinction `min-width: 0` needs
// (see TestFlexZeroBasis). It used to read as "uncapped", so an item told not
// to grow at all absorbed the whole line instead. Everything still floors at
// one cell, since no box here renders in zero columns or rows.
func TestFlexZeroMaxSize(t *testing.T) {
	runCases(t, []renderCase{
		{name: "max-width:0 stops an item growing and hands the space to its sibling", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;max-width:0">aaa</div><div style="flex:1">b</div></div>`, want: "ab                  \n"},
		{name: "max-width:0 clamps a declared flex-basis too", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:8;max-width:0">aaa</div><div>b</div></div>`, want: "ab                  \n"},
		{name: "max-height:0 stops a column item growing", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:4"><div style="flex-grow:1;max-height:0">a</div><div style="flex-grow:1">b</div></div>`, want: "a         \nb         \n          \n          \n"},
	})
}

// TestFlexBasisIntrinsicKeywords covers flex-basis's intrinsic sizing
// keywords. All three size the item from its content while ignoring its own
// width, the way `content` does, and differ only in which measurement they
// ask for: the widest unbreakable run, the never-wrapped width, or the width
// the content actually takes within what's available. The container is
// flex-shrink: 0 throughout so the resolved basis is what gets painted.
func TestFlexBasisIntrinsicKeywords(t *testing.T) {
	runCases(t, []renderCase{
		{name: "min-content is the widest unbreakable run", width: 12, html: `<div style="display:flex;width:100%"><div style="flex-basis:min-content;flex-shrink:0;border-left:'[';border-right:']'">hello big world</div></div>`, want: "[hello]     \n[big  ]     \n[world]     \n"},
		// Wider than the container, which is exactly what max-content means:
		// the width it would take if never forced to wrap.
		{name: "max-content never wraps", width: 12, html: `<div style="display:flex;width:100%"><div style="flex-basis:max-content;flex-shrink:0;border-left:'[';border-right:']'">hello big world</div></div>`, want: "[hello big world]\n"},
		{name: "fit-content wraps to what is available", width: 12, html: `<div style="display:flex;width:100%"><div style="flex-basis:fit-content;flex-shrink:0;border-left:'[';border-right:']'">hello big world</div></div>`, want: "[hello big] \n[world    ] \n"},
		// The item's own width is ignored, the whole point of an intrinsic
		// keyword (and what separates all three from `auto`).
		{name: "an intrinsic basis ignores the item's own width", width: 12, html: `<div style="display:flex;width:100%"><div style="flex-basis:min-content;width:11;flex-shrink:0;border-left:'[';border-right:']'">hello big</div></div>`, want: "[hello]     \n[big  ]     \n"},
		{name: "the flex shorthand accepts them as a basis", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:min-content;border-left:'[';border-right:']'">hello big</div></div>`, want: "[hello big         ]\n"},
	})
}

// TestFlexColumnOuterHeight pins that a column-direction item's resolved main
// size is its whole outer box — border rows included — matching the outer-box
// convention flex-basis/width already follow on the main axis. A bordered item
// used to take its own chrome on top of the size flex layout allotted it,
// overflowing the container by exactly its border rows.
func TestFlexColumnOuterHeight(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-basis counts a column item's border rows", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:4;border-style:solid">a</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n│        │\n└────────┘\nz         \n"},
		{name: "flex-grow fills a column container's height exactly", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:6"><div style="flex-grow:1;border-style:solid">a</div></div>`, want: "┌────────┐\n│a       │\n│        │\n│        │\n│        │\n└────────┘\n"},
		{name: "two grown bordered items split the height without overflow", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:8"><div style="flex-grow:1;border-style:solid">a</div><div style="flex-grow:1;border-style:solid">b</div></div>`, want: "┌────────┐\n│a       │\n│        │\n└────────┘\n┌────────┐\n│b       │\n│        │\n└────────┘\n"},
	})
}

// TestFlexColumnBasisClampedByMinMaxHeight pins that a column-direction flex
// base size is clamped by the item's own min-height/max-height before it is
// rendered — the vertical mirror of resolveMainBasis's min-width/max-width
// clamp, and what CSS.md's flex-basis entry promises for both axes. The
// maximum used to be dropped on the floor: flex layout injects its resolved
// main size as a synthetic `height`, which block.go deliberately ranks above
// max-height, so `flex-basis: 6; max-height: 1` rendered six rows tall.
//
// Both bounds are content-box here while flex sizes are outer, so each is
// compared with the item's own border/padding rows added back on — a
// max-height:1 bordered item is three rows, not one.
func TestFlexColumnBasisClampedByMinMaxHeight(t *testing.T) {
	runCases(t, []renderCase{
		{name: "max-height clamps a taller flex-basis", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:6;max-height:1;border-style:solid">a</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n└────────┘\nz         \n"},
		{name: "min-height raises a shorter flex-basis", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:1;min-height:3;border-style:solid">a</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n│        │\n│        │\n└────────┘\nz         \n"},
		// The minimum applies after the maximum, so a minimum larger than a
		// maximum wins — CSS's own min/max resolution order.
		{name: "a min-height larger than max-height wins", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="flex-basis:1;min-height:3;max-height:2;border-style:solid">a</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n│        │\n│        │\n└────────┘\nz         \n"},
		// A content-sized item is deliberately left alone by the maximum:
		// block.go only clips to max-height under an explicit overflow-y, and
		// reserving fewer rows than the item paints would desynchronize the
		// stack from the boxes in it.
		{name: "max-height does not shorten a content-sized column item", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="max-height:1;border-style:solid">a<br>b</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n│b       │\n└────────┘\nz         \n"},
		{name: "an explicit overflow-y is what makes it clip", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="max-height:1;overflow-y:hidden;border-style:solid">a<br>b</div><div>z</div></div>`, want: "┌────────┐\n│a       │\n└────────┘\nz         \n"},
	})
}

func TestFlexShorthandBasisOnly(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex:<basis> grows the item to fill the line", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:30%;border-style:solid">a</div></div>`, want: "┌──────────────────┐\n│a                 │\n└──────────────────┘\n"},
		{name: "an invalid flex value grants no grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:NaN;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
	})
}

func TestFlexFactorsBelowOne(t *testing.T) {
	runCases(t, []renderCase{
		// Bordered, so each item's resolved main size is visible: basis 10 plus
		// 0.5 of the 10 free columns is 15, not the whole 20.
		{name: "flex-grow:0.5 takes half the free space, not all of it", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:10;flex-grow:0.5;border-style:solid">a</div></div>`, want: "┌─────────────┐     \n│a            │     \n└─────────────┘     \n"},
		{name: "the fraction is the sum across every growable item", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:5;flex-grow:0.25;border-style:solid">a</div><div style="flex-basis:5;flex-grow:0.25;border-style:solid">b</div></div>`, want: "┌──────┐┌─────┐     \n│a     ││b    │     \n└──────┘└─────┘     \n"},
		{name: "factors summing to one or more still absorb the whole leftover", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:5;flex-grow:0.5">a</div><div style="flex-basis:5;flex-grow:0.5">b</div></div>`, want: "a         b         \n"},
		{name: "space the sub-one factors leave behind flows to justify-content", width: 20, html: `<div style="display:flex;width:100%;justify-content:flex-end"><div style="flex-basis:10;flex-grow:0.5">a</div></div>`, want: "     a              \n"},
		{name: "flex-shrink below one absorbs only that fraction of the overflow", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:10;flex-shrink:0.25;border-style:solid">a</div><div style="flex-basis:10;flex-shrink:0.25;border-style:solid">b</div></div>`, want: "┌─────┐┌──────┐\n│a    ││b     │\n└─────┘└──────┘\n"},
	})
}

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

// TestFlexResolvedMainSizeWins pins that an item's *used* main size is the one
// flex layout resolved (basis, adjusted by flex-grow/flex-shrink), not the
// width the item declares for itself. The resolved size used to be passed only
// as available width, so renderBlockContentBox re-resolved the declared width
// and won: a grown item painted at its declared width with the growth left as
// trailing blanks, and a shrunk one painted full width and overflowed the
// container. A border makes the item's real painted extent visible.
func TestFlexResolvedMainSizeWins(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-grow widens an item past its own declared width", width: 12, html: `<div style="display:flex;width:100%"><div style="width:5;flex-grow:1;border-style:solid">a</div><div style="width:5;flex-grow:1;border-style:solid">b</div></div>`, want: "┌────┐┌────┐\n│a   ││b   │\n└────┘└────┘\n"},
		{name: "flex-shrink narrows an item below its own declared width", width: 10, html: `<div style="display:flex;width:100%"><div style="width:10;border-style:solid">a</div><div style="width:10;border-style:solid">b</div></div>`, want: "┌───┐┌───┐\n│a  ││b  │\n└───┘└───┘\n"},
		{name: "an unbordered grown item fills its resolved width too", width: 12, html: `<div style="display:flex;width:100%"><div style="width:5;flex-grow:1">a</div><div style="width:5;flex-grow:1">b</div></div>`, want: "a     b     \n"},
		{name: "min-width still floors the shrunk size", width: 10, html: `<div style="display:flex;width:100%"><div style="width:10;min-width:8;border-style:solid">a</div><div style="width:10;border-style:solid">b</div></div>`, want: "┌──────┐┌─┐\n│a     ││b│\n└──────┘└─┘\n"},
	})
}

// TestFlexNaturalBasisIsContentWidth pins that an item with no flex-basis and
// no width measures its own content, not the width it was offered. The
// measurement render used to go through the ordinary block box model, which
// fills its available width whenever it has to paint a rectangle (a border,
// text-align, a closed box) — so every such item reported the whole container
// as its basis, and flex-shrink's content-proportional split flattened to an
// even one.
func TestFlexNaturalBasisIsContentWidth(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a bordered item's basis is its content, not the container", width: 20, html: `<div style="display:flex;width:100%"><div style="border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		{name: "text-align doesn't inflate an item's basis", width: 20, html: `<div style="display:flex;width:100%"><div style="text-align:center">a</div><div>rest</div></div>`, want: "arest               \n"},
		{name: "a bordered child shrinks with the item being measured", width: 20, html: `<div style="display:flex;width:100%"><div><div style="border-style:solid">a</div></div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		{name: "a nested flex item measures its own items", width: 20, html: `<div style="display:flex;width:100%"><div style="display:flex;border-style:solid"><div>a</div></div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		// A nested container's basis must include its *own* border and padding,
		// not just the items inside it. Measuring the bare layout pass instead
		// under-allotted such an item by its chrome at every level of nesting,
		// which compounds: the innermost items here had 4 columns taken off them
		// by the two wrappers, enough to wrap "aa aa" onto two lines.
		{name: "a nested flex container's basis includes its own border and padding", width: 20, html: `<div style="display:flex;width:100%"><div style="display:flex;border-style:solid;padding-left:1"><div style="display:flex;border-style:solid"><div>aa aa</div></div></div><div>r</div></div>`, want: "┌────────┐r         \n│ ┌─────┐│          \n│ │aa aa││          \n│ └─────┘│          \n└────────┘          \n"},
		// Bases of 10 and 7 against a 5-column deficit: the scaled shrink factor
		// takes 3 from the first and 2 from the second, not 2.5 apiece. Breakable
		// content, so both stay above their own min-content floors and the split
		// is what's actually on display.
		{name: "flex-shrink splits the deficit by content width, not evenly", width: 12, html: `<div style="display:flex;width:100%"><div style="border-style:solid">aa aa aa</div><div style="border-style:solid">bb bb</div></div>`, want: "┌─────┐┌───┐\n│aa aa││bb │\n│aa   ││bb │\n└─────┘└───┘\n"},
		{name: "flex-wrap breaks lines on content widths", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="border-style:solid">aa</div><div style="border-style:solid">bb</div><div style="border-style:solid">cc</div><div style="border-style:solid">dd</div></div>`, want: "┌──┐┌──┐┌──┐\n│aa││bb││cc│\n└──┘└──┘└──┘\n┌──┐        \n│dd│        \n└──┘        \n"},
		// The shrink-to-fit narrowing is scoped to the discarded measurement
		// render; an ordinary block still fills its containing block.
		{name: "a block outside flex layout still fills its available width", width: 20, html: `<div style="border-style:solid">a</div>`, want: "┌──────────────────┐\n│a                 │\n└──────────────────┘\n"},
	})
}

func TestFlexWrap(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-wrap:wrap moves overflowing items to a second line", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a   b     \nc         \n"},
		// wrap-reverse's reversed cross-axis line order isn't implemented, but
		// it still wraps: falling back to nowrap made a container that should
		// have wrapped overflow its width on one line instead.
		{name: "flex-wrap:wrap-reverse wraps, without reversing the line order", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a   b     \nc         \n"},
		{name: "flex-flow's wrap-reverse component wraps too", width: 10, html: `<div style="display:flex;flex-flow:row wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a   b     \nc         \n"},
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

// TestFlexZeroBasis pins that a declared flex-basis of 0 is a *definite* base
// size of nothing, not an absent one. parseSizeVal reports only n > 0 as a
// length (everywhere else in this engine a zero size and an absent one mean the
// same thing), so a declared 0 used to fall through to width and then to the
// item's natural content width — making `flex: 1`, whose shorthand expansion is
// `1 1 0`, lay out as `1 1 auto`: items with different content got
// content-proportional widths instead of equal ones. See parseFlexSizeVal.
//
// The zero has to survive all the way into the distribution, too: because an
// item's *hypothetical* main size raises that zero to the item's automatic
// minimum size, growing from the hypothetical rather than from the flex base
// size leaks content widths back into the split by a smaller amount. See
// growFlexLine, and TestFlexAutomaticMinimumSize for where the minimum does
// legitimately bite.
func TestFlexZeroBasis(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex:1 splits the line equally when content permits", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">bb</div></div>`, want: "┌────────┐┌────────┐\n│a       ││bb      │\n└────────┘└────────┘\n"},
		// A true zero base means unequal factors split in exactly their ratio,
		// not in a ratio skewed by the one cell each item would otherwise
		// reserve first: 40 columns at 1:2 is 13/27, not 14/26.
		{name: "unequal flex factors split in exactly their ratio", width: 40, html: `<div style="display:flex;width:100%"><div style="flex:1;border-style:solid">aaaa</div><div style="flex:2;border-style:solid">bbbb</div></div>`, want: "┌───────────┐┌─────────────────────────┐\n│aaaa       ││bbbb                     │\n└───────────┘└─────────────────────────┘\n"},
		{name: "a column container splits its height in the same ratio", width: 8, html: `<div style="display:flex;flex-direction:column;width:100%;height:9"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">b</div></div>`, want: "┌──────┐\n│a     │\n│      │\n│      │\n└──────┘\n┌──────┐\n│b     │\n│      │\n└──────┘\n"},
		// A zero basis really is zero: both items start from nothing and split
		// the whole line, so the first one's 9 columns of content wrap inside
		// its half rather than buying it a wider share. Reading the basis as
		// auto instead gives 14/6; growing from the *hypothetical* main size
		// (the basis already raised to min-content) gives 12/8. See
		// growFlexLine.
		{name: "flex-basis:0 splits the line equally, whatever the content", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:0;flex-grow:1;border-style:solid">aaaa aaaa</div><div style="flex-basis:0;flex-grow:1;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│aaaa    ││b       │\n│aaaa    ││        │\n└────────┘└────────┘\n"},
		{name: "flex-basis:0% is a zero length too, not an unset one", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:0%;flex-grow:1;border-style:solid">aaaa aaaa</div><div style="flex-basis:0%;flex-grow:1;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│aaaa    ││b       │\n│aaaa    ││        │\n└────────┘└────────┘\n"},
		// Unbreakable content doesn't tilt the split either, as long as every
		// item's equal share clears its own automatic minimum size (8 columns
		// for `bbbbbb` plus its borders, against the 10 it gets).
		{name: "flex:1 stays equal against unbreakable content that fits its share", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">bbbbbb</div></div>`, want: "┌────────┐┌────────┐\n│a       ││bbbbbb  │\n└────────┘└────────┘\n"},
		{name: "min-width:0 changes nothing when no automatic minimum is violated", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;min-width:0;border-style:solid">a</div><div style="flex:1;min-width:0;border-style:solid">bbbbbb</div></div>`, want: "┌────────┐┌────────┐\n│a       ││bbbbbb  │\n└────────┘└────────┘\n"},
		{name: "flex-basis:auto still falls back to natural content width", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:auto;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		{name: "a column item's flex-basis:0 grows from one line, not from its content height", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;height:6"><div style="flex-basis:0;flex-grow:1;border-style:solid">a<br>b</div></div>`, want: "┌──────────┐\n│a         │\n│b         │\n│          │\n│          │\n└──────────┘\n"},
		// The same equal split as the row cases above, on the cross axis: the
		// taller item's own content is its column-direction stand-in for the
		// automatic minimum size, and growing from it rather than from the zero
		// base gives 4/6 instead of 5/5.
		{name: "column flex:1 splits the height equally, whatever the content", width: 14, html: `<div style="display:flex;flex-direction:column;width:100%;height:10"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">b<br>b<br>b</div></div>`, want: "┌────────────┐\n│a           │\n│            │\n│            │\n└────────────┘\n┌────────────┐\n│b           │\n│b           │\n│b           │\n└────────────┘\n"},
	})
}

// TestFlexAutomaticMinimumSize covers CSS's automatic minimum size for flex
// items (Flexbox §4.5 / Sizing 3 §5.1): min-width's initial value is `auto`,
// which on a flex item's main axis resolves to the content-based minimum, so
// flex-shrink can't take an item below the width its content needs. This engine
// used to floor every item at one column instead — the effect of an
// unconditional `min-width: 0` — and an over-shrunk item's content then painted
// straight through its own border and displaced the rest of the line.
func TestFlexAutomaticMinimumSize(t *testing.T) {
	runCases(t, []renderCase{
		// Both items are at their min-content width, so neither shrinks and the
		// line overflows the container — exactly what a browser does here.
		{name: "an item does not shrink below its min-content width", width: 24, html: `<div style="display:flex;width:100%"><div style="border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bbbbbbbbbbbb</div></div>`, want: "┌────────────┐┌────────────┐\n│aaaaaaaaaaaa││bbbbbbbbbbbb│\n└────────────┘└────────────┘\n"},
		{name: "breakable content shrinks freely down to its longest word", width: 16, html: `<div style="display:flex;width:100%"><div style="border-style:solid">aa aa aa aa</div><div style="border-style:solid">bb</div></div>`, want: "┌──────────┐┌──┐\n│aa aa aa  ││bb│\n│aa        ││  │\n└──────────┘└──┘\n"},
		// The second item is frozen at its own min-content, so the whole
		// 8-column deficit falls on the first, which stops at the explicit
		// min-width the automatic minimum would otherwise have raised to 14.
		{name: "an explicit min-width still wins over the automatic one", width: 20, html: `<div style="display:flex;width:100%"><div style="min-width:6;border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bbbbbbbbbbbb</div></div>`, want: "┌────┐┌────────────┐\n│aaaa││bbbbbbbbbbbb│\n└────┘└────────────┘\n"},
		// The two spec-sanctioned opt-outs, which are why `min-width: 0` and
		// `overflow: hidden` are both standard fixes for an item that won't
		// shrink: the automatic minimum applies only to an item whose main-axis
		// overflow is visible, and an explicit min-width replaces it outright.
		{name: "min-width:0 opts out and lets the item shrink to its allotment", width: 24, html: `<div style="display:flex;width:100%"><div style="min-width:0;border-style:solid">aaaaaaaaaaaa</div><div style="min-width:0;border-style:solid">bbbbbbbbbbbb</div></div>`, want: "┌──────────┐┌──────────┐\n│aaaaaaaaaa││bbbbbbbbbb│\n└──────────┘└──────────┘\n"},
		{name: "a non-visible overflow-x opts out the same way", width: 24, html: `<div style="display:flex;width:100%"><div style="overflow:hidden;border-style:solid">aaaaaaaaaaaa</div><div style="overflow:hidden;border-style:solid">bbbbbbbbbbbb</div></div>`, want: "┌──────────┐┌──────────┐\n│aaaaaaaaaa││bbbbbbbbbb│\n└──────────┘└──────────┘\n"},
		// A definite width is the spec's "specified size suggestion", which
		// clamps the content-based minimum — so a width narrower than the
		// content still wins, and the content is what gives way.
		{name: "a declared width narrower than min-content clamps the floor", width: 24, html: `<div style="display:flex;width:100%"><div style="width:6;flex-shrink:0;border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────┐┌──┐              \n│aaaa││bb│              \n└────┘└──┘              \n"},
		// Flex base sizes, not just shrinking, are floored: the hypothetical main
		// size is the base size clamped by the used min/max main size (§9.2), so
		// a declared flex-basis below min-content is raised before line-breaking
		// and grow/shrink ever see it.
		{name: "a flex-basis below min-content is raised to it", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:4;border-style:solid">aaaaaaaa</div><div>rest</div></div>`, want: "┌────────┐rest      \n│aaaaaaaa│          \n└────────┘          \n"},
		// ...but it is raised as a §9.7 step-4c *violation*, not as a head start
		// on the distribution: the second item's equal 15-column share is below
		// its 18-column automatic minimum, so it freezes at 18 and the whole
		// distribution reruns over what's left, handing the first item the
		// remaining 12 — it does not get to keep the 15 the violated round
		// offered it. Growing from each item's hypothetical main size instead
		// gives 8/22, sized by content rather than by the equal flex factors.
		{name: "an item that violates its automatic minimum freezes and the rest is redivided", width: 30, html: `<div style="display:flex;width:100%"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">bbbbbbbbbbbbbbbb</div></div>`, want: "┌──────────┐┌────────────────┐\n│a         ││bbbbbbbbbbbbbbbb│\n└──────────┘└────────────────┘\n"},
		// Narrow enough that the hypothetical main sizes already overflow, so
		// §9.7 step 1 picks the shrink factor — and every item is frozen at its
		// own automatic minimum, since a zero flex base size makes every scaled
		// shrink factor zero. The line overflows the container by a column,
		// which is what a browser does here too.
		{name: "automatic minimums that cannot be shrunk overflow the line", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1;border-style:solid">a</div><div style="flex:1;border-style:solid">bbbbbbbbbbbbbbbb</div></div>`, want: "┌─┐┌────────────────┐\n│a││bbbbbbbbbbbbbbbb│\n└─┘└────────────────┘\n"},
	})
}

// TestFlexItemClippedToAllotment covers the backstop for the cases the
// automatic minimum above can't prevent — a definite width narrower than
// min-content, a min-width:0 opt-out — where CSS itself overflows. A browser
// paints the overflow over the item's siblings; this engine assembles a line by
// concatenation, so an item wider than its allotment would instead *displace*
// every later item on the line into columns that belong to somebody else. The
// item is clipped to its allotment instead, which is a documented deviation
// (see COMPATIBILITY.md).
func TestFlexItemClippedToAllotment(t *testing.T) {
	runCases(t, []renderCase{
		// The clip goes through the item's own box model, so its border survives
		// and the following item still starts exactly where flex put it.
		{name: "an over-allotment item is clipped without losing its border", width: 20, html: `<div style="display:flex;width:100%"><div style="width:6;flex-shrink:0;border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────┐┌──┐          \n│aaaa││bb│          \n└────┘└──┘          \n"},
		// CSS paints no truncation marker for overflow:visible, so text-overflow
		// is honored here purely as an opt-in for a visible cut.
		{name: "text-overflow supplies a marker for the clip when set", width: 20, html: `<div style="display:flex;width:100%"><div style="width:6;flex-shrink:0;text-overflow:ellipsis;border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────┐┌──┐          \n│aaa…││bb│          \n└────┘└──┘          \n"},
		{name: "an item's own overflow:hidden is left to do its own clipping", width: 20, html: `<div style="display:flex;width:100%"><div style="width:6;flex-shrink:0;overflow:hidden;border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────┐┌──┐          \n│aaaa││bb│          \n└────┘└──┘          \n"},
	})
}

// TestFlexContainerOverflowX pins that a flex container honors overflow-x
// hidden/clip on its own content, the way every other box model in this engine
// does (CSS.md's overflow-x entry carves out no flex exception).
// renderFlexContentBox used to ignore the property entirely.
func TestFlexContainerOverflowX(t *testing.T) {
	runCases(t, []renderCase{
		{name: "overflow:hidden truncates a line that overflows the container", width: 20, html: `<div style="display:flex;width:14;overflow:hidden;border-style:solid"><div style="border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────────────┐\n│┌───────────│\n││aaaaaaaaaaa│\n│└───────────│\n└────────────┘\n"},
		{name: "without overflow the same line spills past the container's border", width: 20, html: `<div style="display:flex;width:14;border-style:solid"><div style="border-style:solid">aaaaaaaaaaaa</div><div style="border-style:solid">bb</div></div>`, want: "┌────────────┐\n│┌────────────┐┌──┐│\n││aaaaaaaaaaaa││bb││\n│└────────────┘└──┘│\n└────────────┘\n"},
	})
}

// TestFlexNestedContainerMainSize pins that the main size flex layout resolves
// for an item that is *itself* a flex container reaches that container's own
// layout, rather than being padded on from outside. The height override used to
// be withheld from flex-display items and applied as trailing blank lines after
// renderFlexContentBox had already drawn the bottom border — so the grown rows
// landed below the box instead of inside it, and the nested container's own
// children never saw the height their parent had allotted them.
func TestFlexNestedContainerMainSize(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a grown nested column container's border encloses its allotted height", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;height:6"><div style="display:flex;flex-direction:column;flex-grow:1;border-style:solid"><div>a</div></div></div>`, want: "┌──────────┐\n│a         │\n│          │\n│          │\n│          │\n└──────────┘\n"},
		{name: "the allotted height reaches the nested container's own flex-grow child", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;height:8"><div style="display:flex;flex-direction:column;flex-grow:1;border-style:solid"><div>a</div><div style="flex-grow:1;border-style:solid">b</div></div></div>`, want: "┌──────────┐\n│a         │\n│┌────────┐│\n││b       ││\n││        ││\n││        ││\n│└────────┘│\n└──────────┘\n"},
		{name: "flex-basis on a nested flex container sizes the container itself", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="display:flex;flex-basis:4;border-style:solid"><div>a</div></div><div>z</div></div>`, want: "┌──────────┐\n│a         │\n│          │\n└──────────┘\nz           \n"},
		{name: "align-items:stretch grows a nested row container's own box", width: 12, html: `<div style="display:flex;width:100%"><div style="display:flex;border-style:solid"><div>a</div></div><div>w<br>x<br>y<br>z</div></div>`, want: "┌─┐w        \n│a│x        \n│ │y        \n└─┘z        \n"},
	})
}
