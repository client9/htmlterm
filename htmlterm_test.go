package htmlterm_test

import (
	"regexp"
	"testing"

	"github.com/nickg/htmlterm"
)

// ansiRe strips ANSI CSI escape sequences (colors, bold, etc.) and OSC
// sequences (terminal hyperlinks). Tests compare plain text only.
var ansiRe = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// renderCase is one table-driven test case.
type renderCase struct {
	name  string // sub-test name
	css   string // CSS appended after UA defaults (may be empty)
	html  string
	width int    // terminal width; 0 defaults to 40
	want  string // expected plain-text output after ANSI stripping
}

// runCases is the shared driver for all render test tables.
func runCases(t *testing.T, cases []renderCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			width := tc.width
			if width == 0 {
				width = 40
			}
			r, err := htmlterm.New(tc.css, width)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := r.Render(tc.html)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			got = stripANSI(got)
			if got != tc.want {
				t.Errorf("html: %s\ngot:  %q\nwant: %q", tc.html, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bare text at block level (<html> / <body> direct children)
// ---------------------------------------------------------------------------

func TestBareText(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "bare text in html element",
			html: `<html>Hello World</html>`,
			want: "Hello World",
		},
		{
			name: "bare text in body element",
			html: `<body>hello</body>`,
			want: "hello",
		},
		{
			name: "bare text mixed with block element",
			html: `<body>before<p>paragraph</p>after</body>`,
			want: "beforeparagraph\n\nafter",
		},
		{
			name: "whitespace-only text between elements is ignored",
			html: "<body>\n<p>text</p>\n</body>",
			want: "text\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// display: block
// ---------------------------------------------------------------------------

func TestDisplay_Block(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "p is block with trailing newline",
			html: `<p>hello</p>`,
			want: "hello\n\n", // block \n + UA margin-bottom:1
		},
		{
			name: "adjacent p elements are separated by blank line",
			html: `<p>one</p><p>two</p>`,
			want: "one\n\ntwo\n\n", // first p margin-bottom:1 creates blank line
		},
		{
			name: "h1 is block without extra margin",
			html: `<h1>Title</h1>`,
			want: "Title\n", // display:block, no margin-bottom in UA
		},
		{
			name: "div is block",
			html: `<div>content</div>`,
			want: "content\n",
		},
		{
			name: "section is block",
			html: `<section>sec</section>`,
			want: "sec\n",
		},
		{
			name: "h1 followed by p",
			html: `<h1>Title</h1><p>Body</p>`,
			want: "Title\nBody\n\n", // no blank line between h1 and p (h1 has no mb)
		},
		{
			name: "multiple headings",
			html: `<h1>H1</h1><h2>H2</h2><h3>H3</h3>`,
			want: "H1\nH2\nH3\n",
		},
		{
			name: "blockquote is block",
			html: `<blockquote>quoted</blockquote>`,
			want: "│ quoted  \n", // display:block + UA border-left:│ + padding-left:1 + padding-right:2
		},
		{
			name: "CSS display:block on custom element",
			css:  `span { display: block; }`,
			html: `<span>line one</span><span>line two</span>`,
			want: "line one\nline two\n",
		},
		{
			name: "display:inline overrides UA block on p",
			css:  `p { display: inline; }`,
			html: `<p>hello</p><p>world</p>`,
			want: "helloworld", // both inline, no newlines
		},
	})
}

// ---------------------------------------------------------------------------
// display: inline
// ---------------------------------------------------------------------------

func TestDisplay_Inline(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "span is inline at body level (no newline)",
			html: `<span>hello</span>`,
			want: "hello",
		},
		{
			name: "adjacent inline spans concatenate",
			html: `<span>foo</span><span>bar</span>`,
			want: "foobar",
		},
		{
			name: "inline elements inside block",
			html: `<p>hello <strong>world</strong></p>`,
			want: "hello world\n\n",
		},
		{
			name: "multiple inline tags inside block",
			html: `<p><em>a</em> <strong>b</strong> c</p>`,
			want: "a b c\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// display: none
// ---------------------------------------------------------------------------

func TestDisplay_None(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "display:none via inline style hides element",
			html: `<p>visible</p><p style="display:none">hidden</p><p>after</p>`,
			want: "visible\n\nafter\n\n",
		},
		{
			name: "display:none via CSS class",
			css:  `.gone { display: none; }`,
			html: `<p>a</p><p class="gone">b</p><p>c</p>`,
			want: "a\n\nc\n\n",
		},
		{
			name: "display:none hides inline element",
			html: `<p>before <span style="display:none">hidden</span> after</p>`,
			want: "before  after\n\n", // double space: text nodes before and after
		},
		{
			name: "display:none on block in sequence",
			html: `<div>a</div><div style="display:none">b</div><div>c</div>`,
			want: "a\nc\n",
		},
	})
}

// ---------------------------------------------------------------------------
// display: inline-block
// ---------------------------------------------------------------------------

func TestDisplay_InlineBlock(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "inline-block with fixed width pads content",
			html: `<p><span style="display:inline-block; width:8">hi</span>end</p>`,
			want: "hi      end\n\n", // "hi" padded to 8 chars (6 trailing spaces)
		},
		{
			name: "inline-block without width acts like inline",
			html: `<p><span style="display:inline-block">hi</span>end</p>`,
			want: "hiend\n\n",
		},
		{
			name: "two inline-block spans side by side",
			html: `<p><span style="display:inline-block; width:5">A</span><span style="display:inline-block; width:5">B</span></p>`,
			want: "A    B    \n\n", // each padded to 5 chars
		},
	})
}

// ---------------------------------------------------------------------------
// Margins
// ---------------------------------------------------------------------------

func TestMargins(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "UA p has margin-bottom:1",
			html: `<p>text</p>`,
			want: "text\n\n", // \n from block + \n from margin-bottom:1
		},
		{
			name: "margin-top ignored on first element (nothing above)",
			css:  `p { margin-top: 2; }`,
			html: `<p>first</p>`,
			want: "first\n\n", // mt ignored when sb is empty
		},
		{
			name: "margin-top adds space before element",
			css:  `p { margin-top: 1; }`,
			html: `<div>above</div><p>below</p>`,
			// div: "above\n" (1 trailing \n)
			// p mt=1 → writeMarginNewlines(sb, 2): have 1, need 2, add 1 → "above\n\n"
			// render "below\n" + mb=1 → "above\n\nbelow\n\n"
			want: "above\n\nbelow\n\n",
		},
		{
			name: "margin collapse: equal margins produce one blank line",
			css:  `div { margin-bottom: 1; } p { margin-top: 1; }`,
			html: `<div>above</div><p>below</p>`,
			// div mb=1 → "above\n\n" (2 trailing \n)
			// p mt=1 → need 2, have 2 → nothing added
			// result: "above\n\nbelow\n\n"
			want: "above\n\nbelow\n\n",
		},
		{
			name: "margin collapse: larger wins",
			css:  `div { margin-bottom: 2; } p { margin-top: 1; }`,
			html: `<div>above</div><p>below</p>`,
			// div mb=2 → need 3 trailing → "above\n\n\n"
			// p mt=1 → need 2, have 3 → nothing added
			// result: "above\n\n\nbelow\n\n"
			want: "above\n\n\nbelow\n\n",
		},
		{
			name: "custom margin-bottom on element",
			css:  `h2 { margin-bottom: 2; }`,
			html: `<h2>heading</h2><p>body</p>`,
			// h2 mb=2 → "heading\n\n\n" (3 trailing \n)
			// p mt=0, render "body\n" + mb=1 → "heading\n\n\nbody\n\n"
			want: "heading\n\n\nbody\n\n",
		},
		{
			name: "margin-bottom on last element still applies",
			html: `<p>last</p>`,
			want: "last\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// Padding
// ---------------------------------------------------------------------------

func TestPadding(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "padding-left indents content",
			css:  `div { padding-left: 4; }`,
			html: `<div>hello</div>`,
			want: "    hello\n",
		},
		{
			name: "blockquote has UA border-left and padding",
			html: `<blockquote>quoted</blockquote>`,
			want: "│ quoted  \n", // UA border-left:│ + padding-left:1 + padding-right:2
		},
	})
}

