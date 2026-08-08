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

	// A calc() width behaves like a fixed width in the growth pass: excluded
	// from extra-space distribution, same as {fixed: 5} in "expanded" above,
	// which this case is otherwise identical to.
	calcExpanded := sizeColumns([]colConstraints{
		{natural: 2},
		{natural: 3, maxWidth: 4},
		{calc: "calc(1 + 4)"},
	}, 15, true)
	if !reflect.DeepEqual(calcExpanded, []int{6, 4, 5}) {
		t.Fatalf("sizeColumns calc expand = %#v, want %#v", calcExpanded, []int{6, 4, 5})
	}

	// A calc() width behaves like a percent width in the shrink pass:
	// eligible to shrink, same numeric outcome as the percent case just
	// above, which "calc(100%)" is equivalent to.
	calcShrunk := sizeColumns([]colConstraints{
		{natural: 5, calc: "calc(100%)"},
		{natural: 5, calc: "calc(100%)"},
	}, 29, false)
	if !reflect.DeepEqual(calcShrunk, []int{14, 15}) {
		t.Fatalf("sizeColumns calc shrink = %#v, want %#v", calcShrunk, []int{14, 15})
	}

	// calc() mixing a percent and an absolute term, which fixed/percent
	// alone can't represent (see colConstraints' own doc comment on why
	// calc needs a separate field). No shrink/grow pressure here (natural
	// widths already sum to contentWidth), so this pins resolution alone.
	mixed := sizeColumns([]colConstraints{
		{natural: 8, calc: "calc(50% - 2)"},
		{natural: 8, fixed: 8},
	}, 20, false)
	if !reflect.DeepEqual(mixed, []int{8, 8}) {
		t.Fatalf("sizeColumns calc(50%%-2) at contentWidth=20 = %#v, want %#v (10-2=8)", mixed, []int{8, 8})
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

// TestEffectiveMinMaxCalc mirrors TestEffectiveMinMaxPercent's cases through
// minCalc/maxCalc instead of minPercent/maxPercent, pinning that calc()
// resolves through the same min/max combination rules a plain percentage
// already does (minCalc/maxCalc take priority over minPercent/maxPercent,
// per cellConstraints' mutual exclusivity, so these use only the calc
// fields).
func TestEffectiveMinMaxCalc(t *testing.T) {
	minW, maxW := effectiveMinMax(colConstraints{minCalc: "min(50%, 90%)"}, 20)
	if minW != 10 {
		t.Errorf("minCalc=min(50%%,90%%), contentWidth=20: minW = %d, want 10", minW)
	}
	if maxW != 0 {
		t.Errorf("minCalc only: maxW = %d, want 0", maxW)
	}

	// minCalc loses to a higher minWidth, same priority rule minPercent has.
	minW, _ = effectiveMinMax(colConstraints{minCalc: "calc(25% - 1)", minWidth: 8}, 20)
	if minW != 8 {
		t.Errorf("minWidth beats minCalc: minW = %d, want 8", minW)
	}

	// maxCalc, no maxWidth set (maxW == 0 branch).
	_, maxW = effectiveMinMax(colConstraints{maxCalc: "calc(20% + 1)"}, 20)
	if maxW != 5 {
		t.Errorf("maxCalc=calc(20%%+1), contentWidth=20: maxW = %d, want 5", maxW)
	}

	// maxCalc stricter than maxWidth (mp < maxW branch).
	_, maxW = effectiveMinMax(colConstraints{maxCalc: "clamp(0, 25%, 100)", maxWidth: 8}, 20)
	if maxW != 5 {
		t.Errorf("maxCalc stricter: maxW = %d, want 5", maxW)
	}

	// A calc() that fails to resolve contributes nothing, same as an unset
	// min-width/max-width: contentWidth is always definite for a table
	// column, so this only happens for a malformed declaration.
	minW, maxW = effectiveMinMax(colConstraints{minCalc: "calc(not a length)", maxCalc: "calc(also not one)"}, 20)
	if minW != 0 || maxW != 0 {
		t.Errorf("unresolvable minCalc/maxCalc: (minW, maxW) = (%d, %d), want (0, 0)", minW, maxW)
	}
}

// TestCellConstraintsCalc pins that cellConstraints classifies a calc()/
// min()/max()/clamp() width/min-width/max-width declaration into the calc
// fields, not fixed or percent, and that cellBoxSizingDelta's box-sizing
// adjustment still rides along via calcDelta the same way it already does
// for percentDelta.
func TestCellConstraintsCalc(t *testing.T) {
	r := &Engine{}
	c := r.cellConstraints(map[string]string{
		"width":        "calc(50% - 2)",
		"min-width":    "min(10, 20)",
		"max-width":    "clamp(5, 8, 30)",
		"box-sizing":   "content-box",
		"padding-left": "1", "padding-right": "1",
	}, 0)
	if c.calc != "calc(50% - 2)" || c.fixed != 0 || c.percent != 0 {
		t.Errorf("width: calc=%q fixed=%d percent=%v, want calc set and fixed/percent both zero", c.calc, c.fixed, c.percent)
	}
	if c.minCalc != "min(10, 20)" || c.minWidth != 0 || c.minPercent != 0 {
		t.Errorf("min-width: minCalc=%q minWidth=%d minPercent=%v, want minCalc set and the rest zero", c.minCalc, c.minWidth, c.minPercent)
	}
	if c.maxCalc != "clamp(5, 8, 30)" || c.maxWidth != 0 || c.maxPercent != 0 {
		t.Errorf("max-width: maxCalc=%q maxWidth=%d maxPercent=%v, want maxCalc set and the rest zero", c.maxCalc, c.maxWidth, c.maxPercent)
	}
	// content-box: the box-sizing delta is the cell's own padding (2),
	// added back so the later, unconditional padding subtraction nets back
	// out to exactly the declared content size. Same delta cellBoxSizingDelta
	// would hand a percent-based constraint via percentDelta.
	if c.calcDelta != 2 || c.minCalcDelta != 2 || c.maxCalcDelta != 2 {
		t.Errorf("calcDelta/minCalcDelta/maxCalcDelta = %d/%d/%d, want 2/2/2 (content-box padding)", c.calcDelta, c.minCalcDelta, c.maxCalcDelta)
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
