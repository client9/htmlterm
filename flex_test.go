package htmlterm_test

import "testing"

func TestFlexRow(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row lays out items side by side with no grow", width: 20, html: `<div style="display:flex;width:100%"><div>a</div><div>bb</div><div>ccc</div></div>`, want: "abbccc              \n"},
		{name: "flex-grow distributes leftover space by weight", width: 21, html: `<div style="display:flex;width:100%"><div style="flex-grow:1">a</div><div style="flex-grow:2">b</div></div>`, want: "a      b             \n"},
		{name: "flex shorthand N sets equal grow with zero basis", width: 10, html: `<div style="display:flex;width:100%"><div style="flex:1">a</div><div style="flex:1">b</div></div>`, want: "a    b    \n"},
		{name: "flex-basis percent sets starting main size before grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:30%">a</div><div style="flex-grow:1">b</div></div>`, want: "a     b             \n"},
		{name: "width longhand sets item main size when flex-basis is unset", width: 12, html: `<div style="display:flex;width:100%"><div style="width:4">a</div><div style="flex-grow:1">b</div></div>`, want: "a   b       \n"},
		{name: "items shrink by default when they overflow the container", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:8">aaaaaaaa</div><div style="flex-basis:8">bbbbbbbb</div></div>`, want: "aaaaaaaabbbbbbbb\n"},
		{name: "flex-shrink:0 opts an item out of shrinking, forcing overflow", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:4;flex-shrink:0">a</div><div style="flex-basis:4;flex-shrink:0">b</div><div style="flex-basis:4;flex-shrink:0">c</div></div>`, want: "a   b   c   \n"},
		{name: "justify-content space-between spreads items across the row", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-between"><div>a</div><div>b</div><div>c</div></div>`, want: "a         b        c\n"},
		{name: "justify-content center centers items with no grow", width: 10, html: `<div style="display:flex;width:100%;justify-content:center"><div>ab</div></div>`, want: "    ab    \n"},
		{name: "justify-content flex-end pushes items to the end", width: 10, html: `<div style="display:flex;width:100%;justify-content:flex-end"><div>ab</div></div>`, want: "        ab\n"},
		{name: "justify-content space-around distributes half-shares at the edges", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-around"><div>a</div><div>b</div><div>c</div></div>`, want: "   a     b     c    \n"},
		{name: "column-gap inserts space between items", width: 10, html: `<div style="display:flex;width:100%;gap:2"><div>a</div><div>b</div></div>`, want: "a  b      \n"},
		{name: "align-items center centers shorter items on the cross axis", width: 10, html: `<div style="display:flex;width:100%;align-items:center"><div>a<br>b<br>c</div><div>x</div></div>`, want: "a         \nbx        \nc         \n"},
		{name: "align-items flex-end aligns items to the row's bottom", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-end"><div>a<br>b</div><div>x</div></div>`, want: "a         \nbx        \n"},
		{name: "text-only children between flex items are not laid out as items", width: 10, html: `<div style="display:flex;width:100%">loose text<div>a</div></div>`, want: "a         \n"},
		{name: "display:none child is excluded from layout", width: 10, html: `<div style="display:flex;width:100%"><div style="display:none">a</div><div>b</div></div>`, want: "b         \n"},
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

func TestFlexReverseAndOrder(t *testing.T) {
	runCases(t, []renderCase{
		{name: "row-reverse lays items out right to left", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div>a</div><div>b</div><div>c</div></div>`, want: "cba       \n"},
		{name: "column-reverse stacks items bottom to top", width: 5, html: `<div style="display:flex;flex-direction:column-reverse;width:100%"><div>a</div><div>b</div></div>`, want: "b    \na    \n"},
		{name: "order re-sequences items ascending, ties keep document order", width: 10, html: `<div style="display:flex;width:100%"><div style="order:2">a</div><div style="order:1">b</div><div>c</div></div>`, want: "cba       \n"},
		{name: "order is resolved before row-reverse flips the sequence", width: 10, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div style="order:2">a</div><div style="order:1">b</div></div>`, want: "ab        \n"},
	})
}

func TestFlexAlignSelf(t *testing.T) {
	runCases(t, []renderCase{
		{name: "align-self overrides align-items on the cross axis in a row", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-start"><div>a<br>b</div><div style="align-self:flex-end">x</div></div>`, want: "a         \nbx        \n"},
		{name: "align-self overrides align-items on the cross axis in a column", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;align-items:flex-start"><div>a</div><div style="align-self:center">bbb</div></div>`, want: "a         \n   bbb    \n"},
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
		{name: "row-reverse keeps each item's own margin, not mirrored", width: 20, html: `<div style="display:flex;flex-direction:row-reverse;width:100%"><div style="margin-right:2">a</div><div>b</div></div>`, want: "ba                  \n"},
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
		{name: "flex shorthand's <grow> <shrink> <basis> form shrinks items proportionally", width: 10, html: `<div style="display:flex;width:100%"><div style="flex:0 1 8">a</div><div style="flex:0 1 8">b</div></div>`, want: "a    b    \n"},
	})
}

func TestFlexWrap(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-wrap:wrap moves overflowing items to a second line", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a   b     \nc         \n"},
		{name: "gap sets both column-gap and row-gap, including between wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;gap:1"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a    b    \n          \nc         \n"},
		{name: "nowrap (default) keeps items on one line, shrinking to fit rather than wrapping", width: 10, html: `<div style="display:flex;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "a  b  c   \n"},
		{name: "an item too wide for the container still gets its own line rather than being dropped", width: 6, html: `<div style="display:flex;flex-wrap:wrap;width:100%"><div style="flex-basis:8">aaaaaaaa</div><div style="flex-basis:4">b</div></div>`, want: "aaaaaaaa\nb     \n"},
	})
}

func TestFlexAlignContent(t *testing.T) {
	runCases(t, []renderCase{
		{name: "align-content:flex-end pushes wrapped lines to the container's bottom", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:flex-end"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \n          \n          \na         \nb         \n"},
		{name: "align-content:center splits leftover space evenly above and below wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:center"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \na         \nb         \n          \n          \n"},
		{name: "align-content:space-between puts all leftover space between wrapped lines", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:space-between"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \n          \n          \nb         \n"},
		{name: "align-content has no effect without flex-wrap producing multiple lines", width: 10, html: `<div style="display:flex;width:100%;height:5;align-content:center"><div>a</div></div>`, want: "a         \n"},
	})
}

func TestFlexNesting(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a nested flex row inside a flex column stretches to the column's full width by default", width: 20, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="display:flex;gap:1"><div>a</div><div>b</div></div><div>c</div></div>`, want: "a b                 \nc                   \n"},
		{name: "flex-grow on a nested flex container measures its own natural width, not the outer container's", width: 20, html: `<div style="display:flex;width:100%"><div style="display:flex;flex-direction:column;flex-grow:1"><div>x</div><div>y</div></div><div>z</div></div>`, want: "x                  z\ny                   \n"},
	})
}
