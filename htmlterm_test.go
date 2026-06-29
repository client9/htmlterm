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
	name  string
	css   string
	html  string
	width int
	want  string
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

func TestBareText(t *testing.T) {
	runCases(t, []renderCase{
		{name: "bare text in html element", html: `<html>Hello World</html>`, want: "Hello World"},
		{name: "bare text in body element", html: `<body>hello</body>`, want: "hello"},
		{name: "bare text mixed with block element", html: `<body>before<p>paragraph</p>after</body>`, want: "beforeparagraph\n\nafter"},
		{name: "whitespace-only text between elements is ignored", html: "<body>\n<p>text</p>\n</body>", want: "text\n\n"},
	})
}

func TestDisplay_Block(t *testing.T) {
	runCases(t, []renderCase{
		{name: "p is block with trailing newline", html: `<p>hello</p>`, want: "hello\n\n"},
		{name: "adjacent p elements are separated by blank line", html: `<p>one</p><p>two</p>`, want: "one\n\ntwo\n\n"},
		{name: "h1 is block without extra margin", html: `<h1>Title</h1>`, want: "Title\n"},
		{name: "div is block", html: `<div>content</div>`, want: "content\n"},
		{name: "section is block", html: `<section>sec</section>`, want: "sec\n"},
		{name: "h1 followed by p", html: `<h1>Title</h1><p>Body</p>`, want: "Title\nBody\n\n"},
		{name: "multiple headings", html: `<h1>H1</h1><h2>H2</h2><h3>H3</h3>`, want: "H1\nH2\nH3\n"},
		{name: "blockquote is block", html: `<blockquote>quoted</blockquote>`, want: "│ quoted  \n"},
		{name: "CSS display:block on custom element", css: `span { display: block; }`, html: `<span>line one</span><span>line two</span>`, want: "line one\nline two\n"},
		{name: "display:inline overrides UA block on p", css: `p { display: inline; }`, html: `<p>hello</p><p>world</p>`, want: "helloworld"},
	})
}

func TestDisplay_Inline(t *testing.T) {
	runCases(t, []renderCase{
		{name: "span is inline at body level (no newline)", html: `<span>hello</span>`, want: "hello"},
		{name: "adjacent inline spans concatenate", html: `<span>foo</span><span>bar</span>`, want: "foobar"},
		{name: "inline elements inside block", html: `<p>hello <strong>world</strong></p>`, want: "hello world\n\n"},
		{name: "multiple inline tags inside block", html: `<p><em>a</em> <strong>b</strong> c</p>`, want: "a b c\n\n"},
	})
}

func TestDisplay_None(t *testing.T) {
	runCases(t, []renderCase{
		{name: "display:none via inline style hides element", html: `<p>visible</p><p style="display:none">hidden</p><p>after</p>`, want: "visible\n\nafter\n\n"},
		{name: "display:none via CSS class", css: `.gone { display: none; }`, html: `<p>a</p><p class="gone">b</p><p>c</p>`, want: "a\n\nc\n\n"},
		{name: "display:none hides inline element", html: `<p>before <span style="display:none">hidden</span> after</p>`, want: "before  after\n\n"},
		{name: "display:none on block in sequence", html: `<div>a</div><div style="display:none">b</div><div>c</div>`, want: "a\nc\n"},
		{name: "display:none hides top-level unordered list", html: `<p>before</p><ul style="display:none"><li>hidden</li></ul><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides nested unordered list", html: `<blockquote>before<ul style="display:none"><li>hidden</li></ul>after</blockquote>`, want: "│ beforeafter  \n"},
		{name: "display:none hides table", html: `<p>before</p><table style="display:none"><tr><td>x</td></tr></table><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides hr", html: `<p>before</p><hr style="display:none"><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides br", html: `<p>before<br style="display:none">after</p>`, want: "beforeafter\n\n"},
	})
}

func TestDisplay_InlineBlock(t *testing.T) {
	runCases(t, []renderCase{
		{name: "inline-block with fixed width pads content", html: `<p><span style="display:inline-block; width:8">hi</span>end</p>`, want: "hi      end\n\n"},
		{name: "inline-block without width acts like inline", html: `<p><span style="display:inline-block">hi</span>end</p>`, want: "hiend\n\n"},
		{name: "two inline-block spans side by side", html: `<p><span style="display:inline-block; width:5">A</span><span style="display:inline-block; width:5">B</span></p>`, want: "A    B    \n\n"},
	})
}

