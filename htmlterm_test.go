package htmlterm_test

import (
	"regexp"
	"testing"

	"github.com/client9/htmlterm"
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

func TestOverflowWrap(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "overflow-wrap normal does not break long words",
			css:   `p { overflow-wrap: normal; }`,
			html:  `<p>superlongwordthatiswaytoolong</p>`,
			width: 10,
			want:  "superlongwordthatiswaytoolong\n\n",
		},
		{
			name:  "overflow-wrap break-word hard-breaks long token",
			css:   `p { overflow-wrap: break-word; }`,
			html:  `<p>superlongwordthatiswaytoolong</p>`,
			width: 10,
			want:  "superlongw\nordthatisw\naytoolong\n\n",
		},
		{
			name:  "word-break break-all breaks at every character",
			css:   `p { word-break: break-all; }`,
			html:  `<p>hello world</p>`,
			width: 7,
			want:  "hello w\norld\n\n",
		},
		{
			name:  "overflow-wrap break-word on multi-word text wraps short words normally",
			css:   `p { overflow-wrap: break-word; }`,
			html:  `<p>short words fit fine</p>`,
			width: 10,
			want:  "short\nwords fit\nfine\n\n",
		},
		{
			name:  "word-break overridden by overflow-wrap when both set",
			css:   `p { overflow-wrap: break-word; word-break: normal; }`,
			html:  `<p>verylongtoken</p>`,
			width: 6,
			// "verylongtoken" (13 chars) split at 6: "verylo"+"ngtoke"+"n"
			want: "verylo\nngtoke\nn\n\n",
		},
		{
			name:  "overflow-wrap inherited by child span",
			css:   `div { overflow-wrap: break-word; }`,
			html:  `<div><p>averylongwordindeed</p></div>`,
			width: 8,
			// "averylongwordindeed" (19 chars) split at 8: "averylon"+"gwordind"+"eed"
			// div strips trailing newlines from inner p; only div's own \n remains
			want: "averylon\ngwordind\need\n",
		},
	})
}

func TestTextIndent(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "text-indent indents first line only",
			css:   `p { text-indent: 4; }`,
			html:  `<p>hello world</p>`,
			width: 40,
			want:  "    hello world\n\n",
		},
		{
			name:  "text-indent does not indent subsequent lines",
			css:   `p { text-indent: 2; }`,
			html:  `<p>one two three four</p>`,
			width: 10,
			// indent is applied after wrapping; first line gets "  " prepended
			want: "  one two\nthree four\n\n",
		},
		{
			name:  "text-indent inherited by child block",
			css:   `div { text-indent: 3; }`,
			html:  `<div><p>hello</p></div>`,
			width: 40,
			// div's first content is a block child (p); indent applies to p's own first line
			// div strips p's trailing newline, so result has only div's \n
			want: "   hello\n",
		},
		{
			name:  "text-indent percentage of available width",
			css:   `p { text-indent: 10%; }`,
			html:  `<p>hi</p>`,
			width: 20,
			want:  "  hi\n\n",
		},
	})
}

func TestTabSize(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "tab-size 4 expands tab to 4 spaces in pre",
			html:  `<pre>a&#9;b</pre>`,
			css:   `pre { tab-size: 4; }`,
			width: 40,
			want:  "a   b\n",
		},
		{
			name:  "tab-size 8 (default) expands tab to 8 spaces",
			html:  `<pre>a&#9;b</pre>`,
			width: 40,
			want:  "a       b\n",
		},
		{
			name:  "tab-size 2 aligns to 2-column stops",
			html:  "<pre>ab\tc</pre>",
			css:   `pre { tab-size: 2; }`,
			width: 40,
			want:  "ab  c\n",
		},
		{
			name:  "tab at column zero expands to full tab-size",
			html:  `<pre>&#9;indented</pre>`,
			css:   `pre { tab-size: 4; }`,
			width: 40,
			want:  "    indented\n",
		},
	})
}

func TestMinMaxHeight(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "min-height pads short block",
			css:   `div { min-height: 3; width: 6; }`,
			html:  `<div>hi</div>`,
			width: 20,
			want:  "hi    \n      \n      \n",
		},
		{
			name:  "min-height leaves taller block unchanged",
			css:   `div { min-height: 1; }`,
			html:  `<div>hi</div>`,
			width: 20,
			want:  "hi\n",
		},
		{
			name:  "max-height clips block with overflow:hidden",
			css:   `pre { max-height: 2; overflow: hidden; }`,
			html:  "<pre>line1\nline2\nline3\nline4</pre>",
			width: 20,
			want:  "line1\nline2\n",
		},
		{
			name:  "max-height without overflow:hidden does not clip",
			css:   `pre { max-height: 2; }`,
			html:  "<pre>line1\nline2\nline3</pre>",
			width: 20,
			want:  "line1\nline2\nline3\n",
		},
		{
			name:  "fixed height overrides min-height",
			css:   `div { height: 2; min-height: 5; width: 4; overflow: hidden; }`,
			html:  `<div>hi</div>`,
			width: 20,
			want:  "hi  \n    \n",
		},
	})
}

