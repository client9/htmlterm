package htmlterm_test

import "testing"

func TestBorderStyleOnBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-style:solid draws full box", html: `<div style="border-style:solid; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "border-style:rounded draws rounded box", html: `<div style="border-style:rounded; width:100%">hi</div>`, width: 8, want: "╭──────╮\n│hi    │\n╰──────╯\n"},
		{name: "border-style:heavy draws heavy box", html: `<div style="border-style:heavy; width:100%">hi</div>`, width: 8, want: "┏━━━━━━┓\n┃hi    ┃\n┗━━━━━━┛\n"},
		{name: "border-style:double draws double box", html: `<div style="border-style:double; width:100%">hi</div>`, width: 8, want: "╔══════╗\n║hi    ║\n╚══════╝\n"},
		{name: "border-style:markdown draws only left/right bars", html: `<div style="border-style:markdown">hi</div>`, want: "|hi|\n"},
		{name: "border-style:hidden draws no borders", html: `<div style="border-style:hidden">hi</div>`, want: "hi\n"},
		{name: "border-style:none draws no borders", html: `<div style="border-style:none">hi</div>`, want: "hi\n"},
		{name: "individual border-top overrides preset fill but keeps corners", html: `<div style="border-style:solid; border-top:'═'; width:100%">hi</div>`, width: 8, want: "┌══════┐\n│hi    │\n└──────┘\n"},
		{name: "individual border-left overrides preset char", html: `<div style="border-style:solid; border-left:'▌'; width:100%">hi</div>`, width: 8, want: "┌──────┐\n▌hi    │\n└──────┘\n"},
		{name: "border-style via CSS class", css: `.box { border-style: rounded; width: 100%; }`, html: `<div class="box">ok</div>`, width: 8, want: "╭──────╮\n│ok    │\n╰──────╯\n"},
		{name: "border-width and per-edge widths are accepted no-ops", html: `<div style="border-style:solid; border-width:2px; border-top-width:1px; border-right-width:1px; border-bottom-width:1px; border-left-width:1px; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "border shorthand with style only", html: `<div style="border: rounded; width:100%">hi</div>`, width: 8, want: "╭──────╮\n│hi    │\n╰──────╯\n"},
		{name: "border shorthand with style and color strips clean to box shape", html: `<div style="border: solid red; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "border shorthand with width style color ignores width", html: `<div style="border: 1px solid red; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "border shorthand two-value width-style form is unsupported and no-ops", html: `<div style="border: 2px solid; width:100%">hi</div>`, width: 8, want: "hi      \n"},
		{name: "border-top shorthand style-only picks that preset's top glyph, other edges untouched", html: `<div style="width:100%; border-style:solid; border-top:double">hi</div>`, width: 8, want: "┌══════┐\n│hi    │\n└──────┘\n"},
		{name: "border-top shorthand width-style-color drops width and colors just the top edge", html: `<div style="width:100%; border-top: 1px solid red">hi</div>`, width: 10, want: "──────────\nhi        \n"},
		{name: "border-top:none suppresses just the top edge even with border-style set (regression: used to be silently overridden by the preset)", html: `<div style="width:100%; border-style:solid; border-top:none">hi</div>`, width: 8, want: "│hi    │\n└──────┘\n"},
		{name: "border-left shorthand style-only derives left glyph from a different preset than border-style", html: `<div style="width:8; border-left: rounded">hi</div>`, want: "│hi     \n"},
		{name: "border-top literal quoted glyph still works after adding shorthand grammar", html: `<div style="width:100%; border-style:solid; border-top:'═'; width:100%">hi</div>`, width: 8, want: "┌══════┐\n│hi    │\n└──────┘\n"},
		{name: "border-top shorthand two-value width-style form (no color) is unsupported and no-ops, falling back to border-style", html: `<div style="width:100%; border-top: 1px solid; border-style:rounded">hi</div>`, width: 8, want: "╭──────╮\n│hi    │\n╰──────╯\n"},
	})
}

func TestBlockInInline(t *testing.T) {
	runCases(t, []renderCase{
		{name: "block inside inline breaks line", html: `<p>before<span style="display:block">mid</span>after</p>`, want: "before\nmid\nafter\n\n"},
	})
}

