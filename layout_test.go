package htmlterm_test

import "testing"

func TestBorderStyleOnBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-style:solid draws full box", html: `<div style="border-style:solid; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
		{name: "border-style:rounded draws rounded box", html: `<div style="border-style:rounded; width:100%">hi</div>`, width: 8, want: "╭──────╮\n│hi    │\n╰──────╯\n"},
		{name: "border-style:thick draws thick box", html: `<div style="border-style:thick; width:100%">hi</div>`, width: 8, want: "┏━━━━━━┓\n┃hi    ┃\n┗━━━━━━┛\n"},
		{name: "border-style:double draws double box", html: `<div style="border-style:double; width:100%">hi</div>`, width: 8, want: "╔══════╗\n║hi    ║\n╚══════╝\n"},
		{name: "border-style:markdown draws only left/right bars", html: `<div style="border-style:markdown">hi</div>`, want: "|hi|\n"},
		{name: "border-style:hidden draws no borders", html: `<div style="border-style:hidden">hi</div>`, want: "hi\n"},
		{name: "border-style:none draws no borders", html: `<div style="border-style:none">hi</div>`, want: "hi\n"},
		{name: "individual border-top overrides preset fill but keeps corners", html: `<div style="border-style:solid; border-top:'═'; width:100%">hi</div>`, width: 8, want: "┌══════┐\n│hi    │\n└──────┘\n"},
		{name: "individual border-left overrides preset char", html: `<div style="border-style:solid; border-left:'▌'; width:100%">hi</div>`, width: 8, want: "┌──────┐\n▌hi    │\n└──────┘\n"},
		{name: "border-style via CSS class", css: `.box { border-style: rounded; width: 100%; }`, html: `<div class="box">ok</div>`, width: 8, want: "╭──────╮\n│ok    │\n╰──────╯\n"},
		{name: "border-width and per-edge widths are accepted no-ops", html: `<div style="border-style:solid; border-width:2px; border-top-width:1px; border-right-width:1px; border-bottom-width:1px; border-left-width:1px; width:100%">hi</div>`, width: 8, want: "┌──────┐\n│hi    │\n└──────┘\n"},
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
		{name: "multi-line content from block-in-inline is not re-wrapped", html: `<blockquote><p>A</p><p>B</p></blockquote>`, width: 40, want: "│ A  \n│   \n│ B  \n"},
	})
}

func TestBlockquoteBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{name: "blockquote heading and paragraph no extra trailing spaces", html: `<blockquote><h2>Title</h2><p>Body.</p></blockquote>`, want: "│ Title  \n│ Body.  \n"},
		{name: "two paragraphs in blockquote separated by one blank bordered line", html: `<blockquote><p>A</p><p>B</p></blockquote>`, want: "│ A  \n│   \n│ B  \n"},
	})
}
