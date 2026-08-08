package render

import "testing"

func TestResolveNumber(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		want   float64
		wantOK bool
	}{
		{"bare integer", "3", 3, true},
		{"decimal", "0.5", 0.5, true},
		{"calc arithmetic", "calc(1 - 0.5)", 0.5, true},
		{"bare min", "min(2, 3)", 2, true},
		{"bare max", "max(2, 3)", 3, true},
		{"clamp", "clamp(0, 2, 1)", 1, true},
		{"nested", "calc(min(2, 3) * 2)", 4, true},
		{"percent has no basis to resolve against, fails", "50%", 0, false},
		{"percent inside calc fails the same way", "calc(50% + 1)", 0, false},
		{"NaN is not real CSS number syntax", "NaN", 0, false},
		{"Inf is not real CSS number syntax", "Inf", 0, false},
		{"hex float is Go literal syntax, not CSS", "0x1p2", 0, false},
		{"empty", "", 0, false},
		{"unrecognized keyword", "auto", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveNumber(tt.s)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("resolveNumber(%q) = (%v, %v), want (%v, %v)", tt.s, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestResolveCSSMathInteger(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		want   int
		wantOK bool
	}{
		{"a plain literal is not this function's job", "3", 0, false},
		{"calc rounds down below the half point", "calc(1.4)", 1, true},
		{"calc rounds up at exactly the half point", "calc(1.5)", 2, true},
		{"a negative half point rounds toward positive infinity, not away from zero", "calc(-1.5)", -1, true},
		{"bare max with a fractional argument", "max(10, 20.7)", 21, true},
		{"a whole number stays whole", "calc(2 * 5)", 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveCSSMathInteger(tt.s)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("resolveCSSMathInteger(%q) = (%v, %v), want (%v, %v)", tt.s, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