func TestLogicalSpacingAliases(t *testing.T) {
	runCases(t, []renderCase{
		{name: "logical inline aliases apply block left and right spacing", html: `<div style="width:6; margin-inline-start:2; margin-inline-end:1">x</div>`, want: "  x   \n"},
		{name: "logical block aliases apply block top and bottom padding", html: `<div style="width:3; padding-block-start:1; padding-block-end:1">x</div>`, want: "   \nx  \n   \n"},
	})
}

func TestList(t *testing.T) {
	runCases(t, []renderCase{
		{name: "unordered list renders bullets", html: `<ul><li>alpha</li><li>beta</li></ul>`, want: "    • alpha\n    • beta\n"},
		{name: "ordered list renders numbers", html: `<ol><li>one</li><li>two</li></ol>`, want: "    1. one\n    2. two\n"},
		{name: "list inside blockquote renders with border", html: `<blockquote><ul><li>item</li></ul></blockquote>`, want: "│     • item  \n"},
		{name: "loose list item (p inside li) in blockquote has no extra blank lines", html: `<blockquote><ul><li><p>item</p></li></ul></blockquote>`, want: "│     • item  \n"},
		{name: "multiple loose items in blockquote no extra blank lines", html: `<blockquote><ul><li><p>alpha</p></li><li><p>beta</p></li></ul></blockquote>`, want: "│     • alpha  \n│     • beta  \n"},
		{name: "long item wraps with hanging indent", width: 20, html: `<ul><li>one two three four five six</li></ul>`, want: "    • one two three\n      four five six\n"},
		{name: "ordered list 10+ items aligns single and double digit", width: 40, html: `<ol><li>a</li><li>b</li><li>c</li><li>d</li><li>e</li><li>f</li><li>g</li><li>h</li><li>i</li><li>j</li><li>k</li></ol>`, want: "     1. a\n     2. b\n     3. c\n     4. d\n     5. e\n     6. f\n     7. g\n     8. h\n     9. i\n    10. j\n    11. k\n"},
		{name: "list-style-type none suppresses bullet", css: `ul { list-style-type: none; }`, html: `<ul><li>alpha</li><li>beta</li></ul>`, want: "    alpha\n    beta\n"},
		{name: "list-style-type lower-alpha", css: `ol { list-style-type: lower-alpha; }`, html: `<ol><li>one</li><li>two</li><li>three</li></ol>`, want: "    a. one\n    b. two\n    c. three\n"},
		{name: "list-style-type upper-alpha", css: `ol { list-style-type: upper-alpha; }`, html: `<ol><li>one</li><li>two</li><li>three</li></ol>`, want: "    A. one\n    B. two\n    C. three\n"},
		{name: "list-style-type circle", css: `ul { list-style-type: circle; }`, html: `<ul><li>item</li></ul>`, want: "    ○ item\n"},
		{name: "list-style-type square", css: `ul { list-style-type: square; }`, html: `<ul><li>item</li></ul>`, want: "    ■ item\n"},
		{name: "list-style shorthand in stylesheet", css: `ol { list-style: upper-alpha inside; padding-left: 0; }`, html: `<ol><li>one</li><li>two</li></ol>`, want: "A. one\nB. two\n"},
		{name: "list-style-type lower-roman renders roman numerals", css: `ol { list-style-type: lower-roman; }`, html: `<ol><li>a</li><li>b</li><li>c</li><li>d</li><li>e</li><li>f</li><li>g</li><li>h</li></ol>`, want: "       i. a\n      ii. b\n     iii. c\n      iv. d\n       v. e\n      vi. f\n     vii. g\n    viii. h\n"},
		{name: "list-style-type upper-roman renders roman numerals", css: `ol { list-style-type: upper-roman; }`, html: `<ol><li>a</li><li>b</li><li>c</li><li>d</li></ol>`, want: "      I. a\n     II. b\n    III. c\n     IV. d\n"},
		{name: "lower-roman wrapped item aligns continuation lines", width: 20, css: `ol { list-style-type: lower-roman; }`, html: `<ol><li>one two three</li><li>b</li><li>c</li><li>d</li><li>e</li><li>f</li><li>g</li><li>h</li></ol>`, want: "       i. one two\n          three\n      ii. b\n     iii. c\n      iv. d\n       v. e\n      vi. f\n     vii. g\n    viii. h\n"},
		{name: "padding-left indents list", css: `ul { padding-left: 2; }`, html: `<ul><li>item</li></ul>`, want: "  • item\n"},
		{name: "wrapped item inside blockquote keeps border on all lines", width: 20, html: `<blockquote><ul><li>one two three four five</li></ul></blockquote>`, want: "│     • one two  \n│       three four  \n│       five  \n"},
		{name: "list-style-position inside unordered", css: `ul { list-style-position: inside; }`, html: `<ul><li>alpha</li><li>beta</li></ul>`, want: "    • alpha\n    • beta\n"},
		{name: "list-style-position inside ordered", css: `ol { list-style-position: inside; }`, html: `<ol><li>one</li><li>two</li></ol>`, want: "    1. one\n    2. two\n"},
		{name: "list-style-position inside wraps without hanging indent", width: 20, css: `ul { list-style-position: inside; }`, html: `<ul><li>one two three four five</li></ul>`, want: "    • one two three\n    four five\n"},
		{name: "list-style-position outside wraps with hanging indent", width: 20, html: `<ul><li>one two three four five six</li></ul>`, want: "    • one two three\n      four five six\n"},
		{name: "list-style-type symbols cycles through the list", css: `ul { list-style-type: symbols("A" "B"); }`, html: `<ul><li>one</li><li>two</li><li>three</li></ul>`, want: "    Aone\n    Btwo\n    Athree\n"},
		{name: "list-style shorthand with symbols function", css: `ol { list-style: symbols("A" "B") inside; padding-left: 0; }`, html: `<ol><li>one</li><li>two</li><li>three</li></ol>`, want: "Aone\nBtwo\nAthree\n"},
		{name: "list-style-type symbols with wide emoji glyphs sizes hanging indent by widest item", width: 16, css: `ul { list-style-type: symbols("🟥" "🟨" "🟦"); }`, html: `<ul><li>one two three</li><li>b</li></ul>`, want: "    🟥one two\n      three\n    🟨b\n"},
	})
}

