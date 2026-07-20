package htmlterm_test

import "testing"

func TestWhiteSpace(t *testing.T) {
	runCases(t, []renderCase{
		{name: "normal collapses multiple spaces", html: `<p>hello   world</p>`, want: "hello world\n\n"},
		{name: "normal collapses newlines to space", html: "<p>hello\nworld</p>", want: "hello world\n\n"},
		{name: "normal collapses tabs", html: "<p>hello\tworld</p>", want: "hello world\n\n"},
		{name: "pre preserves newlines", html: "<pre>hello\n  world</pre>", want: "hello\n  world\n"},
		{name: "pre preserves multiple spaces", html: "<pre>a   b</pre>", want: "a   b\n"},
		{name: "nowrap collapses whitespace", css: `p { white-space: nowrap; }`, html: "<p>hello\n  world</p>", want: "hello world\n\n"},
		{name: "pre-line preserves newlines but collapses spaces", css: `p { white-space: pre-line; }`, html: "<p>hello\n  world</p>", want: "hello\n world\n\n"},
		{name: "white-space inherited by child inline elements", html: "<pre><span>hello   world</span></pre>", want: "hello   world\n"},
		{name: "pre-wrap preserves all whitespace", css: `p { white-space: pre-wrap; }`, html: "<p>hello   world\n  end</p>", want: "hello   world\n  end\n\n"},
		{name: "leading space inside nested inline element is preserved after preceding text", html: `<p>Google<span> search</span></p>`, want: "Google search\n\n"},
		{name: "leading space inside doubly-nested inline element is preserved", html: `<p>Google<span><em> search</em></span></p>`, want: "Google search\n\n"},
		{name: "leading space inside display:contents child is preserved after preceding text", html: `<p>Google<span style="display:contents"> search</span></p>`, want: "Google search\n\n"},
		{name: "space between text and inline element is not duplicated", html: `<p>Google <span>search</span></p>`, want: "Google search\n\n"},
		{name: "leading space inside inline element at true line start is still trimmed", html: `<p><span> search</span></p>`, want: "search\n\n"},
		{name: "trailing space inside inline element followed by text is preserved", html: `<p><span>Google </span>search</p>`, want: "Google search\n\n"},
	})
}

func TestBlockOverflow(t *testing.T) {
	runCases(t, []renderCase{
		{name: "nowrap without overflow: text overflows width", css: `p { white-space: nowrap; width: 5; }`, html: `<p>Hello World</p>`, width: 20, want: "Hello World\n\n"},
		{name: "overflow:hidden clips to width (default text-overflow:clip)", css: `p { overflow: hidden; white-space: nowrap; width: 5; }`, html: `<p>Hello World</p>`, width: 20, want: "Hello\n\n"},
		{name: "overflow:hidden with text-overflow:ellipsis", css: `p { overflow: hidden; white-space: nowrap; width: 5; text-overflow: ellipsis; }`, html: `<p>Hello World</p>`, width: 20, want: "Hell…\n\n"},
		{name: "overflow:clip with text-overflow:ellipsis", css: `p { overflow: clip; white-space: nowrap; width: 5; text-overflow: ellipsis; }`, html: `<p>Hello World</p>`, width: 20, want: "Hell…\n\n"},
		{name: "overflow:hidden with normal white-space clips long word", css: `p { overflow: hidden; width: 5; text-overflow: ellipsis; }`, html: `<p>Superlongword</p>`, width: 20, want: "Supe…\n\n"},
		{name: "overflow:hidden without width does not clip", css: `p { overflow: hidden; white-space: nowrap; }`, html: `<p>Hello World</p>`, width: 20, want: "Hello World\n\n"},
		{name: "overflow:hidden with custom text-overflow string", css: `p { overflow: hidden; white-space: nowrap; width: 6; text-overflow: "+"; }`, html: `<p>Hello World</p>`, width: 20, want: "Hello+\n\n"},
	})
}

func TestStyleSources(t *testing.T) {
	runCases(t, []renderCase{
		{name: "<style> tag rules apply", html: `<style>p { margin-bottom: 0; }</style><p>one</p><p>two</p>`, want: "one\ntwo\n"},
		{name: "inline style= wins over stylesheet", css: `p { margin-bottom: 2; }`, html: `<p style="margin-bottom: 0">text</p>`, want: "text\n"},
		{name: "<style> tag overrides UA at equal specificity", html: `<style>p { display: inline; }</style><p>a</p><p>b</p>`, want: "ab"},
	})
}

