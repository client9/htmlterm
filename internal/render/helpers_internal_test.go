package render

import (
	"reflect"
	"testing"
)

// TestConsumeQuotedTokenTrailingBackslashDoesNotPanic is consumeQuotedToken's
// (counter.go, used by parseQuotes for the CSS quotes property) equivalent of
// the consumeCSSQuotedToken regression above.
func TestConsumeQuotedTokenTrailingBackslashDoesNotPanic(t *testing.T) {
	value, rest, ok := consumeQuotedToken(`'a\`)
	if !ok {
		t.Fatalf("consumeQuotedToken ok = false, want true")
	}
	t.Logf("value=%q rest=%q", value, rest)
}

// TestParseCounterFnArgsTrailingBackslashDoesNotPanic is parseCounterFnArgs'
// (counter.go, used for content: counters(name, sep, style)) equivalent of
// the consumeCSSQuotedToken regression above — the sep argument's quoted
// string scan had the same unbounded backslash-escape jump.
func TestParseCounterFnArgsTrailingBackslashDoesNotPanic(t *testing.T) {
	name, sep, style := parseCounterFnArgs(`x, "a\`)
	if name != "x" {
		t.Errorf("name = %q, want %q", name, "x")
	}
	t.Logf("sep=%q style=%q", sep, style)
}

func TestResolveBorderEdgeChar(t *testing.T) {
	topGlyph := func(ts tableStyle) string {
		if ts.top != nil {
			return ts.top.fill
		}
		return ""
	}

	tests := []struct {
		name     string
		raw      string
		wantChar string
		wantOK   bool
	}{
		{name: "unset", raw: "", wantChar: "", wantOK: false},
		{name: "quoted literal glyph", raw: `"═"`, wantChar: "═", wantOK: true},
		{name: "single-quoted literal glyph", raw: `'▌'`, wantChar: "▌", wantOK: true},
		{name: "bareword style keyword resolves via the edge's glyph func", raw: "double", wantChar: "═", wantOK: true},
		{name: "bareword none is present but empty - must not be backfilled by border-style", raw: "none", wantChar: "", wantOK: true},
		{name: "unrecognized bareword falls through to border-style backfill", raw: "not-a-style", wantChar: "", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			char, ok := resolveBorderEdgeChar(tc.raw, topGlyph)
			if char != tc.wantChar || ok != tc.wantOK {
				t.Fatalf("resolveBorderEdgeChar(%q) = (%q, %v), want (%q, %v)", tc.raw, char, ok, tc.wantChar, tc.wantOK)
			}
		})
	}
}

func TestMergeInlineStyleTextDecoration(t *testing.T) {
	base := mergeInlineStyle(newInlineStyle(), map[string]string{"text-decoration": "underline line-through"})
	if !base.underline || !base.strike {
		t.Fatalf("combined decoration not applied: %#v", base)
	}

	reset := mergeInlineStyle(base, map[string]string{"text-decoration": "none"})
	if reset.underline || reset.strike {
		t.Fatalf("text-decoration:none did not reset flags: %#v", reset)
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

func TestParseCSSContentStringNoOOBPanic(t *testing.T) {
	r := &Engine{}
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

// TestLastRuneSeesThroughANSIStyling verifies lastRune reports a styled
// trailing space's true last visible rune (' '), not the last byte of its
// closing SGR reset — the fix that lets appendTextSegment keep a styled
// run's trailing space inside its own ANSI span (see inline.go) without
// breaking whitespace-collapse decisions that key off lastRune.
func TestLastRuneSeesThroughANSIStyling(t *testing.T) {
	tokens := []wrapToken{{text: "\x1b[31mred \x1b[0m"}}
	r, ok := lastRune(tokens)
	if !ok || r != ' ' {
		t.Fatalf("lastRune(styled trailing space) = (%q, %v), want (' ', true)", r, ok)
	}
}
