package cssengine

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestFoldKeywordValue(t *testing.T) {
	cases := []struct {
		name string
		prop string
		val  string
		want string
	}{
		{"a keyword folds", "display", "FLEX", "flex"},
		{"mixed case folds", "border-collapse", "Collapse", "collapse"},
		{"a multi-word keyword folds", "word-break", "BREAK-ALL", "break-all"},
		{"units fold too", "width", "10CH", "10ch"},
		{"an already-lowercase value is untouched", "display", "flex", "flex"},
		{"a value with no letters is untouched", "margin-top", "4", "4"},

		// Author text emitted verbatim.
		{"content is never folded", "content", `"Hello World"`, `"Hello World"`},
		{"content idents are not folded either", "content", "OPEN-QUOTE", "OPEN-QUOTE"},
		{"quotes are never folded", "quotes", `"<<" ">>"`, `"<<" ">>"`},
		{"font-family is never folded", "font-family", "Times New Roman", "Times New Roman"},

		// Custom identifiers: CSS defines these as case-sensitive, and this
		// engine really does distinguish them (see TestKeywordCaseCounterNames).
		{"counter-reset names are not folded", "counter-reset", "MyCounter 4", "MyCounter 4"},
		{"counter-increment names are not folded", "counter-increment", "MyCounter", "MyCounter"},
		{"counter-set names are not folded", "counter-set", "MyCounter", "MyCounter"},

		// Custom properties: names and values both case-sensitive per spec.
		{"a custom property value is not folded", "--Theme", "MiXeD", "MiXeD"},

		// Quoted payloads on otherwise-foldable properties. parseCSSString
		// (render/block.go) requires the quotes, so the quote is a reliable
		// marker for "this is author text, not a keyword".
		{"a quoted border glyph survives", "border-left", `"AB"`, `"AB"`},
		{"a single-quoted corner glyph survives", "border-top-left-corner", "'Z'", "'Z'"},
		{"a quoted bullet survives", "list-style-type", `"Xy "`, `"Xy "`},
		{"quoted symbols() survive", "list-style-type", "symbols('Ab' 'Cd')", "symbols('Ab' 'Cd')"},
		{"a quoted truncation marker survives", "text-overflow", "'XY'", "'XY'"},
		// ...while the same properties' unquoted keyword forms still fold.
		{"an unquoted border keyword folds", "border-left", "SOLID", "solid"},
		{"an unquoted bullet keyword folds", "list-style-type", "SQUARE", "square"},
		{"an unquoted marker keyword folds", "text-overflow", "ELLIPSIS", "ellipsis"},

		// An unresolved var() names a case-sensitive custom property.
		{"an unresolved var() reference is not folded", "display", "var(--Foo)", "var(--Foo)"},
		{"a var() with a fallback is not folded", "display", "var(--Foo, FLEX)", "var(--Foo, FLEX)"},
		{"a token merely ending in var is still folded", "display", "AVAR(X)", "avar(x)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := foldKeywordValue(tc.prop, tc.val); got != tc.want {
				t.Errorf("foldKeywordValue(%q, %q) = %q, want %q", tc.prop, tc.val, got, tc.want)
			}
		})
	}
}

// TestKeywordCaseThroughCascade pins that folding actually reaches a resolved
// declaration by each of the routes that can produce one, since they assemble
// their maps independently: a stylesheet rule and an inline style= attribute
// both land in Direct, while a var() pointing at an *ancestor's* custom
// property is only substituted by Resolve's own final pass, after Direct has
// already run and deliberately left it alone.
func TestKeywordCaseThroughCascade(t *testing.T) {
	cases := []struct {
		name string
		css  string
		html string
		prop string
		want string
	}{
		{"stylesheet rule", `p { display: FLEX }`, `<p id="a">x</p>`, "display", "flex"},
		{"inline style attribute", ``, `<p id="a" style="display:FLEX">x</p>`, "display", "flex"},
		{"same-element var()", ``, `<p id="a" style="--d:FLEX;display:var(--d)">x</p>`, "display", "flex"},
		{"inherited var() from an ancestor", `body { --d: FLEX } p { display: var(--d) }`, `<p id="a">x</p>`, "display", "flex"},
		{"an inherited keyword is folded once, at its source", `body { text-transform: UPPERCASE }`, `<p id="a">x</p>`, "text-transform", "uppercase"},
		{"a var()-supplied string is still not folded", `body { --t: "MiXeD" } p { content: var(--t) }`, `<p id="a">x</p>`, "content", `"MiXeD"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := ParseStylesheet(tc.css)
			if err != nil {
				t.Fatalf("ParseStylesheet() error = %v", err)
			}
			doc, err := html.Parse(strings.NewReader(tc.html))
			if err != nil {
				t.Fatalf("html.Parse: %v", err)
			}
			n := findElementByID(doc, "a")
			if n == nil {
				t.Fatal(`<p id="a"> not found`)
			}
			if got := (Cascade{Rules: rules}).Resolve(n)[tc.prop]; got != tc.want {
				t.Errorf("Resolve()[%q] = %q, want %q", tc.prop, got, tc.want)
			}
		})
	}
}

// TestKeywordCaseCounterNames is the evidence behind counter-reset and friends
// being on caseSensitiveValueProps: CSS custom identifiers are case-sensitive,
// so these two really are different counters and folding the property's value
// would silently merge them.
func TestKeywordCaseCounterNames(t *testing.T) {
	rules, err := ParseStylesheet(`p { counter-reset: Xy 7; counter-increment: xY }`)
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
	got := (Cascade{Rules: rules}).Resolve(n)
	if got["counter-reset"] != "Xy 7" {
		t.Errorf("counter-reset = %q, want %q", got["counter-reset"], "Xy 7")
	}
	if got["counter-increment"] != "xY" {
		t.Errorf("counter-increment = %q, want %q", got["counter-increment"], "xY")
	}
}