func TestInheritance(t *testing.T) {
	runCases(t, []renderCase{
		{name: "white-space inherited from parent", css: `div { white-space: pre; }`, html: `<div><span>hello   world</span></div>`, want: "hello   world\n"},
		{name: "display not inherited", css: `div { display: none; }`, html: `<div><p>inside</p></div>`, want: ""},
	})
}

func TestTextTransform(t *testing.T) {
	runCases(t, []renderCase{
		{name: "uppercase", html: `<p style="text-transform:uppercase">hello world</p>`, want: "HELLO WORLD\n\n"},
		{name: "lowercase", html: `<p style="text-transform:lowercase">HELLO WORLD</p>`, want: "hello world\n\n"},
		{name: "capitalize", html: `<p style="text-transform:capitalize">hello world</p>`, want: "Hello World\n\n"},
		{name: "capitalize strips leading space at block start", html: `<p style="text-transform:capitalize"> hello world</p>`, want: "Hello World\n\n"},
		{name: "none is a no-op", html: `<p style="text-transform:none">Hello World</p>`, want: "Hello World\n\n"},
		{name: "inherited by child inline elements", html: `<p style="text-transform:uppercase">hello <strong>world</strong></p>`, want: "HELLO WORLD\n\n"},
		{name: "child inline element overrides inherited transform", html: `<p style="text-transform:uppercase">hello <span style="text-transform:lowercase">WORLD</span></p>`, want: "HELLO world\n\n"},
		{name: "none cancels inherited transform on inline child", html: `<p style="text-transform:uppercase">BEFORE <span style="text-transform:none">none</span> AFTER</p>`, want: "BEFORE none AFTER\n\n"},
		{name: "via CSS class", css: `.shout { text-transform: uppercase; }`, html: `<p class="shout">hello</p>`, want: "HELLO\n\n"},
		{name: "table cell uppercase", html: `<table style="border-style:hidden"><tr><td style="text-transform:uppercase;width:5">hello</td></tr></table>`, want: "HELLO\n"},
		{name: "table cell capitalize", html: `<table style="border-style:hidden"><tr><td style="text-transform:capitalize;width:11">hello world</td></tr></table>`, want: "Hello World\n"},
		{name: "superscript digits", html: `<p style="text-transform:superscript">0123456789</p>`, want: "⁰¹²³⁴⁵⁶⁷⁸⁹\n\n"},
		{name: "superscript letters", html: `<p style="text-transform:superscript">abcdefghijklmnoprstuvwxyz</p>`, want: "ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻ\n\n"},
		{name: "superscript symbols", html: `<p style="text-transform:superscript">+-=()</p>`, want: "⁺⁻⁼⁽⁾\n\n"},
		{name: "superscript unmapped chars pass through", html: `<p style="text-transform:superscript">q Q !</p>`, want: "q Q !\n\n"},
		{name: "subscript digits", html: `<p style="text-transform:subscript">0123456789</p>`, want: "₀₁₂₃₄₅₆₇₈₉\n\n"},
		{name: "subscript mapped letters", html: `<p style="text-transform:subscript">aehklmnopstx</p>`, want: "ₐₑₕₖₗₘₙₒₚₛₜₓ\n\n"},
		{name: "subscript symbols", html: `<p style="text-transform:subscript">+-=()</p>`, want: "₊₋₌₍₎\n\n"},
		{name: "subscript unmapped chars pass through", html: `<p style="text-transform:subscript">bcdfgijqruvwyz</p>`, want: "bcdfgijqruvwyz\n\n"},
		{name: "sup element uses superscript transform", html: `<p>H<sup>2</sup>O</p>`, want: "H²O\n\n"},
		{name: "sub element uses subscript transform", html: `<p>H<sub>2</sub>O</p>`, want: "H₂O\n\n"},
		{name: "sup inherits to inline children", html: `<p>x<sup><strong>n</strong></sup></p>`, want: "xⁿ\n\n"},
	})
}

