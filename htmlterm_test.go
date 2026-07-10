package htmlterm_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
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
			r, err := htmlterm.New(htmlterm.Options{CSS: tc.css, Width: width})
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

func TestOptionsHeight(t *testing.T) {
	render := func(height int) string {
		t.Helper()
		r, err := htmlterm.New(htmlterm.Options{Width: 40, Height: height})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		out, err := r.Render(`<p>hi</p>`)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return stripANSI(out)
	}
	linesOf := func(out string) []string {
		return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	}

	natural := render(htmlterm.SizeNatural)
	naturalLines := linesOf(natural)
	if len(naturalLines) == 0 || naturalLines[0] != "hi" {
		t.Fatalf("unexpected SizeNatural render: %q", natural)
	}

	t.Run("SizeAutomatic outside Loop behaves like SizeNatural", func(t *testing.T) {
		if got := render(htmlterm.SizeAutomatic); got != natural {
			t.Errorf("render(SizeAutomatic) = %q, want same as SizeNatural %q", got, natural)
		}
	})

	t.Run("positive height pads short content with blank lines", func(t *testing.T) {
		want := len(naturalLines) + 3
		lines := linesOf(render(want))
		if len(lines) != want {
			t.Fatalf("got %d lines, want %d: %v", len(lines), want, lines)
		}
		if lines[0] != "hi" {
			t.Errorf("first line = %q, want %q", lines[0], "hi")
		}
		for _, l := range lines[len(naturalLines):] {
			if l != "" {
				t.Errorf("expected padded lines blank, got %q in %v", l, lines)
			}
		}
	})

	t.Run("positive height truncates tall content", func(t *testing.T) {
		lines := linesOf(render(1))
		if len(lines) != 1 || lines[0] != "hi" {
			t.Errorf("render(1) lines = %v, want [%q]", lines, "hi")
		}
	})
}

func TestIgnoreDocumentCSS(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, IgnoreDocumentCSS: true})
	if err != nil {
		t.Fatal(err)
	}
	// <style> block sets display:none — ignored.
	got, err := r.Render(`<style>p { display: none; }</style><p>hello</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if stripANSI(got) != "hello\n\n" {
		t.Errorf("<style> block not ignored: got %q", got)
	}
	// inline style= sets display:none — ignored.
	got, err = r.Render(`<p style="display:none">hello</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if stripANSI(got) != "hello\n\n" {
		t.Errorf("inline style= not ignored: got %q", got)
	}
}

