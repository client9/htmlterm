package cssengine

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestCustomPropCaseSensitive(t *testing.T) {
	rules, err := ParseStylesheet(`p { --Foo: upper; --foo: lower; color: var(--Foo); background-color: var(--foo); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["--Foo"] != "upper" || got["--foo"] != "lower" {
		t.Fatalf("--Foo=%q --foo=%q, want distinct properties (upper, lower) - custom property names are case-sensitive", got["--Foo"], got["--foo"])
	}
	if got["color"] != "upper" || got["background-color"] != "lower" {
		t.Fatalf("color=%q background-color=%q, want (upper, lower)", got["color"], got["background-color"])
	}
}

func TestVarBasicSubstitution(t *testing.T) {
	rules, err := ParseStylesheet(`:root { --x: red; } p { color: var(--x); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<html><body><p id="a">x</p></body></html>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "red" {
		t.Fatalf(`Resolve()["color"] = %q, want "red"`, got["color"])
	}
}

func TestVarInheritsToDescendantThatDoesNotRedeclare(t *testing.T) {
	// The main use case: a descendant that never mentions --brand at all
	// must still see the ancestor's value - this is the case the Resolve()
	// inheritance fill-in loop must handle via a second loop over the
	// parent's own resolved keys, not by widening the inheritableProps loop.
	rules, err := ParseStylesheet(`#gp { --brand: blue; } p { color: var(--brand); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="gp"><div id="mid"><p id="a">x</p></div></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "blue" {
		t.Fatalf(`Resolve()["color"] = %q, want "blue" (--brand should inherit through #mid, which never redeclares it)`, got["color"])
	}
	if got["--brand"] != "blue" {
		t.Fatalf(`Resolve()["--brand"] = %q, want "blue" to have inherited onto p itself`, got["--brand"])
	}
}

func TestVarMoreSpecificDescendantOverridesAncestor(t *testing.T) {
	rules, err := ParseStylesheet(`#gp { --x: red; } #child { --x: green; } p { color: var(--x); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="gp"><div id="child"><p id="a">x</p></div></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "green" {
		t.Fatalf(`Resolve()["color"] = %q, want "green" (nearest declaration should win)`, got["color"])
	}
}

func TestVarFallback(t *testing.T) {
	rules, err := ParseStylesheet(`p { color: var(--undefined, blue); background-color: var(--defined, blue); --defined: red; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "blue" {
		t.Fatalf(`Resolve()["color"] = %q, want "blue" (fallback used when --undefined isn't defined anywhere)`, got["color"])
	}
	if got["background-color"] != "red" {
		t.Fatalf(`Resolve()["background-color"] = %q, want "red" (fallback ignored when --defined is defined)`, got["background-color"])
	}
}

func TestVarNestedFallback(t *testing.T) {
	rules, err := ParseStylesheet(`p { color: var(--a, var(--b, green)); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "green" {
		t.Fatalf(`Resolve()["color"] = %q, want "green" (both --a and --b undefined, falls through nested fallback)`, got["color"])
	}
}

func TestVarChainedCustomProperties(t *testing.T) {
	rules, err := ParseStylesheet(`p { --a: var(--b); --b: red; color: var(--a); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["--a"] != "red" {
		t.Fatalf(`Resolve()["--a"] = %q, want "red" (--a should resolve through the --b chain)`, got["--a"])
	}
	if got["color"] != "red" {
		t.Fatalf(`Resolve()["color"] = %q, want "red"`, got["color"])
	}
}

func TestVarCycleResolvesToEmptyNotHang(t *testing.T) {
	rules, err := ParseStylesheet(`p { --a: var(--b); --b: var(--a); color: var(--a, fallback); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	// If resolveCustomProps' cycle guard were broken, this call would hang
	// or stack-overflow rather than return - relying on `go test`'s own
	// -timeout to surface that as a failure naming this test.
	got := Cascade{Rules: rules}.Resolve(n)
	if got["--a"] != "" {
		t.Errorf(`Resolve()["--a"] = %q, want "" (cyclic reference)`, got["--a"])
	}
	// --a is defined (cyclically) so var(--a, fallback) resolves to its
	// (empty) cyclic value, not the fallback - only a genuinely absent name
	// falls back.
	if got["color"] != "" {
		t.Errorf(`Resolve()["color"] = %q, want "" (--a is defined, just cyclically empty, so its fallback is not used)`, got["color"])
	}
}

func TestVarImportantOnCustomProperty(t *testing.T) {
	rules, err := ParseStylesheet(`#a { --x: blue; } p { --x: red !important; color: var(--x); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "red" {
		t.Fatalf(`Resolve()["color"] = %q, want "red" (!important should win for custom properties too)`, got["color"])
	}
}

func TestPseudoElementContentVarWithFallback(t *testing.T) {
	rules, err := ParseStylesheet(`p::before { content: var(--icon, "»"); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	cascade := Cascade{Rules: rules}
	got := cascade.PseudoElement(n, "before", nil)
	if got["content"] != `"»"` {
		t.Fatalf(`PseudoElement()["content"] = %q, want %q (var() should resolve before render's own content-tokenizer sees it)`, got["content"], `"»"`)
	}
}

func TestPseudoElementContentVarFromEnv(t *testing.T) {
	rules, err := ParseStylesheet(`p::before { content: var(--icon, "»"); } p { --icon: "★"; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	cascade := Cascade{Rules: rules}
	env := customPropSubset(cascade.Resolve(n))
	got := cascade.PseudoElement(n, "before", env)
	if got["content"] != `"★"` {
		t.Fatalf(`PseudoElement()["content"] = %q, want %q (caller-supplied env should satisfy var(--icon))`, got["content"], `"★"`)
	}
}

func TestDirectSameElementOnlyVarGap(t *testing.T) {
	// Option A's documented scope cut (docs/proposals/VARIABLES.md): Direct()
	// resolves var() against a custom property declared on the SAME element,
	// but not one that's only declared on an ancestor - pinned here so a
	// future change to Direct() is a deliberate decision, not an accidental
	// regression either way.
	rules, err := ParseStylesheet(`#same { --n: 3; counter-reset: var(--n); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="same"></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	same := findElementByID(doc, "same")
	if same == nil {
		t.Fatal(`#same not found`)
	}
	got := Cascade{Rules: rules}.Direct(same)
	if got["counter-reset"] != "3" {
		t.Fatalf(`Direct(#same)["counter-reset"] = %q, want "3" (--n is declared on the same element)`, got["counter-reset"])
	}

	rules2, err := ParseStylesheet(`#anc { --n: 3; } #child { counter-reset: var(--n); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc2, err := html.Parse(strings.NewReader(`<div id="anc"><div id="child"></div></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	child := findElementByID(doc2, "child")
	if child == nil {
		t.Fatal(`#child not found`)
	}
	got2 := Cascade{Rules: rules2}.Direct(child)
	if got2["counter-reset"] != "var(--n)" {
		t.Fatalf(`Direct(#child)["counter-reset"] = %q, want unresolved literal "var(--n)" (Direct() has no ancestor context - --n is only declared on #anc)`, got2["counter-reset"])
	}
}

func TestDirectVarDoesNotBlockLaterAncestorResolution(t *testing.T) {
	// The bug this guards against: if Direct() ate ancestor-only var()
	// references (collapsing them to "" because they're not defined among
	// n's own custom props), Resolve() would never get a chance to resolve
	// them against the actual ancestor value. margin-top isn't even a
	// custom property itself, just an ordinary property using var() - this
	// is the general case, not just the counter-reset carve-out.
	rules, err := ParseStylesheet(`#anc { --gap: 4; } p { margin-top: var(--gap); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="anc"><p id="a">x</p></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["margin-top"] != "4" {
		t.Fatalf(`Resolve()["margin-top"] = %q, want "4" (var(--gap) should resolve against the ancestor's --gap)`, got["margin-top"])
	}
}

func TestVarShorthandFanOutGap(t *testing.T) {
	// Documented limitation (see CSS.md): expandShorthand runs once at parse
	// time, before any per-node var() resolution is possible, so
	// "margin: var(--sides)" cannot fan out into four independent sides -
	// after substitution all four sides get the same literal string.
	rules, err := ParseStylesheet(`p { --sides: 1 2 3 4; margin: var(--sides); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	want := "1 2 3 4"
	for _, side := range []string{"margin-top", "margin-right", "margin-bottom", "margin-left"} {
		if got[side] != want {
			t.Errorf(`Resolve()[%q] = %q, want the documented (not ideal) fan-out failure %q`, side, got[side], want)
		}
	}
}

func TestVarNotSubstitutedInsideQuotedString(t *testing.T) {
	// A quoted string containing the literal text "var(...)" must pass
	// through untouched - scanVarCalls' top-level scan has to skip quoted
	// spans instead of treating everything as fair game for a var() match,
	// or literal content like content: "see var(--x) docs" gets corrupted.
	rules, err := ParseStylesheet(`p::before { content: "literal var(--x) text"; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.PseudoElement(n, "before", nil)
	want := `"literal var(--x) text"`
	if got["content"] != want {
		t.Fatalf(`PseudoElement()["content"] = %q, want %q (literal "var(" text inside a quoted string must not be parsed as a real var() call)`, got["content"], want)
	}
}

func TestVarAdjacentToQuotedFallbackStillResolves(t *testing.T) {
	// A real var() call still resolves correctly right next to (not inside)
	// a quoted fallback - the quote-skip fix must not break the ordinary
	// fallback path.
	rules, err := ParseStylesheet(`p::before { content: var(--icon, "»"); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.PseudoElement(n, "before", nil)
	if got["content"] != `"»"` {
		t.Fatalf(`PseudoElement()["content"] = %q, want %q`, got["content"], `"»"`)
	}
}

func TestVarUnsetOnCustomPropertyInherits(t *testing.T) {
	// Custom properties are unconditionally inheritable, so "unset" on one
	// must act like "inherit" (take the ancestor's value), the same as any
	// of the fixed inheritableProps - not like "initial" (which is what
	// happens to every other non-inheritable property under "unset").
	rules, err := ParseStylesheet(`#gp { --x: red; } p { --x: unset; color: var(--x); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="gp"><p id="a">x</p></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["--x"] != "red" || got["color"] != "red" {
		t.Fatalf(`--x=%q color=%q, want both "red" (unset on a custom property should inherit)`, got["--x"], got["color"])
	}
}

func TestVarInheritOnCustomProperty(t *testing.T) {
	rules, err := ParseStylesheet(`#gp { --x: red; } p { --x: inherit; color: var(--x); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="gp"><p id="a">x</p></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "red" {
		t.Fatalf(`Resolve()["color"] = %q, want "red"`, got["color"])
	}
}

func TestVarInitialOnCustomPropertyDoesNotLeakLiteralKeyword(t *testing.T) {
	// Regression guard: Direct()'s own-element-only var() substitution must
	// not treat a not-yet-resolved CSS-wide keyword ("--x: initial") as if
	// it were --x's real value - doing so would let the literal text
	// "initial" leak into anything referencing --x via var() before
	// Resolve()'s keyword-handling loop ever cancels --x. "initial" cancels
	// --x entirely, so var(--x, fallback) must fall back, not print the
	// word "initial".
	rules, err := ParseStylesheet(`#gp { --x: red; } p { --x: initial; color: var(--x, fallback); }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<div id="gp"><p id="a">x</p></div>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Resolve(n)
	if got["color"] != "fallback" {
		t.Fatalf(`Resolve()["color"] = %q, want "fallback" (initial cancels --x on p; must not print the literal word "initial")`, got["color"])
	}
}