func TestFontVariant(t *testing.T) {
	runCases(t, []renderCase{
		{name: "small-caps uppercases inline text", html: `<p style="font-variant:small-caps">hello world</p>`, want: "HELLO WORLD\n\n"},
		{name: "normal is a no-op", html: `<p style="font-variant:normal">hello world</p>`, want: "hello world\n\n"},
		{name: "small-caps via class", css: `.sc { font-variant: small-caps; }`, html: `<p class="sc">hello world</p>`, want: "HELLO WORLD\n\n"},
		{name: "text-transform overrides small-caps", html: `<p style="font-variant:small-caps;text-transform:lowercase">HELLO</p>`, want: "hello\n\n"},
		{name: "small-caps in table cell", html: `<table style="border-style:hidden"><tr><td style="font-variant:small-caps;width:5">hello</td></tr></table>`, want: "HELLO\n"},
	})
}

func TestNewElements(t *testing.T) {
	runCases(t, []renderCase{
		{name: "ins renders content", html: `<p>before <ins>inserted</ins> after</p>`, want: "before inserted after\n\n"},
		{name: "dfn renders content", html: `<p><dfn>term</dfn> is defined here</p>`, want: "term is defined here\n\n"},
		{name: "small renders content", html: `<p>main <small>fine print</small></p>`, want: "main fine print\n\n"},
		{name: "q wraps content in curly quotes", html: `<p>She said <q>hello</q>.</p>`, want: "She said “hello”.\n\n"},
		{name: "q standalone at body level", html: `<q>quoted</q>`, want: "“quoted”"},
		{name: "abbr with title appends expansion", html: `<p>The <abbr title="HyperText Markup Language">HTML</abbr> spec.</p>`, width: 80, want: "The HTML (HyperText Markup Language) spec.\n\n"},
		{name: "abbr without title renders as-is", html: `<p><abbr>CSS</abbr></p>`, want: "CSS\n\n"},
		{name: "abbr standalone at body level", html: `<abbr title="Cascading Style Sheets">CSS</abbr>`, want: "CSS (Cascading Style Sheets)"},
		{name: "dl with single dt and dd", html: `<dl><dt>Term</dt><dd>Definition.</dd></dl>`, want: "Term\n    Definition.\n\n"},
		{name: "dl with multiple entries", html: `<dl><dt>Alpha</dt><dd>First.</dd><dt>Beta</dt><dd>Second.</dd></dl>`, want: "Alpha\n    First.\nBeta\n    Second.\n\n"},
		{name: "figcaption inside figure renders as italic block", html: `<figure><figcaption>Caption text</figcaption></figure>`, want: "Caption text\n"},
		{name: "figure with nested content and figcaption", html: `<figure><p>Content here.</p><figcaption>Fig 1</figcaption></figure>`, want: "Content here.\n\nFig 1\n"},
	})
}

func TestPseudoElements(t *testing.T) {
	runCases(t, []renderCase{
		{name: "before content string", css: `p::before { content: "→ "; }`, html: `<p>hello</p>`, want: "→ hello\n\n"},
		{name: "after content string", css: `p::after { content: " ←"; }`, html: `<p>hello</p>`, want: "hello ←\n\n"},
		{name: "before and after", css: `p::before { content: "["; } p::after { content: "]"; }`, html: `<p>hi</p>`, want: "[hi]\n\n"},
		{name: "content none suppresses output", css: `p::before { content: none; }`, html: `<p>hello</p>`, want: "hello\n\n"},
		{name: "single colon :before also works", css: `p:before { content: "• "; }`, html: `<p>item</p>`, want: "• item\n\n"},
		{name: "element scoped — div::before does not fire on p", css: `div::before { content: "X"; }`, html: `<p>para</p>`, want: "para\n\n"},
		{name: "element scoped — div::before fires on div", css: `div::before { content: "> "; }`, html: `<div>content</div>`, want: "> content\n"},
		{name: "before with styling (color stripped in plain-text comparison)", css: `p::before { content: "! "; color: #ff0000; }`, html: `<p>warning</p>`, want: "! warning\n\n"},
		{name: "before inherits parent inline context", css: `p { font-weight: bold; } p::before { content: "★ "; }`, html: `<p>bold</p>`, want: "★ bold\n\n"},
		{name: "ancestor selector in pseudo-element rule", css: `div p::before { content: "• "; }`, html: `<div><p>item</p></div>`, want: "• item\n\n"},
		{name: "ancestor selector does not fire outside context", css: `div p::before { content: "• "; }`, html: `<p>item</p>`, want: "item\n\n"},
	})
}