func TestVisibilityHidden(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "visibility hidden block preserves vertical space",
			css:   `p { visibility: hidden; }`,
			html:  `<p>hello</p>`,
			width: 20,
			want:  "     \n\n",
		},
		{
			name:  "visibility hidden block between visible blocks",
			css:   `.hidden { visibility: hidden; }`,
			html:  `<p>before</p><p class="hidden">secret</p><p>after</p>`,
			width: 20,
			want:  "before\n\n      \n\nafter\n\n",
		},
		{
			name:  "visibility hidden table cell shows blank",
			css:   `.secret { visibility: hidden; }`,
			html:  `<table style="border-style:none"><tr><th>A</th><th class="secret">B</th></tr></table>`,
			width: 20,
			want:  "A  \n",
		},
		{
			name:  "visibility inherited from parent",
			css:   `div { visibility: hidden; }`,
			html:  `<div><p>secret</p></div>`,
			width: 20,
			// div strips p's trailing newlines; only div's own \n remains
			want: "      \n",
		},
	})
}

func TestCaptionSide(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "caption-side top is default (caption above table)",
			html:  `<table style="border-style:none; caption-side:top"><caption>Title</caption><tr><th>Name</th></tr><tr><td>Alice</td></tr></table>`,
			width: 20,
			want:  "Title\nName \nAlice\n",
		},
		{
			name:  "caption-side bottom places caption below table",
			html:  `<table style="border-style:none; caption-side:bottom"><caption>Footer</caption><tr><th>Name</th></tr><tr><td>Alice</td></tr></table>`,
			width: 20,
			want:  "Name \nAlice\nFooter\n",
		},
	})
}

func TestRenderDisplayNodeBodyLevel(t *testing.T) {
	// Elements at body level (not inside a block) go through renderDisplayNode.
	// This exercises branches that are not reached when elements are inside <p> etc.
	runCases(t, []renderCase{
		// <a href> at body level: tests the href= nodeAttr call in renderDisplayNode
		{name: "anchor with href at body level renders link text", html: `<a href="https://example.com">link</a>`, want: "link"},

		// display:inline-block with percent width at body level
		{name: "inline-block percent width pads content", html: `<span style="display:inline-block; width:50%">hi</span>end`, width: 20, want: "hi        end"},

		// visibility:hidden on inline element at body level
		{name: "visibility hidden inline blanks visible chars", html: `<span style="visibility:hidden">hello</span>`, want: "     "},
	})
}

func TestBlockBorderNoneVariants(t *testing.T) {
	runCases(t, []renderCase{
		// border-right: none disables right border
		{name: "border-right none disables", html: `<p style="border-right:none">hello</p>`, want: "hello\n\n"},
		// border-bottom: none disables bottom border
		{name: "border-bottom none disables", html: `<p style="border-bottom:none">hello</p>`, want: "hello\n\n"},
		// border-style with border-color propagates color to all four sides (ANSI stripped; box chars visible)
		{name: "border-style normal with border-color", html: `<p style="width:100%; border-style:normal; border-color:#888888">hi</p>`, width: 12, want: "┌──────────┐\n│hi        │\n└──────────┘\n\n"},
	})
}

