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
		{name: "child combinator matches direct children only", css: `div > p { text-transform: uppercase; }`, html: `<div><p>direct</p><section><p>nested</p></section></div>`, want: "DIRECT\n\nnested\n"},
		{name: "descendant combinator still matches all levels", css: `div p { text-transform: uppercase; }`, html: `<div><p>direct</p><section><p>nested</p></section></div>`, want: "DIRECT\n\nNESTED\n"},
		{name: "child combinator in a deeper chain", css: `div > p > span { text-transform: uppercase; }`, html: `<div><p><span>deep</span> rest</p></div>`, want: "DEEP rest\n"},
		{name: "mixed child and descendant combinators", css: `div > ul li { text-transform: uppercase; }`, html: `<div><ul><li>match</li></ul></div><ul><li>no-match</li></ul>`, want: "    • MATCH\n    • no-match\n"},
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
	})
}
