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
	}
	for _, tc := range tests {
		if got := specificity(parseSelector(tc.sel)); got != tc.want {
			t.Fatalf("specificity(%q) = %#v, want %#v", tc.sel, got, tc.want)
		}
	}
}