func TestListStyleVariants(t *testing.T) {
	runCases(t, []renderCase{
		{name: "list-style-type disc is default for ul", html: `<ul><li>a</li></ul>`, want: "    • a\n"},
		{name: "list-style-type circle", html: `<ul style="list-style-type:circle"><li>a</li></ul>`, want: "    ○ a\n"},
		{name: "list-style-type square", html: `<ul style="list-style-type:square"><li>a</li></ul>`, want: "    ■ a\n"},
		{name: "list-style-type none on ul", html: `<ul style="list-style-type:none"><li>a</li><li>b</li></ul>`, want: "    a\n    b\n"},
		{name: "list-style-type lower-alpha", html: `<ol style="list-style-type:lower-alpha"><li>x</li><li>y</li></ol>`, want: "    a. x\n    b. y\n"},
		{name: "list-style-type upper-alpha", html: `<ol style="list-style-type:upper-alpha"><li>x</li><li>y</li></ol>`, want: "    A. x\n    B. y\n"},
		{name: "list-style-type none on ol", html: `<ol style="list-style-type:none"><li>x</li></ol>`, want: "    x\n"},
		{name: "list-style-position inside", html: `<ol style="list-style-position:inside; padding-left:0"><li>a</li><li>b</li></ol>`, want: "1. a\n2. b\n"},
		{name: "list-style-position inside with long item wraps to indent", html: `<ol style="list-style-position:inside; padding-left:0"><li>one two three four five six seven</li></ol>`, width: 20, want: "1. one two three\nfour five six\nseven\n"},
		{name: "list with margin-left adds extra indent", html: `<ul style="margin-left:2"><li>a</li></ul>`, want: "      • a\n"},
		{name: "list-style-type custom string double-quoted", html: `<ul style="list-style-type:&quot;→ &quot;"><li>a</li><li>b</li></ul>`, want: "    → a\n    → b\n"},
		{name: "list-style-type custom string single-quoted", html: `<ul style="list-style-type:'* '"><li>a</li><li>b</li></ul>`, want: "    * a\n    * b\n"},
		{name: "list-style-type custom string on ol", html: `<ol style="list-style-type:'# '"><li>x</li><li>y</li></ol>`, want: "    # x\n    # y\n"},
		// li::marker — text layout must be unchanged after ANSI strip
		{name: "li::marker color keeps layout", css: `li::marker { color: #888888; }`, html: `<ul><li>a</li><li>b</li></ul>`, want: "    • a\n    • b\n"},
		{name: "li::marker on ol keeps layout", css: `li::marker { color: #ff0000; }`, html: `<ol><li>x</li><li>y</li></ol>`, want: "    1. x\n    2. y\n"},
		{name: "li::marker with double-colon keeps layout", css: `li::marker { color: #444444; }`, html: `<ul><li>hi</li></ul>`, want: "    • hi\n"},
	})
}

func TestNewHTMLElements(t *testing.T) {
	runCases(t, []renderCase{
		// img
		{name: "img with alt renders bracketed text", html: `<p>See <img src="x.png" alt="diagram"> here</p>`, want: "See [diagram] here\n\n"},
		{name: "img without alt renders nothing", html: `<p>before<img src="x.png">after</p>`, want: "beforeafter\n\n"},

		// noscript
		{name: "noscript content renders (no JS in terminal)", html: `<noscript><p>no js</p></noscript>`, want: "no js\n\n"},

		// wbr
		{name: "wbr emits nothing", html: `<p>word<wbr>break</p>`, want: "wordbreak\n\n"},

		// address
		{name: "address is block with italic style", html: `<address>Author Name</address>`, want: "Author Name\n"},

		// menu (alias for ul)
		{name: "menu renders as unordered list", html: `<menu><li>one</li><li>two</li></menu>`, want: "    • one\n    • two\n"},

		// ol start attribute
		{name: "ol start=5 begins counter at 5", html: `<ol start="5"><li>a</li><li>b</li></ol>`, want: "    5. a\n    6. b\n"},
		{name: "ol start=1 is same as default", html: `<ol start="1"><li>x</li><li>y</li></ol>`, want: "    1. x\n    2. y\n"},
		{name: "ol start=9 crossing into two digits sizes prefix correctly", html: `<ol start="9"><li>a</li><li>b</li></ol>`, want: "     9. a\n    10. b\n"},
		{name: "ol without start defaults to 1", html: `<ol><li>x</li></ol>`, want: "    1. x\n"},

		// details / summary — p's margin-bottom is absorbed by the outer details block
		{name: "details renders expanded with summary as bold block", html: `<details><summary>Title</summary><p>body</p></details>`, want: "Title\nbody\n"},
		{name: "details without summary renders children", html: `<details><p>text</p></details>`, want: "text\n"},

		// caption — appears before the table; centered when table is wider than caption
		{
			name:  "table caption appears before table rows",
			html:  `<table style="border-style:none"><caption>Title</caption><tr><th>Name</th></tr><tr><td>Alice</td></tr></table>`,
			width: 20,
			want:  "Title\nName \nAlice\n",
		},
		{
			name:  "table caption centered when table is wider",
			html:  `<table><caption>Hi</caption><tr><th style="width:10">Col</th></tr></table>`,
			width: 40,
			// width=10 col, border-style:normal: overhead=│+│=2, tableW=12
			// "Hi"(2) centered in 12: 5 left + "Hi" + 5 right
			want: "     Hi     \n┌──────────┐\n│Col       │\n├──────────┤\n└──────────┘\n",
		},
	})
}
