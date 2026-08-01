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
		// the exact slot sizes are 2.83 at each edge and 5.67 between each pair.
		// Largest-remainder rounding gives 3, 6, 5, 3, which keeps the two edges
		// equal and puts the one column that can't be split evenly in a gap.
		{name: "justify-content space-around distributes half-shares at the edges", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-around"><div>a</div><div>b</div><div>c</div></div>`, want: "   a      b     c   \n"},
		{name: "justify-content space-evenly makes every gap, edges included, equal", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-evenly"><div>a</div><div>b</div><div>c</div></div>`, want: "     a    b    c    \n"},
		{name: "justify-content start is a synonym for flex-start", width: 10, html: `<div style="display:flex;width:100%;justify-content:start"><div>ab</div></div>`, want: "ab        \n"},
		{name: "justify-content end is a synonym for flex-end", width: 10, html: `<div style="display:flex;width:100%;justify-content:end"><div>ab</div></div>`, want: "        ab\n"},
		{name: "justify-content normal behaves as the unset default", width: 10, html: `<div style="display:flex;width:100%;justify-content:normal"><div>ab</div></div>`, want: "ab        \n"},
		{name: "column-gap inserts space between items", width: 10, html: `<div style="display:flex;width:100%;gap:2"><div>a</div><div>b</div></div>`, want: "a  b      \n"},
		{name: "align-items center centers shorter items on the cross axis", width: 10, html: `<div style="display:flex;width:100%;align-items:center"><div>a<br>b<br>c</div><div>x</div></div>`, want: "a         \nbx        \nc         \n"},
		{name: "align-items flex-end aligns items to the row's bottom", width: 10, html: `<div style="display:flex;width:100%;align-items:flex-end"><div>a<br>b</div><div>x</div></div>`, want: "a         \nbx        \n"},
		{name: "loose text before an item becomes an anonymous item of its own", width: 20, html: `<div style="display:flex;width:100%">loose text<div>a</div></div>`, want: "loose texta         \n"},
		{name: "display:none child is excluded from layout", width: 10, html: `<div style="display:flex;width:100%"><div style="display:none">a</div><div>b</div></div>`, want: "b         \n"},
	})
}