func TestWordWrap(t *testing.T) {
	runCases(t, []renderCase{
		{name: "long paragraph wraps at terminal width", html: `<p>one two three four five six seven</p>`, width: 20, want: "one two three four\nfive six seven\n\n"},
		{name: "short paragraph does not wrap", html: `<p>hello world</p>`, width: 20, want: "hello world\n\n"},
		{name: "white-space:nowrap skips word wrap", html: `<p style="white-space:nowrap">one two three four five six</p>`, width: 20, want: "one two three four five six\n\n"},
		{name: "pre block skips word wrap", html: `<pre>one two three four five six</pre>`, width: 20, want: "one two three four five six\n"},
		{name: "blockquote text wraps inside border and padding", html: `<blockquote>one two three four five six</blockquote>`, width: 20, want: "│ one two three  \n│ four five six  \n"},
		{name: "explicit CSS width constrains wrap width", html: `<p style="width:10">hello world end</p>`, width: 40, want: "hello     \nworld end \n\n"},
		{name: "multi-line content from block-in-inline is not re-wrapped", html: `<blockquote><p>A</p><p>B</p></blockquote>`, width: 40, want: "│ A  \n│   \n│ B  \n\n"},
	})
}

func TestPositionRelative(t *testing.T) {
	runCases(t, []renderCase{
		{name: "position:relative with no offset is a no-op", html: `<div style="position:relative">hi</div>`, want: "hi\n"},
		{name: "top/left shift paints over a later sibling, original slot blanked", html: `<div style="position:relative;top:1;left:2">AB</div><div>1</div><div>2</div><div>3</div>`, width: 10, want: "  \n1 AB\n2\n3\n"},
	})
}

