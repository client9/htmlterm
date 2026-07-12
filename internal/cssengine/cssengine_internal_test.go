package cssengine

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExpandShorthand(t *testing.T) {
	tests := []struct {
		name string
		prop string
		val  string
		want map[string]string
	}{
		{name: "margin one value", prop: "margin", val: "2", want: map[string]string{
			"margin-top": "2", "margin-right": "2", "margin-bottom": "2", "margin-left": "2",
		}},
		{name: "padding two values", prop: "padding", val: "1 3", want: map[string]string{
			"padding-top": "1", "padding-right": "3", "padding-bottom": "1", "padding-left": "3",
		}},
		{name: "margin three values", prop: "margin", val: "1 2 3", want: map[string]string{
			"margin-top": "1", "margin-right": "2", "margin-bottom": "3", "margin-left": "2",
		}},
		{name: "padding four values", prop: "padding", val: "1 2 3 4", want: map[string]string{
			"padding-top": "1", "padding-right": "2", "padding-bottom": "3", "padding-left": "4",
		}},
		{name: "invalid arity falls back", prop: "margin", val: "1 2 3 4 5", want: map[string]string{"margin": "1 2 3 4 5"}},
		{name: "list-style type and position", prop: "list-style", val: "upper-roman inside", want: map[string]string{
			"list-style-type": "upper-roman", "list-style-position": "inside",
		}},
		{name: "list-style position before type", prop: "list-style", val: "outside square", want: map[string]string{
			"list-style-type": "square", "list-style-position": "outside",
		}},
		{name: "list-style custom string preserves spaces", prop: "list-style", val: `"-> " inside`, want: map[string]string{
			"list-style-type": `"-> "`, "list-style-position": "inside",
		}},
		{name: "list-style ignores url image", prop: "list-style", val: `url("bullet image.png") circle inside`, want: map[string]string{
			"list-style-type": "circle", "list-style-position": "inside",
		}},
		{name: "background extracts named color", prop: "background", val: "red", want: map[string]string{"background-color": "red"}},
		{name: "background extracts color among unsupported tokens", prop: "background", val: "url(bg.png) no-repeat center/cover #123456", want: map[string]string{"background-color": "#123456"}},
		{name: "background extracts functional color", prop: "background", val: "rgb(255 0 0) fixed", want: map[string]string{"background-color": "rgb(255 0 0)"}},
		{name: "background ignores url without color", prop: "background", val: "url(bg.png) no-repeat center/cover", want: map[string]string{}},
		{name: "margin block start alias", prop: "margin-block-start", val: "1", want: map[string]string{"margin-top": "1"}},
		{name: "margin block end alias", prop: "margin-block-end", val: "2", want: map[string]string{"margin-bottom": "2"}},
		{name: "margin inline start alias", prop: "margin-inline-start", val: "3", want: map[string]string{"margin-left": "3"}},
		{name: "margin inline end alias", prop: "margin-inline-end", val: "4", want: map[string]string{"margin-right": "4"}},
		{name: "padding block start alias", prop: "padding-block-start", val: "5", want: map[string]string{"padding-top": "5"}},
		{name: "padding block end alias", prop: "padding-block-end", val: "6", want: map[string]string{"padding-bottom": "6"}},
		{name: "padding inline start alias", prop: "padding-inline-start", val: "7", want: map[string]string{"padding-left": "7"}},
		{name: "padding inline end alias", prop: "padding-inline-end", val: "8", want: map[string]string{"padding-right": "8"}},
		{name: "overflow one value sets both axes", prop: "overflow", val: "auto", want: map[string]string{"overflow-x": "auto", "overflow-y": "auto"}},
		{name: "overflow two values set x then y", prop: "overflow", val: "hidden scroll", want: map[string]string{"overflow-x": "hidden", "overflow-y": "scroll"}},
		{name: "overflow invalid arity falls back", prop: "overflow", val: "hidden scroll auto", want: map[string]string{"overflow": "hidden scroll auto"}},
		{name: "border-color one value", prop: "border-color", val: "red", want: map[string]string{
			"border-color": "red", "border-top-color": "red", "border-right-color": "red", "border-bottom-color": "red", "border-left-color": "red",
		}},
		{name: "border-color two values", prop: "border-color", val: "red blue", want: map[string]string{
			"border-color": "red blue", "border-top-color": "red", "border-right-color": "blue", "border-bottom-color": "red", "border-left-color": "blue",
		}},
		{name: "border-color four values", prop: "border-color", val: "red blue green yellow", want: map[string]string{
			"border-color": "red blue green yellow", "border-top-color": "red", "border-right-color": "blue", "border-bottom-color": "green", "border-left-color": "yellow",
		}},
		{name: "border-color functional color is not split on internal spaces", prop: "border-color", val: "rgb(255 0 0)", want: map[string]string{
			"border-color": "rgb(255 0 0)", "border-top-color": "rgb(255 0 0)", "border-right-color": "rgb(255 0 0)", "border-bottom-color": "rgb(255 0 0)", "border-left-color": "rgb(255 0 0)",
		}},
		{name: "border-color invalid arity falls back", prop: "border-color", val: "red blue green yellow purple", want: map[string]string{"border-color": "red blue green yellow purple"}},
		{name: "border one value is style only", prop: "border", val: "solid", want: map[string]string{"border-style": "solid"}},
		{name: "border two values are style then color", prop: "border", val: "solid red", want: map[string]string{
			"border-style": "solid", "border-color": "red", "border-top-color": "red", "border-right-color": "red", "border-bottom-color": "red", "border-left-color": "red",
		}},
		{name: "border three values ignore leading width", prop: "border", val: "1px solid red", want: map[string]string{
			"border-style": "solid", "border-color": "red", "border-top-color": "red", "border-right-color": "red", "border-bottom-color": "red", "border-left-color": "red",
		}},
		{name: "border three values with keyword width matching a style name still resolves positionally", prop: "border", val: "thick solid red", want: map[string]string{
			"border-style": "solid", "border-color": "red", "border-top-color": "red", "border-right-color": "red", "border-bottom-color": "red", "border-left-color": "red",
		}},
		{name: "border functional color is not split on internal spaces", prop: "border", val: "solid rgb(255 0 0)", want: map[string]string{
			"border-style": "solid", "border-color": "rgb(255 0 0)", "border-top-color": "rgb(255 0 0)", "border-right-color": "rgb(255 0 0)", "border-bottom-color": "rgb(255 0 0)", "border-left-color": "rgb(255 0 0)",
		}},
		{name: "border invalid arity falls back", prop: "border", val: "1px solid red extra", want: map[string]string{"border": "1px solid red extra"}},
		{name: "border-top quoted literal glyph passes through unchanged", prop: "border-top", val: `"═"`, want: map[string]string{"border-top": `"═"`}},
		{name: "border-top bareword none passes through unchanged for table.go's literal check", prop: "border-top", val: "none", want: map[string]string{"border-top": "none"}},
		{name: "border-top one value is style only", prop: "border-top", val: "solid", want: map[string]string{"border-top": "solid"}},
		{name: "border-top two values are style then color", prop: "border-top", val: "solid red", want: map[string]string{"border-top": "solid", "border-top-color": "red"}},
		{name: "border-top three values ignore leading width", prop: "border-top", val: "1px solid red", want: map[string]string{"border-top": "solid", "border-top-color": "red"}},
		{name: "border-left functional color is not split on internal spaces", prop: "border-left", val: "solid rgb(255 0 0)", want: map[string]string{"border-left": "solid", "border-left-color": "rgb(255 0 0)"}},
		{name: "border-right invalid arity falls back", prop: "border-right", val: "1px solid red extra", want: map[string]string{"border-right": "1px solid red extra"}},
		{name: "border-bottom one value is style only", prop: "border-bottom", val: "double", want: map[string]string{"border-bottom": "double"}},
		{name: "gap one value sets both axes", prop: "gap", val: "2", want: map[string]string{"row-gap": "2", "column-gap": "2"}},
		{name: "gap two values set row then column", prop: "gap", val: "1 2", want: map[string]string{"row-gap": "1", "column-gap": "2"}},
		{name: "gap invalid arity falls back", prop: "gap", val: "1 2 3", want: map[string]string{"gap": "1 2 3"}},
		{name: "flex none sets no grow no shrink auto basis", prop: "flex", val: "none", want: map[string]string{"flex-grow": "0", "flex-shrink": "0", "flex-basis": "auto"}},
		{name: "flex auto sets grow and shrink to 1 with auto basis", prop: "flex", val: "auto", want: map[string]string{"flex-grow": "1", "flex-shrink": "1", "flex-basis": "auto"}},
		{name: "flex initial sets no grow, shrink 1, auto basis", prop: "flex", val: "initial", want: map[string]string{"flex-grow": "0", "flex-shrink": "1", "flex-basis": "auto"}},
		{name: "flex single number sets grow with zero basis", prop: "flex", val: "2", want: map[string]string{"flex-grow": "2", "flex-shrink": "1", "flex-basis": "0"}},
		{name: "flex single basis value with no number", prop: "flex", val: "30%", want: map[string]string{"flex-basis": "30%"}},
		{name: "flex two numbers are grow then shrink with zero basis", prop: "flex", val: "1 2", want: map[string]string{"flex-grow": "1", "flex-shrink": "2", "flex-basis": "0"}},
		{name: "flex number then basis defaults shrink to 1", prop: "flex", val: "1 30%", want: map[string]string{"flex-grow": "1", "flex-shrink": "1", "flex-basis": "30%"}},
		{name: "flex three values are grow shrink basis", prop: "flex", val: "1 2 30%", want: map[string]string{"flex-grow": "1", "flex-shrink": "2", "flex-basis": "30%"}},
		{name: "flex invalid arity falls back", prop: "flex", val: "1 2 3 4", want: map[string]string{"flex": "1 2 3 4"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandShorthand(tc.prop, tc.val); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandShorthand(%q, %q) = %#v, want %#v", tc.prop, tc.val, got, tc.want)
			}
		})
	}
}