func TestOptionsStylesheets(t *testing.T) {
	// Cascade order is CSS then Stylesheets in order, same as a page
	// stylesheet followed by however many <link> sheets it loads — later
	// same-specificity declarations win.
	r, err := htmlterm.New(htmlterm.Options{
		Width:       40,
		CSS:         `p { text-transform: uppercase; }`,
		Stylesheets: []string{`p { text-transform: none; }`, `p { text-transform: capitalize; }`},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p>hi there</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Hi There\n\n"; stripANSI(got) != want {
		t.Errorf("Stylesheets order not respected: got %q, want %q", stripANSI(got), want)
	}
}

func TestStripHiddenInline(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{
		Width:             40,
		IgnoreDocumentCSS: true,
		StripHiddenInline: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "display none",
			html: `<p style="display:none">hidden</p><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "visibility hidden",
			html: `<p style="visibility:hidden">hidden</p><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "opacity zero",
			html: `<p style="opacity:0">hidden</p><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "zero height with overflow hidden",
			html: `<div style="height:0;overflow:hidden">hidden</div><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "zero max-height with overflow clip",
			html: `<div style="max-height:0px;overflow:clip">hidden</div><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "hidden ancestor removes styled children too",
			html: `<div style="display:none"><p style="color:red">hidden child</p></div><p>visible</p>`,
			want: "visible\n\n",
		},
		{
			name: "class-based hiding via style block is out of scope",
			html: `<style>.gone { display: none; }</style><p class="gone">still shown</p>`,
			want: "still shown\n\n",
		},
		{
			name: "zero height without overflow hidden is not stripped",
			html: `<div style="height:0">shown</div>`,
			want: "shown\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Render(tc.html)
			if err != nil {
				t.Fatal(err)
			}
			if stripANSI(got) != tc.want {
				t.Errorf("html: %s\ngot:  %q\nwant: %q", tc.html, stripANSI(got), tc.want)
			}
		})
	}
}

func TestStripHiddenInlineDisabledByDefault(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, IgnoreDocumentCSS: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p style="display:none">hidden</p><p>visible</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if stripANSI(got) != "hidden\n\nvisible\n\n" {
		t.Errorf("expected hidden element to remain when StripHiddenInline is unset, got %q", stripANSI(got))
	}
}

func TestTerminalControlSanitization(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render("<p>\x1b[31mred\x1b[0m</p>")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(got) {
		t.Fatalf("raw text control sequence reached output: %q", got)
	}
	if stripANSI(got) != "red\n\n" {
		t.Fatalf("sanitized text got %q", stripANSI(got))
	}

	got, err = r.Render(`<style>p::before { content: "\1b[31m"; }</style><p>red</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(got) {
		t.Fatalf("CSS content control sequence reached output: %q", got)
	}
	if stripANSI(got) != "red\n\n" {
		t.Fatalf("sanitized CSS content got %q", stripANSI(got))
	}
}

func TestBackgroundShorthandColor(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{
		CSS:     `span { background: url(bg.png) no-repeat center/cover #00ff00; }`,
		Width:   40,
		Profile: colorprofile.TrueColor,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p><span style="background: red">hot</span> <span>go</span></p>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[48;2;255;0;0m") {
		t.Fatalf("inline background shorthand did not emit red background: %q", got)
	}
	if !strings.Contains(got, "\x1b[48;2;0;255;0m") {
		t.Fatalf("stylesheet background shorthand did not emit green background: %q", got)
	}
	if stripANSI(got) != "hot go\n\n" {
		t.Fatalf("background shorthand changed text output: %q", stripANSI(got))
	}
}

func TestBorderColorShorthandOnBlock(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 12, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<div style="width:100%; border-style:solid; border-color:#ff0000">hi</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border-color shorthand did not color block border: %q", got)
	}
	if stripANSI(got) != "┌──────────┐\n│hi        │\n└──────────┘\n" {
		t.Fatalf("border-color shorthand changed box shape: %q", stripANSI(got))
	}

	got, err = r.Render(`<div style="width:100%; border-top:'─'; border-right:'│'; border-bottom:'─'; border-left:'│'; border-color: #ff0000 #00ff00">hi</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border-color two-value shorthand did not color top/bottom red: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;0;255;0m") {
		t.Fatalf("border-color two-value shorthand did not color left/right green: %q", got)
	}
}

func TestBorderShorthandOnBlock(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 12, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<div style="width:100%; border: 1px solid #ff0000">hi</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border shorthand did not color block border: %q", got)
	}
	if stripANSI(got) != "┌──────────┐\n│hi        │\n└──────────┘\n" {
		t.Fatalf("border shorthand drew the wrong box shape: %q", stripANSI(got))
	}
}

func TestBorderEdgeShorthand(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 12, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<div style="width:100%; border-top: 1px solid #ff0000">hi</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border-top shorthand did not color the top edge: %q", got)
	}
	if stripANSI(got) != "────────────\nhi          \n" {
		t.Fatalf("border-top shorthand drew the wrong shape: %q", stripANSI(got))
	}

	got, err = r.Render(`<div style="width:100%; border-left: solid #00ff00; border-right: solid #0000ff">hi</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;0;255;0m") {
		t.Fatalf("border-left shorthand did not color the left edge green: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;0;0;255m") {
		t.Fatalf("border-right shorthand did not color the right edge blue: %q", got)
	}
	if stripANSI(got) != "│hi        │\n" {
		t.Fatalf("border-left/border-right shorthand drew the wrong shape: %q", stripANSI(got))
	}
}

func TestTableBorderEdgeColor(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<table style="border-top-color:#ff0000; border-left-color:#00ff00; border-right-color:#0000ff; border-bottom-color:#ffff00"><tr><td style="width:3">A</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border-top-color did not color the top edge red: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;0;255;0m") {
		t.Fatalf("border-left-color did not color the left edge green: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;0;0;255m") {
		t.Fatalf("border-right-color did not color the right edge blue: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;255;255;0m") {
		t.Fatalf("border-bottom-color did not color the bottom edge yellow: %q", got)
	}
	if stripANSI(got) != "┌───┐\n│A  │\n└───┘\n" {
		t.Fatalf("table per-edge color changed the box shape: %q", stripANSI(got))
	}

	got, err = r.Render(`<table style="border-color:#888888; border-top-color:#ff0000"><tr><td style="width:3">A</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("border-top-color did not override the uniform border-color for the top edge: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;136;136;136m") {
		t.Fatalf("border-color did not remain the fallback for edges without their own override: %q", got)
	}
}

// TestStyledTrailingSpaceStaysInsideANSISpan guards against a regression of
// the bug found via cmd/htmlterm-tui: a styled run's trailing space used to
// be pushed outside its ANSI span (appendTextSegment, inline.go) so that
// button::before's "[ " content lost its :focus background-color on the
// space between the bracket and the label. The space must now stay inside
// the same open/close SGR pair as the rest of the run.
func TestStyledTrailingSpaceStaysInsideANSISpan(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p><span style="background-color:#ff0000">red bg </span>next</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "red bg \x1b[m") {
		t.Fatalf("styled trailing space fell outside its ANSI span: %q", got)
	}
	if stripANSI(got) != "red bg next\n\n" {
		t.Fatalf("styled trailing space changed visible text: %q", stripANSI(got))
	}
}

// TestBlockBoundaryTrailingSpaceTrimIsANSISafe guards block.go's
// end-of-block trailing-space trim (removing the one stray space a styled
// run can leave right before a block boundary): it must trim the space
// itself, not corrupt the surrounding SGR escape sequences, even now that
// the space can be inside a styled span rather than always plain.
func TestBlockBoundaryTrailingSpaceTrimIsANSISafe(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p><span style="background-color:#ff0000">text </span></p>`)
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[48;2;255;0;0mtext\x1b[m\n\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestTransparentBackgroundColorIsNoOp guards against background-color:
// transparent being parsed as opaque black (csscolorparser resolves
// "transparent" to rgba(0,0,0,0), and RGB-only style.go code used to render
// that as a black background since it never looked at the alpha channel).
// Messy HTML (e.g. email markup) sets background-color: transparent to mean
// "no background here", which on a terminal with no compositing model should
// be a no-op, not opaque black.
func TestTransparentBackgroundColorIsNoOp(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<p style="background-color: transparent;">Hello World</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("background-color:transparent emitted an ANSI escape: %q", got)
	}
	want := "Hello World\n\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOSCHyperlinkSanitizesHref(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render("<a href=\"https://example.com\x1b]8;;evil\x1b\\\">link</a>")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(got, "\x1b]8;;"); count != 2 {
		t.Fatalf("href injected OSC sequences: count=%d output=%q", count, got)
	}
}

func TestNoscriptDoesNotCorruptOuterCounters(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "noscript recursive render preserves outer counter map",
			css:  `body { counter-reset: sec; } p { counter-increment: sec; } p::before { content: counter(sec) ". "; }`,
			html: `<p>a</p><noscript><b>fallback</b></noscript><p>b</p>`,
			want: "1. a\n\nfallback2. b\n\n",
		},
	})
}

// TestMalformedQuotedCSSDoesNotPanic guards against a regression where a
// quoted CSS token ending in a trailing, unescaped backslash — e.g. from a
// style= attribute crafted by untrusted input — overshot the source
// string's length while scanning the backslash escape, causing a slice
// bounds panic instead of just failing to parse the malformed value. A
// panic here would crash the whole renderer on untrusted HTML, which is
// this package's core use case.
func TestMalformedQuotedCSSDoesNotPanic(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<li style="list-style: 'a\">x</li>`)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("got=%q", got)
}

func TestBareText(t *testing.T) {
	runCases(t, []renderCase{
		{name: "bare text in html element", html: `<html>Hello World</html>`, want: "Hello World"},
		{name: "bare text in body element", html: `<body>hello</body>`, want: "hello"},
		{name: "bare text mixed with block element", html: `<body>before<p>paragraph</p>after</body>`, want: "beforeparagraph\n\nafter"},
		{name: "whitespace-only text between elements is ignored", html: "<body>\n<p>text</p>\n</body>", want: "text\n\n"},
		{name: "bare root text wraps at terminal width", html: `one two three four five six seven`, width: 20, want: "one two three four\nfive six seven"},
		{name: "bare body text wraps without trailing newline", html: `<body>one two three four five six</body>`, width: 14, want: "one two three\nfour five six"},
		{name: "root inline elements wrap together", html: `<span>one two</span> <strong>three four five</strong>`, width: 14, want: "one two three\nfour five"},
	})
}

func TestWhitespaceNormalization(t *testing.T) {
	runCases(t, []renderCase{
		// Block-level edge whitespace: source newlines around heading content
		// collapse to a single space which must be stripped, not rendered.
		{name: "heading with surrounding newlines strips edge whitespace",
			html: "<h1>\nHeading\n</h1>",
			want: "Heading\n"},
		{name: "heading with indented content strips edge whitespace",
			html: "<h2>\n  Heading 2\n</h2>",
			want: "Heading 2\n"},
		// Internal newline inside a heading text node collapses to a single space.
		{name: "internal newline in heading collapses to single space",
			html: "<h1>My\nHeading</h1>",
			want: "My Heading\n"},
		// ::before content ending with a space followed by a text node whose
		// leading space was produced by a source newline: must collapse to one space.
		{name: "pseudo-element trailing space collapses with normalized leading space",
			css:  `h2::before { content: "## "; }`,
			html: "<h2>\nHeading 2\n</h2>",
			want: "## Heading 2\n"},
		// Adjacent spaces across inline element boundaries collapse.
		{name: "trailing space in inline element collapses with following normalized space",
			html: "<p><em>word </em> next</p>",
			want: "word next\n\n"},
		// Bare text node directly after a block element must not inherit a
		// leading space from the collapsed source newline between them.
		{name: "bare text after block element has no leading space",
			html: "<h2>Title</h2>\nBare text",
			want: "Title\nBare text"},
		{name: "bare text after block with surrounding newlines has no leading space",
			html: "<h2>\nTitle\n</h2>\n\nBare text",
			want: "Title\nBare text"},
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
		{name: "display:none hides inline element", html: `<p>before <span style="display:none">hidden</span> after</p>`, want: "before after\n\n"},
		{name: "display:none on block in sequence", html: `<div>a</div><div style="display:none">b</div><div>c</div>`, want: "a\nc\n"},
		{name: "display:none hides top-level unordered list", html: `<p>before</p><ul style="display:none"><li>hidden</li></ul><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides nested unordered list", html: `<blockquote>before<ul style="display:none"><li>hidden</li></ul>after</blockquote>`, want: "│ beforeafter  \n"},
		{name: "display:none hides table", html: `<p>before</p><table style="display:none"><tr><td>x</td></tr></table><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides hr", html: `<p>before</p><hr style="display:none"><p>after</p>`, want: "before\n\nafter\n\n"},
		{name: "display:none hides br", html: `<p>before<br style="display:none">after</p>`, want: "beforeafter\n\n"},
	})
}

func TestDisplay_BlockSectioningElements(t *testing.T) {
	runCases(t, []renderCase{
		{name: "hgroup is block", html: `<hgroup>group</hgroup>`, want: "group\n"},
		{name: "search is block", html: `<search>search</search>`, want: "search\n"},
		{name: "hgroup followed by p", html: `<hgroup>Title</hgroup><p>Body</p>`, want: "Title\nBody\n\n"},
	})
}

func TestGlobalHiddenAttributes(t *testing.T) {
	runCases(t, []renderCase{
		{name: "hidden attribute hides element", html: `<p>visible</p><p hidden>hidden</p><p>after</p>`, want: "visible\n\nafter\n\n"},
		{name: "hidden attribute hides element with children", html: `<div hidden><p>a</p><p>b</p></div><p>after</p>`, want: "after\n\n"},
		{name: "aria-hidden=true hides element", html: `<p>visible</p><p aria-hidden="true">hidden</p><p>after</p>`, want: "visible\n\nafter\n\n"},
		{name: "aria-hidden=false does not hide element", html: `<p>a</p><p aria-hidden="false">b</p><p>c</p>`, want: "a\n\nb\n\nc\n\n"},
		{name: "hidden attribute hides inline element", html: `<p>before <span hidden>hidden</span> after</p>`, want: "before after\n\n"},
		{name: "hidden can be overridden by more specific CSS", css: `.force-show { display: block !important; }`, html: `<p>a</p><p hidden class="force-show">b</p>`, want: "a\n\nb\n\n"},
	})
}

func TestDisplay_Contents(t *testing.T) {
	runCases(t, []renderCase{
		{name: "root-level contents splices block children into flow", html: `<div style="display:contents"><p>one</p><p>two</p></div>`, want: "one\n\ntwo\n"},
		{name: "root-level contents with plain inline content", html: `<span style="display:contents">hello</span>`, want: "hello"},
		{name: "nested contents splices inline text with surrounding siblings", html: `<p>before <span style="display:contents">middle</span> after</p>`, want: "before middle after\n\n"},
		{name: "nested contents does not force its own line break", html: `<p>a<span style="display:contents">b</span>c</p>`, want: "abc\n\n"},
		{name: "contents child that is itself block still forces its own line", html: `<span style="display:contents"><div>block one</div><div>block two</div></span>`, want: "block one\nblock two"},
		{name: "contents inside a block container splices children into that container", html: `<div><span style="display:contents"><p>one</p><p>two</p></span></div>`, want: "one\n\ntwo\n"},
		{name: "contents element's own margin/padding/border is ignored", html: `<span style="display:contents; margin-left:4; padding-left:4; border-left:'|'">text</span>`, want: "text"},
		{name: "display:none on a contents child still hides it", html: `<span style="display:contents"><p>a</p><p style="display:none">b</p><p>c</p></span>`, want: "a\n\nc\n"},
		{name: "contents on a table cell child does not error", html: `<table><tr><td><span style="display:contents">x</span></td></tr></table>`, want: "┌─┐\n│x│\n└─┘\n"},
	})
}

func TestDisplay_ContentsInheritedStyleOnly(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}

	// color is inherited: a display:contents element's own color still
	// reaches its spliced-in text children.
	got, err := r.Render(`<p>before <span style="display:contents; color:#ff0000">red</span> after</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Fatalf("display:contents did not inherit color to children: %q", got)
	}
	if stripANSI(got) != "before red after\n\n" {
		t.Fatalf("display:contents changed text output: %q", stripANSI(got))
	}

	// background-color is not inherited: since a display:contents element
	// generates no box, its own background-color must not leak onto its
	// children (unlike a plain inline span, which does apply it).
	got, err = r.Render(`<span style="display:contents; background-color:#ffff00">bg</span>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\x1b[48;2;255;255;0m") {
		t.Fatalf("display:contents leaked its own background-color onto children: %q", got)
	}

	got, err = r.Render(`<span style="background-color:#ffff00">bg</span>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\x1b[48;2;255;255;0m") {
		t.Fatalf("plain inline span lost its own background-color: %q", got)
	}
}

func TestDisplay_InlineBlock(t *testing.T) {
	runCases(t, []renderCase{
		{name: "inline-block with fixed width pads content", html: `<p><span style="display:inline-block; width:8">hi</span>end</p>`, want: "hi      end\n\n"},
		{name: "inline-block without width acts like inline", html: `<p><span style="display:inline-block">hi</span>end</p>`, want: "hiend\n\n"},
		{name: "two inline-block spans side by side", html: `<p><span style="display:inline-block; width:5">A</span><span style="display:inline-block; width:5">B</span></p>`, want: "A    B    \n\n"},
		{name: "inline-block min-width pads content", html: `<p><span style="display:inline-block; min-width:5">hi</span>end</p>`, want: "hi   end\n\n"},
		{name: "inline-block max-width clamps explicit width", html: `<p><span style="display:inline-block; width:8; max-width:5">hi</span>end</p>`, want: "hi   end\n\n"},
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
		{name: "padding-top inside border-left", css: `div { border-left: "│"; padding-top: 1; width: 5; }`, html: `<div>hi</div>`, want: "│    \n│hi  \n"},
		{name: "padding-bottom inside border-left", css: `div { border-left: "│"; padding-bottom: 1; width: 5; }`, html: `<div>hi</div>`, want: "│hi  \n│    \n"},
		{name: "height expands short content with blank lines", css: `div { height: 3; width: 5; }`, html: `<div>x</div>`, want: "x    \n     \n     \n"},
		{name: "height clips overflow with overflow:hidden", css: `div { height: 1; overflow: hidden; }`, html: "<div>a<br>b</div>", want: "a\n"},
		{name: "height no-ops when content already fills height", css: `div { height: 2; width: 5; }`, html: "<div>a<br>b</div>", want: "a    \nb    \n"},
		// Padding wider than the available box must shrink (right side first,
		// then left) rather than pushing the box past its available width —
		// mirrors the equivalent table-cell clamp in TestTableCellPadding.
		{name: "padding exceeds available width clamps to keep 1-char content", css: `div { border-left: '|'; border-right: '|'; padding-left: 5; padding-right: 5; }`, html: `<div>X</div>`, width: 5, want: "|  X|\n"},
		// Regression test: an explicit width smaller than its own
		// border+padding used to be discarded entirely (hasExplicitWidth
		// left false), silently falling back to full auto/shrink-wrap
		// sizing instead of clamping to a 1-column content minimum — the
		// overflow-x:hidden truncation to "h" (rather than the full
		// "hello") only engages when hasExplicitWidth is actually true.
		{name: "width smaller than its own border+padding clamps instead of being discarded", css: `div { width: 2; padding-left: 1; padding-right: 1; border-left: '|'; border-right: '|'; overflow-x: hidden; white-space: nowrap; }`, html: `<div>hello</div>`, width: 5, want: "| h |\n"},
	})
}

func TestColGroup(t *testing.T) {
	runCases(t, []renderCase{
		{
			name:  "col width-attr sets fixed column width",
			html:  `<table style="border-style:none"><colgroup><col style="width:6"><col style="width:4"></colgroup><tr><th>Name</th><th>Age</th></tr><tr><td>Alice</td><td>30</td></tr></table>`,
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
		{name: "border-left adds char to each line", html: `<p style="border-left:'│'">hello</p>`, want: "│hello\n\n"},
		{name: "border-left with color strips clean", html: `<p style="border-left:'│'; border-left-color:#ff0000">hi</p>`, want: "│hi\n\n"},
		{name: "border-right appends char", html: `<p style="border-right:'│'">hello</p>`, want: "hello│\n\n"},
		{name: "border-left and right together", html: `<p style="border-left:'│'; border-right:'│'">hello</p>`, want: "│hello│\n\n"},
		{name: "border-left none disables", html: `<p style="border-left:none">hello</p>`, want: "hello\n\n"},
		{name: "border-left on multiline pre", html: "<pre style=\"border-left:'│'\">line1\nline2</pre>", want: "│line1\n│line2\n"},
		{name: "border-left via CSS class", css: `.note { border-left: "▌"; }`, html: `<p class="note">text</p>`, want: "▌text\n\n"},
		{name: "blockquote UA has border-left and padding", html: `<blockquote>quoted</blockquote>`, want: "│ quoted  \n"},
		{name: "margin-left adds spaces outside border", html: `<p style="margin-left:4; border-left:'|'; padding-left:1">hi</p>`, want: "    | hi\n\n"},
		{name: "margin-right adds spaces outside border", html: `<p style="margin-right:4; border-right:'|'; padding-right:1">hi</p>`, want: "hi |    \n\n"},
		{name: "margin-left only no border", html: `<p style="margin-left:2">hello</p>`, want: "  hello\n\n"},
		{name: "nested border-left stacks characters", html: `<div style="border-left:'|'"><div style="border-left:'|'">inner</div></div>`, want: "||inner\n"},
		{name: "width:100% fills renderer width with borders", html: `<p style="width:100%; border-left:'['; border-right:']'">hi</p>`, want: "[hi                  ]\n\n", width: 22},
		{name: "width:100% with margin subtracts margin from line width", html: `<p style="width:100%; margin-left:2; margin-right:2; border-left:'['; border-right:']'">hi</p>`, want: "  [hi                  ]  \n\n", width: 26},
		{name: "border-top draws horizontal rule before content", html: `<p style="border-top:'─'">hi</p>`, width: 10, want: "──────────\nhi\n\n"},
		{name: "border-bottom draws horizontal rule after content", html: `<p style="border-bottom:'─'">hi</p>`, width: 10, want: "hi\n──────────\n\n"},
		{name: "border-top none is disabled", html: `<p style="border-top:none">hi</p>`, width: 10, want: "hi\n\n"},
		{name: "all four borders with width:100%", html: `<p style="width:100%; border-top:'─'; border-bottom:'─'; border-left:'['; border-right:']'">hi</p>`, width: 12, want: "────────────\n[hi        ]\n────────────\n\n"},
		{name: "corners replace fill endpoints on top and bottom rules", html: `<p style="width:100%; border-top:'─'; border-bottom:'─'; border-left:'│'; border-right:'│'; border-top-left-corner:'┌'; border-top-right-corner:'┐'; border-bottom-left-corner:'└'; border-bottom-right-corner:'┘'">hi</p>`, width: 12, want: "┌──────────┐\n│hi        │\n└──────────┘\n\n"},
		{name: "corner without opposite border uses fill for that side", html: `<p style="width:100%; border-top:'─'; border-top-left-corner:'┌'">hi</p>`, width: 12, want: "┌───────────\nhi          \n\n"},
		{name: "border-top and border-bottom with margin-left", html: `<p style="width:100%; border-top:'─'; border-bottom:'─'; margin-left:2">hi</p>`, width: 12, want: "  ──────────\n  hi        \n  ──────────\n\n"},
		{name: "border-top with color strips clean", html: `<p style="border-top:'─'; border-top-color:#ff0000">hi</p>`, width: 10, want: "──────────\nhi\n\n"},
		{name: "margin auto both centers element", html: `<p style="width:10; margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin-left auto pushes element to right", html: `<p style="width:10; margin-left:auto">hi</p>`, width: 20, want: "          hi        \n\n"},
		{name: "margin-right auto fills trailing space", html: `<p style="width:10; margin-right:auto">hi</p>`, width: 20, want: "hi                  \n\n"},
		{name: "margin auto with percent width centers", html: `<p style="width:50%; margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin-left and margin-right as percentages", html: `<p style="margin-left:25%; margin-right:25%">hi</p>`, width: 80, want: "                    hi                    \n\n"},
		{name: "margin auto without explicit width is ignored", html: `<p style="margin-left:auto; margin-right:auto">hi</p>`, width: 20, want: "hi\n\n"},
		{name: "margin auto center via CSS class", css: `.center { width: 10; margin-left: auto; margin-right: auto; }`, html: `<p class="center">hi</p>`, width: 20, want: "     hi             \n\n"},
		{name: "margin auto center with borders", html: `<p style="width:12; margin-left:auto; margin-right:auto; border-left:'['; border-right:']'">hi</p>`, width: 20, want: "    [hi        ]    \n\n"},
		{name: "min-width expands explicit block width", html: `<p style="width:5; min-width:8">hi</p>`, width: 20, want: "hi      \n\n"},
		{name: "max-width clamps explicit block width", html: `<p style="width:12; max-width:8">hi</p>`, width: 20, want: "hi      \n\n"},
		{name: "max-width constrains auto block width", html: `<p style="max-width:10">one two three</p>`, width: 40, want: "one two   \nthree     \n\n"},
		{name: "percentage min-width expands block width", html: `<p style="width:5; min-width:50%">hi</p>`, width: 20, want: "hi        \n\n"},
		{name: "border-right aligns on word-wrapped lines", html: `<p style="border-right:'|'">one two three four five</p>`, width: 14, want: "one two three|\nfour five    |\n\n"},
		{name: "padding-right aligns on word-wrapped lines", html: `<p style="padding-right:2">one two three four five</p>`, width: 15, want: "one two three  \nfour five      \n\n"},
		{name: "border-right single-line content stays flush", html: `<p style="border-right:'|'">hello</p>`, width: 20, want: "hello|\n\n"},
		{name: "border-right with margin-right aligns on wrapped lines", html: `<p style="border-right:'|'; margin-right:2">one two three four five</p>`, width: 16, want: "one two three|  \nfour five    |  \n\n"},
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
		// A <br> inside a hidden cell still occupies its line in the row height;
		// blanking must preserve line count, not collapse the cell to one line.
		{
			name:  "visibility hidden table cell with br preserves line count",
			css:   `.secret { visibility: hidden; }`,
			html:  `<table style="border-style:none"><tr><td class="secret">a<br>b</td></tr><tr><td>XX</td></tr></table>`,
			width: 20,
			want:  "  \n  \nXX\n",
		},
		{
			name:  "visibility inherited from parent",
			css:   `div { visibility: hidden; }`,
			html:  `<div><p>secret</p></div>`,
			width: 20,
			// div strips p's trailing newlines; only div's own \n remains
			want: "      \n",
		},
		{
			name:  "visibility hidden inline-block blanks content",
			html:  `<p><span style="display:inline-block; visibility:hidden">secret</span>end</p>`,
			width: 20,
			want:  "      end\n\n",
		},
		// display:block element with visibility:hidden must blank (fix for case "block" in renderDisplayNode)
		{
			name:  "visibility hidden display:block element blanks content",
			html:  `<div style="visibility:hidden">secret</div>`,
			width: 20,
			want:  "      \n",
		},
		// display:block child inside renderInlineAcc must also blank (fix for case "block" in renderInlineAcc)
		{
			name:  "visibility hidden block child in block context blanks content",
			html:  `<section><div style="display:block; visibility:hidden">s</div>ok</section>`,
			width: 20,
			want:  " \nok\n",
		},
		// <li visibility:hidden> must blank (fix: renderList checks li's own visibility)
		{
			name:  "visibility hidden li element blanks content and prefix",
			html:  `<ul><li style="visibility:hidden">secret</li><li>visible</li></ul>`,
			width: 20,
			want:  "            \n    • visible\n",
		},
		// quoteDepth must be restored after rendering a visibility:hidden element
		// so subsequent <q> elements use the correct nesting level.
		{
			name:  "quoteDepth not leaked from visibility:hidden block element",
			css:   `div::before { content: open-quote; }`,
			html:  `<div style="visibility:hidden">x</div><q>y</q>`,
			width: 40,
			want:  "  \n“y”",
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
		{name: "border-style solid with border-color", html: `<p style="width:100%; border-style:solid; border-color:#888888">hi</p>`, width: 12, want: "┌──────────┐\n│hi        │\n└──────────┘\n\n"},
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
		{name: "list-style shorthand sets type and position", html: `<ol style="list-style:upper-roman inside; padding-left:0"><li>x</li><li>y</li></ol>`, want: " I. x\nII. y\n"},
		{name: "list-style shorthand ignores url image", html: `<ul style="list-style:url(bullet.png) square"><li>a</li></ul>`, want: "    ■ a\n"},
		{name: "list-style shorthand custom string keeps trailing space", html: `<ul style="list-style:'-> ' inside; padding-left:0"><li>a</li><li>b</li></ul>`, want: "-> a\n-> b\n"},
		{name: "list with margin-left adds extra indent", html: `<ul style="margin-left:2"><li>a</li></ul>`, want: "      • a\n"},
		{name: "list-style-type custom string double-quoted", html: `<ul style="list-style-type:&quot;→ &quot;"><li>a</li><li>b</li></ul>`, want: "    → a\n    → b\n"},
		{name: "list-style-type custom string single-quoted", html: `<ul style="list-style-type:'* '"><li>a</li><li>b</li></ul>`, want: "    * a\n    * b\n"},
		{name: "list-style-type custom string on ol", html: `<ol style="list-style-type:'# '"><li>x</li><li>y</li></ol>`, want: "    # x\n    # y\n"},
		{name: "list-style-type custom string decodes CSS escape sequences", html: `<ul style="list-style-type:'\2192  '"><li>a</li></ul>`, want: "    → a\n"},
		// li::marker — text layout must be unchanged after ANSI strip
		{name: "li::marker color keeps layout", css: `li::marker { color: #888888; }`, html: `<ul><li>a</li><li>b</li></ul>`, want: "    • a\n    • b\n"},
		{name: "li::marker on ol keeps layout", css: `li::marker { color: #ff0000; }`, html: `<ol><li>x</li><li>y</li></ol>`, want: "    1. x\n    2. y\n"},
		{name: "li::marker with double-colon keeps layout", css: `li::marker { color: #444444; }`, html: `<ul><li>hi</li></ul>`, want: "    • hi\n"},
		// CSS \A escape inside a quoted list-style-type string must be stripped so
		// it does not break visual-width accounting or produce a literal newline.
		{name: "list-style-type custom string strips CSS \\A newline escape", html: `<ul style="list-style-type:'\A* '"><li>word1 word2 word3 word4 word5</li></ul>`, width: 16,
			want: "    * word1\n      word2\n      word3\n      word4\n      word5\n"},
	})
}

func TestListBoxModel(t *testing.T) {
	runCases(t, []renderCase{
		// margin-bottom / margin-top
		{name: "ul margin-bottom separates list from following paragraph",
			css:  `ul { margin-bottom: 1; }`,
			html: `<ul><li>a</li></ul><p>after</p>`,
			want: "    • a\n\nafter\n\n"},
		{name: "ol margin-bottom separates list from following paragraph",
			css:  `ol { margin-bottom: 1; }`,
			html: `<ol><li>a</li></ol><p>after</p>`,
			want: "    1. a\n\nafter\n\n"},
		{name: "ul margin-top adds space before list",
			html: `<p>before</p><ul style="margin-top:2"><li>a</li></ul>`,
			want: "before\n\n\n    • a\n"},
		{name: "ol margin-top adds space before list",
			html: `<p>before</p><ol style="margin-top:2"><li>a</li></ol>`,
			want: "before\n\n\n    1. a\n"},

		// padding-top / padding-bottom
		{name: "ul padding-top adds blank line before items",
			html: `<ul style="padding-top:1"><li>a</li></ul>`,
			want: "\n    • a\n"},
		{name: "ul padding-bottom adds blank line after items",
			html: `<ul style="padding-bottom:1"><li>a</li></ul>`,
			want: "    • a\n\n"},
		{name: "ol padding-top adds blank line before items",
			html: `<ol style="padding-top:1"><li>a</li></ol>`,
			want: "\n    1. a\n"},
		{name: "ol padding-bottom adds blank line after items",
			html: `<ol style="padding-bottom:1"><li>a</li></ol>`,
			want: "    1. a\n\n"},

		// margin-right / padding-right reduce wrap width
		// width=20, UA padding-left=4, bullet "• "=2 → contentWidth = 20-4-2 = 14
		// with right-side indent of 4 → availWidth = 16, contentWidth = 10
		// "one two three four" wraps as: "one two" (7) + "three four" (10)
		{name: "ul margin-right reduces content wrap width",
			html:  `<ul style="margin-right:4"><li>one two three four</li></ul>`,
			width: 20,
			want:  "    • one two\n      three four\n"},
		{name: "ul padding-right reduces content wrap width",
			html:  `<ul style="padding-right:4"><li>one two three four</li></ul>`,
			width: 20,
			want:  "    • one two\n      three four\n"},
		// ol prefix "1. " is 3 chars → contentWidth = 20-4(padding-left)-4(right)-3(prefix) = 9
		// "one two three four" wraps as: "one two" (7) + "three" (5) + "four" (4)
		{name: "ol margin-right reduces content wrap width",
			html:  `<ol style="margin-right:4"><li>one two three four</li></ol>`,
			width: 20,
			want:  "    1. one two\n       three\n       four\n"},
		{name: "ol padding-right reduces content wrap width",
			html:  `<ol style="padding-right:4"><li>one two three four</li></ol>`,
			width: 20,
			want:  "    1. one two\n       three\n       four\n"},
	})
}

func TestNestedList(t *testing.T) {
	runCases(t, []renderCase{
		// direct-child nesting (nested list is sibling of <li>, not inside one)
		{name: "direct-child ol in ol matches proper nesting",
			html: `<ol><li>outer 1</li><ol><li>inner 1</li><li>inner 2</li></ol><li>outer 2</li><li>outer 3</li></ol>`,
			want: "    1. outer 1\n           1. inner 1\n           2. inner 2\n    2. outer 2\n    3. outer 3\n"},
		{name: "direct-child ul in ul",
			html: `<ul><li>a</li><ul><li>b</li><li>c</li></ul><li>d</li></ul>`,
			want: "    • a\n          • b\n          • c\n    • d\n"},
		{name: "direct-child ol in ul",
			html: `<ul><li>a</li><ol><li>b</li><li>c</li></ol><li>d</li></ul>`,
			want: "    • a\n          1. b\n          2. c\n    • d\n"},
		// proper nesting (nested list inside <li>) produces same indentation
		{name: "proper ol in li",
			html: `<ol><li>outer 1<ol><li>inner 1</li><li>inner 2</li></ol></li><li>outer 2</li><li>outer 3</li></ol>`,
			want: "    1. outer 1\n           1. inner 1\n           2. inner 2\n    2. outer 2\n    3. outer 3\n"},
		// edge: nested list before any <li>
		{name: "direct-child nested list before first li",
			html: `<ol><ol><li>inner</li></ol><li>outer</li></ol>`,
			want: "           1. inner\n    1. outer\n"},
		// edge: nested list after all <li>
		{name: "direct-child nested list after last li",
			html: `<ol><li>outer</li><ol><li>inner</li></ol></ol>`,
			want: "    1. outer\n           1. inner\n"},
		// three levels deep
		{name: "three levels of direct-child nesting",
			html: `<ol><li>a</li><ol><li>b</li><ol><li>c</li></ol></ol><li>d</li></ol>`,
			want: "    1. a\n           1. b\n                  1. c\n    2. d\n"},
	})
}

func TestNewHTMLElements(t *testing.T) {
	runCases(t, []renderCase{
		// img
		{name: "img with alt renders alt text", html: `<p>See <img src="x.png" alt="diagram"> here</p>`, want: "See diagram here\n\n"},
		{name: "img without alt renders nothing", html: `<p>before<img src="x.png">after</p>`, want: "beforeafter\n\n"},

		// abbr
		{name: "abbr with title appends title in parens", html: `<p><abbr title="HyperText Markup Language">HTML</abbr></p>`, want: "HTML (HyperText Markup Language)\n\n"},
		{name: "abbr without title renders text only", html: `<p><abbr>CSS</abbr></p>`, want: "CSS\n\n"},
		{name: "abbr title override via CSS", css: `abbr[title]::after { content: " [" attr(title) "]"; }`, html: `<p><abbr title="W3C">W3C</abbr></p>`, want: "W3C [W3C]\n\n"},

		// hr
		{name: "hr renders full-width rule", html: `<hr>`, width: 10, want: "──────────\n"},
		{name: "hr between paragraphs", html: `<p>above</p><hr><p>below</p>`, width: 5, want: "above\n\n─────\nbelow\n\n"},
		{name: "hr character override via CSS", css: `hr { border-top: "="; }`, html: `<hr>`, width: 5, want: "=====\n"},
		{name: "hr color override via CSS", css: `hr { border-top-color: #ff0000; }`, html: `<hr>`, width: 5, want: "─────\n"},

		// noscript
		{name: "noscript content renders (no JS in terminal)", html: `<noscript><p>no js</p></noscript>`, want: "no js\n\n"},

		// wbr
		{name: "wbr emits nothing", html: `<p>word<wbr>break</p>`, want: "wordbreak\n\n"},

		// template
		{name: "template emits nothing at top level", html: `before<template><p>hidden</p></template>after`, want: "beforeafter"},
		{name: "template emits nothing in inline content", html: `<p>before<template><span>hidden</span></template>after</p>`, want: "beforeafter\n\n"},
		{name: "template emits nothing in table cells", html: `<table style="border-style:hidden"><tr><td style="width:11">before<template>hidden</template>after</td></tr></table>`, want: "beforeafter\n"},
		{name: "template style is inert", html: `<template><style>p { display: none; }</style></template><p>visible</p>`, want: "visible\n\n"},
		{name: "template counters are inert", css: `body { counter-reset: n; } p { counter-increment: n; } p::before { content: counter(n) ". "; }`, html: `<p>a</p><template><p>hidden</p></template><p>b</p>`, want: "1. a\n\n2. b\n\n"},

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
			// width=10 col, border-style:solid: overhead=│+│=2, tableW=12
			// "Hi"(2) centered in 12: 5 left + "Hi" + 5 right
			want: "     Hi     \n┌──────────┐\n│Col       │\n├──────────┤\n└──────────┘\n",
		},
	})
}

func TestPseudoElementContent(t *testing.T) {
	runCases(t, []renderCase{
		{name: "::before content prepended to p", css: `p::before { content: "→ "; }`, html: `<p>hello</p>`, want: "→ hello\n\n"},
		{name: "::after content appended to p", css: `p::after { content: " ←"; }`, html: `<p>hello</p>`, want: "hello ←\n\n"},
		{name: "::before and ::after together", css: `p::before { content: "["; } p::after { content: "]"; }`, html: `<p>hi</p>`, want: "[hi]\n\n"},
		{name: "::before content none suppresses injection", css: `p::before { content: none; }`, html: `<p>hello</p>`, want: "hello\n\n"},
		{name: "::before text-transform uppercase on pseudo", css: `p::before { content: "note: "; text-transform: uppercase; }`, html: `<p>hello</p>`, want: "NOTE: hello\n\n"},
		{name: "::before inherits parent text-transform", css: `p { text-transform: uppercase; } p::before { content: "note: "; }`, html: `<p>hello</p>`, want: "NOTE: HELLO\n\n"},
		{name: "::before own text-transform overrides parent", css: `p { text-transform: uppercase; } p::before { content: "Note: "; text-transform: lowercase; }`, html: `<p>hello</p>`, want: "note: HELLO\n\n"},
		{name: "::before on pre prepends to code", css: `pre::before { content: "$ "; }`, html: `<pre>code</pre>`, want: "$ code\n"},
		{name: "::after on pre appends to code", css: `pre::after { content: " #"; }`, html: `<pre>code</pre>`, want: "code #\n"},
		{name: "::before on multiline pre is on first line only", css: `pre::before { content: "> "; }`, html: "<pre>line1\nline2</pre>", want: "> line1\nline2\n"},
		// CSS string escape sequences
		{name: `content \A escape inserts newline`, css: "pre::before { content: \"```\\A\"; }", html: "<pre>code</pre>", want: "```\ncode\n"},
		{name: `content \\ escape inserts backslash`, css: `p::before { content: "a\\b"; }`, html: `<p>x</p>`, want: "a\\bx\n\n"},
		{name: `content \" escape inserts quote`, css: `p::before { content: "say \"hi\""; }`, html: `<p>.</p>`, want: "say \"hi\".\n\n"},
		{name: `content hex \000A escape inserts newline`, css: "pre::before { content: \"```\\000A\"; }", html: "<pre>code</pre>", want: "```\ncode\n"},
		// img CSS override: Markdown output
		{name: "img markdown via CSS attr(alt) and attr(src)", css: `img::before { content: "![" attr(alt) "](" attr(src) ")"; }`, html: `<img src="photo.jpg" alt="sunset">`, want: "![sunset](photo.jpg)"},
		{name: "img markdown with no alt via CSS", css: `img::before { content: "![" attr(alt) "](" attr(src) ")"; }`, html: `<img src="photo.jpg">`, want: "![](photo.jpg)"},
		{name: "img CSS override suppresses alt brackets", css: `img[alt]::before { content: attr(alt); }`, html: `<p>See <img src="x.png" alt="graph"> here</p>`, want: "See graph here\n\n"},
		// attr() in content
		{name: "attr(href) in ::after renders link as markdown", css: `a::before { content: "["; } a::after { content: "](" attr(href) ")"; }`, html: `<a href="https://example.com">Example</a>`, want: "[Example](https://example.com)"},
		{name: "attr() with missing attribute yields empty string", css: `a::after { content: attr(data-missing); }`, html: `<a href="#">click</a>`, want: "click"},
		{name: "attr() concatenated with string tokens", css: `a::after { content: " [" attr(href) "]"; }`, html: `<a href="/page">link</a>`, want: "link [/page]"},
		// counter() and counter-increment / counter-reset
		{name: "counter() numbers headings sequentially",
			css:  `h2 { counter-increment: sec; } h2::before { content: counter(sec) ". "; }`,
			html: `<h2>A</h2><h2>B</h2><h2>C</h2>`,
			want: "1. A\n2. B\n3. C\n"},
		{name: "counter-reset restarts counter",
			css:  `h2 { counter-increment: sec; } h2::before { content: counter(sec) ". "; } div { counter-reset: sec; }`,
			html: `<div><h2>A</h2><h2>B</h2></div><div><h2>C</h2></div>`,
			want: "1. A\n2. B\n1. C\n"},
		{name: "counter() with lower-alpha style suppressing list marker",
			css:  `ul { list-style-type: none; } li { counter-increment: item; } li::before { content: counter(item, lower-alpha) ") "; }`,
			html: `<ul><li>x</li><li>y</li><li>z</li></ul>`,
			want: "    a) x\n    b) y\n    c) z\n"},
		{name: "counter() with upper-roman style suppressing list marker",
			css:  `ul { list-style-type: none; } li { counter-increment: ch; } li::before { content: counter(ch, upper-roman) ". "; }`,
			html: `<ul><li>one</li><li>two</li><li>three</li></ul>`,
			want: "    I. one\n    II. two\n    III. three\n"},
		{name: "counter-reset with explicit start value",
			css:  `h2 { counter-increment: sec; } h2::before { content: counter(sec) ". "; } div { counter-reset: sec 4; }`,
			html: `<div><h2>A</h2><h2>B</h2></div>`,
			want: "5. A\n6. B\n"},
		{name: "counters() produces nested numbering",
			css:  `ol { counter-reset: item; list-style-type: none; } li { counter-increment: item; } li::before { content: counters(item, ".") " "; }`,
			html: `<ol><li>A<ol><li>B</li><li>C</li></ol></li><li>D</li></ol>`,
			want: "    1 A\n        1.1 B\n        1.2 C\n    2 D\n"},
		{name: "counters() separator containing ) is not truncated",
			css:  `ol { counter-reset: item; list-style-type: none; } li { counter-increment: item; } li::before { content: counters(item, ")") " "; }`,
			html: `<ol><li>A<ol><li>B</li></ol></li></ol>`,
			want: "    1 A\n        1)1 B\n"},
		{name: "counter() with lower-alpha style",
			css:  `p { counter-increment: c; } p::before { content: counter(c, lower-alpha) ". "; }`,
			html: `<p>a</p><p>b</p><p>c</p>`,
			want: "a. a\n\nb. b\n\nc. c\n\n"},
		{name: "counter() lower-alpha n>26 produces multi-char sequence",
			css:  `ol { counter-reset: c 24; list-style-type: none; } li { counter-increment: c; } li::before { content: counter(c, lower-alpha) ". "; }`,
			html: `<ol><li>a</li><li>b</li><li>c</li><li>d</li></ol>`,
			want: "    y. a\n    z. b\n    aa. c\n    ab. d\n"},
		// open-quote / close-quote
		{name: "open-quote and close-quote on q element use smart quotes",
			html: `<q>hello</q>`,
			want: "“hello”"},
		{name: "nested q elements use second-level quotes",
			html: `<q>outer <q>inner</q> end</q>`,
			want: "“outer ‘inner’ end”"},
		{name: "custom quotes property overrides defaults",
			css:  `q { quotes: '"' '"'; }`,
			html: `<q>hello</q>`,
			want: `"hello"`},
		{name: "open-quote close-quote in ::before ::after on non-q element",
			css:  `div::before { content: open-quote; } div::after { content: close-quote; }`,
			html: `<div>text</div>`,
			want: "“text”\n"},
	})
}

func TestPreVerticalSpacing(t *testing.T) {
	runCases(t, []renderCase{
		{name: "pre padding-top adds blank line", html: `<pre style="padding-top:1; width:10">code</pre>`, want: "          \ncode      \n"},
		{name: "pre padding-bottom adds blank line", html: `<pre style="padding-bottom:1; width:10">code</pre>`, want: "code      \n          \n"},
		{name: "pre padding-top and bottom", html: `<pre style="padding-top:1; padding-bottom:1; width:10">code</pre>`, want: "          \ncode      \n          \n"},
		{name: "pre margin-top adds newline before", html: `<div>x</div><pre style="margin-top:1">code</pre>`, want: "x\n\ncode\n"},
		{name: "pre margin-bottom adds newline after", html: `<pre style="margin-bottom:1">code</pre><div>x</div>`, want: "code\n\nx\n"},
		{name: "pre ::before with padding-top", css: `pre::before { content: "$ "; }`, html: `<pre style="padding-top:1; width:10">code</pre>`, want: "          \n$ code    \n"},
		{name: "pre ::after with padding-bottom", css: `pre::after { content: " #"; }`, html: `<pre style="padding-bottom:1; width:10">code</pre>`, want: "code #    \n          \n"},
	})
}

func TestMaxBlankLines(t *testing.T) {
	render := func(maxBlankLines int, css, htmlStr string) string {
		r, err := htmlterm.New(htmlterm.Options{Width: 40, MaxBlankLines: maxBlankLines, CSS: css})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		got, err := r.Render(htmlStr)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return stripANSI(got)
	}

	// Large margins collapse to MaxBlankLines blank lines (MaxBlankLines+1 newlines).
	got := render(1, `div { margin-bottom: 5; }`, `<div>A</div><div>B</div>`)
	if got != "A\n\nB\n\n" {
		t.Errorf("large margin collapse: got %q want %q", got, "A\n\nB\n\n")
	}

	// ::before content with \A newlines is capped.
	got = render(1, `div::before { content: ">\A\A\A"; }`, `<div>text</div>`)
	if got != ">\n\ntext\n" {
		t.Errorf("::before newline cap: got %q want %q", got, ">\n\ntext\n")
	}

	// ::after content with \A newlines is capped.
	got = render(1, `div::after { content: "\A\A\A<"; }`, `<div>text</div>`)
	if got != "text\n\n<\n" {
		t.Errorf("::after newline cap: got %q want %q", got, "text\n\n<\n")
	}

	// <pre> internal blank lines are NOT capped.
	got = render(1, "", "<pre>line1\n\n\n\nline2</pre>")
	if got != "line1\n\n\n\nline2\n" {
		t.Errorf("pre content unaffected: got %q want %q", got, "line1\n\n\n\nline2\n")
	}

	// MaxBlankLines: 0 disables capping (default behavior).
	got = render(0, `div { margin-bottom: 5; }`, `<div>A</div><div>B</div>`)
	if got != "A\n\n\n\n\n\nB\n\n\n\n\n\n" {
		t.Errorf("MaxBlankLines=0 disabled: got %q", got)
	}

	// Bare \n writes between margin calls must not accumulate past the cap.
	// Five consecutive empty paragraphs (margin-bottom:1 each) must not
	// produce more than MaxBlankLines+1 consecutive newlines anywhere.
	got = render(2, "", `<p></p><p></p><p></p><p></p><p></p>`)
	if regexp.MustCompile(`\n{4,}`).MatchString(got) {
		t.Errorf("bare newline accumulation: got %q (contains 4+ consecutive newlines with MaxBlankLines=2)", got)
	}

	// A trailing <br><br> inside one element (2 blank lines of real content)
	// followed immediately by a sibling with its own margin-top: sized so
	// neither contribution alone reaches the cap. Current cappedWriter
	// semantics take the MAX of the two (WriteAtLeastNewlines "ensures at
	// least n newlines are pending", never sums) — confirmed empirically:
	// uncapped (MaxBlankLines:0) this is "text\n\n\nnext\n" (2 blank lines
	// total), not 4 blank lines, so a box-based rewrite's margin-collapse
	// arithmetic must treat a child's own trailing blank content as
	// equivalent to a margin-bottom value for collapse purposes against the
	// next sibling's margin-top, not simply concatenate-then-add.
	got = render(0, `div { margin-top: 2; }`, `<div>text<br><br></div><div>next</div>`)
	if got != "text\n\n\nnext\n" {
		t.Errorf("br-tail + margin-top must collapse via max, not sum: got %q want %q", got, "text\n\n\nnext\n")
	}
	got = render(10, `div { margin-top: 2; }`, `<div>text<br><br></div><div>next</div>`)
	if got != "text\n\n\nnext\n" {
		t.Errorf("br-tail + margin-top (capping enabled but not triggered): got %q want %q", got, "text\n\n\nnext\n")
	}

	// A <pre> block nested inside a <div>, followed by a sibling with its
	// own margin-top: the pre exemption must survive crossing the boundary
	// into the next element's margin-collapse arithmetic.
	got = render(2, `div { margin-top: 5; }`, "<div><pre>a\n\n\n\nb</pre></div><div>next</div>")
	if got != "a\n\n\nb\n\n\nnext\n" {
		t.Errorf("pre exemption across element boundary: got %q want %q", got, "a\n\n\nb\n\n\nnext\n")
	}
}
