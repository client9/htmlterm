package htmlterm

import (
	"reflect"
	"testing"
)

func TestExpandShorthand(t *testing.T) {
	tests := []struct {
		name string
		prop string
		val  string
		want map[string]string
	}{
		{
			name: "margin one value",
			prop: "margin",
			val:  "2",
			want: map[string]string{
				"margin-top":    "2",
				"margin-right":  "2",
				"margin-bottom": "2",
				"margin-left":   "2",
			},
		},
		{
			name: "padding two values",
			prop: "padding",
			val:  "1 3",
			want: map[string]string{
				"padding-top":    "1",
				"padding-right":  "3",
				"padding-bottom": "1",
				"padding-left":   "3",
			},
		},
		{
			name: "margin three values",
			prop: "margin",
			val:  "1 2 3",
			want: map[string]string{
				"margin-top":    "1",
				"margin-right":  "2",
				"margin-bottom": "3",
				"margin-left":   "2",
			},
		},
		{
			name: "padding four values",
			prop: "padding",
			val:  "1 2 3 4",
			want: map[string]string{
				"padding-top":    "1",
				"padding-right":  "2",
				"padding-bottom": "3",
				"padding-left":   "4",
			},
		},
		{
			name: "invalid arity falls back",
			prop: "margin",
			val:  "1 2 3 4 5",
			want: map[string]string{"margin": "1 2 3 4 5"},
		},
		{
			name: "list-style type and position",
			prop: "list-style",
			val:  "upper-roman inside",
			want: map[string]string{
				"list-style-type":     "upper-roman",
				"list-style-position": "inside",
			},
		},
		{
			name: "list-style position before type",
			prop: "list-style",
			val:  "outside square",
			want: map[string]string{
				"list-style-type":     "square",
				"list-style-position": "outside",
			},
		},
		{
			name: "list-style custom string preserves spaces",
			prop: "list-style",
			val:  `"-> " inside`,
			want: map[string]string{
				"list-style-type":     `"-> "`,
				"list-style-position": "inside",
			},
		},
		{
			name: "list-style ignores url image",
			prop: "list-style",
			val:  `url("bullet image.png") circle inside`,
			want: map[string]string{
				"list-style-type":     "circle",
				"list-style-position": "inside",
			},
		},
		{
			name: "background extracts named color",
			prop: "background",
			val:  "red",
			want: map[string]string{"background-color": "red"},
		},
		{
			name: "background extracts color among unsupported tokens",
			prop: "background",
			val:  "url(bg.png) no-repeat center/cover #123456",
			want: map[string]string{"background-color": "#123456"},
		},
		{
			name: "background extracts functional color",
			prop: "background",
			val:  "rgb(255 0 0) fixed",
			want: map[string]string{"background-color": "rgb(255 0 0)"},
		},
		{
			name: "background ignores url without color",
			prop: "background",
			val:  "url(bg.png) no-repeat center/cover",
			want: map[string]string{},
		},
		{
			name: "margin block start alias",
			prop: "margin-block-start",
			val:  "1",
			want: map[string]string{"margin-top": "1"},
		},
		{
			name: "margin block end alias",
			prop: "margin-block-end",
			val:  "2",
			want: map[string]string{"margin-bottom": "2"},
		},
		{
			name: "margin inline start alias",
			prop: "margin-inline-start",
			val:  "3",
			want: map[string]string{"margin-left": "3"},
		},
		{
			name: "margin inline end alias",
			prop: "margin-inline-end",
			val:  "4",
			want: map[string]string{"margin-right": "4"},
		},
		{
			name: "padding block start alias",
			prop: "padding-block-start",
			val:  "5",
			want: map[string]string{"padding-top": "5"},
		},
		{
			name: "padding block end alias",
			prop: "padding-block-end",
			val:  "6",
			want: map[string]string{"padding-bottom": "6"},
		},
		{
			name: "padding inline start alias",
			prop: "padding-inline-start",
			val:  "7",
			want: map[string]string{"padding-left": "7"},
		},
		{
			name: "padding inline end alias",
			prop: "padding-inline-end",
			val:  "8",
			want: map[string]string{"padding-right": "8"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandShorthand(tc.prop, tc.val); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandShorthand(%q, %q) = %#v, want %#v", tc.prop, tc.val, got, tc.want)
			}
		})
	}
}