// ---------------------------------------------------------------------------
// Vertical box model: padding-top, padding-bottom, height
// ---------------------------------------------------------------------------

func TestVerticalBoxModel(t *testing.T) {
	runCases(t, []renderCase{
		{
			// Blank padding row is innerW (4) spaces wide, matching content area.
			name: "padding-top inserts blank row above content",
			css:  `div { padding-top: 1; width: 4; }`,
			html: `<div>hi</div>`,
			want: "    \nhi  \n",
		},
		{
			name: "padding-bottom inserts blank row below content",
			css:  `div { padding-bottom: 1; width: 4; }`,
			html: `<div>hi</div>`,
			want: "hi  \n    \n",
		},
		{
			name: "padding-top and bottom both present",
			css:  `div { padding-top: 2; padding-bottom: 1; width: 4; }`,
			html: `<div>text</div>`,
			want: "    \n    \ntext\n    \n",
		},
		{
			// With border-left:│ and width:5, innerW=4; blank row = 4 spaces, bordered.
			name: "padding-top inside border-left",
			css:  `div { border-left: │; padding-top: 1; width: 5; }`,
			html: `<div>hi</div>`,
			want: "│    \n│hi  \n",
		},
		{
			name: "padding-bottom inside border-left",
			css:  `div { border-left: │; padding-bottom: 1; width: 5; }`,
			html: `<div>hi</div>`,
			want: "│hi  \n│    \n",
		},
		{
			name: "height expands short content with blank lines",
			css:  `div { height: 3; width: 5; }`,
			html: `<div>x</div>`,
			want: "x    \n     \n     \n",
		},
		{
			name: "height clips overflow with overflow:hidden",
			css:  `div { height: 1; overflow: hidden; }`,
			html: "<div>a<br>b</div>",
			want: "a\n",
		},
		{
			name: "height no-ops when content already fills height",
			css:  `div { height: 2; width: 5; }`,
			html: "<div>a<br>b</div>",
			want: "a    \nb    \n",
		},
	})
}

// ---------------------------------------------------------------------------
// block borders
// ---------------------------------------------------------------------------

