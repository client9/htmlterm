package htmlterm

import "testing"

func TestParsePaddingLen(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want int
	}{
		{"empty is zero", "", 0},
		{"bare integer", "3", 3},
		{"ch suffix", "4ch", 4},
		{"percent is ignored (not abs)", "50%", 0},
		{"zero is zero", "0", 0},
		{"garbage is zero", "auto", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePaddingLen(tc.v); got != tc.want {
				t.Errorf("parsePaddingLen(%q) = %d, want %d", tc.v, got, tc.want)
			}
		})
	}
}

func TestResolveMarginSide(t *testing.T) {
	tests := []struct {
		name       string
		v          string
		availWidth int
		wantVal    int
		wantAuto   bool
	}{
		{"auto keyword", "auto", 20, 0, true},
		{"auto with surrounding space", "  auto  ", 20, 0, true},
		{"absolute value", "3", 20, 3, false},
		{"percent resolves against availWidth", "50%", 20, 10, false},
		{"empty resolves to zero", "", 20, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotAuto := resolveMarginSide(tc.v, tc.availWidth)
			if gotVal != tc.wantVal || gotAuto != tc.wantAuto {
				t.Errorf("resolveMarginSide(%q, %d) = (%d, %v), want (%d, %v)",
					tc.v, tc.availWidth, gotVal, gotAuto, tc.wantVal, tc.wantAuto)
			}
		})
	}
}

func TestSplitAutoMargins(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		ml, mr    int
		mlAuto    bool
		mrAuto    bool
		wantML    int
		wantMR    int
	}{
		{"both auto splits evenly", 10, 0, 0, true, true, 5, 5},
		{"both auto with odd remainder favors right", 9, 0, 0, true, true, 4, 5},
		{"only left auto absorbs all remaining", 10, 0, 3, true, false, 10, 3},
		{"only right auto absorbs all remaining", 10, 4, 0, false, true, 4, 10},
		{"neither auto leaves values untouched", 10, 4, 3, false, false, 4, 3},
		{"negative remaining clamps to zero", -5, 0, 0, true, true, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotML, gotMR := splitAutoMargins(tc.remaining, tc.ml, tc.mr, tc.mlAuto, tc.mrAuto)
			if gotML != tc.wantML || gotMR != tc.wantMR {
				t.Errorf("splitAutoMargins(%d, %d, %d, %v, %v) = (%d, %d), want (%d, %d)",
					tc.remaining, tc.ml, tc.mr, tc.mlAuto, tc.mrAuto, gotML, gotMR, tc.wantML, tc.wantMR)
			}
		})
	}
}

func TestClampCellPadding(t *testing.T) {
	tests := []struct {
		name          string
		width, pl, pr int
		wantPL        int
		wantPR        int
		wantContentW  int
	}{
		{"padding fits, no clamp", 10, 2, 3, 2, 3, 5},
		{"nonpositive width clamps to nothing", 0, 2, 3, 0, 0, 0},
		{"padding exactly consumes width minus one", 5, 2, 2, 2, 2, 1},
		{"padding overflows, shrinks right first", 4, 2, 3, 2, 1, 1},
		{"padding overflows past right alone, then left", 5, 6, 1, 4, 0, 1},
		{"padding overflows entirely, both clamp to zero", 1, 10, 10, 0, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPL, gotPR, gotContentW := clampCellPadding(tc.width, tc.pl, tc.pr)
			if gotPL != tc.wantPL || gotPR != tc.wantPR || gotContentW != tc.wantContentW {
				t.Errorf("clampCellPadding(%d, %d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tc.width, tc.pl, tc.pr, gotPL, gotPR, gotContentW, tc.wantPL, tc.wantPR, tc.wantContentW)
			}
		})
	}
}