func TestParseCSSIgnoresComments(t *testing.T) {
	rules, err := parseCSS(`/* disabled rule */
table { border-style: none; }
td/* comment */{ white-space: normal; }`)
	if err != nil {
		t.Fatalf("parseCSS() error = %v", err)
	}
	want := []rule{
		{selector: "table", decls: map[string]string{"border-style": "none"}},
		{selector: "td", decls: map[string]string{"white-space": "normal"}},
	}
	if !reflect.DeepEqual(rules, want) {
		t.Fatalf("parseCSS() = %#v, want %#v", rules, want)
	}
}

func TestMergeInlineStyleTextDecoration(t *testing.T) {
	base := mergeInlineStyle(inlineStyle{}, map[string]string{"text-decoration": "underline line-through"})
	if !base.underline || !base.strike {
		t.Fatalf("combined decoration not applied: %#v", base)
	}

	reset := mergeInlineStyle(base, map[string]string{"text-decoration": "none"})
	if reset.underline || reset.strike {
		t.Fatalf("text-decoration:none did not reset flags: %#v", reset)
	}
}

func TestSplitANSITokens(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x07"
	oscEnd := "\x1b]8;;\x07"
	text := "pre " + osc + "link text" + oscEnd + " post\t\x1b[31mred\x1b[0m"
	want := []string{
		"pre",
		osc + "link",
		"text" + oscEnd,
		"post",
		"\x1b[31mred\x1b[0m",
	}
	if got := splitANSITokens(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("splitANSITokens() = %#v, want %#v", got, want)
	}
}

func TestStripANSI(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x1b\\"
	oscEnd := "\x1b]8;;\x1b\\"
	text := "a\x1b[31mred\x1b[0m" + osc + "link" + oscEnd + "b"
	if got := stripANSI(text); got != "aredlinkb" {
		t.Fatalf("stripANSI() = %q, want %q", got, "aredlinkb")
	}
}

// TestANSIIntermediateByte verifies that two-byte escape sequences whose first
// byte is an intermediate (0x20–0x3F, e.g. ESC '(' for ISO 2022 G0 charset)
// are fully consumed and do not leak their final byte as visible content.
func TestANSIIntermediateByte(t *testing.T) {
	// ESC '(' 'B' — G0 charset designation to US-ASCII (ISO 2022).
	seq := "\x1b(B"

	// ansiVisibleLen must not count the final byte 'B' as visible.
	if got := ansiVisibleLen(seq); got != 0 {
		t.Errorf("ansiVisibleLen(ESC ( B) = %d, want 0", got)
	}
	// stripANSI must consume the whole 3-byte sequence, producing no output.
	if got := stripANSI(seq); got != "" {
		t.Errorf("stripANSI(ESC ( B) = %q, want %q", got, "")
	}

	// Mixed: visible text around the sequence.
	mixed := "ab" + seq + "cd"
	if got := ansiVisibleLen(mixed); got != 4 {
		t.Errorf("ansiVisibleLen(mixed) = %d, want 4", got)
	}
	if got := stripANSI(mixed); got != "abcd" {
		t.Errorf("stripANSI(mixed) = %q, want %q", got, "abcd")
	}
	// splitANSITokens must produce one token containing the word with the
	// sequence attached — the sequence must not cause an extra word split.
	if got := splitANSITokens("hi" + seq + " there"); !reflect.DeepEqual(got, []string{"hi" + seq, "there"}) {
		t.Errorf("splitANSITokens with intermediate escape = %#v", got)
	}
}

func TestToRoman(t *testing.T) {
	if got := toRoman(49, false); got != "xlix" {
		t.Fatalf("toRoman lower = %q, want %q", got, "xlix")
	}
	if got := toRoman(944, true); got != "CMXLIV" {
		t.Fatalf("toRoman upper = %q, want %q", got, "CMXLIV")
	}
}

func TestMaxRomanPrefixWidth(t *testing.T) {
	tests := []struct{ count, want int }{
		{1, 3},    // "i. "
		{3, 5},    // "iii. "
		{8, 6},    // "viii. "
		{18, 7},   // "xviii. "
		{28, 8},   // "xxviii. "
		{38, 9},   // "xxxviii. "
		{88, 10},  // "lxxxviii. "
		{500, 13}, // "cccxcix. " is not widest; 388="ccclxxxviii"=11 → 388. "ccclxxxviii. "=13
	}
	for _, tc := range tests {
		if got := maxRomanPrefixWidth(tc.count); got != tc.want {
			t.Errorf("maxRomanPrefixWidth(%d) = %d, want %d", tc.count, got, tc.want)
		}
	}
}