func TestBlockBorders(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "border-left adds char to each line",
			html: `<p style="border-left:│">hello</p>`,
			want: "│hello\n\n",
		},
		{
			name: "border-left with color strips clean",
			html: `<p style="border-left:│; border-left-color:#ff0000">hi</p>`,
			want: "│hi\n\n",
		},
		{
			name: "border-right appends char",
			html: `<p style="border-right:│">hello</p>`,
			want: "hello│\n\n",
		},
		{
			name: "border-left and right together",
			html: `<p style="border-left:│; border-right:│">hello</p>`,
			want: "│hello│\n\n",
		},
		{
			name: "border-left none disables",
			html: `<p style="border-left:none">hello</p>`,
			want: "hello\n\n",
		},
		{
			name: "border-left on multiline pre",
			html: "<pre style=\"border-left:│\">line1\nline2</pre>",
			want: "│line1\n│line2\n",
		},
		{
			name: "border-left via CSS class",
			css:  `.note { border-left: ▌; }`,
			html: `<p class="note">text</p>`,
			want: "▌text\n\n",
		},
		{
			name: "blockquote UA has border-left and padding",
			html: `<blockquote>quoted</blockquote>`,
			want: "│ quoted  \n",
		},
		{
			name: "margin-left adds spaces outside border",
			html: `<p style="margin-left:4; border-left:|; padding-left:1">hi</p>`,
			want: "    | hi\n\n",
		},
		{
			name: "margin-right adds spaces outside border",
			html: `<p style="margin-right:4; border-right:|; padding-right:1">hi</p>`,
			want: "hi |    \n\n",
		},
		{
			name: "margin-left only no border",
			html: `<p style="margin-left:2">hello</p>`,
			want: "  hello\n\n",
		},
		{
			name: "nested border-left stacks characters",
			html: `<div style="border-left:|"><div style="border-left:|">inner</div></div>`,
			want: "||inner\n",
		},
		{
			name:  "width:100% fills renderer width with borders",
			html:  `<p style="width:100%; border-left:[; border-right:]">hi</p>`,
			want:  "[hi                  ]\n\n",
			width: 22,
		},
		{
			name:  "width:100% with margin subtracts margin from line width",
			html:  `<p style="width:100%; margin-left:2; margin-right:2; border-left:[; border-right:]">hi</p>`,
			want:  "  [hi                  ]  \n\n",
			width: 26,
		},
		{
			name:  "border-top draws horizontal rule before content",
			html:  `<p style="border-top:─">hi</p>`,
			width: 10,
			want:  "──────────\nhi\n\n",
		},
		{
			name:  "border-bottom draws horizontal rule after content",
			html:  `<p style="border-bottom:─">hi</p>`,
			width: 10,
			want:  "hi\n──────────\n\n",
		},
		{
			name:  "border-top none is disabled",
			html:  `<p style="border-top:none">hi</p>`,
			width: 10,
			want:  "hi\n\n",
		},
		{
			name:  "all four borders with width:100%",
			html:  `<p style="width:100%; border-top:─; border-bottom:─; border-left:[; border-right:]">hi</p>`,
			width: 12,
			// totalW=12, inner=12-0-1-1-0=10, hBorderWidth=10+1+1=12
			// content = "[hi        ]" (12 chars)
			// top/bot = "────────────" (12 × ─)
			want: "────────────\n[hi        ]\n────────────\n\n",
		},
		{
			name:  "corners replace fill endpoints on top and bottom rules",
			html:  `<p style="width:100%; border-top:─; border-bottom:─; border-left:│; border-right:│; border-top-left-corner:┌; border-top-right-corner:┐; border-bottom-left-corner:└; border-bottom-right-corner:┘">hi</p>`,
			width: 12,
			// totalW=12, inner=10, hBorderWidth=12
			// top  = "┌──────────┐" (corner + 10×─ + corner)
			// rows = "│hi        │"
			// bot  = "└──────────┘"
			want: "┌──────────┐\n│hi        │\n└──────────┘\n\n",
		},
		{
			name:  "corner without opposite border uses fill for that side",
			html:  `<p style="width:100%; border-top:─; border-top-left-corner:┌">hi</p>`,
			width: 12,
			// only left corner set; right end uses fill (─); content padded to 12 by width:100%
			want: "┌───────────\nhi          \n\n",
		},
		{
			name:  "border-top and border-bottom with margin-left",
			html:  `<p style="width:100%; border-top:─; border-bottom:─; margin-left:2">hi</p>`,
			width: 12,
			// totalW=12, ml=2, mr=0, inner=12-2-0-0-0=10, hBorderWidth=10
			// content = "hi        " (10 chars wide via st.Width(10))
			// top/bot = "──────────" (10 × ─)
			// applyLineEdges(ml=2): prepend "  " to every line
			want: "  ──────────\n  hi        \n  ──────────\n\n",
		},
		{
			name:  "border-top with color strips clean",
			html:  `<p style="border-top:─; border-top-color:#ff0000">hi</p>`,
			width: 10,
			want:  "──────────\nhi\n\n",
		},
		// margin-left: auto / margin-right: auto
		{
			name:  "margin auto both centers element",
			html:  `<p style="width:10; margin-left:auto; margin-right:auto">hi</p>`,
			width: 20,
			// hBorderWidth=10, remaining=10, ml=5, mr=5
			// content="hi        " (Width(10)), line="     hi             " (5+10+5)
			want: "     hi             \n\n",
		},
		{
			name:  "margin-left auto pushes element to right",
			html:  `<p style="width:10; margin-left:auto">hi</p>`,
			width: 20,
			// hBorderWidth=10, remaining=10, ml=10, mr=0
			// content="hi        " (Width(10)), line="          hi        " (10+10)
			want: "          hi        \n\n",
		},
		{
			name:  "margin-right auto fills trailing space",
			html:  `<p style="width:10; margin-right:auto">hi</p>`,
			width: 20,
			// hBorderWidth=10, remaining=10, ml=0, mr=10
			// content="hi        " (Width(10)), line="hi                  " (10+10)
			want: "hi                  \n\n",
		},
		{
			name:  "margin auto with percent width centers",
			html:  `<p style="width:50%; margin-left:auto; margin-right:auto">hi</p>`,
			width: 20,
			// totalW=10, hBorderWidth=10, remaining=10, ml=5, mr=5
			want: "     hi             \n\n",
		},
		{
			name:  "margin-left and margin-right as percentages",
			html:  `<p style="margin-left:25%; margin-right:25%">hi</p>`,
			width: 80,
			// ml=20, mr=20, innerW=40, content word-wrapped to 40 chars
			want: "                    hi                    \n\n",
		},
		{
			name:  "margin auto without explicit width is ignored",
			html:  `<p style="margin-left:auto; margin-right:auto">hi</p>`,
			width: 20,
			// no width set → element fills available, auto margins = 0
			want: "hi\n\n",
		},
		{
			name:  "margin auto center via CSS class",
			css:   `.center { width: 10; margin-left: auto; margin-right: auto; }`,
			html:  `<p class="center">hi</p>`,
			width: 20,
			want:  "     hi             \n\n",
		},
		{
			name:  "margin auto center with borders",
			html:  `<p style="width:12; margin-left:auto; margin-right:auto; border-left:[; border-right:]">hi</p>`,
			width: 20,
			// hBorderWidth=12 ([ + 10 content + ]), remaining=8, ml=4, mr=4
			want: "    [hi        ]    \n\n",
		},
		// Right-edge alignment: when word-wrap produces multiple lines of varying
		// length, border-right and padding-right must form a straight vertical edge.
		{
			name:  "border-right aligns on word-wrapped lines",
			html:  `<p style="border-right:|">one two three four five</p>`,
			width: 14,
			// innerW = 14 - 1(border-right) = 13
			// word-wrap at 13: "one two three" (13), "four five" (9)
			// "four five" padded to 13 before border appended
			want: "one two three|\nfour five    |\n\n",
		},
		{
			name:  "padding-right aligns on word-wrapped lines",
			html:  `<p style="padding-right:2">one two three four five</p>`,
			width: 15,
			// innerW = 15 - 2(padding-right) = 13
			// word-wrap at 13: "one two three" (13), "four five" (9)
			// "four five" padded to 13 then "  " appended
			want: "one two three  \nfour five      \n\n",
		},
		{
			name:  "border-right single-line content stays flush",
			html:  `<p style="border-right:|">hello</p>`,
			width: 20,
			// single line — no padding, border is flush with content
			want: "hello|\n\n",
		},
		{
			name:  "border-right with margin-right aligns on wrapped lines",
			html:  `<p style="border-right:|; margin-right:2">one two three four five</p>`,
			width: 16,
			// innerW = 16 - 1(border-right) - 2(margin-right) = 13
			// word-wrap at 13: "one two three" (13), "four five" (9)
			// "four five" padded to 13 → border → margin-right "  "
			want: "one two three|  \nfour five    |  \n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// white-space
// ---------------------------------------------------------------------------

func TestWhiteSpace(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "normal collapses multiple spaces",
			html: `<p>hello   world</p>`,
			want: "hello world\n\n",
		},
		{
			name: "normal collapses newlines to space",
			html: "<p>hello\nworld</p>",
			want: "hello world\n\n",
		},
		{
			name: "normal collapses tabs",
			html: "<p>hello\tworld</p>",
			want: "hello world\n\n",
		},
		{
			name: "pre preserves newlines",
			html: "<pre>hello\n  world</pre>",
			want: "hello\n  world\n",
		},
		{
			name: "pre preserves multiple spaces",
			html: "<pre>a   b</pre>",
			want: "a   b\n",
		},
		{
			name: "nowrap collapses whitespace",
			css:  `p { white-space: nowrap; }`,
			html: "<p>hello\n  world</p>",
			want: "hello world\n\n",
		},
		{
			name: "pre-line preserves newlines but collapses spaces",
			css:  `p { white-space: pre-line; }`,
			html: "<p>hello\n  world</p>",
			want: "hello\n world\n\n", // \n kept, two spaces collapsed to one
		},
		{
			name: "white-space inherited by child inline elements",
			html: "<pre><span>hello   world</span></pre>",
			want: "hello   world\n", // pre sets white-space:pre, span inherits
		},
		{
			name: "pre-wrap preserves all whitespace",
			css:  `p { white-space: pre-wrap; }`,
			html: "<p>hello   world\n  end</p>",
			want: "hello   world\n  end\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// text-overflow (table cells)
// ---------------------------------------------------------------------------
//
// These tests use border-style:hidden (no outer frame, space separator)
// to keep expected strings simple.

func TestTextOverflow(t *testing.T) {
	// Helper: wraps content in a single-cell hidden-border table.
	cell := func(attrs, content string) string {
		return `<table style="border-style:hidden"><tr><td ` + attrs + `>` + content + `</td></tr></table>`
	}

	runCases(t, []renderCase{
		{
			name: "ellipsis (default) truncates with …",
			html: cell(`width="5"`, "Hello World"),
			want: "Hell…\n",
		},
		{
			name: "clip truncates without marker",
			html: cell(`style="text-overflow:clip" width="5"`, "Hello World"),
			want: "Hello\n",
		},
		{
			name: "custom string marker",
			html: cell(`style='text-overflow:"+"' width="5"`, "Hello World"),
			want: "Hell+\n",
		},
		{
			name: "no truncation when content fits",
			html: cell(`width="11"`, "Hello World"),
			want: "Hello World\n",
		},
		{
			name: "ellipsis on exact fit needs no truncation",
			html: cell(`width="5"`, "Hello"),
			want: "Hello\n",
		},
		{
			name: "clip width=1 takes first rune",
			html: cell(`style="text-overflow:clip" width="1"`, "Hello"),
			want: "H\n",
		},
		{
			name: "text-overflow via CSS class",
			css:  `.clip td { text-overflow: clip; }`,
			html: `<table class="clip" style="border-style:hidden"><tr><td width="5">Hello World</td></tr></table>`,
			want: "Hello\n",
		},
	})
}

// ---------------------------------------------------------------------------
// Table cell padding
// ---------------------------------------------------------------------------

func TestTableCellPadding(t *testing.T) {
	// All cases use border-style:hidden (no outer frame, space separator between
	// columns) to keep expected strings focused on padding behaviour.
	hidden := `style="border-style:hidden"`

	runCases(t, []renderCase{
		// --- horizontal padding ---
		{
			name: "padding-left indents cell content",
			// width=6, pl=1 → contentW=5; Width(5).Render("ab")="ab   "; " "+"ab   "=" ab   "
			html: `<table ` + hidden + `><tr><td style="padding-left:1" width="6">ab</td></tr></table>`,
			want: " ab   \n",
		},
		{
			name: "padding-right adds space after cell content",
			// width=6, pr=1 → contentW=5; Width(5).Render("ab")="ab   "; "ab   "+" "="ab    "
			html: `<table ` + hidden + `><tr><td style="padding-right:1" width="6">ab</td></tr></table>`,
			want: "ab    \n",
		},
		{
			name: "padding-left and padding-right both set",
			// width=7, pl=1, pr=1 → contentW=5; Width(5).Render("ab")="ab   "; " ab    "
			html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1" width="7">ab</td></tr></table>`,
			want: " ab    \n",
		},
		{
			name: "natural width includes padding when no explicit width set",
			// natural = runeLen("ab") + pl(1) + pr(1) = 4; widths[0]=4; contentW=2
			// Width(2).Render("ab")="ab"; " ab " (4 wide)
			html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1">ab</td></tr></table>`,
			want: " ab \n",
		},
		{
			name: "padding-left truncates content to reduced content width",
			// width=5, pl=1 → contentW=4; "Hello"(5) truncated to 4: "Hel…"; " Hel…"
			html: `<table ` + hidden + `><tr><td style="padding-left:1" width="5">Hello</td></tr></table>`,
			want: " Hel…\n",
		},
		// --- vertical padding ---
		{
			name: "padding-top adds blank line above content",
			// width=5, pt=1 → lines=["","ab"]; height=2
			// line0: Width(5).Render("")="     "; line1: Width(5).Render("ab")="ab   "
			html: `<table ` + hidden + `><tr><td style="padding-top:1" width="5">ab</td></tr></table>`,
			want: "     \nab   \n",
		},
		{
			name: "padding-bottom adds blank line below content",
			// width=5, pb=1 → lines=["ab",""]; height=2
			html: `<table ` + hidden + `><tr><td style="padding-bottom:1" width="5">ab</td></tr></table>`,
			want: "ab   \n     \n",
		},
		{
			name: "padding-top 2 adds two blank lines above",
			html: `<table ` + hidden + `><tr><td style="padding-top:2" width="4">X</td></tr></table>`,
			want: "    \n    \nX   \n",
		},
		// --- combined padding ---
		{
			name: "padding-top with padding-left",
			// width=6, pl=1, pt=1 → contentW=5; lines=["","ab"]
			// line0: " "+"     "="      " (6); line1: " "+"ab   "=" ab   " (6)
			html: `<table ` + hidden + `><tr><td style="padding-top:1; padding-left:1" width="6">ab</td></tr></table>`,
			want: "      \n ab   \n",
		},
		{
			name: "all four sides of padding",
			// width=7, pl=1, pr=1, pt=1, pb=1 → contentW=5; lines=["","ab",""]
			// line0: " "+"     "+" "="       " (7)
			// line1: " "+"ab   "+" "=" ab    " (7)
			// line2: " "+"     "+" "="       " (7)
			html: `<table ` + hidden + `><tr><td style="padding-left:1; padding-right:1; padding-top:1; padding-bottom:1" width="7">ab</td></tr></table>`,
			want: "       \n ab    \n       \n",
		},
		// --- padding in multi-column row ---
		{
			name: "padding-top in one cell raises row height for sibling",
			// cell0: width=3, pt=1; lines=["","X"]; height=2
			// cell1: width=3, no padding; lines=["Y"]; height stays 2 from cell0
			// Cells are top-aligned: cell1 renders "Y" at line0, blank at line1.
			// line0: "   "(cell0 blank)+" "+"Y  "(cell1) = "    Y  " (7)
			// line1: "X  "(cell0)+" "+"   "(cell1 blank) = "X      " (7)
			html: `<table ` + hidden + `><tr><td style="padding-top:1" width="3">X</td><td width="3">Y</td></tr></table>`,
			want: "    Y  \nX      \n",
		},
		// --- padding on th header cells ---
		{
			name: "padding-left on th header",
			// th: width=7, pl=1 → " Name  " (contentW=6); td: width=7, no pad → "val    "
			html: `<table ` + hidden + `><tr><th style="padding-left:1" width="7">Name</th></tr><tr><td width="7">val</td></tr></table>`,
			want: " Name  \nval    \n",
		},
		{
			name: "padding-top on th header adds blank row before header text",
			// th: width=4, pt=1 → lines=["","Hi"]; height=2
			// header has 2 lines; data row has 1 line
			// hdr line0: "    "; hdr line1: "Hi  "; data line0: "ok  "
			html: `<table ` + hidden + `><tr><th style="padding-top:1" width="4">Hi</th></tr><tr><td width="4">ok</td></tr></table>`,
			want: "    \nHi  \nok  \n",
		},
		// --- padding interacts with wrapping (white-space:normal) ---
		{
			name: "padding-left with wrapping cell",
			// width=7, pl=1 → contentW=6; "Hello World" wraps to ["Hello","World"]
			// line0: " "+"Hello "=" Hello " (7); line1: " "+"World "=" World " (7)
			html: `<table ` + hidden + `><tr><td style="padding-left:1; white-space:normal" width="7">Hello World</td></tr></table>`,
			want: " Hello \n World \n",
		},
	})
}

// ---------------------------------------------------------------------------
// Table layout
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// overflow + text-overflow (block elements)
// ---------------------------------------------------------------------------

func TestBlockOverflow(t *testing.T) {
	runCases(t, []renderCase{
		{
			// Without overflow, nowrap alone lets text exceed the box width.
			name:  "nowrap without overflow: text overflows width",
			css:   `p { white-space: nowrap; width: 5; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hello World\n\n",
		},
		{
			// overflow:hidden + width clips content (default text-overflow is clip, no marker).
			name:  "overflow:hidden clips to width (default text-overflow:clip)",
			css:   `p { overflow: hidden; white-space: nowrap; width: 5; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hello\n\n",
		},
		{
			// overflow:hidden + text-overflow:ellipsis adds the ellipsis marker.
			name:  "overflow:hidden with text-overflow:ellipsis",
			css:   `p { overflow: hidden; white-space: nowrap; width: 5; text-overflow: ellipsis; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hell…\n\n",
		},
		{
			// overflow:clip is an alias for hidden.
			name:  "overflow:clip with text-overflow:ellipsis",
			css:   `p { overflow: clip; white-space: nowrap; width: 5; text-overflow: ellipsis; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hell…\n\n",
		},
		{
			// overflow:hidden without nowrap: word-wrap still runs; overflow then
			// clips only lines that are still too long (e.g. a long unbreakable word).
			name:  "overflow:hidden with normal white-space clips long word",
			css:   `p { overflow: hidden; width: 5; text-overflow: ellipsis; }`,
			html:  `<p>Superlongword</p>`,
			width: 20,
			want:  "Supe…\n\n",
		},
		{
			// overflow:hidden without an explicit width has no effect.
			name:  "overflow:hidden without width does not clip",
			css:   `p { overflow: hidden; white-space: nowrap; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hello World\n\n",
		},
		{
			// Custom text-overflow string.
			name:  "overflow:hidden with custom text-overflow string",
			css:   `p { overflow: hidden; white-space: nowrap; width: 6; text-overflow: "+"; }`,
			html:  `<p>Hello World</p>`,
			width: 20,
			want:  "Hello+\n\n",
		},
	})
}

// ---------------------------------------------------------------------------

func TestTable(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "two-column hidden-border table",
			html:  `<table style="border-style:hidden"><tr><td width="3">A</td><td width="5">Hello</td></tr></table>`,
			width: 40,
			// widths=[3,5], overhead=0+(2-1)*1+0=1 (one space sep between cols)
			// wait: left="" right="" sep=" ", overhead = 0 + 1*1 + 0 = 1
			// col A: "A  " (padded to 3), sep " ", col Hello: "Hello"
			want: "A   Hello\n",
		},
		{
			name:  "normal border style: single header and data row",
			html:  `<table><tr><th width="3">H1</th><th width="4">H2</th></tr><tr><td>A</td><td>Long</td></tr></table>`,
			width: 40,
			want:  "┌───┬────┐\n│H1 │H2  │\n├───┼────┤\n│A  │Long│\n└───┴────┘\n",
		},
		{
			name:  "table width:100% expands flexible column",
			css:   `table { width: 100%; border-style: hidden; }`,
			html:  `<table><tr><td width="5">fixed</td><td>flex</td></tr></table>`,
			width: 20,
			// overhead: left="" sep=" " right="" → overhead = 0+1+0 = 1
			// contentWidth = 20-1 = 19
			// col0: fixed=5, col1: flexible, natural=4
			// After fixed: total=5, flex gets 19-5=14 → col1 width=14
			want: "fixed flex          \n",
		},
	})
}

// ---------------------------------------------------------------------------
// Multi-line table cells (white-space: normal opt-in wrapping)
// ---------------------------------------------------------------------------

func TestTableMultiLine(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "white-space:normal wraps cell content",
			html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="5">Hello World</td></tr></table>`,
			// wrapToWidth("Hello World", 5) → ["Hello", "World"]
			want: "Hello\nWorld\n",
		},
		{
			name: "white-space:nowrap still truncates",
			html: `<table style="border-style:hidden"><tr><td style="white-space:nowrap" width="5">Hello World</td></tr></table>`,
			want: "Hell…\n",
		},
		{
			name: "multi-column row where one cell wraps",
			html: `<table style="border-style:hidden"><tr><td width="3">A</td><td style="white-space:normal" width="5">Hi there</td></tr></table>`,
			// col0: "A" (nowrap UA default) → ["A"], col1: "Hi there" wraps → ["Hi", "there"]
			// height=2
			// line0: "A  "(pad3) + " " + "Hi   "(pad5) = "A   Hi   "
			// line1: "   "(empty,pad3) + " " + "there" = "    there"
			want: "A   Hi   \n    there\n",
		},
		{
			name: "long word is hard-broken",
			html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="4">Superlongword</td></tr></table>`,
			// "Superlongword" (13 runes) hard-breaks at width=4 → ["Supe","rlon","gwor","d"]
			want: "Supe\nrlon\ngwor\nd   \n",
		},
		{
			name: "short content still fits on one line",
			html: `<table style="border-style:hidden"><tr><td style="white-space:normal" width="10">Hello</td></tr></table>`,
			want: "Hello     \n",
		},
		{
			name: "wrapping with bordered table",
			html: `<table><tr><th width="5">Name</th></tr><tr><td style="white-space:normal" width="5">Al Bob</td></tr></table>`,
			// header row: "┌─────┐\n│Name │\n├─────┤\n"
			// data row wraps to ["Al", "Bob"]
			// data line0: "│Al   │"
			// data line1: "│Bob  │"
			// bottom: "└─────┘"
			want: "┌─────┐\n│Name │\n├─────┤\n│Al   │\n│Bob  │\n└─────┘\n",
		},
	})
}

// ---------------------------------------------------------------------------
// <style> tag and inline style= attribute
// ---------------------------------------------------------------------------

func TestStyleSources(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "<style> tag rules apply",
			html: `<style>p { margin-bottom: 0; }</style><p>one</p><p>two</p>`,
			want: "one\ntwo\n", // overrides UA margin-bottom:1
		},
		{
			name: "inline style= wins over stylesheet",
			css:  `p { margin-bottom: 2; }`,
			html: `<p style="margin-bottom: 0">text</p>`,
			want: "text\n", // inline style overrides
		},
		{
			name: "<style> tag overrides UA at equal specificity",
			html: `<style>p { display: inline; }</style><p>a</p><p>b</p>`,
			want: "ab",
		},
	})
}

// ---------------------------------------------------------------------------
// Inheritance
// ---------------------------------------------------------------------------

func TestInheritance(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "white-space inherited from parent",
			css:  `div { white-space: pre; }`,
			html: `<div><span>hello   world</span></div>`,
			want: "hello   world\n", // span inherits white-space:pre from div
		},
		{
			name: "display not inherited",
			css:  `div { display: none; }`,
			html: `<div><p>inside</p></div>`,
			want: "", // div hidden; p is never reached
		},
	})
}

// ---------------------------------------------------------------------------
// text-transform
// ---------------------------------------------------------------------------

func TestTextTransform(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "uppercase",
			html: `<p style="text-transform:uppercase">hello world</p>`,
			want: "HELLO WORLD\n\n",
		},
		{
			name: "lowercase",
			html: `<p style="text-transform:lowercase">HELLO WORLD</p>`,
			want: "hello world\n\n",
		},
		{
			name: "capitalize",
			html: `<p style="text-transform:capitalize">hello world</p>`,
			want: "Hello World\n\n",
		},
		{
			name: "capitalize strips leading space at block start",
			html: `<p style="text-transform:capitalize"> hello world</p>`,
			want: "Hello World\n\n",
		},
		{
			name: "none is a no-op",
			html: `<p style="text-transform:none">Hello World</p>`,
			want: "Hello World\n\n",
		},
		{
			name: "inherited by child inline elements",
			html: `<p style="text-transform:uppercase">hello <strong>world</strong></p>`,
			want: "HELLO WORLD\n\n",
		},
		{
			name: "child inline element overrides inherited transform",
			html: `<p style="text-transform:uppercase">hello <span style="text-transform:lowercase">WORLD</span></p>`,
			want: "HELLO world\n\n",
		},
		{
			name: "none cancels inherited transform on inline child",
			html: `<p style="text-transform:uppercase">BEFORE <span style="text-transform:none">none</span> AFTER</p>`,
			want: "BEFORE none AFTER\n\n",
		},
		{
			name: "via CSS class",
			css:  `.shout { text-transform: uppercase; }`,
			html: `<p class="shout">hello</p>`,
			want: "HELLO\n\n",
		},
		{
			name: "table cell uppercase",
			html: `<table style="border-style:hidden"><tr><td style="text-transform:uppercase" width="5">hello</td></tr></table>`,
			want: "HELLO\n",
		},
		{
			name: "table cell capitalize",
			html: `<table style="border-style:hidden"><tr><td style="text-transform:capitalize" width="11">hello world</td></tr></table>`,
			want: "Hello World\n",
		},
		{
			name: "superscript digits",
			html: `<p style="text-transform:superscript">0123456789</p>`,
			want: "⁰¹²³⁴⁵⁶⁷⁸⁹\n\n",
		},
		{
			name: "superscript letters",
			html: `<p style="text-transform:superscript">abcdefghijklmnoprstuvwxyz</p>`,
			want: "ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻ\n\n",
		},
		{
			name: "superscript symbols",
			html: `<p style="text-transform:superscript">+-=()</p>`,
			want: "⁺⁻⁼⁽⁾\n\n",
		},
		{
			name: "superscript unmapped chars pass through",
			html: `<p style="text-transform:superscript">q Q !</p>`,
			want: "q Q !\n\n",
		},
		{
			name: "subscript digits",
			html: `<p style="text-transform:subscript">0123456789</p>`,
			want: "₀₁₂₃₄₅₆₇₈₉\n\n",
		},
		{
			name: "subscript mapped letters",
			html: `<p style="text-transform:subscript">aehklmnopstx</p>`,
			want: "ₐₑₕₖₗₘₙₒₚₛₜₓ\n\n",
		},
		{
			name: "subscript symbols",
			html: `<p style="text-transform:subscript">+-=()</p>`,
			want: "₊₋₌₍₎\n\n",
		},
		{
			name: "subscript unmapped chars pass through",
			html: `<p style="text-transform:subscript">bcdfgijqruvwyz</p>`,
			want: "bcdfgijqruvwyz\n\n",
		},
		{
			name: "sup element uses superscript transform",
			html: `<p>H<sup>2</sup>O</p>`,
			want: "H²O\n\n",
		},
		{
			name: "sub element uses subscript transform",
			html: `<p>H<sub>2</sub>O</p>`,
			want: "H₂O\n\n",
		},
		{
			name: "sup inherits to inline children",
			html: `<p>x<sup><strong>n</strong></sup></p>`,
			want: "xⁿ\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// border-style preset on block elements
// ---------------------------------------------------------------------------

func TestBorderStyleOnBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "border-style:normal draws full box",
			html:  `<div style="border-style:normal; width:100%">hi</div>`,
			width: 8,
			// inner=6, hBorderWidth=8
			want: "┌──────┐\n│hi    │\n└──────┘\n",
		},
		{
			name:  "border-style:rounded draws rounded box",
			html:  `<div style="border-style:rounded; width:100%">hi</div>`,
			width: 8,
			want:  "╭──────╮\n│hi    │\n╰──────╯\n",
		},
		{
			name:  "border-style:thick draws thick box",
			html:  `<div style="border-style:thick; width:100%">hi</div>`,
			width: 8,
			want:  "┏━━━━━━┓\n┃hi    ┃\n┗━━━━━━┛\n",
		},
		{
			name:  "border-style:double draws double box",
			html:  `<div style="border-style:double; width:100%">hi</div>`,
			width: 8,
			want:  "╔══════╗\n║hi    ║\n╚══════╝\n",
		},
		{
			name: "border-style:markdown draws only left/right bars",
			html: `<div style="border-style:markdown">hi</div>`,
			// markdown preset: left=| right=| (ASCII pipe), top=nil bottom=nil
			want: "|hi|\n",
		},
		{
			name: "border-style:hidden draws no borders",
			html: `<div style="border-style:hidden">hi</div>`,
			want: "hi\n",
		},
		{
			name: "border-style:none draws no borders",
			html: `<div style="border-style:none">hi</div>`,
			want: "hi\n",
		},
		{
			name:  "individual border-top overrides preset fill but keeps corners",
			html:  `<div style="border-style:normal; border-top:═; width:100%">hi</div>`,
			width: 8,
			// border-top:═ overrides the ─ fill from normal; corners still come from preset
			want: "┌══════┐\n│hi    │\n└──────┘\n",
		},
		{
			name:  "individual border-left overrides preset char",
			html:  `<div style="border-style:normal; border-left:▌; width:100%">hi</div>`,
			width: 8,
			// border-left:▌ overrides │; top/bottom corners still from preset
			want: "┌──────┐\n▌hi    │\n└──────┘\n",
		},
		{
			name:  "border-style via CSS class",
			css:   `.box { border-style: rounded; width: 100%; }`,
			html:  `<div class="box">ok</div>`,
			width: 8,
			want:  "╭──────╮\n│ok    │\n╰──────╯\n",
		},
	})
}

// ---------------------------------------------------------------------------
// Block element inside inline flow (display:block inside renderInline)
// ---------------------------------------------------------------------------

func TestBlockInInline(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "block inside inline breaks line",
			html: `<p>before<span style="display:block">mid</span>after</p>`,
			// renderInline(p): text "before", then span display:block →
			//   writeMarginNewlines(&sb, 0+1=1): sb="before" len>0, ensures 1 trailing \n → "before\n"
			//   render span inline → "mid"
			//   sb.WriteByte('\n') → "before\nmid\n"
			//   mb=0, no action
			// then text "after" → "before\nmid\nafter"
			// p block wraps: "before\nmid\nafter\n" + mb:1 → "before\nmid\nafter\n\n"
			want: "before\nmid\nafter\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

func TestList(t *testing.T) {
	runCases(t, []renderCase{
		{
			// UA default: ul { padding-left: 4 } → indent=4, prefix="• " (2 cols)
			name: "unordered list renders bullets",
			html: `<ul><li>alpha</li><li>beta</li></ul>`,
			want: "    • alpha\n    • beta\n",
		},
		{
			// UA default: ol { padding-left: 4 } → indent=4, prefix="1. " (3 cols)
			name: "ordered list renders numbers",
			html: `<ol><li>one</li><li>two</li></ol>`,
			want: "    1. one\n    2. two\n",
		},
		{
			// blockquote innerW=36 (40-1-1-2), list indent=4 → "│ " + "    • item" + "  "
			name: "list inside blockquote renders with border",
			html: `<blockquote><ul><li>item</li></ul></blockquote>`,
			want: "│     • item  \n",
		},
		{
			// goldmark wraps loose-list items in <p>; the <p> margin-bottom must
			// not produce blank bordered lines inside the blockquote.
			name: "loose list item (p inside li) in blockquote has no extra blank lines",
			html: `<blockquote><ul><li><p>item</p></li></ul></blockquote>`,
			want: "│     • item  \n",
		},
		{
			name: "multiple loose items in blockquote no extra blank lines",
			html: `<blockquote><ul><li><p>alpha</p></li><li><p>beta</p></li></ul></blockquote>`,
			want: "│     • alpha  \n│     • beta  \n",
		},
		{
			// Long item must wrap at terminal width with hanging indent (content
			// columns aligned under first word, not under bullet).
			// width=20, indent=4, prefix=2 → contentWidth=14. hangStr=6 spaces.
			name:  "long item wraps with hanging indent",
			width: 20,
			html:  `<ul><li>one two three four five six</li></ul>`,
			want:  "    • one two three\n      four five six\n",
		},
		{
			// width=40, 11 items → prefixWidth=4 (" 1. "), indent=4, contentWidth=32
			name:  "ordered list 10+ items aligns single and double digit",
			width: 40,
			html:  `<ol><li>a</li><li>b</li><li>c</li><li>d</li><li>e</li><li>f</li><li>g</li><li>h</li><li>i</li><li>j</li><li>k</li></ol>`,
			want:  "     1. a\n     2. b\n     3. c\n     4. d\n     5. e\n     6. f\n     7. g\n     8. h\n     9. i\n    10. j\n    11. k\n",
		},
		{
			// list-style-type:none → prefixWidth=0, but UA indent=4 still applies
			name: "list-style-type none suppresses bullet",
			css:  `ul { list-style-type: none; }`,
			html: `<ul><li>alpha</li><li>beta</li></ul>`,
			want: "    alpha\n    beta\n",
		},
		{
			// indent=4 (UA), prefix="a. " (3 cols) → contentWidth=33
			name: "list-style-type lower-alpha",
			css:  `ol { list-style-type: lower-alpha; }`,
			html: `<ol><li>one</li><li>two</li><li>three</li></ol>`,
			want: "    a. one\n    b. two\n    c. three\n",
		},
		{
			// User CSS padding-left:2 overrides UA padding-left:4 (same specificity, last wins)
			name: "padding-left indents list",
			css:  `ul { padding-left: 2; }`,
			html: `<ul><li>item</li></ul>`,
			want: "  • item\n",
		},
		{
			// blockquote innerW=16 (20-1-1-2), list indent=4, prefix=2 → contentWidth=10
			// "one two" fits (7), "three four" fits (10), "five" on 3rd line
			name:  "wrapped item inside blockquote keeps border on all lines",
			width: 20,
			html:  `<blockquote><ul><li>one two three four five</li></ul></blockquote>`,
			want:  "│     • one two  \n│       three four  \n│       five  \n",
		},
	})
}

// ---------------------------------------------------------------------------
// Word wrapping for block elements
// ---------------------------------------------------------------------------

func TestWordWrap(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "long paragraph wraps at terminal width",
			html:  `<p>one two three four five six seven</p>`,
			width: 20,
			// wrapW=20, "one two three four"=18 fits, "five"=23>20 → break
			want: "one two three four\nfive six seven\n\n",
		},
		{
			name:  "short paragraph does not wrap",
			html:  `<p>hello world</p>`,
			width: 20,
			want:  "hello world\n\n",
		},
		{
			name:  "white-space:nowrap skips word wrap",
			html:  `<p style="white-space:nowrap">one two three four five six</p>`,
			width: 20,
			want:  "one two three four five six\n\n",
		},
		{
			name:  "pre block skips word wrap",
			html:  `<pre>one two three four five six</pre>`,
			width: 20,
			want:  "one two three four five six\n",
		},
		{
			name:  "blockquote text wraps inside border and padding",
			html:  `<blockquote>one two three four five six</blockquote>`,
			width: 20,
			// hBorderWidth=20, wrapW=20-1(border-left)-1(padding-left)-2(padding-right)=16
			// "one two three"=13 fits; adding "four"→18>16 → break
			want: "│ one two three  \n│ four five six  \n",
		},
		{
			name:  "explicit CSS width constrains wrap width",
			html:  `<p style="width:10">hello world end</p>`,
			width: 40,
			// inner=10, wrapW=10; "hello"=5 fits, "hello world"=11>10 → break
			// "world end"=9 fits on line 2; layoutSt.Width(10) pads both lines
			want: "hello     \nworld end \n\n",
		},
		{
			name:  "multi-line content from block-in-inline is not re-wrapped",
			html:  `<blockquote><p>A</p><p>B</p></blockquote>`,
			width: 40,
			// Block-in-inline content has newlines; word wrap is skipped to preserve layout.
			want: "│ A  \n│   \n│ B  \n",
		},
	})
}

// ---------------------------------------------------------------------------
// Blockquote with block children
// ---------------------------------------------------------------------------

func TestBlockquoteBlocks(t *testing.T) {
	runCases(t, []renderCase{
		{
			// Blockquote lines must not be padded to the width of the longest
			// line — only the UA padding-left:1 and padding-right:2 should appear.
			name: "blockquote heading and paragraph no extra trailing spaces",
			html: `<blockquote><h2>Title</h2><p>Body.</p></blockquote>`,
			// h2: bold "Title" → block-in-inline, margin-bottom=0
			// p:  "Body."    → block-in-inline, margin-bottom=1 (collapsed by TrimRight)
			// content after trim: "Title\nBody."
			// padding: " Title  \n Body.  "
			// border:  "│ Title  \n│ Body.  "
			want: "│ Title  \n│ Body.  \n",
		},
		{
			// p margin-bottom:1 creates one blank bordered line between paragraphs.
			name: "two paragraphs in blockquote separated by one blank bordered line",
			html: `<blockquote><p>A</p><p>B</p></blockquote>`,
			// A\n\nB after trim; applyLineEdges adds " "/"  " to every line including blank
			want: "│ A  \n│   \n│ B  \n",
		},
	})
}

// ---------------------------------------------------------------------------
// CSS selector improvements: #id, multi-class, >, pseudo-classes, [attr]
// ---------------------------------------------------------------------------

func TestSelectors(t *testing.T) {
	runCases(t, []renderCase{
		// --- #id ---
		{
			name: "#id selector targets element with matching id",
			css:  `#hero { text-transform: uppercase; }`,
			html: `<p id="hero">featured</p><p>normal</p>`,
			want: "FEATURED\n\nnormal\n\n",
		},
		{
			name: "#id selector does not match a different id",
			css:  `#other { text-transform: uppercase; }`,
			html: `<p id="hero">text</p>`,
			want: "text\n\n",
		},
		{
			name: "#id in descendant selector chain",
			css:  `#main p { text-transform: uppercase; }`,
			html: `<div id="main"><p>inside</p></div><p>outside</p>`,
			// div has no margin-bottom; p's trailing blank line is eaten by TrimRight inside div
			want: "INSIDE\noutside\n\n",
		},
		{
			name: "element#id combined selector",
			css:  `p#hero { text-transform: uppercase; }`,
			html: `<p id="hero">match</p><div id="hero">no-match</div>`,
			// p#hero matches only the <p>, not the <div> (wrong element)
			want: "MATCH\n\nno-match\n",
		},
		// --- multiple classes ---
		{
			name: "multi-class requires all classes to be present",
			css:  `.warn.big { text-transform: uppercase; }`,
			html: `<p class="warn big">both</p><p class="warn">warn-only</p><p class="big">big-only</p>`,
			want: "BOTH\n\nwarn-only\n\nbig-only\n\n",
		},
		{
			name: "element plus two classes",
			css:  `p.a.b { text-transform: uppercase; }`,
			html: `<p class="a b">para</p><div class="a b">div</div>`,
			// div has the right classes but wrong element
			want: "PARA\n\ndiv\n",
		},
		{
			name: "extra classes on element do not prevent match",
			css:  `.highlight { text-transform: uppercase; }`,
			html: `<p class="highlight extra">text</p>`,
			want: "TEXT\n\n",
		},
		// --- child combinator > ---
		{
			name: "child combinator matches direct children only",
			css:  `div > p { text-transform: uppercase; }`,
			// direct <p> matches; <p> inside <section> does not (section is its parent)
			html: `<div><p>direct</p><section><p>nested</p></section></div>`,
			want: "DIRECT\n\nnested\n",
		},
		{
			name: "descendant combinator still matches all levels",
			css:  `div p { text-transform: uppercase; }`,
			html: `<div><p>direct</p><section><p>nested</p></section></div>`,
			want: "DIRECT\n\nNESTED\n",
		},
		{
			name: "child combinator in a deeper chain",
			css:  `div > p > span { text-transform: uppercase; }`,
			html: `<div><p><span>deep</span> rest</p></div>`,
			// div has no margin-bottom; p's trailing blank line is eaten by TrimRight inside div
			want: "DEEP rest\n",
		},
		{
			name: "mixed child and descendant combinators",
			// div > ul means ul is a direct child of div; li just needs to be a descendant of that ul
			css:  `div > ul li { text-transform: uppercase; }`,
			html: `<div><ul><li>match</li></ul></div><ul><li>no-match</li></ul>`,
			// UA ul { padding-left: 4 } adds 4-space indent; inner ul matches, outer ul does not
			want: "    • MATCH\n    • no-match\n",
		},
		// --- pseudo-classes ---
		{
			name: ":first-child matches first element sibling",
			css:  `li:first-child { text-transform: uppercase; }`,
			html: `<ul><li>one</li><li>two</li><li>three</li></ul>`,
			// UA ul { padding-left: 4 } produces 4-space indent before each item
			want: "    • ONE\n    • two\n    • three\n",
		},
		{
			name: ":last-child matches last element sibling",
			css:  `li:last-child { text-transform: uppercase; }`,
			html: `<ul><li>one</li><li>two</li><li>three</li></ul>`,
			want: "    • one\n    • two\n    • THREE\n",
		},
		{
			name: ":first-child and :last-child both match a single item",
			css:  `li:first-child { text-transform: uppercase; } li:last-child { text-transform: uppercase; }`,
			html: `<ul><li>only</li></ul>`,
			want: "    • ONLY\n",
		},
		{
			name: ":nth-child(odd) matches 1st 3rd 5th element siblings",
			css:  `p:nth-child(odd) { text-transform: uppercase; }`,
			html: `<div><p>one</p><p>two</p><p>three</p></div>`,
			want: "ONE\n\ntwo\n\nTHREE\n",
		},
		{
			name: ":nth-child(even) matches 2nd 4th element siblings",
			css:  `p:nth-child(even) { text-transform: uppercase; }`,
			html: `<div><p>one</p><p>two</p><p>three</p></div>`,
			want: "one\n\nTWO\n\nthree\n",
		},
		{
			name: ":nth-child(odd) on table rows styles odd rows",
			css:  `tr:nth-child(odd) td { text-transform: uppercase; }`,
			html: `<table style="border-style:hidden"><tr><td>r1</td></tr><tr><td>r2</td></tr><tr><td>r3</td></tr></table>`,
			want: "R1\nr2\nR3\n",
		},
		// --- attribute selectors ---
		{
			name: "[attr] presence selector hides elements with the attribute",
			css:  `p[data-hide] { display: none; }`,
			html: `<p>visible</p><p data-hide>hidden</p><p>after</p>`,
			want: "visible\n\nafter\n\n",
		},
		{
			name: "[attr] matches attribute with empty value",
			css:  `span[data-mark] { text-transform: uppercase; }`,
			html: `<p><span data-mark="">marked</span> plain</p>`,
			want: "MARKED plain\n\n",
		},
		{
			name: "[attr=val] exact-value selector",
			css:  `p[data-style=big] { text-transform: uppercase; }`,
			html: `<p data-style="big">large</p><p data-style="small">tiny</p>`,
			want: "LARGE\n\ntiny\n\n",
		},
		{
			name: "[attr=val] with quoted value in CSS",
			css:  `p[lang="en"] { text-transform: uppercase; }`,
			html: `<p lang="en">english</p><p lang="fr">french</p>`,
			want: "ENGLISH\n\nfrench\n\n",
		},
		{
			name: "[attr=val] does not match wrong value",
			css:  `a[href=https://example.com] { text-transform: uppercase; }`,
			html: `<p><a href="https://example.com">right</a> <a href="https://other.com">wrong</a></p>`,
			want: "RIGHT wrong\n\n",
		},
	})
}

// ---------------------------------------------------------------------------
// New elements: ins, dfn, small, q, abbr, dl/dt/dd, figure/figcaption
// ---------------------------------------------------------------------------

func TestNewElements(t *testing.T) {
	runCases(t, []renderCase{
		// ins — underline (ANSI stripped, so just verify content passes through)
		{
			name: "ins renders content",
			html: `<p>before <ins>inserted</ins> after</p>`,
			want: "before inserted after\n\n",
		},
		// dfn — italic (ANSI stripped)
		{
			name: "dfn renders content",
			html: `<p><dfn>term</dfn> is defined here</p>`,
			want: "term is defined here\n\n",
		},
		// small — dimmed color (ANSI stripped)
		{
			name: "small renders content",
			html: `<p>main <small>fine print</small></p>`,
			want: "main fine print\n\n",
		},
		// q — wraps in Unicode curly quotes
		{
			name: "q wraps content in curly quotes",
			html: `<p>She said <q>hello</q>.</p>`,
			want: "She said “hello”.\n\n",
		},
		{
			name: "q standalone at body level",
			html: `<q>quoted</q>`,
			want: "“quoted”",
		},
		// abbr — appends title expansion
		{
			name:  "abbr with title appends expansion",
			html:  `<p>The <abbr title="HyperText Markup Language">HTML</abbr> spec.</p>`,
			width: 80,
			want:  "The HTML (HyperText Markup Language) spec.\n\n",
		},
		{
			name: "abbr without title renders as-is",
			html: `<p><abbr>CSS</abbr></p>`,
			want: "CSS\n\n",
		},
		{
			name: "abbr standalone at body level",
			html: `<abbr title="Cascading Style Sheets">CSS</abbr>`,
			want: "CSS (Cascading Style Sheets)",
		},
		// dl / dt / dd — definition list
		{
			name: "dl with single dt and dd",
			html: `<dl><dt>Term</dt><dd>Definition.</dd></dl>`,
			// dt: block, bold (ANSI stripped) → "Term\n"
			// dd: block, padding-left:4 → "    Definition.\n"
			// dl: margin-bottom:1 → trailing blank line
			want: "Term\n    Definition.\n\n",
		},
		{
			name: "dl with multiple entries",
			html: `<dl><dt>Alpha</dt><dd>First.</dd><dt>Beta</dt><dd>Second.</dd></dl>`,
			want: "Alpha\n    First.\nBeta\n    Second.\n\n",
		},
		// figure / figcaption
		{
			name: "figcaption inside figure renders as italic block",
			html: `<figure><figcaption>Caption text</figcaption></figure>`,
			want: "Caption text\n",
		},
		{
			name: "figure with nested content and figcaption",
			html: `<figure><p>Content here.</p><figcaption>Fig 1</figcaption></figure>`,
			// p: "Content here.\n\n" (block-in-inline with margin-bottom:1)
			// figcaption: "Fig 1\n"
			// figure TrimRight → "Content here.\n\nFig 1"
			// renderDisplayNode adds "\n" → "Content here.\n\nFig 1\n"
			want: "Content here.\n\nFig 1\n",
		},
	})
}