// TestFlexAnonymousItems covers loose text inside a flex container. Flexbox §4
// wraps each contiguous run of it in an anonymous block container flex item,
// and drops a run that is entirely whitespace. Such an item has no element of
// its own, so it inherits the container's style and is unreachable by any
// selector.
func TestFlexAnonymousItems(t *testing.T) {
	runCases(t, []renderCase{
		{name: "text between two items is its own item", width: 20, html: `<div style="display:flex;width:100%"><div>a</div>mid<div>b</div></div>`, want: "amidb               \n"},
		{name: "an element child breaks a run in two", width: 20, html: `<div style="display:flex;width:100%">one<span>|</span>two</div>`, want: "one|two             \n"},
		{name: "a comment does not break a run", width: 20, html: `<div style="display:flex;width:100%">one<!--x-->two</div>`, want: "onetwo              \n"},
		// The whitespace-only exception is the reason indented markup doesn't
		// grow phantom items: the newline and indentation on either side of the
		// <span> would otherwise be two more items, each contributing a column.
		{name: "whitespace between items is not an item", width: 20, html: "<div style=\"display:flex;width:100%\">\n  <span>a</span>\n  <span>b</span>\n</div>", want: "ab                  \n"},
		{name: "a run's own leading and trailing whitespace collapses away", width: 20, html: `<div style="display:flex;width:100%"><div>[</div>  mid  <div>]</div></div>`, want: "[mid]               \n"},
		{name: "whitespace-only text is kept when white-space preserves it", width: 20, html: `<div style="display:flex;width:100%;white-space:pre"><div>a</div>   <div>b</div></div>`, want: "a   b               \n"},
		{name: "an anonymous item takes part in justify-content", width: 20, html: `<div style="display:flex;width:100%;justify-content:space-between">a<div>b</div></div>`, want: "a                  b\n"},
		{name: "an anonymous item is aligned on the cross axis like any other", width: 20, html: `<div style="display:flex;width:100%;align-items:flex-end">x<div>a<br>b</div></div>`, want: " a                  \nxb                  \n"},
		{name: "an anonymous item inherits the container's inheritable style", width: 20, html: `<div style="display:flex;width:100%;text-transform:uppercase">loose<div>a</div></div>`, want: "LOOSEA              \n"},
		// An anonymous box is unselectable in CSS: it has no element for a
		// selector to match, so a rule targeting the container's children can't
		// reach it. Only inheritance can.
		{name: "an anonymous item is not reachable by a selector", width: 20, css: `div > *{text-transform:uppercase}`, html: `<div style="display:flex;width:100%">loose<div>a</div></div>`, want: "looseA              \n"},
		// flex-grow's default of 0 is visible through justify-content: an item
		// that grew would leave nothing for justify-content to distribute.
		{name: "an anonymous item's own flex factors are the CSS defaults", width: 20, html: `<div style="display:flex;width:100%;justify-content:flex-end">loose<div>a</div></div>`, want: "              loosea\n"},
		// flex-shrink defaults to 1 on an anonymous item too, so it takes its
		// share of an overflow. Its automatic minimum size still floors it at
		// its longest word.
		{name: "an anonymous item shrinks and wraps like any other", width: 10, html: `<div style="display:flex;width:100%">loose text<div>a</div></div>`, want: "loose    a\ntext      \n"},
		{name: "gap applies on both sides of an anonymous item", width: 12, html: `<div style="display:flex;width:100%;gap:1">a<div>b</div>c</div>`, want: "a b c       \n"},
		{name: "column direction stacks anonymous items with the rest", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%">one<div>b</div>three</div>`, want: "one         \nb           \nthree       \n"},
		// A display:contents child generates no box, so its own loose text is
		// hoisted into the container as an anonymous item there, exactly as its
		// element children are.
		{name: "loose text inside a display:contents child is hoisted", width: 12, html: `<div style="display:flex;width:100%"><span style="display:contents">a<b>b</b></span></div>`, want: "ab          \n"},
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
		// stretch applies only to an *auto* cross size (Flexbox §8.3), so the
		// item's own definite width wins outright and its own min/max still
		// clamps how far it stretches. text-align resolves within the size that
		// leaves, which is what makes the difference visible: a stretched item
		// right-aligns against the container's edge, a non-stretched one against
		// its own.
		{name: "a definite width opts an item out of stretching", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="width:6;text-align:right">a</div></div>`, want: "     a      \n"},
		{name: "max-width caps how far a stretched item grows", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="max-width:6;text-align:right;border-style:solid">a</div></div>`, want: "┌────┐      \n│   a│      \n└────┘      \n"},
		{name: "min-width larger than the container still wins over stretch", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%"><div style="min-width:14;text-align:right">a</div></div>`, want: "             a\n"},
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

// TestInlineFlexBoxModel pins that an inline-flex container has the same
// margin/border/padding box every other box in this engine has, and clips to
// its own overflow-x/overflow-y the same way a block-level display:flex
// container does.
//
// It used to have none of that. renderInlineFlexContent called the layout pass
// underneath the box model directly, so a bordered inline-flex painted its
// items and nothing else: no border, no padding, no margin, and no clipping,
// which left CSS.md's overflow entry true of `display: flex` and false of the
// `inline-flex` it names alongside it.
//
// A bordered container is three lines tall, so it breaks the line it sits in.
// That is the documented behavior of every atomic inline box taller than one
// line, inline-block included, not something particular to flex; see
// COMPATIBILITY.md.
func TestInlineFlexBoxModel(t *testing.T) {
	runCases(t, []renderCase{
		// The space after "x" is a soft-wrap point here, since the bordered box
		// is three lines tall and takes the line below, so it is trimmed off the
		// end of the line rather than left dangling. See wordWrapTokens'
		// trimTail.
		{name: "border draws around the items and shrinks to fit them", width: 40, html: `<p>x <span style="display:inline-flex;border-style:solid"><span>a</span><span>b</span></span></p>`, want: "x\n┌──┐\n│ab│\n└──┘\n\n"},
		{name: "padding reserves columns inside the container", width: 40, html: `<p>x<span style="display:inline-flex;padding-left:2;padding-right:2"><span>a</span></span>y</p>`, want: "x  a  y\n\n"},
		{name: "margin offsets the container within the inline flow", width: 40, html: `<p>x<span style="display:inline-flex;margin-left:4"><span>a</span></span>y</p>`, want: "x    ay\n\n"},
		// A declared width is a border-box size here, as it is on every other
		// box, so the border characters come out of it rather than adding to it.
		{name: "a declared width is the container's total painted width", width: 40, html: `<p>x<span style="display:inline-flex;width:10;border-style:solid;justify-content:space-between"><span>a</span><span>b</span></span>y</p>`, want: "x\n┌────────┐\n│a      b│\n└────────┘\ny\n\n"},
	})
}

// TestInlineFlexOverflowClips pins CSS.md's overflow entry for the inline half
// of `display: flex`/`inline-flex`. Both axes clip from inside the container's
// own border box, and text-overflow supplies the marker on the horizontal one.
func TestInlineFlexOverflowClips(t *testing.T) {
	runCases(t, []renderCase{
		{name: "overflow-x hidden truncates an over-wide item to the container", width: 40, html: `<p>x<span style="display:inline-flex;width:6;overflow-x:hidden;text-overflow:ellipsis"><span style="flex-shrink:0;width:8">aaaaaaaa</span></span>y</p>`, want: "xaaaaa…y\n\n"},
		{name: "overflow-y hidden truncates the container to its own height", width: 40, html: `<p>x<span style="display:inline-flex;flex-direction:column;height:1;overflow-y:hidden"><span>a</span><span>b</span></span>y</p>`, want: "xay\n\n"},
		// Failing an explicit height, max-height is what overflow-y clips to,
		// and the bottom border still closes below the last visible row.
		{name: "overflow-y hidden clips to max-height from inside the border", width: 40, html: `<p>x<span style="display:inline-flex;flex-direction:column;max-height:1;overflow-y:hidden;border-style:solid"><span>a</span><span>b</span></span>y</p>`, want: "x\n┌─┐\n│a│\n└─┘\ny\n\n"},
		// Without an overflow of its own the container is left to overflow, the
		// same as any other box in this engine.
		{name: "no overflow declaration leaves the item painting past the container", width: 40, html: `<p>x<span style="display:inline-flex;width:6"><span style="flex-shrink:0;width:8">aaaaaaaa</span></span>y</p>`, want: "xaaaaaaaay\n\n"},
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

// TestFlexRowReverseWrap pins that row-reverse flips each line's placement, not
// the sequence line breaking runs over. Items are collected into lines in
// order-modified *document* order whichever way the main axis points (§9.3
// collects lines before §9.5 places anything); the reversed direction only
// moves where along the line each item lands.
//
// This used to reverse the whole item sequence before breaking lines, which
// changed line membership: five equal items over three lines came out 5/4, 3/2,
// 1 instead of 1/2, 3/4, 5. Both the grouping and each line's contents were
// wrong.
func TestFlexRowReverseWrap(t *testing.T) {
	runCases(t, []renderCase{
		// Lines hold {1,2}, {3,4}, {5} — the same membership as plain `wrap` —
		// each painted right-to-left and packed against the right edge, since
		// row-reverse puts main-start there.
		{name: "row-reverse wrap breaks lines in document order and reverses within each", width: 12, css: `.r{display:flex;flex-direction:row-reverse;flex-wrap:wrap;width:12}.r>div{width:5}`, html: `<div class="r"><div>1</div><div>2</div><div>3</div><div>4</div><div>5</div></div>`, want: "  2    1    \n  4    3    \n       5    \n"},
		{name: "plain wrap is the same membership, laid out left to right", width: 12, css: `.r{display:flex;flex-wrap:wrap;width:12}.r>div{width:5}`, html: `<div class="r"><div>1</div><div>2</div><div>3</div><div>4</div><div>5</div></div>`, want: "1    2      \n3    4      \n5           \n"},
		// justify-content is still mirrored per line, so flex-end packs each
		// line against the left edge with its items still right-to-left.
		{name: "row-reverse wrap mirrors justify-content per line", width: 12, css: `.r{display:flex;flex-direction:row-reverse;flex-wrap:wrap;width:12;justify-content:flex-end}.r>div{width:5}`, html: `<div class="r"><div>1</div><div>2</div><div>3</div><div>4</div><div>5</div></div>`, want: "2    1      \n4    3      \n5           \n"},
		// order still resolves first, so the sequence broken into lines is
		// b,a and the single line is then painted a,b.
		{name: "order still resolves before row-reverse under wrap", width: 12, css: `.r{display:flex;flex-direction:row-reverse;flex-wrap:wrap;width:12}.r>div{width:5}`, html: `<div class="r"><div style="order:2">a</div><div style="order:1">b</div></div>`, want: "  a    b    \n"},
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
		// The cross axis. An auto margin-top/margin-bottom absorbs the free
		// space between the item and its line's own height, and takes
		// precedence over align-items/align-self (§8.1).
		{name: "margin-top and margin-bottom auto center an item on its line", width: 12, html: `<div style="display:flex;width:100%"><div>a<br>a<br>a</div><div style="margin-top:auto;margin-bottom:auto">b</div></div>`, want: "a           \nab          \na           \n"},
		{name: "margin-top:auto alone drops an item to the bottom of its line", width: 12, html: `<div style="display:flex;width:100%"><div>a<br>a<br>a</div><div style="margin-top:auto">b</div></div>`, want: "a           \na           \nab          \n"},
		{name: "margin-bottom:auto alone holds an item at the top", width: 12, html: `<div style="display:flex;width:100%"><div>a<br>a<br>a</div><div style="margin-bottom:auto">b</div></div>`, want: "ab          \na           \na           \n"},
		{name: "a cross-axis auto margin overrides align-items", width: 12, html: `<div style="display:flex;width:100%;align-items:flex-start"><div>a<br>a<br>a</div><div style="margin-top:auto">b</div></div>`, want: "a           \na           \nab          \n"},
		// An auto cross margin also opts the item out of stretching, the free
		// space being claimed by the margin instead. Without that, the bordered
		// item would grow to all five rows and there would be nothing to center
		// it within.
		{name: "an auto cross margin opts the item out of align-items:stretch", width: 12, html: `<div style="display:flex;width:100%;height:5"><div>a</div><div style="margin-top:auto;margin-bottom:auto;border-style:solid">b</div></div>`, want: "a           \n ┌─┐        \n │b│        \n └─┘        \n            \n"},
		{name: "an auto cross margin with no free space to absorb resolves to zero", width: 12, html: `<div style="display:flex;width:100%"><div>a<br>a<br>a</div><div style="margin-top:auto;margin-bottom:auto;border-style:solid">b</div></div>`, want: "a┌─┐        \na│b│        \na└─┘        \n"},
	})
}

// TestFlexPercentageGap covers percentage row-gap/column-gap. CSS Box Alignment
// §8.3 resolves each against the container's own content size in that gap's own
// axis, and against zero when that size is indefinite.
func TestFlexPercentageGap(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a percentage column-gap resolves against the container's content width", width: 12, html: `<div style="display:flex;width:100%;column-gap:25%"><div>a</div><div>b</div></div>`, want: "a   b       \n"},
		{name: "a fractional column of gap truncates, like every other percentage here", width: 12, html: `<div style="display:flex;width:100%;gap:10%"><div>a</div><div>b</div></div>`, want: "a b         \n"},
		{name: "a percentage row-gap resolves against the container's explicit height", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;height:8;row-gap:25%"><div>a</div><div>b</div></div>`, want: "a           \n            \n            \nb           \n            \n            \n            \n            \n"},
		// No declared height means no definite size in that axis, so the
		// percentage resolves to zero rather than to the content's own height.
		{name: "a percentage row-gap against an indefinite height is zero", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;row-gap:25%"><div>a</div><div>b</div></div>`, want: "a           \nb           \n"},
		{name: "a percentage row-gap applies between wrapped lines too", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:8;row-gap:25%;align-content:flex-start"><div style="flex-basis:8">a</div><div style="flex-basis:8">b</div></div>`, want: "a           \n            \n            \nb           \n            \n            \n            \n            \n"},
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

// TestFlexColumnGrowCeilingIncludesChrome pins that a column item's max-height
// caps its *outer* grown height, its own border rules and padding rows added
// back on. max-height is content-box in this engine while every main size flex
// resolves is an outer one, a conversion clampFlexMainHeight and the
// align-items:stretch cap both make. The flex-grow ceiling used to compare the
// two directly, so a bordered item stopped growing exactly its own chrome short
// of what it asked for: max-height:3 came out three rows in total, one of them
// content, rather than three content rows inside its border.
func TestFlexColumnGrowCeilingIncludesChrome(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a bordered item grows to max-height plus its border rows", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:10"><div style="flex-grow:1;max-height:3;border-style:solid">a</div><div>b</div></div>`, want: "┌────────┐\n│a       │\n│        │\n│        │\n└────────┘\nb         \n          \n          \n          \n          \n"},
		{name: "padding rows count toward the ceiling the same way", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:8"><div style="flex-grow:1;max-height:2;padding-top:1">a</div><div>b</div></div>`, want: "          \na         \n          \nb         \n          \n          \n          \n          \n"},
		// An unbordered item is the case that already worked, kept as the
		// control that the chrome conversion isn't being applied twice.
		{name: "an item with no chrome is capped at max-height itself", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:8"><div style="flex-grow:1;max-height:3">a</div><div>b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n          \n          \n"},
	})
}

// TestFlexColumnPercentAgainstIndefiniteHeight pins that a percentage min/max
// height on a column-direction item is ignored when the container's own main
// size is indefinite, which here means it has a min-height but no explicit
// height. CSS resolves a percentage against an indefinite basis to auto, or to
// none for a maximum, since the container's real main size is max(content,
// min-height) and isn't known until its content is laid out.
//
// The indefinite basis used to be carried as a plain 0, which resolves every
// percentage to a definite *zero*: `max-height: 50%` became a maximum of no
// rows and froze the item where it stood, and a percentage clamp on a declared
// flex-basis crushed that basis to a single row. See indefiniteMainSize.
func TestFlexColumnPercentAgainstIndefiniteHeight(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a percentage max-height doesn't stop flex-grow under a min-height container", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:6"><div style="flex-grow:1;max-height:50%">a</div><div>b</div></div>`, want: "a         \n          \n          \n          \n          \nb         \n"},
		{name: "a percentage max-height doesn't crush a declared flex-basis", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:6"><div style="flex-basis:4;flex-grow:0;max-height:75%">a</div><div>b</div></div>`, want: "a         \n          \n          \n          \nb         \n          \n"},
		{name: "a percentage min-height doesn't raise an item either", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:4"><div style="flex-basis:1;min-height:200%">a</div><div>b</div></div>`, want: "a         \nb         \n          \n          \n"},
		// The explicit-height control: there the basis is definite, so the same
		// percentage resolves and bites, capping growth at half of eight rows.
		{name: "an explicit container height makes the percentage resolve", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:8"><div style="flex-grow:1;max-height:50%">a</div><div>b</div></div>`, want: "a         \n          \n          \n          \nb         \n          \n          \n          \n"},
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
		// wrap-reverse breaks the same lines as wrap, since line breaking runs
		// along the main axis, and stacks them the other way.
		{name: "flex-wrap:wrap-reverse stacks the lines bottom to top", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "c         \na   b     \n"},
		{name: "flex-flow's wrap-reverse component reverses too", width: 10, html: `<div style="display:flex;flex-flow:row wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "c         \na   b     \n"},
		{name: "wrap-reverse leaves each line's own item order alone", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div><div style="flex-basis:4">d</div></div>`, want: "c   d     \na   b     \n"},
		// The two reverses are independent: row-reverse packs each line against
		// the right edge and runs its items right to left, wrap-reverse puts
		// the first line at the bottom. Each item's own text stays left-aligned
		// inside its own four-column box.
		{name: "row-reverse and wrap-reverse flip both axes independently", width: 10, html: `<div style="display:flex;flex-flow:row-reverse wrap-reverse;width:100%"><div style="flex-basis:4">a</div><div style="flex-basis:4">b</div><div style="flex-basis:4">c</div></div>`, want: "      c   \n  b   a   \n"},
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

// TestFlexAlignContentStretch covers align-content's initial value. Unlike
// every other value, stretch doesn't pack the lines somewhere inside the
// container's leftover cross space: it hands that space to the lines
// themselves, equally, growing each one (§8.4).
func TestFlexAlignContentStretch(t *testing.T) {
	runCases(t, []renderCase{
		// Four leftover rows over two one-row lines: each line becomes three
		// rows tall, so the second line starts halfway down rather than
		// directly under the first.
		{name: "the unset default grows each line by an equal share", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n"},
		{name: "align-content:stretch is the same as leaving it unset", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:stretch"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n"},
		{name: "align-content:normal behaves as the unset default", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:6;align-content:normal"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \nb         \n          \n          \n"},
		// The line grows, and under align-items: stretch (also the default) so
		// does the item on it, which is what makes a bordered item's own bottom
		// rule move down rather than a blank row appearing below a closed box.
		{name: "a stretched line stretches its items' own boxes", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:8"><div style="flex-basis:8;border-style:solid">a</div><div style="flex-basis:8;border-style:solid">b</div></div>`, want: "┌──────┐    \n│a     │    \n│      │    \n└──────┘    \n┌──────┐    \n│b     │    \n│      │    \n└──────┘    \n"},
		// align-items is what places an item inside its line, and it opts out
		// of growing with it. The line is still four rows tall; the item just
		// sits at the top of them.
		{name: "align-items:flex-start keeps the item its own size within the grown line", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:8;align-items:flex-start"><div style="flex-basis:8;border-style:solid">a</div><div style="flex-basis:8;border-style:solid">b</div></div>`, want: "┌──────┐    \n│a     │    \n└──────┘    \n            \n┌──────┐    \n│b     │    \n└──────┘    \n            \n"},
		{name: "row-gap comes out of the space before it is shared", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:9;gap:1"><div style="flex-basis:8;border-style:solid">a</div><div style="flex-basis:8;border-style:solid">b</div></div>`, want: "┌──────┐    \n│a     │    \n│      │    \n└──────┘    \n            \n┌──────┐    \n│b     │    \n│      │    \n└──────┘    \n"},
		// Negative leftover space makes stretch identical to flex-start (§8.4),
		// which here means the container simply overflows its height.
		{name: "a container shorter than its lines stretches nothing", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:2"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \nb         \n"},
		// align-content applies to multi-line containers only, so a nowrap
		// container's single line takes the container's whole cross size
		// through align-items instead, as it always has.
		{name: "a nowrap container is unaffected", width: 10, html: `<div style="display:flex;width:100%;height:6"><div>a</div></div>`, want: "a         \n          \n          \n          \n          \n          \n"},
		{name: "a wrapping container at a single line grows that one line", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:4"><div style="border-style:solid">a</div></div>`, want: "┌─┐         \n│a│         \n│ │         \n└─┘         \n"},
		// An odd remainder can't be split across lines on a character grid, so
		// it goes to the earliest lines, one row apiece.
		{name: "an odd leftover row goes to the first line", width: 10, html: `<div style="display:flex;flex-wrap:wrap;width:100%;height:5"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "a         \n          \n          \nb         \n          \n"},
	})
}

// TestFlexWrapReverseCrossAxis covers what wrap-reverse changes beyond the line
// order: it swaps the cross-start and cross-end edges (§5.2), so every
// cross-axis alignment keyword means the opposite edge from what it means under
// plain wrap. The main axis, and so justify-content, is untouched.
func TestFlexWrapReverseCrossAxis(t *testing.T) {
	runCases(t, []renderCase{
		// Cross-start is the bottom, so align-content: flex-start packs the
		// lines there.
		{name: "align-content:flex-start packs wrapped lines against the bottom", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;height:5;align-content:flex-start"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \n          \nb         \na         \n"},
		{name: "align-content:flex-end packs them against the top instead", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;height:5;align-content:flex-end"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "b         \na         \n          \n          \n          \n"},
		// center and the space-* values are symmetric about the container's
		// middle, so only the line order distinguishes them from plain wrap.
		{name: "align-content:center is unchanged apart from the line order", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;height:6;align-content:center"><div style="flex-basis:6">a</div><div style="flex-basis:6">b</div></div>`, want: "          \n          \nb         \na         \n          \n          \n"},
		// Within a line, cross-start is the line's own bottom row.
		{name: "align-items:flex-start aligns items to the bottom of their line", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;align-items:flex-start"><div>a<br>a</div><div>b</div></div>`, want: "a         \nab        \n"},
		// stretch, the unset default, fills the line whichever way the cross
		// axis points, so it is unaffected.
		{name: "the default stretch is unaffected", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%"><div>a<br>a</div><div>b</div></div>`, want: "ab        \na         \n"},
		{name: "align-items:flex-end aligns them to the top", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;align-items:flex-end"><div>a<br>a</div><div>b</div></div>`, want: "ab        \na         \n"},
		{name: "align-self is mirrored the same way as align-items", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;align-items:flex-end"><div>a<br>a</div><div style="align-self:flex-start">b</div></div>`, want: "a         \nab        \n"},
		{name: "align-items:center is its own mirror image", width: 11, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;align-items:center"><div>a<br>a<br>a</div><div>b</div></div>`, want: "a          \nab         \na          \n"},
		{name: "justify-content is untouched by wrap-reverse", width: 10, html: `<div style="display:flex;flex-wrap:wrap-reverse;width:100%;justify-content:flex-end"><div>ab</div></div>`, want: "        ab\n"},
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
		// A zero is dimensionless in CSS, so every spelling of it is the same
		// definite base size - including units this engine otherwise ignores
		// outright. `flex: 1 1 0px` is the single most common spelling of the
		// idiom in real stylesheets, and used to lay out as `flex: 1 1 auto`
		// (12/8 here) because the unit made the length unparseable. See
		// parseFlexSizeVal.
		{name: "flex-basis:0px is a zero length, not an ignored unit", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:0px;flex-grow:1;border-style:solid">aaaa aaaa</div><div style="flex-basis:0px;flex-grow:1;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│aaaa    ││b       │\n│aaaa    ││        │\n└────────┘└────────┘\n"},
		{name: "flex:1 1 0px shorthand splits the line equally", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 1 0px;border-style:solid">aaaa aaaa</div><div style="flex:1 1 0px;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│aaaa    ││b       │\n│aaaa    ││        │\n└────────┘└────────┘\n"},
		{name: "flex:1 1 0rem shorthand splits the line equally", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 1 0rem;border-style:solid">aaaa aaaa</div><div style="flex:1 1 0rem;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│aaaa    ││b       │\n│aaaa    ││        │\n└────────┘└────────┘\n"},
		{name: "a non-zero px length is still ignored, falling back to auto", width: 20, html: `<div style="display:flex;width:100%"><div style="flex-basis:4px;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
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

// TestFlexContainerMinHeight pins that a flex container's own min-height is
// live, as it is for any other block box (block.go). It used to be dropped
// entirely: only an explicit `height` reached flex layout, so a container that
// declared a minimum simply came out at its content height, and in `column`
// direction its flex-grow children had no definite main size to fill — the
// "sidebar at least this tall, contents share the rest" pattern did nothing.
//
// A minimum is a floor and not a target, so unlike `height` it can only grow
// the container: content taller than the minimum has already satisfied it and
// must not be handed to flex-shrink as a deficit.
func TestFlexContainerMinHeight(t *testing.T) {
	runCases(t, []renderCase{
		{name: "min-height gives a column container a definite main size to grow into", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:4"><div style="flex:1;border-style:solid">a</div></div>`, want: "┌──────────┐\n│a         │\n│          │\n└──────────┘\n"},
		{name: "min-height pads a column container short of it", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:4"><div>a</div><div>b</div></div>`, want: "a           \nb           \n            \n            \n"},
		{name: "content taller than min-height is not shrunk to fit it", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;min-height:2"><div>a</div><div>b</div><div>c</div></div>`, want: "a           \nb           \nc           \n"},
		{name: "min-height gives a row container's line its cross size", width: 12, html: `<div style="display:flex;width:100%;min-height:3;align-items:center"><div>a</div></div>`, want: "            \na           \n            \n"},
		{name: "an explicit height still wins over min-height", width: 12, html: `<div style="display:flex;flex-direction:column;width:100%;height:2;min-height:6"><div>a</div></div>`, want: "a           \n            \n"},
		{name: "min-height on a wrapping row feeds align-content", width: 12, html: `<div style="display:flex;flex-wrap:wrap;width:100%;min-height:4;align-content:flex-end"><div style="width:8">a</div><div style="width:8">b</div></div>`, want: "            \n            \na           \nb           \n"},
	})
}

// TestFlexContainerOverflowY is the vertical counterpart of
// TestFlexContainerOverflowX: a flex container is no exception to CSS.md's
// overflow entry, but nothing in flex layout ever shortened the container, so
// `height`/`max-height` plus an explicit overflow used to be ignored outright
// and the content ran past both. As for an ordinary block box, clipping
// requires the explicit overflow — `max-height` alone never truncates.
func TestFlexContainerOverflowY(t *testing.T) {
	runCases(t, []renderCase{
		{name: "overflow-y:hidden clips a column container to its height", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:2;overflow-y:hidden"><div>a</div><div>b</div><div>c</div><div>d</div></div>`, want: "a         \nb         \n"},
		{name: "overflow-y:clip clips to max-height when no height is set", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;max-height:2;overflow-y:clip"><div>a</div><div>b</div><div>c</div></div>`, want: "a         \nb         \n"},
		{name: "height wins over max-height, matching an ordinary block box", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:3;max-height:1;overflow-y:hidden"><div>a</div><div>b</div><div>c</div><div>d</div></div>`, want: "a         \nb         \nc         \n"},
		{name: "max-height without an overflow declaration does not clip", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;max-height:2"><div>a</div><div>b</div><div>c</div></div>`, want: "a         \nb         \nc         \n"},
		{name: "a clipped container's border still closes below its last visible row", width: 10, html: `<div style="display:flex;flex-direction:column;width:100%;height:2;overflow-y:hidden;border-style:solid"><div>a</div><div>b</div><div>c</div></div>`, want: "┌────────┐\n│a       │\n│b       │\n└────────┘\n"},
		{name: "a row container clips to its height too", width: 10, html: `<div style="display:flex;width:100%;height:1;overflow-y:hidden"><div>a<br>b<br>c</div></div>`, want: "a         \n"},
	})
}

// TestFlexShorthandInvalidComponents pins that the `flex` shorthand validates
// every component before assigning any longhand. The two- and three-token forms
// used to assign positionally without looking, so `flex: 30% 1` set flex-grow
// to a basis token (parsing to 0, collapsing the item) rather than resolving to
// the grow:1/basis:30% a browser gives it, and a junk component still handed
// out the shorthand's 1/1 grow/shrink defaults.
func TestFlexShorthandInvalidComponents(t *testing.T) {
	runCases(t, []renderCase{
		// `[ <'flex-grow'> <'flex-shrink'>? || <'flex-basis'> ]` — the basis may
		// lead the grow/shrink pair as well as trail it, so these two are the
		// same declaration written both ways and must lay out identically.
		{name: "flex:1 30% is grow 1 with a 30% basis", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 30%;border-style:solid">a</div><div>rest</div></div>`, want: "┌──────────────┐rest\n│a             │    \n└──────────────┘    \n"},
		{name: "flex:30% 1 is the same declaration with the basis first", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:30% 1;border-style:solid">a</div><div>rest</div></div>`, want: "┌──────────────┐rest\n│a             │    \n└──────────────┘    \n"},
		{name: "flex:30% 1 2 puts the basis ahead of grow and shrink", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:30% 1 2;border-style:solid">a</div><div>rest</div></div>`, want: "┌──────────────┐rest\n│a             │    \n└──────────────┘    \n"},
		// An invalid component leaves every longhand at its initial value
		// (grow 0, shrink 1, basis auto), so the item sits at its natural width
		// rather than growing on the strength of the shorthand's defaults.
		{name: "flex:1 junk grants no grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 junk;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		{name: "flex:1 1 junk grants no grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 1 junk;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		{name: "flex:junk 1 grants no grow", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:junk 1;border-style:solid">a</div><div>rest</div></div>`, want: "┌─┐rest             \n│a│                 \n└─┘                 \n"},
		// A bare number is both a valid <number> and a valid basis here, so the
		// grow-first reading has to win the overlap.
		{name: "flex:1 2 stays grow 1 shrink 2 with a zero basis", width: 20, html: `<div style="display:flex;width:100%"><div style="flex:1 2;border-style:solid">a</div><div style="flex:1 2;border-style:solid">b</div></div>`, want: "┌────────┐┌────────┐\n│a       ││b       │\n└────────┘└────────┘\n"},
	})
}

// TestFlexItemElementDispatch covers flex items whose box isn't produced by the
// ordinary block box model. Flex items are blockified, but blockification only
// changes an element's *outer* display type — a <table> item is still a table,
// a <ul> item is still a list, and an <input> item still synthesizes its
// content from its attributes. Flex layout used to hand every non-flex item
// straight to the block box model, which walks an element's children as
// ordinary content: tables came out as their cells' text run together, lists
// lost their markers, and a control with no children at all — every <input>,
// the single most common thing to put in a `display: flex` row — rendered as
// nothing whatsoever.
func TestFlexItemElementDispatch(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a text input renders its field, not an empty box", width: 24, html: `<div style="display:flex;width:100%"><input value="hi"><div>|</div></div>`, want: "hi                  |   \n"},
		{name: "a checkbox renders its glyph", width: 10, html: `<div style="display:flex;width:100%"><input type="checkbox" checked><div>x</div></div>`, want: "☑x        \n"},
		{name: "a radio renders its glyph", width: 10, html: `<div style="display:flex;width:100%"><input type="radio" checked><div>x</div></div>`, want: "●x        \n"},
		{name: "a select renders its closed-state widget", width: 12, html: `<div style="display:flex;width:100%"><select><option>aa</option></select></div>`, want: "[ aa ▾]     \n"},
		{name: "a progress bar renders its bar", width: 12, html: `<div style="display:flex;width:100%"><progress value="0.5" style="width:4"></progress></div>`, want: "██░░        \n"},
		{name: "a meter renders its bar", width: 12, html: `<div style="display:flex;width:100%"><meter value="0.5" style="width:4"></meter></div>`, want: "██░░        \n"},
		{name: "an unordered list keeps its markers", width: 12, html: `<div style="display:flex;width:100%"><ul><li>a</li><li>b</li></ul></div>`, want: "    • a     \n    • b     \n"},
		{name: "an ordered list keeps its numbering", width: 12, html: `<div style="display:flex;width:100%"><ol><li>a</li><li>b</li></ol></div>`, want: "    1. a    \n    2. b    \n"},
		{name: "a table goes through the table renderer", width: 14, css: `table{border-collapse:collapse}td{border-style:solid;padding-left:1;padding-right:1}`, html: `<div style="display:flex;width:100%"><table><tr><td>a</td><td>b</td></tr></table></div>`, want: "┌───┬───┐     \n│ a │ b │     \n└───┴───┘     \n"},
	})
}

// TestBlockifiedFormControl pins the same dispatch on the other path that
// blockifies a control: author CSS saying so outright. This is where the flex
// fix lives (renderBlockContentBox), so `display: block` on an <input> is fixed
// by the same change and must stay fixed.
func TestBlockifiedFormControl(t *testing.T) {
	runCases(t, []renderCase{
		{name: "display:block on an input still renders its field", width: 12, html: `<input value="hi" style="display:block">`, want: "hi\n"},
		{name: "a bordered block input keeps its field inside its own border", width: 12, html: `<input value="hi" style="display:block;width:8;border-style:solid">`, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "display:block on a select still renders its widget", width: 12, html: `<select style="display:block"><option>aa</option></select>`, want: "[ aa ▾]\n"},
	})
}

// TestFlexColumnAutomaticMinimumHeight covers column direction's automatic
// minimum size — `min-height: auto` on a flex item's main axis resolving to the
// content-based minimum (Flexbox §4.5), the vertical counterpart of the row
// direction rule the automatic-minimum-size tests below cover. An item is never
// shrunk below the height its content needs, so the deficit goes to the items
// that can actually absorb it.
//
// The floor used to be one row, absent an explicit min-height. Since nothing
// truncates a box vertically without an explicit overflow, that made the shrink
// distribution a fiction: an item was credited with rows it went on to paint
// anyway, the items that could genuinely shrink were under-shrunk by exactly
// that much, and the container overflowed its own declared height.
func TestFlexColumnAutomaticMinimumHeight(t *testing.T) {
	// .a is three hard rows of content and cannot shrink; .b starts at
	// flex-basis: 6 and can. The container is 6 rows, so the deficit is 3 and
	// all of it has to come out of .b.
	const css = `.c{display:flex;flex-direction:column;height:6;width:10}.a{border-left:"A"}.b{flex-basis:6;border-left:"B"}`
	const html = `<div class="c"><div class="a">one<br>two<br>three</div><div class="b">x</div></div>`
	runCases(t, []renderCase{
		{name: "an item is not shrunk below its content height", width: 10, css: css, html: html, want: "Aone      \nAtwo      \nAthree    \nBx        \nB         \nB         \n"},
		// §4.5's own opt-out: the automatic minimum doesn't apply to an item
		// whose overflow is not visible in the main axis — and a non-visible
		// overflow-y is also the one case where this engine really does
		// truncate a box to the height flex hands it, so the shrink is real.
		// Deficit 3 splits by scaled shrink factor (3 : 6) into 1 and 2.
		{name: "overflow-y:hidden opts out, and the item really is clipped", width: 10, css: css + `.a{overflow-y:hidden}`, html: html, want: "Aone      \nAtwo      \nBx        \nB         \nB         \nB         \n"},
		// Nothing can absorb the deficit, so the container overflows rather than
		// crushing its items — the same outcome a browser reaches by the same
		// route, and what row direction already does with an all-floored line.
		{name: "a container nobody can shrink for overflows instead", width: 10, css: `.c{display:flex;flex-direction:column;height:2;width:10}.a{border-left:"A"}.b{border-left:"B"}`, html: `<div class="c"><div class="a">one<br>two</div><div class="b">x<br>y</div></div>`, want: "Aone      \nAtwo      \nBx        \nBy        \n"},
	})
}

// TestFlexAlignmentIsSafeOnOverflow pins that every alignment value packs
// start-ward once the free space goes negative — CSS's own *safe* alignment
// ("if the size of the alignment subject overflows the alignment container, the
// alignment subject is aligned as if the alignment mode were start", Box
// Alignment §4.2).
//
// Browsers default to *unsafe* alignment on the main axis, so `center` there
// overflows equally off both edges and `flex-end` off the start edge. A
// character grid has nowhere to put columns pushed off the container's start
// edge — they'd land in its own border/padding, or in a sibling — and losing
// them outright is exactly what safe alignment exists to prevent. Cross-axis
// alignment of an over-tall item behaves the same way, for the same reason.
func TestFlexAlignmentIsSafeOnOverflow(t *testing.T) {
	// Three unshrinkable 5-column items in a 6-column container: 9 columns of
	// overflow, so every value below has negative free space to work with.
	const row = `.r{display:flex;width:6}.r>div{flex-shrink:0;width:5}`
	const items = `<div class="r"><div>aa</div><div>bb</div><div>cc</div></div>`
	const want = "aa   bb   cc   \n"
	runCases(t, []renderCase{
		{name: "justify-content:center packs start-ward when the line overflows", width: 16, css: row + `.r{justify-content:center}`, html: items, want: want},
		{name: "justify-content:flex-end packs start-ward when the line overflows", width: 16, css: row + `.r{justify-content:flex-end}`, html: items, want: want},
		{name: "justify-content:space-between packs start-ward when the line overflows", width: 16, css: row + `.r{justify-content:space-between}`, html: items, want: want},
		{name: "justify-content:space-around packs start-ward when the line overflows", width: 16, css: row + `.r{justify-content:space-around}`, html: items, want: want},
		{name: "justify-content:space-evenly packs start-ward when the line overflows", width: 16, css: row + `.r{justify-content:space-evenly}`, html: items, want: want},
		// Cross axis: the container's height is shorter than its one item, so
		// align-items has negative free space of its own.
		{name: "align-items:center keeps an over-tall item at the line's top", width: 6, html: `<div style="display:flex;width:100%;height:1;align-items:center"><div>a<br>b<br>c</div></div>`, want: "a     \nb     \nc     \n"},
		{name: "align-items:flex-end keeps an over-tall item at the line's top", width: 6, html: `<div style="display:flex;width:100%;height:1;align-items:flex-end"><div>a<br>b<br>c</div></div>`, want: "a     \nb     \nc     \n"},
	})
}

// TestFlexSpaceAroundEdgesStayEqual pins that space-around's two edge pads come
// out the same width whenever the free space allows it. The distribution used
// to be built by splitting the leftover into 2n half-shares and recombining
// them pairwise, and splitEvenly hands its remainder to the earliest parts, so
// the surplus piled up at the start of the line: the four-column case below
// came out as a 1-column leading pad, a 2-column gap, a 1-column gap and
// nothing at all on the trailing edge. Weighting the n+1 real slots and
// rounding by largest remainder puts the edges back in step, and makes
// space-around agree with space-evenly where the two coincide.
//
// The trailing pad isn't visible directly (a block-level container fills its
// width regardless), so each case reads it off where the last item lands.
func TestFlexSpaceAroundEdgesStayEqual(t *testing.T) {
	const row = `.r{display:flex;width:100%;justify-content:space-around}.r>div{width:2}`
	runCases(t, []renderCase{
		// 10 columns, three 2-wide items, 4 free: every slot rounds to 1, so the
		// last item ends one column short of the right edge.
		{name: "four free columns over three items pad every slot equally", width: 10, css: row, html: `<div class="r"><div>a</div><div>b</div><div>c</div></div>`, want: " a  b  c  \n"},
		// 12 columns, two 2-wide items, 8 free: exact slots are 2/4/2.
		{name: "an exactly divisible line splits into half, whole, half", width: 12, css: row, html: `<div class="r"><div>a</div><div>b</div></div>`, want: "  a     b   \n"},
		// A single item is centered, both edges taking half the leftover. Seven
		// free columns can't split evenly, so the odd one goes to the leading
		// edge, the same way center's own integer division rounds.
		{name: "a single item is centered", width: 9, css: row, html: `<div class="r"><div>a</div></div>`, want: "    a    \n"},
	})
}

// TestFlexIntrinsicMainSize covers a flex container being *measured* rather
// than laid out: as a flex item of another container, where the answer is its
// flex base size, and inside a table cell, where it is the column's natural
// width. Both used to come back as the whole probe budget.
//
// The cause was that flex-grow, justify-content, and auto margins all spend
// whatever space the container is handed, so a container probed at a generous
// measurement budget expanded to fill it and renderFlexContentBox's
// shrink-to-fit narrowing found nothing to narrow. Row-direction layout now
// suppresses that distribution under a measurement render (see
// measuringIntrinsicWidth in internal/render/flex.go), leaving each item at its
// hypothetical main size and the line at the sum of those.
//
// Column direction never had the bug: a stretched column item is deliberately
// left at whatever width it rendered to rather than padded out to the
// container's, so the container already narrowed to its widest item.
func TestFlexIntrinsicMainSize(t *testing.T) {
	const nest = `.o{display:flex;width:100%}.i{display:flex}.i>div{width:3}`
	runCases(t, []renderCase{
		// The inner container's two 3-wide items make it 6 wide, so Z sits at
		// column 6 whatever the items do with their own free space. It used to
		// land at column 20, the full width of the outer container.
		{name: "flex-grow inside a measured container does not widen it", width: 20, css: nest + `.i>div{flex-grow:1}`, html: `<div class="o"><div class="i"><div>a</div><div>b</div></div><div>Z</div></div>`, want: "a  b  Z             \n"},
		{name: "justify-content inside a measured container does not widen it", width: 20, css: nest + `.i{justify-content:space-between}`, html: `<div class="o"><div class="i"><div>a</div><div>b</div></div><div>Z</div></div>`, want: "a  b  Z             \n"},
		{name: "an auto margin inside a measured container does not widen it", width: 20, css: nest + `.i>div+div{margin-left:auto}`, html: `<div class="o"><div class="i"><div>a</div><div>b</div></div><div>Z</div></div>`, want: "a  b  Z             \n"},
		// Stage 1's accepted shortfall, pinned so stage 2 flips a marked
		// assertion rather than surprising somebody. `flex: 1` is `1 1 0`, and a
		// zero flex base size makes an item's hypothetical main size its
		// automatic minimum size, meaning min-content. Summing those measures
		// this container at 3 columns: "aa bb" contributes its longest word and
		// wraps to two lines, "c" contributes 1.
		//
		// Flexbox §9.9 instead takes each item's *max-content* contribution (5
		// and 1), divides by the flex-grow factor to get a desired flex fraction
		// (5 and 1), applies the largest of them to every item, and so sizes
		// this container at 10: two items of 5, with "aa bb" on one line. Note
		// that is wider than the same items with no flex factors at all, which
		// is the counter-intuitive part and the reason stage 2 wants a real use
		// case first. See docs/proposals/FLEX_INTRINSIC_SIZING.md.
		{name: "flex:1 items measure at min-content, stage 1's known shortfall", width: 20, css: `.o{display:flex;width:100%}.i{display:flex}.i>div{flex:1}`, html: `<div class="o"><div class="i"><div>aa bb</div><div>c</div></div><div>Z</div></div>`, want: "aacZ                \nbb                  \n"},
	})
}
