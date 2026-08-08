package render

import "testing"

func TestResolveLength(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		basis   int
		basisOK bool
		want    int
		wantOK  bool
	}{
		{name: "bare integer", s: "10", basis: 100, basisOK: true, want: 10, wantOK: true},
		{name: "ch spelling", s: "10ch", basis: 100, basisOK: true, want: 10, wantOK: true},
		{name: "percent", s: "50%", basis: 100, basisOK: true, want: 50, wantOK: true},
		{name: "literal zero is not a length", s: "0", basis: 100, basisOK: true, want: 0, wantOK: false},
		{name: "percent against indefinite basis is not a length", s: "50%", basis: 0, basisOK: false, want: 0, wantOK: false},
		{name: "unrecognized unit is not a length", s: "10px", basis: 100, basisOK: true, want: 0, wantOK: false},
		{name: "calc mixing percent and absolute", s: "calc(50% + 4)", basis: 10, basisOK: true, want: 9, wantOK: true},
		{name: "calc percent against indefinite basis fails whole expression", s: "calc(50% + 4)", basis: 0, basisOK: false, want: 0, wantOK: false},
		{name: "calc with no percent needs no basis", s: "calc(2 * 4ch)", basis: 0, basisOK: false, want: 8, wantOK: true},
		{name: "bare min", s: "min(50%, 20)", basis: 10, basisOK: true, want: 5, wantOK: true},
		{name: "calc producing zero is still ok", s: "calc(50% - 50%)", basis: 10, basisOK: true, want: 0, wantOK: true},
		{name: "calc producing a negative value is still ok, unclamped here", s: "calc(50% - 100)", basis: 10, basisOK: true, want: -95, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveLength(tt.s, tt.basis, tt.basisOK)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("resolveLength(%q, %d, %v) = (%d, %v), want (%d, %v)",
					tt.s, tt.basis, tt.basisOK, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestResolveCSSSizePercentStaysSetEvenWhenItResolvesToZero is a regression
// test: a first version of resolveCSSSize (built on resolveLength) rejected
// any resolved value <= 0 as "not set", which broke `width: 1%` on a
// 10-column container — a real, definite width of 0 columns, not the same
// as the declaration being absent. Caught by htmlterm_test.go's progress-bar
// case (a 1%-wide progress bar rendering at its full 20-column default
// instead of collapsing to 0), pinned here directly so it can't regress
// silently again. Only a literal zero length (which parseSizeVal itself
// never accepts) or an unrecognized value means "not set"; a percentage
// that legitimately rounds down to zero cells stays set.
func TestResolveCSSSizePercentStaysSetEvenWhenItResolvesToZero(t *testing.T) {
	v, ok := resolveCSSSize("1%", 10)
	if !ok || v != 0 {
		t.Errorf(`resolveCSSSize("1%%", 10) = (%d, %v), want (0, true)`, v, ok)
	}

	// A literal zero, by contrast, really is "not set".
	if _, ok := resolveCSSSize("0", 10); ok {
		t.Error(`resolveCSSSize("0", 10) ok = true, want false`)
	}

	// calc()'s own zero, and a calc()-derived negative clamped to zero,
	// both stay "set" for the same reason "1%" does: the declaration is a
	// real, valid length, just one that happens to resolve to (or below)
	// zero cells.
	if v, ok := resolveCSSSize("calc(50% - 50%)", 10); !ok || v != 0 {
		t.Errorf(`resolveCSSSize("calc(50%% - 50%%)", 10) = (%d, %v), want (0, true)`, v, ok)
	}
	if v, ok := resolveCSSSize("calc(50% - 1000)", 10); !ok || v != 0 {
		t.Errorf(`resolveCSSSize("calc(50%% - 1000)", 10) = (%d, %v), want (0, true) (clamped, not unset)`, v, ok)
	}
}

// TestResolveMarginSideNegativeIsNotClamped pins margin's "no clamp" sign
// policy (unlike padding, resolveVerticalMargin, and the width/height
// family, all of which clamp a calc()-derived negative to 0): real CSS
// allows a negative margin-left/margin-right, and this is the one place in
// the whole per-property table (docs/proposals/CSS_MATH.md's Design
// section) where that negative value is meant to survive unmodified. Tested
// directly, at this level, rather than through a full render: the visual
// effect several layers downstream (available-width accounting for the rest
// of the box) is real but not what this test is about pinning.
func TestResolveMarginSideNegativeIsNotClamped(t *testing.T) {
	v, isAuto := resolveMarginSide("calc(1 - 5)", 20)
	if isAuto || v != -4 {
		t.Errorf(`resolveMarginSide("calc(1 - 5)", 20) = (%d, %v), want (-4, false)`, v, isAuto)
	}
}

func TestResolveCSSHeightPercentStaysSetEvenWhenItResolvesToZero(t *testing.T) {
	v, ok := resolveCSSHeight("1%", 10)
	if !ok || v != 0 {
		t.Errorf(`resolveCSSHeight("1%%", 10) = (%d, %v), want (0, true)`, v, ok)
	}
	if _, ok := resolveCSSHeight("0", 10); ok {
		t.Error(`resolveCSSHeight("0", 10) ok = true, want false`)
	}
	// Indefinite containing-block height: a percentage is not a length at
	// all here, calc-wrapped or not (CSS2.1 10.5).
	if _, ok := resolveCSSHeight("50%", 0); ok {
		t.Error(`resolveCSSHeight("50%", 0) ok = true, want false`)
	}
	if _, ok := resolveCSSHeight("calc(50% + 4)", 0); ok {
		t.Error(`resolveCSSHeight("calc(50% + 4)", 0) ok = true, want false`)
	}
}