func stripRuleParts(rules []Rule) []Rule {
	out := make([]Rule, len(rules))
	for i, rl := range rules {
		rl.parts = nil
		out[i] = rl
	}
	return out
}

func TestParseCSSIgnoresComments(t *testing.T) {
	rules, err := ParseStylesheet(`/* disabled rule */
table { border-style: none; }
td/* comment */{ white-space: normal; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	want := []Rule{
		{selector: "table", decls: map[string]declValue{"border-style": {value: "none"}}},
		{selector: "td", decls: map[string]declValue{"white-space": {value: "normal"}}},
	}
	if got := stripRuleParts(rules); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseStylesheet() = %#v, want %#v", got, want)
	}
}

func TestSplitImportant(t *testing.T) {
	tests := []struct {
		in            string
		wantVal       string
		wantImportant bool
	}{
		{"none", "none", false},
		{"none !important", "none", true},
		{"none!important", "none", true},
		{"none !IMPORTANT", "none", true},
		{"none !ImPoRtAnT", "none", true},
		{"  none  !important  ", "none", true},
		{"", "", false},
		{"important", "important", false},
	}
	for _, tc := range tests {
		gotVal, gotImportant := splitImportant(tc.in)
		if gotVal != tc.wantVal || gotImportant != tc.wantImportant {
			t.Errorf("splitImportant(%q) = (%q, %v), want (%q, %v)", tc.in, gotVal, gotImportant, tc.wantVal, tc.wantImportant)
		}
	}
}

func TestParseCSSTracksImportant(t *testing.T) {
	rules, err := ParseStylesheet(`p { display: none !important; color: red; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	want := []Rule{
		{selector: "p", decls: map[string]declValue{
			"display": {value: "none", important: true},
			"color":   {value: "red"},
		}},
	}
	if got := stripRuleParts(rules); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseStylesheet() = %#v, want %#v", got, want)
	}
}

func TestParseCSSImportantThroughShorthand(t *testing.T) {
	rules, err := ParseStylesheet(`p { margin: 1 2 !important; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("ParseStylesheet() returned %d rules, want 1", len(rules))
	}
	for _, prop := range []string{"margin-top", "margin-right", "margin-bottom", "margin-left"} {
		if dv := rules[0].decls[prop]; !dv.important {
			t.Errorf("decls[%q].important = false, want true (decls: %#v)", prop, rules[0].decls)
		}
	}
}

// findElementByID returns the first element node with the given id attribute
// under n (including n itself), or nil if none is found.
func findElementByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode && nodeAttr(n, "id") == id {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElementByID(c, id); found != nil {
			return found
		}
	}
	return nil
}

func TestCascadeImportantOverridesHigherSpecificityNormal(t *testing.T) {
	rules, err := ParseStylesheet(`#a { color: blue; } p { color: red !important; }`)
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
	got := Cascade{Rules: rules}.Direct(n)
	if got["color"] != "red" {
		t.Fatalf(`Direct()["color"] = %q, want "red" (!important on lower-specificity rule should win)`, got["color"])
	}
}

func TestCascadeImportantRespectsSpecificityAmongImportantRules(t *testing.T) {
	// #a is declared first and p second, so a naive "last write wins" merge
	// would pick p's value; specificity must still be honored within the
	// important tier.
	rules, err := ParseStylesheet(`#a { color: blue !important; } p { color: red !important; }`)
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
	got := Cascade{Rules: rules}.Direct(n)
	if got["color"] != "blue" {
		t.Fatalf(`Direct()["color"] = %q, want "blue" (higher-specificity !important rule should win)`, got["color"])
	}
}

func TestCascadeStylesheetImportantBeatsInlineNormal(t *testing.T) {
	rules, err := ParseStylesheet(`p { color: red !important; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a" style="color: blue">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Direct(n)
	if got["color"] != "red" {
		t.Fatalf(`Direct()["color"] = %q, want "red" (stylesheet !important should beat non-important inline style)`, got["color"])
	}
}

func TestCascadeInlineImportantBeatsStylesheetImportant(t *testing.T) {
	rules, err := ParseStylesheet(`p { color: red !important; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a" style="color: blue !important">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules}.Direct(n)
	if got["color"] != "blue" {
		t.Fatalf(`Direct()["color"] = %q, want "blue" (!important inline style should beat !important stylesheet rule)`, got["color"])
	}
}

func TestCascadeIgnoreInlineSkipsImportantInline(t *testing.T) {
	rules, err := ParseStylesheet(`p { color: red; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	doc, err := html.Parse(strings.NewReader(`<p id="a" style="color: blue !important">x</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "a")
	if n == nil {
		t.Fatal(`<p id="a"> not found`)
	}
	got := Cascade{Rules: rules, IgnoreInline: true}.Direct(n)
	if got["color"] != "red" {
		t.Fatalf(`Direct()["color"] = %q, want "red" (IgnoreInline should skip inline style regardless of !important)`, got["color"])
	}
}

func TestCascadePseudoElementImportant(t *testing.T) {
	rules, err := ParseStylesheet(`#a::before { content: "low"; } p::before { content: "high" !important; }`)
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
	got := Cascade{Rules: rules}.PseudoElement(n, "before")
	if got["content"] != `"high"` {
		t.Fatalf(`PseudoElement()["content"] = %q, want %q (!important should win for pseudo-elements too)`, got["content"], `"high"`)
	}
}

func TestPseudoElementDoesNotMutateSharedCachedParts(t *testing.T) {
	rules, err := ParseStylesheet(`p::before { content: "> "; }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	cascade := Cascade{Rules: rules}

	doc, err := html.Parse(strings.NewReader(`<p id="a">one</p><p id="b">two</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	var a, b *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			switch nodeAttr(n, "id") {
			case "a":
				a = n
			case "b":
				b = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if a == nil || b == nil {
		t.Fatal("both <p> elements not found in parsed doc")
	}

	for i, n := range []*html.Node{a, b} {
		got := cascade.PseudoElement(n, "before")
		if got["content"] != `"> "` {
			t.Errorf("call %d: PseudoElement(%s)[\"content\"] = %q, want %q", i, nodeAttr(n, "id"), got["content"], `"> "`)
		}
	}
	if rules[0].parts[len(rules[0].parts)-1].pseudoElem != "before" {
		t.Errorf("rules[0].parts' pseudoElem = %q after matching, want unchanged %q", rules[0].parts[len(rules[0].parts)-1].pseudoElem, "before")
	}
}

func TestParseInlineDeclsStripsImportant(t *testing.T) {
	got := ParseDeclarations(`display: none !important; color: red`)
	want := map[string]string{"display": "none", "color": "red"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseDeclarations() = %#v, want %#v", got, want)
	}
}

func TestConsumeCSSQuotedTokenTrailingBackslashDoesNotPanic(t *testing.T) {
	s := `'a\`
	i := consumeCSSQuotedToken(s, 0)
	if i > len(s) {
		t.Fatalf("consumeCSSQuotedToken returned index %d beyond len(s)=%d", i, len(s))
	}
	_ = s[:i]
}

func TestParseCSSCommaInsideFunctionalPseudoIsNotAGroupSeparator(t *testing.T) {
	// A naive strings.Split(sel, ",") would break "a:is(.x, .y), b" into
	// "a:is(.x", " .y)", " b" — the first two fragments have unbalanced
	// parens and can never match a real element.
	rules, err := ParseStylesheet(`a:is(.x, .y), b { color: red }`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("ParseStylesheet() returned %d rules, want 2: %#v", len(rules), rules)
	}
	if rules[0].selector != "a:is(.x, .y)" || rules[1].selector != "b" {
		t.Fatalf("unexpected selectors: %q, %q", rules[0].selector, rules[1].selector)
	}

	doc, err := html.Parse(strings.NewReader(`<div><a id="a1" class="x">x</a></div><b id="b1">y</b>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	cascade := Cascade{Rules: rules}
	if got := cascade.Direct(findElementByID(doc, "a1")); got["color"] != "red" {
		t.Errorf(`Direct(#a1)["color"] = %q, want "red"`, got["color"])
	}
	if got := cascade.Direct(findElementByID(doc, "b1")); got["color"] != "red" {
		t.Errorf(`Direct(#b1)["color"] = %q, want "red"`, got["color"])
	}
}

func TestParseSelectorGroupCommaInsideFunctionalPseudoIsNotAGroupSeparator(t *testing.T) {
	group := ParseSelectorGroup("a:is(.x, .y), b")
	if len(group.groups) != 2 {
		t.Fatalf("ParseSelectorGroup() produced %d groups, want 2: %#v", len(group.groups), group.groups)
	}
}

func TestAttrSelectorBracketInsideQuotedValue(t *testing.T) {
	// A quote-blind bracket scan would stop at the "]" embedded in the
	// quoted value, truncating the selector and producing an attrSel that
	// can never match a real attribute.
	got, ok := parseAttrSel(`title="a]b"`)
	if !ok {
		t.Fatalf(`parseAttrSel("title=\"a]b\"") returned !ok`)
	}
	want := attrSel{key: "title", op: opEquals, val: "a]b"}
	if got != want {
		t.Fatalf(`parseAttrSel("title=\"a]b\"") = %#v, want %#v`, got, want)
	}

	doc, err := html.Parse(strings.NewReader(`<p id="a" title="a]b">x</p><p id="b" title="ab">y</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	parts := parseSelector(`[title="a]b"]`)
	if !matchSelector(findElementByID(doc, "a"), parts, "") {
		t.Errorf(`[title="a]b"] should match #a`)
	}
	if matchSelector(findElementByID(doc, "b"), parts, "") {
		t.Errorf(`[title="a]b"] should not match #b`)
	}
}

func TestPseudoClassNestedArgumentIsCachedNotReparsed(t *testing.T) {
	// parseSimpleSelector pre-parses :not()/:is()/:where() arguments once
	// into pseudoClass.notParts/isParts rather than leaving matchPseudo to
	// re-parse the raw string on every match attempt; confirm the cached
	// forms round-trip through parsing and matching correctly.
	part := parseSimpleSelector("p:not(.a):is(.b, .c)")
	if len(part.pseudos) != 2 {
		t.Fatalf("parseSimpleSelector(%q).pseudos has %d entries, want 2", "p:not(.a):is(.b, .c)", len(part.pseudos))
	}
	notPC, isPC := part.pseudos[0], part.pseudos[1]
	if len(notPC.notParts) != 1 || notPC.notParts[0].classes[0] != "a" {
		t.Fatalf("pseudos[0].notParts = %#v, want one parsed .a", notPC.notParts)
	}
	if len(isPC.isParts) != 2 || isPC.isParts[0].classes[0] != "b" || isPC.isParts[1].classes[0] != "c" {
		t.Fatalf("pseudos[1].isParts = %#v, want parsed .b and .c", isPC.isParts)
	}

	doc, err := html.Parse(strings.NewReader(`<p id="p1" class="b">x</p><p id="p2" class="a b">y</p><p id="p3" class="d">z</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	parts := parseSelector("p:not(.a):is(.b, .c)")
	tests := []struct {
		id   string
		want bool
	}{
		{"p1", true},  // has .b, lacks .a
		{"p2", false}, // has .a, excluded by :not(.a)
		{"p3", false}, // has neither .b nor .c
	}
	for _, tc := range tests {
		n := findElementByID(doc, tc.id)
		if n == nil {
			t.Fatalf("element #%s not found", tc.id)
		}
		if got := matchSelector(n, parts, ""); got != tc.want {
			t.Errorf("matchSelector(%q, #%s) = %v, want %v", "p:not(.a):is(.b, .c)", tc.id, got, tc.want)
		}
	}
}

func TestNotWithSelectorList(t *testing.T) {
	// Per CSS Selectors Level 4, :not() takes a full selector list, so
	// :not(.a, .b) must exclude elements matching EITHER .a OR .b — not be
	// parsed as one unsplit ".a, .b" compound selector (which used to fold
	// the comma into a garbage class name and make the pseudo-class
	// vacuously true for every element).
	part := parseSimpleSelector("a:not(.a, .b)")
	if len(part.pseudos) != 1 {
		t.Fatalf("parseSimpleSelector(%q).pseudos has %d entries, want 1", "a:not(.a, .b)", len(part.pseudos))
	}
	notParts := part.pseudos[0].notParts
	if len(notParts) != 2 || notParts[0].classes[0] != "a" || notParts[1].classes[0] != "b" {
		t.Fatalf("pseudos[0].notParts = %#v, want parsed .a and .b", notParts)
	}

	doc, err := html.Parse(strings.NewReader(`<a id="a1" class="b"></a><a id="a2" class="a"></a><a id="a3" class="c"></a>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	parts := parseSelector("a:not(.a, .b)")
	tests := []struct {
		id   string
		want bool
	}{
		{"a1", false}, // has .b, excluded by :not(.a, .b)
		{"a2", false}, // has .a, excluded by :not(.a, .b)
		{"a3", true},  // has neither .a nor .b
	}
	for _, tc := range tests {
		n := findElementByID(doc, tc.id)
		if n == nil {
			t.Fatalf("element #%s not found", tc.id)
		}
		if got := matchSelector(n, parts, ""); got != tc.want {
			t.Errorf("matchSelector(%q, #%s) = %v, want %v", "a:not(.a, .b)", tc.id, got, tc.want)
		}
	}

	// Specificity of :not(<list>) is that of the most specific selector in
	// the list, same rule as :is() — :not(#a, .b) should score as an ID.
	if got, want := specificity(parseSelector(":not(#a, .b)")), (specificityScore{ids: 1}); got != want {
		t.Fatalf("specificity(%q) = %#v, want %#v", ":not(#a, .b)", got, want)
	}
}

func TestParseCSSCommaAndShorthand(t *testing.T) {
	rules, err := ParseStylesheet("p, div { margin: 1 2; padding: 3; }")
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("ParseStylesheet() returned %d rules, want 2", len(rules))
	}
	for _, r := range rules {
		if r.selector != "p" && r.selector != "div" {
			t.Fatalf("unexpected selector %q", r.selector)
		}
		if r.decls["margin-left"].value != "2" || r.decls["padding-bottom"].value != "3" {
			t.Fatalf("unexpected decls for %q: %#v", r.selector, r.decls)
		}
	}
}

func TestParseCSSLogicalSpacingAliases(t *testing.T) {
	rules, err := ParseStylesheet(`p {
		margin-block-start: 1;
		margin-inline-end: auto;
		padding-block-end: 2;
		padding-inline-start: 3ch;
	}`)
	if err != nil {
		t.Fatalf("ParseStylesheet() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("ParseStylesheet() returned %d rules, want 1", len(rules))
	}
	decls := rules[0].decls
	want := map[string]string{
		"margin-top":     "1",
		"margin-right":   "auto",
		"padding-bottom": "2",
		"padding-left":   "3ch",
	}
	for k, v := range want {
		if decls[k].value != v {
			t.Fatalf("decls[%q] = %q, want %q; decls: %#v", k, decls[k].value, v, decls)
		}
	}
}

func TestParseInlineDeclsLogicalSpacingAliases(t *testing.T) {
	decls := ParseDeclarations("margin-inline-start: 2; margin-left: 4; padding: 1; padding-block-end: 3")
	want := map[string]string{
		"margin-left":    "4",
		"padding-top":    "1",
		"padding-right":  "1",
		"padding-bottom": "3",
		"padding-left":   "1",
	}
	for k, v := range want {
		if decls[k] != v {
			t.Fatalf("decls[%q] = %q, want %q; decls: %#v", k, decls[k], v, decls)
		}
	}
}

func TestParseAttrSelOperators(t *testing.T) {
	tests := []struct {
		in   string
		want attrSel
	}{
		{in: "data-x", want: attrSel{key: "data-x", op: opExists}},
		{in: "data-x=value", want: attrSel{key: "data-x", op: opEquals, val: "value"}},
		{in: `data-x="a~=b"`, want: attrSel{key: "data-x", op: opEquals, val: "a~=b"}},
		{in: `data-x~="value"`, want: attrSel{key: "data-x", op: opIncludes, val: "value"}},
		{in: `lang|='en'`, want: attrSel{key: "lang", op: opDashMatch, val: "en"}},
		{in: "href^=https://", want: attrSel{key: "href", op: opPrefix, val: "https://"}},
		{in: "href$=.pdf", want: attrSel{key: "href", op: opSuffix, val: ".pdf"}},
		{in: "href*=example", want: attrSel{key: "href", op: opSubstring, val: "example"}},
	}
	for _, tc := range tests {
		got, ok := parseAttrSel(tc.in)
		if !ok {
			t.Fatalf("parseAttrSel(%q) returned !ok", tc.in)
		}
		if got != tc.want {
			t.Fatalf("parseAttrSel(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestStructuralPseudoClasses(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<ul id="list">
			<li id="a">a</li>
			<li id="b">b</li>
			<li id="c">c</li>
			<li id="d">d</li>
			<li id="e">e</li>
		</ul>
		<div id="mixed">
			<span id="s1">x</span>
			<p id="p1">y</p>
			<span id="s2">z</span>
			<p id="p2">w</p>
		</div>
		<div id="lonely"><i id="only">solo</i></div>
		<div id="e1"></div>
		<div id="e2"> </div>
		<div id="e3"><!-- comment --></div>
		<div id="e4">x</div>
	`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	find := func(id string) *html.Node { return findElementByID(doc, id) }

	tests := []struct {
		sel  string
		id   string
		want bool
	}{
		{"li:nth-child(2n+1)", "a", true},
		{"li:nth-child(2n+1)", "b", false},
		{"li:nth-child(2n+1)", "e", true},
		{"li:nth-child(-n+2)", "a", true},
		{"li:nth-child(-n+2)", "b", true},
		{"li:nth-child(-n+2)", "c", false},
		{"li:nth-child(3)", "c", true},
		{"li:nth-child(3)", "b", false},
		{"li:nth-last-child(1)", "e", true},
		{"li:nth-last-child(1)", "d", false},
		{"li:nth-last-child(odd)", "e", true},
		{"span:nth-of-type(2)", "s2", true},
		{"span:nth-of-type(2)", "s1", false},
		{"p:nth-last-of-type(1)", "p2", true},
		{"p:nth-last-of-type(1)", "p1", false},
		{"span:first-of-type", "s1", true},
		{"span:first-of-type", "s2", false},
		{"p:last-of-type", "p2", true},
		{"p:last-of-type", "p1", false},
		{"i:only-of-type", "only", true},
		{"i:only-child", "only", true},
		{"span:only-child", "s1", false},
		{"div:empty", "e1", true},
		{"div:empty", "e2", false},
		{"div:empty", "e3", true},
		{"div:empty", "e4", false},
	}
	for _, tc := range tests {
		t.Run(tc.sel+"/"+tc.id, func(t *testing.T) {
			n := find(tc.id)
			if n == nil {
				t.Fatalf("element #%s not found", tc.id)
			}
			parts := parseSelector(tc.sel)
			if got := matchSelector(n, parts, ""); got != tc.want {
				t.Errorf("matchSelector(%q, #%s) = %v, want %v", tc.sel, tc.id, got, tc.want)
			}
		})
	}
}

func TestGeneralSiblingCombinator(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<div id="root">
			<h2 id="h1">intro</h2>
			<p id="p1">a</p>
			<span id="s1">b</span>
			<p id="p2">c</p>
		</div>
		<div id="other">
			<p id="p3">d</p>
		</div>
	`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	find := func(id string) *html.Node { return findElementByID(doc, id) }

	tests := []struct {
		sel  string
		id   string
		want bool
	}{
		// h2 ~ p matches any later <p> sibling of an <h2>, not just the
		// immediately adjacent one.
		{"h2 ~ p", "p1", true},
		{"h2 ~ p", "p2", true},
		{"h2 ~ p", "s1", false},
		// No preceding <h2> sibling in the "other" subtree.
		{"h2 ~ p", "p3", false},
		// + only matches the immediately following sibling.
		{"h2 + p", "p1", true},
		{"h2 + p", "p2", false},
		// Combinators compose: match a <p> preceded by a <span> which is
		// itself preceded (anywhere) by an <h2>.
		{"h2 ~ span ~ p", "p2", true},
		{"h2 ~ span ~ p", "p1", false},
	}
	for _, tc := range tests {
		t.Run(tc.sel+"/"+tc.id, func(t *testing.T) {
			n := find(tc.id)
			if n == nil {
				t.Fatalf("element #%s not found", tc.id)
			}
			parts := parseSelector(tc.sel)
			if got := matchSelector(n, parts, ""); got != tc.want {
				t.Errorf("matchSelector(%q, #%s) = %v, want %v", tc.sel, tc.id, got, tc.want)
			}
		})
	}
}

func TestParseNth(t *testing.T) {
	tests := []struct {
		in     string
		wantA  int
		wantB  int
		wantOK bool
	}{
		{"odd", 2, 1, true},
		{"even", 2, 0, true},
		{"3", 0, 3, true},
		{"-3", 0, -3, true},
		{"2n", 2, 0, true},
		{"2n+1", 2, 1, true},
		{"2n-1", 2, -1, true},
		{"-n+3", -1, 3, true},
		{"n", 1, 0, true},
		{"n+3", 1, 3, true},
		{"", 0, 0, false},
		{"bogus", 0, 0, false},
	}
	for _, tc := range tests {
		a, b, ok := parseNth(tc.in)
		if a != tc.wantA || b != tc.wantB || ok != tc.wantOK {
			t.Errorf("parseNth(%q) = (%d, %d, %v), want (%d, %d, %v)", tc.in, a, b, ok, tc.wantA, tc.wantB, tc.wantOK)
		}
	}
}

func TestSelectorSpecificityUniversalAndRoot(t *testing.T) {
	tests := []struct {
		sel  string
		want specificityScore
	}{
		{sel: "*", want: specificityScore{}},
		{sel: "*.hot", want: specificityScore{classes: 1}},
		{sel: "*::before", want: specificityScore{elements: 1}},
		{sel: ":root", want: specificityScore{classes: 1}},
		{sel: ":not(*)", want: specificityScore{}},
		{sel: ":is(#a, .b)", want: specificityScore{ids: 1}},
		{sel: ":is(.a, .b)", want: specificityScore{classes: 1}},
		{sel: ":is(p, span)", want: specificityScore{elements: 1}},
		{sel: ":where(#a, .b)", want: specificityScore{}},
		{sel: "p:where(.a)", want: specificityScore{elements: 1}},
	}
	for _, tc := range tests {
		if got := specificity(parseSelector(tc.sel)); got != tc.want {
			t.Fatalf("specificity(%q) = %#v, want %#v", tc.sel, got, tc.want)
		}
	}
}

func TestIsWherePseudoClasses(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<header id="h">head</header>
		<footer id="f">foot</footer>
		<p id="p1" class="warn">a</p>
		<p id="p2">b</p>
	`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	find := func(id string) *html.Node { return findElementByID(doc, id) }

	tests := []struct {
		sel  string
		id   string
		want bool
	}{
		{":is(header, footer)", "h", true},
		{":is(header, footer)", "f", true},
		{":is(header, footer)", "p1", false},
		{":where(header, footer)", "h", true},
		{":where(header, footer)", "p1", false},
		{"p:is(.warn, #p2)", "p1", true},
		{"p:is(.warn, #p2)", "p2", true},
		{"p:where(.warn, #p2)", "p1", true},
	}
	for _, tc := range tests {
		t.Run(tc.sel+"/"+tc.id, func(t *testing.T) {
			n := find(tc.id)
			if n == nil {
				t.Fatalf("element #%s not found", tc.id)
			}
			parts := parseSelector(tc.sel)
			if got := matchSelector(n, parts, ""); got != tc.want {
				t.Errorf("matchSelector(%q, #%s) = %v, want %v", tc.sel, tc.id, got, tc.want)
			}
		})
	}

	// :where() always contributes zero specificity, even with a
	// high-specificity argument, so a plain element selector can still
	// override it via later source order / normal cascade rules; the
	// specificity itself must literally be zero.
	if got := specificity(parseSelector(":where(#a.b.c)")); got != (specificityScore{}) {
		t.Fatalf("specificity(%q) = %#v, want zero", ":where(#a.b.c)", got)
	}
	// :is() takes the specificity of its most specific argument.
	if got := specificity(parseSelector(":is(.a, #b, span)")); got != (specificityScore{ids: 1}) {
		t.Fatalf("specificity(%q) = %#v, want {ids:1}", ":is(.a, #b, span)", got)
	}
}