func TestMargins(t *testing.T) {
	runCases(t, []renderCase{
		{name: "UA p has margin-bottom:1", html: `<p>text</p>`, want: "text\n\n"},
		{name: "margin-top ignored on first element (nothing above)", css: `p { margin-top: 2; }`, html: `<p>first</p>`, want: "first\n\n"},
		{name: "margin-top adds space before element", css: `p { margin-top: 1; }`, html: `<div>above</div><p>below</p>`, want: "above\n\nbelow\n\n"},
		{name: "margin collapse: equal margins produce one blank line", css: `div { margin-bottom: 1; } p { margin-top: 1; }`, html: `<div>above</div><p>below</p>`, want: "above\n\nbelow\n\n"},
		{name: "margin collapse: larger wins", css: `div { margin-bottom: 2; } p { margin-top: 1; }`, html: `<div>above</div><p>below</p>`, want: "above\n\n\nbelow\n\n"},
		{name: "custom margin-bottom on element", css: `h2 { margin-bottom: 2; }`, html: `<h2>heading</h2><p>body</p>`, want: "heading\n\n\nbody\n\n"},
		{name: "margin-bottom on last element still applies", html: `<p>last</p>`, want: "last\n\n"},
	})
}

func TestPadding(t *testing.T) {
	runCases(t, []renderCase{
		{name: "padding-left indents content", css: `div { padding-left: 4; }`, html: `<div>hello</div>`, want: "    hello\n"},
		{name: "blockquote has UA border-left and padding", html: `<blockquote>quoted</blockquote>`, want: "│ quoted  \n"},
	})
}

func TestVerticalBoxModel(t *testing.T) {
	runCases(t, []renderCase{
		{name: "padding-top inserts blank row above content", css: `div { padding-top: 1; width: 4; }`, html: `<div>hi</div>`, want: "    \nhi  \n"},
		{name: "padding-bottom inserts blank row below content", css: `div { padding-bottom: 1; width: 4; }`, html: `<div>hi</div>`, want: "hi  \n    \n"},
		{name: "padding-top and bottom both present", css: `div { padding-top: 2; padding-bottom: 1; width: 4; }`, html: `<div>text</div>`, want: "    \n    \ntext\n    \n"},
		{name: "padding-top inside border-left", css: `div { border-left: │; padding-top: 1; width: 5; }`, html: `<div>hi</div>`, want: "│    \n│hi  \n"},
		{name: "padding-bottom inside border-left", css: `div { border-left: │; padding-bottom: 1; width: 5; }`, html: `<div>hi</div>`, want: "│hi  \n│    \n"},
		{name: "height expands short content with blank lines", css: `div { height: 3; width: 5; }`, html: `<div>x</div>`, want: "x    \n     \n     \n"},
		{name: "height clips overflow with overflow:hidden", css: `div { height: 1; overflow: hidden; }`, html: "<div>a<br>b</div>", want: "a\n"},
		{name: "height no-ops when content already fills height", css: `div { height: 2; width: 5; }`, html: "<div>a<br>b</div>", want: "a    \nb    \n"},
	})
}

func TestColGroup(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "col width-attr sets fixed column width",
			html:  `<table style="border-style:none"><colgroup><col width="6"><col width="4"></colgroup><tr><th>Name</th><th>Age</th></tr><tr><td>Alice</td><td>30</td></tr></table>`,
			width: 40,
			want:  "Name   Age \nAlice  30  \n",
		},
		{
			name:  "col style width sets fixed column width",
			html:  `<table style="border-style:none"><colgroup><col style="width:6"><col style="width:4"></colgroup><tr><th>Name</th><th>Age</th></tr><tr><td>Alice</td><td>30</td></tr></table>`,
			width: 40,
			want:  "Name   Age \nAlice  30  \n",
		},
		{
			name:  "col span covers multiple columns",
			html:  `<table style="border-style:none"><colgroup><col span="2" style="width:6"></colgroup><tr><th>A</th><th>B</th></tr><tr><td>foo</td><td>bar</td></tr></table>`,
			width: 40,
			want:  "A      B     \nfoo    bar   \n",
		},
		{
			name:  "colgroup span with no col children",
			html:  `<table style="border-style:none"><colgroup span="2" style="width:5"></colgroup><tr><th>X</th><th>Y</th></tr><tr><td>a</td><td>b</td></tr></table>`,
			width: 40,
			want:  "X     Y    \na     b    \n",
		},
		{
			name:  "cell style overrides col style",
			html:  `<table style="border-style:none"><colgroup><col style="width:10"></colgroup><tr><th style="width:6">Name</th></tr><tr><td>Alice</td></tr></table>`,
			width: 40,
			want:  "Name  \nAlice \n",
		},
		{
			name:  "col css selector sets column width",
			css:   `col.narrow { width: 5; }`,
			html:  `<table style="border-style:none"><colgroup><col class="narrow"><col class="narrow"></colgroup><tr><th>A</th><th>B</th></tr><tr><td>foo</td><td>bar</td></tr></table>`,
			width: 40,
			want:  "A     B    \nfoo   bar  \n",
		},
	})
}

