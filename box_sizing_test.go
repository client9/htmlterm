package htmlterm_test

import "testing"

// TestBoxSizing covers box-sizing itself: the UA stylesheet's implicit
// border-box default, an explicit border-box override (a no-op, since it's
// already the default), and an explicit content-box opt-in on both width
// and height. See docs/proposals/BOX_SIZING.md.
func TestBoxSizing(t *testing.T) {
	runCases(t, []renderCase{
		{name: "default box-sizing is border-box: declared width is the whole outer box", width: 10, html: `<div style="width:6;padding-left:1;padding-right:1;border-style:solid">a</div>`, want: "┌────┐\n│ a  │\n└────┘\n"},
		{name: "explicit box-sizing:border-box matches the default exactly", width: 10, html: `<div style="width:6;padding-left:1;padding-right:1;border-style:solid;box-sizing:border-box">a</div>`, want: "┌────┐\n│ a  │\n└────┘\n"},
		{name: "box-sizing:content-box makes width a pure content size, chrome adds on top", width: 10, html: `<div style="width:6;padding-left:1;padding-right:1;border-style:solid;box-sizing:content-box">a</div>`, want: "┌────────┐\n│ a      │\n└────────┘\n"},
		{name: "default box-sizing is border-box for height too: declared height is the whole outer box", width: 8, html: `<div style="height:4;border-style:solid">a</div>`, want: "┌──────┐\n│a     │\n│      │\n└──────┘\n"},
		{name: "box-sizing:content-box makes height a pure content size, chrome adds on top", width: 8, html: `<div style="height:4;border-style:solid;box-sizing:content-box">a</div>`, want: "┌──────┐\n│a     │\n│      │\n│      │\n│      │\n└──────┘\n"},
		{name: "a UA-default border-box height too small for its own chrome floors at one content line rather than disappearing", width: 8, html: `<div style="height:1;border-style:solid">a</div>`, want: "┌──────┐\n│a     │\n└──────┘\n"},
		{name: "an explicit override on one selector doesn't affect a sibling still on the UA default", width: 10, css: `.content { box-sizing: content-box; }`, html: `<div class="content" style="width:6;border-style:solid">a</div><div style="width:6;border-style:solid">b</div>`, want: "┌──────┐\n│a     │\n└──────┘\n┌────┐\n│b   │\n└────┘\n"},
		{name: "box-sizing:content-box still subtracts margin from width, only border and padding add on top", width: 10, html: `<div style="width:100%;margin-left:2;box-sizing:content-box;border-style:solid">x</div>`, want: "  ┌────────┐\n  │x       │\n  └────────┘\n"},
		{name: "the border-box default subtracts margin too, so width:100% still lines up exactly with the terminal", width: 10, html: `<div style="width:100%;margin-left:2;border-style:solid">x</div>`, want: "  ┌──────┐\n  │x     │\n  └──────┘\n"},
	})
}

// TestBoxSizingFlexItemMainAxisExempt confirms flex items stay outer-sized
// on their main axis regardless of their own box-sizing (see
// docs/proposals/BOX_SIZING.md's "Flex items" section): an item that
// explicitly opts into content-box still paints at exactly its flex-basis,
// border and padding included, the same as it would under the border-box
// default. Without renderFlexItemBoxSized's pre-subtraction, this item's
// content-box width would grow past its flex-basis by its own chrome
// instead of painting flush with it.
func TestBoxSizingFlexItemMainAxisExempt(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-basis on a content-box item still sizes its whole outer box", width: 10, html: `<div style="display:flex"><div style="flex-basis:6;box-sizing:content-box;border-style:solid;padding-left:1;padding-right:1">a</div></div>`, want: "┌────┐    \n│ a  │    \n└────┘    \n"},
		{name: "the same item under the border-box default paints identically", width: 10, html: `<div style="display:flex"><div style="flex-basis:6;border-style:solid;padding-left:1;padding-right:1">a</div></div>`, want: "┌────┐    \n│ a  │    \n└────┘    \n"},
		{name: "column-direction flex-basis on a content-box item still sizes its whole outer box (height axis)", width: 8, html: `<div style="display:flex;flex-direction:column"><div style="flex-basis:6;box-sizing:content-box;border-style:solid;padding-top:1;padding-bottom:1">a</div></div>`, want: "┌──────┐\n│      │\n│a     │\n│      │\n│      │\n└──────┘\n"},
		{name: "the same column item under the border-box default paints identically", width: 8, html: `<div style="display:flex;flex-direction:column"><div style="flex-basis:6;border-style:solid;padding-top:1;padding-bottom:1">a</div></div>`, want: "┌──────┐\n│      │\n│a     │\n│      │\n│      │\n└──────┘\n"},
	})
}