func TestPositionAbsoluteFixed(t *testing.T) {
	runCases(t, []renderCase{
		{name: "position:fixed anchors to the viewport and reserves no space in normal flow", html: `<div style="position:fixed;top:1;left:2;width:4">FX</div><div>1</div><div>2</div><div>3</div>`, width: 20, want: "1\n2 FX  \n3\n"},
		{name: "position:absolute with no positioned ancestor falls back to the document root", html: `<div style="position:absolute;top:0;left:0;width:3">AB</div>`, width: 10, want: "AB "},
	})
}

func TestBlockquoteBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{name: "blockquote heading and paragraph no extra trailing spaces", html: `<blockquote><h2>Title</h2><p>Body.</p></blockquote>`, want: "│ Title  \n│ Body.  \n\n"},
		{name: "two paragraphs in blockquote separated by one blank bordered line", html: `<blockquote><p>A</p><p>B</p></blockquote>`, want: "│ A  \n│   \n│ B  \n\n"},
	})
}

// TestVisibilityHiddenPreservesColumns is a regression test for
// blankLineVisible measuring in runes: hiding a double-width payload freed
// fewer columns than it occupied, sliding the following content left. That
// defeats the only reason to choose visibility:hidden over display:none.
func TestVisibilityHiddenPreservesColumns(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "hidden CJK still occupies its six columns",
			html: `<div><span style="visibility:hidden">日本語</span>|end</div>`,
			want: "      |end\n",
		},
		{
			name: "hidden emoji still occupies its two columns",
			html: `<div><span style="visibility:hidden">🔥</span>|end</div>`,
			want: "  |end\n",
		},
	})
}

// TestCustomListMarkerIndentsWrappedLinesByColumns is a regression test for
// listItemPrefixWidth measuring a custom list-style-type in runes: the
// hanging indent on a wrapped item came up short, so continuation lines
// didn't line up under the first line's content.
func TestCustomListMarkerIndentsWrappedLinesByColumns(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "emoji marker indents continuation to the content column",
			css:   `ul { list-style-type: "🔥 " }`,
			html:  `<ul><li>aaa bbb ccc ddd</li></ul>`,
			width: 18,
			// "    🔥 " is 4 + 2 + 1 = 7 columns, so the wrapped line is
			// indented by 7 spaces, not 6.
			want: "    🔥 aaa bbb ccc\n       ddd\n",
		},
	})
}

// TestEmptyBoxHasNoContentRow covers a block box whose content came out empty.
// In CSS such a box has zero content *height*: an empty bordered <div> draws
// its top and bottom rule adjacent with nothing between them, and a lone
// border-top — which is what <hr> is — is a single rule.
//
// Block content here is always at least one line, since that line is how block
// flow separates one box from the next, so an empty box has to give it up
// explicitly. It didn't: a bordered empty div rendered three rows instead of
// two, and `<hr style="width: 20">` two instead of one. The old special case
// tested the box's rendered text for emptiness at the point the rules are
// applied, by which time an explicit width, a side border, or horizontal
// padding had already filled that line with spaces.
func TestEmptyBoxHasNoContentRow(t *testing.T) {
	runCases(t, []renderCase{
		{name: "an empty bordered box's rules meet", width: 12, css: `.x{border-style:solid;width:5}`, html: `<div class="x"></div>`, want: "┌───┐\n└───┘\n"},
		{name: "the same without an explicit width", width: 12, css: `.x{border-style:solid}`, html: `<div class="x"></div>`, want: "┌──────────┐\n└──────────┘\n"},
		{name: "a lone rule is the whole box", width: 12, css: `.x{border-top:"-";width:5}`, html: `<div class="x"></div>`, want: "-----\n"},
		{name: "hr is a single rule at any width", width: 12, css: `hr{width:5}`, html: `<hr>`, want: "─────\n"},
		{name: "whitespace-only content is empty too", width: 12, css: `.x{border-style:solid;width:5}`, html: `<div class="x">   </div>`, want: "┌───┐\n└───┘\n"},
		// Padding rows are the box's own, and survive: CSS keeps them around a
		// zero-height content box. Only the content row goes.
		{name: "vertical padding rows survive", width: 12, css: `.x{border-style:solid;padding-top:1;padding-bottom:1;width:5}`, html: `<div class="x"></div>`, want: "┌───┐\n│   │\n│   │\n└───┘\n"},
		// A declared height or min-height is precisely the request to reserve
		// rows, so neither is dropped - including a min-height of exactly one,
		// which is what an empty box would otherwise look like.
		{name: "an explicit height still reserves its rows", width: 12, css: `.x{border-style:solid;height:2;width:5}`, html: `<div class="x"></div>`, want: "┌───┐\n│   │\n│   │\n└───┘\n"},
		{name: "min-height:1 still reserves its row", width: 12, css: `.x{border-style:solid;min-height:1;width:5}`, html: `<div class="x"></div>`, want: "┌───┐\n│   │\n└───┘\n"},
		// With nothing drawn around it, an empty box keeps its blank line rather
		// than collapsing to nothing - a zero-line box has no way to separate
		// itself from its siblings in block flow.
		{name: "an undecorated empty box keeps its line", width: 12, css: `.x{width:5}`, html: `<div class="x"></div>`, want: "     \n"},
		{name: "empty boxes still separate their siblings", width: 12, css: `.x{border-style:solid;width:5}`, html: `<div class="x"></div><div class="x"></div>`, want: "┌───┐\n└───┘\n┌───┐\n└───┘\n"},
		// A <textarea> is a field the user types into, sized in real HTML by its
		// `rows` attribute rather than by its content, so an empty one is not an
		// empty box - dropping its row would leave an editable box with nowhere
		// to show the caret.
		{name: "an empty textarea keeps its field row", width: 14, html: `<textarea></textarea>`, want: "┌────────────┐\n│            │\n└────────────┘\n"},
	})
}