func TestBlockBorders(t *testing.T) {
	runCases(t, []renderCase{
		{name: "border-left adds char to each line", html: `<p style="border-left:│">hello</p>`, want: "│hello\n\n"},
		{name: "border-left with color strips clean", html: `<p style="border-left:│; border-left-color:#ff0000">hi</p>`, want: "│hi\n\n"},
		{name: "border-right appends char", html: `<p style="border-right:│">hello</p>`, want: "hello│\n\n"},
		{name: "border-left and right together", html: `<p style="border-left:│; border-right:│">hello</p>`, want: "│hello│\n\n"},
		{name: "border-left none disables", html: `<p style="border-left:none">hello</p>`, want: "hello\n\n"},
		{name: "border-left on multiline pre", html: "<pre style=\"border-left:│\">line1\nline2</pre>", want: "│line1\n│line2\n"},
		{name: "border-left via CSS class", css: `.note { border-left: ▌; }`, html: `<p class="note">text</p>`, want: "▌text\n\n"},
		{name: "blockquote UA has border-left and padding", html: `<blockquote>quoted</blockquote>`, want: "│ quoted  \n"},
		{name: "margin-left adds spaces outside border", html: `<p style="margin-left:4; border-left:|; padding-left:1">hi</p>`, want: "    | hi\n\n"},
		{name: "margin-right adds spaces outside border", html: `<p style="margin-right:4; border-right:|; padding-right:1">hi</p>`, want: "hi |    \n\n"},
		{name: "margin-left only no border", html: `<p style="margin-left:2">hello</p>`, want: "  hello\n\n"},
		{name: "nested border-left stacks characters", html: `<div style="border-left:|"><div style="border-left:|">inner</div></div>`, want: "||inner\n"},
		{name: "width:100% fills renderer width with borders", html: `<p style="width:100%; border-left:[; border-right:]">hi</p>`, want: "[hi                  ]\n\n", width: 22},
		{name: "width:100% with margin subtracts margin from line width", html: `<p style="width:100%; margin-left:2; margin-right:2; border-left:[; border-right:]">hi</p>`, want: "  [hi                  ]  \n\n", width: 26},
		{name: "border-top draws horizontal rule before content", html: `<p style="border-top:─">hi</p>`, width: 10, want: "──────────\nhi\n\n"},
		{name: "border-bottom draws horizontal rule after content", html: `<p style="border-bottom:─">hi</p>`, width: 10, want: "hi\n──────────\n\n"},
		{name: "border-top none is disabled", html: `<p style="border-top:none">hi</p>`, width: 10, want: "hi\n\n"},
		{name: "all four borders with width:100%", html: `<p style="width:100%; border-top:─; border-bottom:─; border-left:[; border-right:]">hi</p>`, width: 12, want: "────────────\n[hi        ]\n────────────\n\n"},
		{name: "corners replace fill endpoints on top and bottom rules", html: `<p style="width:100%; border-top:─; border-bottom:─; border-left:│; border-right:│; border-top-left-corner:┌; border-top-right-corner:┐; border-bottom-left-corner:└; border-bottom-right-corner:┘">hi</p>`, width: 12, want: "┌──────────┐\n│hi        │\n└──────────┘\n\n"},
		{name: "corner without opposite border uses fill for that side", html: `<p style="width:100%; border-top:─; border-top-left-corner:┌">hi</p>`, width: 12, want: "┌───────────\nhi          \n\n"},
		{name: "border-top and border-bottom with margin-left", html: `<p style="width:100%; border-top:─; border-bottom:─; margin-left:2">hi</p>`, width: 12, want: "  ──────────\n  hi        \n  ──────────\n\n"},
		{name: "border-top with color strips clean", html: `<p style="border-top:─; border-top-color:#ff0000">hi</p>`, width: 10, want: "──────────\nhi\n\n"},
		{name: "margin auto both centers element", html: `<p style="width:10; margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin-left auto pushes element to right", html: `<p style="width:10; margin-left:auto">hi</p>`, width: 20, want: "          hi        \n\n"},
		{name: "margin-right auto fills trailing space", html: `<p style="width:10; margin-right:auto">hi</p>`, width: 20, want: "hi                  \n\n"},
		{name: "margin auto with percent width centers", html: `<p style="width:50%; margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin-left and margin-right as percentages", html: `<p style="margin-left:25%; margin-right:25%">hi</p>`, width: 80, want: "                    hi                    \n\n"},
		{name: "margin auto without explicit width is ignored", html: `<p style="margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "hi\n\n"},
		{name: "margin auto center via CSS class", css: `.center { width: 10; margin-left: auto; margin-right: auto; }`, html: `<p class="center">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin auto center with borders", html: `<p style="width:12; margin-left:auto; margin-right:auto; border-left:[; border-right:]">hi</p>`, width: 20, want: "    [hi        ]    \n\n"},
		{name: "border-right aligns on word-wrapped lines", html: `<p style="border-right:|">one two three four five</p>`, width: 14, want: "one two three|\nfour five    |\n\n"},
		{name: "padding-right aligns on word-wrapped lines", html: `<p style="padding-right:2">one two three four five</p>`, width: 15, want: "one two three  \nfour five      \n\n"},
		{name: "border-right single-line content stays flush", html: `<p style="border-right:|">hello</p>`, width: 20, want: "hello|\n\n"},
		{name: "border-right with margin-right aligns on wrapped lines", html: `<p style="border-right:|; margin-right:2">one two three four five</p>`, width: 16, want: "one two three|  \nfour five    |  \n\n"},
	})
}