// TestBoxSizingFlexMaxHeightClamps is a regression test for a bug caught
// during code review: several flex.go helpers that clamp a column-direction
// item's height against its own max-height (flex-grow's ceiling,
// align-items:stretch's cap) kept adding the item's own border/padding rows
// unconditionally, as if max-height were always content-box. That was
// correct before box-sizing existed as a real distinction, since height was
// unconditionally content-box then, but became wrong once box-sizing started
// defaulting to border-box: a max-height:4 bordered item grew to 6 rows
// (4 declared + 2 chrome) instead of the declared 4 total. See
// mainAxisHeightBound and docs/proposals/BOX_SIZING.md.
func TestBoxSizingFlexMaxHeightClamps(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-grow stops at max-height's whole outer box under the border-box default", width: 10, height: 20, html: `<div style="display:flex;flex-direction:column;height:20"><div style="flex-grow:1;max-height:4;border-style:solid">a</div></div>`, want: "┌────────┐\n│a       │\n│        │\n└────────┘\n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n          \n"},
		{name: "align-items:stretch stops at max-height's whole outer box under the border-box default", width: 10, html: `<div style="display:flex"><div style="max-height:3;border-style:solid">a</div><div>1<br>2<br>3<br>4<br>5<br>6<br>7<br>8</div></div>`, want: "┌─┐1      \n│a│2      \n└─┘3      \n   4      \n   5      \n   6      \n   7      \n   8      \n"},
	})
}

// TestBoxSizingFlexMaxWidthClamps is flexGrowCeiling's row-direction
// counterpart to TestBoxSizingFlexMaxHeightClamps, a regression test for a
// bug caught during code review: flexGrowCeiling and clampFlexWidth compared
// a declared min-width/max-width directly against an item's outer flex size
// with no box-sizing conversion, unlike their already-fixed column-direction
// counterparts (flexGrowHeightCeiling, clampFlexMainHeight). A content-box
// item's max-width:4 grew to 4 total columns instead of 4 content columns
// plus its own border; see mainAxisWidthBound.
func TestBoxSizingFlexMaxWidthClamps(t *testing.T) {
	runCases(t, []renderCase{
		{name: "flex-grow stops at max-width's whole outer box under content-box", width: 20, html: `<div style="display:flex;width:20"><div style="flex-grow:1;max-width:4;box-sizing:content-box;border-style:solid">a</div></div>`, want: "┌────┐              \n│a   │              \n└────┘              \n"},
	})
}

// TestBoxSizingFlexItemMarginNotDoubleSubtracted is a regression test for a
// bug caught during code review: renderFlexItemBoxSized's width override
// pre-subtracted blockHorizontalChrome, which folds in horizontal margin, to
// convert a content-box item's flex-resolved outer size into the content
// width block.go's own formula expects. But block.go's content-box width
// formula already subtracts margin from the declared width on its own
// (margin-subtraction predates box-sizing), so a margined content-box flex
// item lost its margin's width twice, painting margin-left + margin-right
// columns narrower than its flex-basis. See
// contentBoxAdjustedOverride and blockHorizontalBorderPadding.
func TestBoxSizingFlexItemMarginNotDoubleSubtracted(t *testing.T) {
	runCases(t, []renderCase{
		{name: "a content-box flex item with margin still paints its whole flex-basis, margin included", width: 20, html: `<div style="display:flex"><div style="flex-basis:12;margin-left:2;box-sizing:content-box;border-style:solid">a</div></div>`, want: "  ┌────────┐        \n  │a       │        \n  └────────┘        \n"},
	})
}