// TestEmptyFlexContainerBox is TestEmptyBoxHasNoContentRow for a flex container
// with no items, which had the same phantom content row — plus a second bug the
// first one hid: the early return for "no items" skipped the height-reservation
// passes entirely, so an empty flex container ignored its own declared height.
func TestEmptyFlexContainerBox(t *testing.T) {
	runCases(t, []renderCase{
		{name: "an empty flex container's rules meet", width: 12, css: `.f{display:flex;border-style:solid;width:6}`, html: `<div class="f"></div>`, want: "┌────┐\n└────┘\n"},
		{name: "column direction is the same", width: 12, css: `.f{display:flex;flex-direction:column;border-style:solid;width:6}`, html: `<div class="f"></div>`, want: "┌────┐\n└────┘\n"},
		// Loose text is an anonymous flex item, so a container holding only text
		// is not an empty container. "loose" is unbreakable and wider than the
		// four content columns this container has, so it overflows its own
		// border, the same graceful overflow any bordered block gives content
		// it can't fit.
		{name: "a container of only loose text holds an anonymous item", width: 12, css: `.f{display:flex;border-style:solid;width:6}`, html: `<div class="f">loose</div>`, want: "┌────┐\n│loose│\n└────┘\n"},
		{name: "a container of only whitespace is empty", width: 12, css: `.f{display:flex;border-style:solid;width:6}`, html: "<div class=\"f\">\n  \n</div>", want: "┌────┐\n└────┘\n"},
		{name: "a container whose only item is display:none is empty", width: 12, css: `.f{display:flex;border-style:solid;width:6}`, html: `<div class="f"><span style="display:none">x</span></div>`, want: "┌────┐\n└────┘\n"},
		{name: "an empty container still reserves its declared height", width: 12, css: `.f{display:flex;border-style:solid;height:2;width:6}`, html: `<div class="f"></div>`, want: "┌────┐\n│    │\n│    │\n└────┘\n"},
		{name: "and its min-height", width: 12, css: `.f{display:flex;border-style:solid;min-height:2;width:6}`, html: `<div class="f"></div>`, want: "┌────┐\n│    │\n│    │\n└────┘\n"},
		{name: "an empty flex item is a two-row box on its line", width: 14, css: `.f{display:flex;border-style:solid;width:12}.i{display:flex;border-style:solid}`, html: `<div class="f"><div class="i"></div><span>x</span></div>`, want: "┌──────────┐\n│┌─┐x      │\n│└─┘       │\n└──────────┘\n"},
	})
}
