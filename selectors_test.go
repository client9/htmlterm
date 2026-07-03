package htmlterm_test

import "testing"

func TestSelectors(t *testing.T) {
	runCases(t, []renderCase{
		{name: "#id selector targets element with matching id", css: `#hero { text-transform: uppercase; }`, html: `<p id="hero">featured</p><p>normal</p>`, want: "FEATURED\n\nnormal\n\n"},
		{name: "#id selector does not match a different id", css: `#other { text-transform: uppercase; }`, html: `<p id="hero">text</p>`, want: "text\n\n"},
		{name: "#id in descendant selector chain", css: `#main p { text-transform: uppercase; }`, html: `<div id="main"><p>inside</p></div><p>outside</p>`, want: "INSIDE\noutside\n\n"},
		{name: "element#id combined selector", css: `p#hero { text-transform: uppercase; }`, html: `<p id="hero">match</p><div id="hero">no-match</div>`, want: "MATCH\n\nno-match\n"},
		{name: "multi-class requires all classes to be present", css: `.warn.big { text-transform: uppercase; }`, html: `<p class="warn big">both</p><p class="warn">warn-only</p><p class="big">big-only</p>`, want: "BOTH\n\nwarn-only\n\nbig-only\n\n"},
		{name: "element plus two classes", css: `p.a.b { text-transform: uppercase; }`, html: `<p class="a b">para</p><div class="a b">div</div>`, want: "PARA\n\ndiv\n"},
		{name: "extra classes on element do not prevent match", css: `.highlight { text-transform: uppercase; }`, html: `<p class="highlight extra">text</p>`, want: "TEXT\n\n"},
		{name: "universal selector matches any element", css: `* { text-transform: uppercase; }`, html: `<p>alpha <span>beta</span></p><div>gamma</div>`, want: "ALPHA BETA\n\nGAMMA\n"},
		{name: "universal selector has zero specificity", css: `p { text-transform: lowercase; } * { text-transform: uppercase; }`, html: `<p>MiX</p>`, want: "mix\n\n"},
		{name: "universal selector combines with classes", css: `*.hot { text-transform: uppercase; }`, html: `<p class="hot">match</p><p>plain</p>`, want: "MATCH\n\nplain\n\n"},
		{name: "child combinator matches direct children only", css: `div > p { text-transform: uppercase; }`, html: `<div><p>direct</p><section><p>nested</p></section></div>`, want: "DIRECT\n\nnested\n"},
		{name: "descendant combinator still matches all levels", css: `div p { text-transform: uppercase; }`, html: `<div><p>direct</p><section><p>nested</p></section></div>`, want: "DIRECT\n\nNESTED\n"},
		{name: "child combinator in a deeper chain", css: `div > p > span { text-transform: uppercase; }`, html: `<div><p><span>deep</span> rest</p></div>`, want: "DEEP rest\n"},
		{name: "mixed child and descendant combinators", css: `div > ul li { text-transform: uppercase; }`, html: `<div><ul><li>match</li></ul></div><ul><li>no-match</li></ul>`, want: "    • MATCH\n    • no-match\n"},
		{name: ":root matches document element", css: `:root { text-transform: uppercase; }`, html: `<p>from root</p>`, want: "FROM ROOT\n\n"},
		{name: ":root has pseudo-class specificity", css: `:root { text-transform: uppercase; } html { text-transform: lowercase; }`, html: `<p>MiX</p>`, want: "MIX\n\n"},
		{name: ":root works in combinator chains", css: `:root > body p { text-transform: uppercase; }`, html: `<p>inside body</p>`, want: "INSIDE BODY\n\n"},
		{name: ":first-child matches first element sibling", css: `li:first-child { text-transform: uppercase; }`, html: `<ul><li>one</li><li>two</li><li>three</li></ul>`, want: "    • ONE\n    • two\n    • three\n"},
		{name: ":last-child matches last element sibling", css: `li:last-child { text-transform: uppercase; }`, html: `<ul><li>one</li><li>two</li><li>three</li></ul>`, want: "    • one\n    • two\n    • THREE\n"},
		{name: ":first-child and :last-child both match a single item", css: `li:first-child { text-transform: uppercase; } li:last-child { text-transform: uppercase; }`, html: `<ul><li>only</li></ul>`, want: "    • ONLY\n"},
		{name: ":nth-child(odd) matches 1st 3rd 5th element siblings", css: `p:nth-child(odd) { text-transform: uppercase; }`, html: `<div><p>one</p><p>two</p><p>three</p></div>`, want: "ONE\n\ntwo\n\nTHREE\n"},
		{name: ":nth-child(even) matches 2nd 4th element siblings", css: `p:nth-child(even) { text-transform: uppercase; }`, html: `<div><p>one</p><p>two</p><p>three</p></div>`, want: "one\n\nTWO\n\nthree\n"},
		{name: ":nth-child(odd) on table rows styles odd rows", css: `tr:nth-child(odd) td { text-transform: uppercase; }`, html: `<table style="border-style:hidden"><tr><td>r1</td></tr><tr><td>r2</td></tr><tr><td>r3</td></tr></table>`, want: "R1\nr2\nR3\n"},
		{name: "[attr] presence selector hides elements with the attribute", css: `p[data-hide] { display: none; }`, html: `<p>visible</p><p data-hide>hidden</p><p>after</p>`, want: "visible\n\nafter\n\n"},
		{name: "[attr] matches attribute with empty value", css: `span[data-mark] { text-transform: uppercase; }`, html: `<p><span data-mark="">marked</span> plain</p>`, want: "MARKED plain\n\n"},
		{name: "[attr=val] exact-value selector", css: `p[data-style=big] { text-transform: uppercase; }`, html: `<p data-style="big">large</p><p data-style="small">tiny</p>`, want: "LARGE\n\ntiny\n\n"},
		{name: "[attr=val] with quoted value in CSS", css: `p[lang="en"] { text-transform: uppercase; }`, html: `<p lang="en">english</p><p lang="fr">french</p>`, want: "ENGLISH\n\nfrench\n\n"},
		{name: "[attr=val] does not match wrong value", css: `a[href=https://example.com] { text-transform: uppercase; }`, html: `<p><a href="https://example.com">right</a> <a href="https://other.com">wrong</a></p>`, want: "RIGHT wrong\n\n"},
		{name: "[attr~=val] matches whitespace-separated word", css: `p[data-tags~=beta] { text-transform: uppercase; }`, html: `<p data-tags="alpha beta gamma">word</p><p data-tags="alphabet gamma">partial</p>`, want: "WORD\n\npartial\n\n"},
		{name: "[attr|=val] matches language subcode", css: `p[lang|=en] { text-transform: uppercase; }`, html: `<p lang="en">base</p><p lang="en-US">regional</p><p lang="english">word</p>`, want: "BASE\n\nREGIONAL\n\nword\n\n"},
		{name: "[attr^=val] matches prefix", css: `a[href^="https://"] { text-transform: uppercase; }`, html: `<p><a href="https://example.com">secure</a> <a href="http://example.com">plain</a></p>`, want: "SECURE plain\n\n"},
		{name: "[attr$=val] matches suffix", css: `a[href$=".pdf"] { text-transform: uppercase; }`, html: `<p><a href="/files/report.pdf">pdf</a> <a href="/files/report.pdfx">pdfx</a></p>`, want: "PDF pdfx\n\n"},
		{name: "[attr*=val] matches substring", css: `a[href*=example] { text-transform: uppercase; }`, html: `<p><a href="https://example.com">example</a> <a href="https://other.test">other</a></p>`, want: "EXAMPLE other\n\n"},

		// Adjacent sibling combinator (+)
		{name: "adjacent sibling matches immediately following sibling", css: `h2 + p { text-transform: uppercase; }`, html: `<h2>Title</h2><p>first</p><p>second</p>`, want: "Title\nFIRST\n\nsecond\n\n"},
		{name: "adjacent sibling does not match non-adjacent sibling", css: `h2 + p { text-transform: uppercase; }`, html: `<p>before</p><h2>Title</h2><p>after</p>`, want: "before\n\nTitle\nAFTER\n\n"},
		{name: "adjacent sibling does not match when element is between them", css: `h2 + p { text-transform: uppercase; }`, html: `<h2>Title</h2><div>divider</div><p>para</p>`, want: "Title\ndivider\npara\n\n"},
		{name: "adjacent sibling with class on subject", css: `h2 + p.lead { text-transform: uppercase; }`, html: `<h2>Head</h2><p class="lead">match</p><p class="lead">no</p>`, want: "Head\nMATCH\n\nno\n\n"},
		{name: "adjacent sibling in a chain with descendant", css: `div h2 + p { text-transform: uppercase; }`, html: `<div><h2>A</h2><p>yes</p></div><h2>B</h2><p>no</p>`, want: "A\nYES\nB\nno\n\n"},

		// :not() pseudo-class
		{name: ":not(element) excludes matching element", css: `p:not(h2) { text-transform: uppercase; }`, html: `<p>para</p><h2>head</h2>`, want: "PARA\n\nhead\n"},
		{name: ":not(.class) excludes elements with the class", css: `td:not(.highlight) { text-transform: uppercase; }`, html: `<table style="border-style:hidden"><tr><td>plain</td><td class="highlight">hi</td></tr></table>`, want: "PLAIN hi\n"},
		{name: ":not(element) with type matches everything else", css: `li:not(.skip) { text-transform: uppercase; }`, html: `<ul><li>one</li><li class="skip">two</li><li>three</li></ul>`, want: "    • ONE\n    • two\n    • THREE\n"},
		{name: ":not() combined with element selector", css: `p:not(.muted) { text-transform: uppercase; }`, html: `<p>normal</p><p class="muted">quiet</p><p>also</p>`, want: "NORMAL\n\nquiet\n\nALSO\n\n"},
		{name: "id specificity beats many classes", css: `#x { text-transform: lowercase; } .a.b.c.d.e.f.g.h.i.j.k { text-transform: uppercase; }`, html: `<p id="x" class="a b c d e f g h i j k">MiX</p>`, want: "mix\n\n"},

		// Newline as whitespace in selectors (CSS allows any whitespace as descendant combinator).
		{name: "newline between selector parts acts as descendant combinator",
			css:  "div\np { text-transform: uppercase; }",
			html: `<div><p>inside</p></div><p>outside</p>`,
			want: "INSIDE\noutside\n\n"},
		{name: "newline after child combinator is skipped",
			css:  "div >\np { text-transform: uppercase; }",
			html: `<div><p>direct</p><section><p>nested</p></section></div>`,
			want: "DIRECT\n\nnested\n"},
	})
}