func TestListItemPrefixWidthRoman(t *testing.T) {
	if got := listItemPrefixWidth("lower-roman", true, 8); got != len("viii. ") {
		t.Fatalf("lower-roman width = %d, want %d", got, len("viii. "))
	}
	// width=0: raw prefix, no padding (used during width measurement).
	if got := listItemPrefix("lower-roman", true, 8, 0); got != "viii. " {
		t.Fatalf("lower-roman prefix(width=0) = %q, want %q", got, "viii. ")
	}
	// width=prefixWidth: right-aligned to match hangStr.
	if got := listItemPrefix("lower-roman", true, 1, 6); got != "   i. " {
		t.Fatalf("lower-roman prefix(width=6, n=1) = %q, want %q", got, "   i. ")
	}
	if got := listItemPrefix("upper-roman", true, 3, 5); got != "III. " {
		t.Fatalf("upper-roman prefix(width=5, n=3) = %q, want %q", got, "III. ")
	}
}

func TestSizeColumnsRespectsMaxAndShrinkMin(t *testing.T) {
	expanded := sizeColumns([]colConstraints{
		{natural: 2},
		{natural: 3, maxWidth: 4},
		{fixed: 5},
	}, 15, true)
	if !reflect.DeepEqual(expanded, []int{6, 4, 5}) {
		t.Fatalf("sizeColumns expand = %#v, want %#v", expanded, []int{6, 4, 5})
	}

	shrunk := sizeColumns([]colConstraints{
		{natural: 8, minWidth: 6},
		{natural: 7},
		{fixed: 5},
	}, 15, false)
	if !reflect.DeepEqual(shrunk, []int{6, 4, 5}) {
		t.Fatalf("sizeColumns shrink = %#v, want %#v", shrunk, []int{6, 4, 5})
	}

	balanced := sizeColumns([]colConstraints{
		{natural: 35},
		{natural: 35},
	}, 34, false)
	if !reflect.DeepEqual(balanced, []int{17, 17}) {
		t.Fatalf("sizeColumns balanced shrink = %#v, want %#v", balanced, []int{17, 17})
	}

	percentShrunk := sizeColumns([]colConstraints{
		{natural: 5, percent: 1},
		{natural: 5, percent: 1},
	}, 29, false)
	if !reflect.DeepEqual(percentShrunk, []int{14, 15}) {
		t.Fatalf("sizeColumns percent shrink = %#v, want %#v", percentShrunk, []int{14, 15})
	}
}

func TestAlignAndEdgeHelpersPreserveTrailingNewline(t *testing.T) {
	aligned := alignLines("a\nbb\n", "center", 4)
	if aligned != " a  \n bb \n" {
		t.Fatalf("alignLines() = %q, want %q", aligned, " a  \n bb \n")
	}

	edged := applyLineEdges("x\ny\n", "[", "]")
	if edged != "[x]\n[y]\n" {
		t.Fatalf("applyLineEdges() = %q, want %q", edged, "[x]\n[y]\n")
	}
}

func TestParseCSSCommaAndShorthand(t *testing.T) {
	rules, err := parseCSS("p, div { margin: 1 2; padding: 3; }")
	if err != nil {
		t.Fatalf("parseCSS() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("parseCSS() returned %d rules, want 2", len(rules))
	}
	for _, r := range rules {
		if r.selector != "p" && r.selector != "div" {
			t.Fatalf("unexpected selector %q", r.selector)
		}
		if r.decls["margin-left"] != "2" || r.decls["padding-bottom"] != "3" {
			t.Fatalf("unexpected decls for %q: %#v", r.selector, r.decls)
		}
	}
}

func TestParseCSSLogicalSpacingAliases(t *testing.T) {
	rules, err := parseCSS(`p {
		margin-block-start: 1;
		margin-inline-end: auto;
		padding-block-end: 2;
		padding-inline-start: 3ch;
	}`)
	if err != nil {
		t.Fatalf("parseCSS() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("parseCSS() returned %d rules, want 1", len(rules))
	}
	decls := rules[0].decls
	want := map[string]string{
		"margin-top":     "1",
		"margin-right":   "auto",
		"padding-bottom": "2",
		"padding-left":   "3ch",
	}
	for k, v := range want {
		if decls[k] != v {
			t.Fatalf("decls[%q] = %q, want %q; decls: %#v", k, decls[k], v, decls)
		}
	}
}

func TestParseInlineDeclsLogicalSpacingAliases(t *testing.T) {
	decls := parseInlineDecls("margin-inline-start: 2; margin-left: 4; padding: 1; padding-block-end: 3")
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

func TestEffectiveMinMaxPercent(t *testing.T) {
	// minPercent > 0 path
	minW, maxW := effectiveMinMax(colConstraints{minPercent: 0.5}, 20)
	if minW != 10 {
		t.Errorf("minPercent=0.5, contentWidth=20: minW = %d, want 10", minW)
	}
	if maxW != 0 {
		t.Errorf("minPercent=0.5: maxW = %d, want 0", maxW)
	}

	// minPercent loses to a higher minWidth
	minW, _ = effectiveMinMax(colConstraints{minPercent: 0.25, minWidth: 8}, 20)
	if minW != 8 {
		t.Errorf("minWidth beats minPercent: minW = %d, want 8", minW)
	}

	// maxPercent > 0 path, no maxWidth set (maxW == 0 branch)
	_, maxW = effectiveMinMax(colConstraints{maxPercent: 0.25}, 20)
	if maxW != 5 {
		t.Errorf("maxPercent=0.25, contentWidth=20: maxW = %d, want 5", maxW)
	}

	// maxPercent stricter than maxWidth (mp < maxW branch)
	_, maxW = effectiveMinMax(colConstraints{maxPercent: 0.25, maxWidth: 8}, 20)
	if maxW != 5 {
		t.Errorf("maxPercent stricter: maxW = %d, want 5", maxW)
	}

	// maxWidth stricter than maxPercent (mp >= maxW, no update)
	_, maxW = effectiveMinMax(colConstraints{maxPercent: 0.5, maxWidth: 3}, 20)
	if maxW != 3 {
		t.Errorf("maxWidth stricter: maxW = %d, want 3", maxW)
	}
}

func TestSplitAtVisualWidthEdgeCases(t *testing.T) {
	// Empty string early return
	got := splitAtVisualWidth("", 5)
	if !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("splitAtVisualWidth(%q, 5) = %#v, want [%q]", "", got, "")
	}

	// OSC hyperlink sequences don't count toward visible width and are preserved
	osc := "\x1b]8;;https://example.com\x07"
	// "abc" (3 visible) + OSC + "def" (3 visible), split at width 3
	text := "abc" + osc + "def"
	lines := splitAtVisualWidth(text, 3)
	if len(lines) != 2 {
		t.Fatalf("OSC split: want 2 lines, got %d: %#v", len(lines), lines)
	}
	if v := ansiVisibleLen(lines[0]); v != 3 {
		t.Errorf("OSC split lines[0] visible len = %d, want 3", v)
	}
	if v := stripANSI(lines[0]); v != "abc" {
		t.Errorf("OSC split lines[0] text = %q, want %q", v, "abc")
	}
	// The hyperlink span is reopened on the continuation line so it stays
	// active across the break (see ansiCarry in textutil.go).
	if lines[1] != osc+"def" {
		t.Errorf("OSC split lines[1] = %q, want %q", lines[1], osc+"def")
	}
}

func TestCopyMapNonEmpty(t *testing.T) {
	src := map[string]string{"color": "red", "font-weight": "bold"}
	dst := copyMap(src)
	if !reflect.DeepEqual(dst, src) {
		t.Errorf("copyMap(%v) = %v, want same", src, dst)
	}
	// Mutation isolation
	dst["color"] = "blue"
	if src["color"] != "red" {
		t.Errorf("copyMap result shares storage with source")
	}
}

func TestParseCSSContentStringNoOOBPanic(t *testing.T) {
	r := &Renderer{}
	// Quoted string whose last byte is a backslash (malformed CSS): must not panic.
	got := r.parseCSSContentString(`"abc\`, nil)
	_ = got
	// Single-quoted variant.
	got = r.parseCSSContentString(`'x\`, nil)
	_ = got
	// Backslash is second-to-last followed by a closing quote (valid escape).
	got = r.parseCSSContentString(`"a\"`, nil)
	_ = got
}

func TestClampCellPaddingZeroWidth(t *testing.T) {
	// Zero-width column must produce zero content, not a stray space character.
	for _, tc := range []struct{ pl, pr int }{{0, 0}, {2, 2}, {1, 0}} {
		pl, pr, cw := clampCellPadding(0, tc.pl, tc.pr)
		if pl != 0 || pr != 0 || cw != 0 {
			t.Errorf("clampCellPadding(0,%d,%d) = (%d,%d,%d), want (0,0,0)", tc.pl, tc.pr, pl, pr, cw)
		}
	}
}
