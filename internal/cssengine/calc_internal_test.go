package cssengine

import (
	"math"
	"testing"
)

// evalExpr is a test helper: parse s as a top-level math function and, on a
// successful parse, evaluate it against basis/basisOK in one call.
func evalExpr(t *testing.T, s string, basis float64, basisOK bool) (float64, bool) {
	t.Helper()
	e, ok := ParseMathExpr(s)
	if !ok {
		return 0, false
	}
	return Eval(e, basis, basisOK)
}

func TestEvalArithmeticAndPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		basis float64
		want  float64
	}{
		{"simple sum", "calc(1 + 2)", 0, 3},
		{"simple difference", "calc(5 - 2)", 0, 3},
		{"product binds tighter than sum", "calc(2 + 3 * 4)", 0, 14},
		{"parens override precedence", "calc((2 + 3) * 4)", 0, 20},
		{"division", "calc(9 / 3)", 0, 3},
		{"chained left-to-right same precedence", "calc(10 - 2 - 3)", 0, 5},
		{"decimal literals", "calc(2.5 + 1.5)", 0, 4},
		{"multiply by ch length", "calc(1.5 * 4ch)", 0, 6},
		{"ch and bare number are the same value", "calc(4ch + 2)", 0, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := evalExpr(t, tt.expr, tt.basis, true)
			if !ok {
				t.Fatalf("evalExpr(%q) failed to resolve, want %v", tt.expr, tt.want)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("evalExpr(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvalMixedPercentAndAbsolute(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		basis float64
		want  float64
	}{
		{"percent plus absolute", "calc(50% + 4)", 10, 9},
		{"absolute minus percent", "calc(100% - 4)", 10, 6},
		{"percent divided by number", "calc(100% / 3)", 9, 3},
		{"percent alone", "calc(50%)", 20, 10},
		{"literal 0% still resolves correctly with a real basis", "calc(0% + 5)", 10, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := evalExpr(t, tt.expr, tt.basis, true)
			if !ok {
				t.Fatalf("evalExpr(%q, basis=%v) failed to resolve, want %v", tt.expr, tt.basis, tt.want)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("evalExpr(%q, basis=%v) = %v, want %v", tt.expr, tt.basis, got, tt.want)
			}
		})
	}
}

func TestEvalMinMaxClamp(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		basis float64
		want  float64
	}{
		{"bare min, percent branch wins", "min(50%, 20)", 10, 5},
		{"bare max, percent branch loses", "max(10, 20%)", 10, 10},
		{"bare clamp picks the max bound", "clamp(10, 50%, 30)", 200, 30},
		{"bare clamp picks the min bound", "clamp(10, 50%, 30)", 2, 10},
		{"bare clamp picks the middle value", "clamp(10, 50%, 30)", 40, 20},
		{"min with more than two arguments", "min(30, 10, 20)", 0, 10},
		{"max with more than two arguments", "max(30, 10, 20)", 0, 30},
		{"nested min inside calc", "calc(min(50%, 40) + 2)", 10, 7},
		{"nested clamp inside min", "min(clamp(1, 2, 3), 4)", 0, 2},
		{"deep nesting", "clamp(min(1, 2), max(3, 4), 100)", 0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := evalExpr(t, tt.expr, tt.basis, true)
			if !ok {
				t.Fatalf("evalExpr(%q, basis=%v) failed to resolve, want %v", tt.expr, tt.basis, tt.want)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("evalExpr(%q, basis=%v) = %v, want %v", tt.expr, tt.basis, got, tt.want)
			}
		})
	}
}

func TestEvalSignedLiterals(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		basis float64
		want  float64
	}{
		{"negative leaf in min", "min(-4, 10)", 0, -4},
		{"plus then negative literal", "calc(4 + -2)", 0, 2},
		{"minus then negative literal", "calc(4 - -2)", 0, 6},
		{"leading negative percent", "calc(-50%)", 10, -5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := evalExpr(t, tt.expr, tt.basis, true)
			if !ok {
				t.Fatalf("evalExpr(%q) failed to resolve, want %v", tt.expr, tt.want)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("evalExpr(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvalIndefiniteBasisFailsWhole(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"bare percent", "calc(50%)"},
		{"percent combined with absolute", "calc(50% + 4)"},
		{"percent that would algebraically cancel", "calc(100% - 100% + 1)"},
		{"percent branch inside bare min", "min(50%, 20)"},
		{"percent branch inside bare max", "max(50%, 20)"},
		{"percent branch inside clamp", "clamp(10, 50%, 30)"},
		// Regression: a leaf's "is this a percentage" fact used to live
		// entirely in "pct != 0", indistinguishable from a plain 0. A
		// literal 0% is still syntactically a percentage and still has to
		// fail here, not silently resolve as if the "0%" weren't there.
		{"literal 0% is still a percentage", "calc(0% + 5)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := evalExpr(t, tt.expr, 0, false); ok {
				t.Errorf("evalExpr(%q, basisOK=false) succeeded, want failure (CSS2.1 10.5: a percentage against an indefinite basis is not a length, and the whole math function fails with it)", tt.expr)
			}
		})
	}
}

func TestEvalIndefiniteBasisNoPercentStillResolves(t *testing.T) {
	// An expression with no percentage anywhere in it never needs a basis
	// at all, indefinite or not.
	got, ok := evalExpr(t, "calc(2 * 4ch)", 0, false)
	if !ok || got != 8 {
		t.Errorf("evalExpr(calc(2 * 4ch), basisOK=false) = (%v, %v), want (8, true)", got, ok)
	}
}

func TestParseMathExprRejections(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"unbalanced open paren", "calc(1 + 2"},
		{"trailing garbage after call", "calc(1 + 2)x"},
		{"binary minus with no surrounding whitespace", "calc(4-2)"},
		{"binary minus missing space before", "calc(4 -2)"},
		{"binary minus missing space after", "calc(4- 2)"},
		{"unrecognized unit", "calc(4px + 2)"},
		{"vw unit not understood by this parser directly", "calc(2vw + 4)"},
		{"intrinsic keyword operand", "calc(auto + 4)"},
		{"clamp with two arguments", "clamp(1, 2)"},
		{"clamp with four arguments", "clamp(1, 2, 3, 4)"},
		{"min with no arguments", "min()"},
		{"empty calc", "calc()"},
		{"multiply two percentages", "calc(50% * 50%)"},
		{"multiply two percentages, one of them a literal zero", "calc(0% * 50%)"},
		{"divide by a percentage", "calc(4 / 50%)"},
		{"divide by a literal zero percentage", "calc(4 / 0%)"},
		{"not a math function at all", "50%"},
		{"unknown function name", "foo(1 + 2)"},
		{"min-content is not min()", "min-content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := ParseMathExpr(tt.expr); ok {
				t.Errorf("ParseMathExpr(%q) succeeded, want rejection", tt.expr)
			}
		})
	}
}

func TestEvalRejections(t *testing.T) {
	// These parse successfully (they're structurally valid) but fail at
	// Eval time, once the divisor's actual value is known.
	tests := []struct {
		name string
		expr string
	}{
		{"literal division by zero", "calc(4 / 0)"},
		{"division by an expression that evaluates to zero", "calc(4 / (2 - 2))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := ParseMathExpr(tt.expr)
			if !ok {
				t.Fatalf("ParseMathExpr(%q) failed to parse, want a structurally valid parse that fails at Eval", tt.expr)
			}
			if _, ok := Eval(e, 0, true); ok {
				t.Errorf("Eval(%q) succeeded, want failure (division by zero)", tt.expr)
			}
		})
	}
}

func TestParseMathExprCaseInsensitive(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		{"CALC(1 + 2)", 3},
		{"Calc(1 + 2)", 3},
		{"MIN(1, 2)", 1},
		{"Max(1, 2)", 2},
		{"CLAMP(1, 5, 10)", 5},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, ok := evalExpr(t, tt.expr, 0, true)
			if !ok || got != tt.want {
				t.Errorf("evalExpr(%q) = (%v, %v), want (%v, true)", tt.expr, got, ok, tt.want)
			}
		})
	}
}

func TestIsMathFunctionToken(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"calc(1 + 2)", true},
		{"CALC(1 + 2)", true},
		{"  calc(1 + 2)  ", true},
		{"min(1, 2)", true},
		{"max(1, 2)", true},
		{"clamp(1, 2, 3)", true},
		{"50%", false},
		{"40", false},
		{"40ch", false},
		{"auto", false},
		{"min-content", false},
		{"max-content", false},
		{"calc(1 + 2", false},
		{"calc(1+2)x", false},
		{"fit-content", false},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := IsMathFunctionToken(tt.s); got != tt.want {
				t.Errorf("IsMathFunctionToken(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestParseCommaArgsNestedCommasStayNested(t *testing.T) {
	// min(clamp(1, 2, 3), 4) has one top-level comma, not three: clamp's
	// own two inner commas must stay part of its own argument.
	got, ok := evalExpr(t, "min(clamp(1, 2, 3), 4)", 0, true)
	if !ok || got != 2 {
		t.Fatalf("evalExpr(min(clamp(1, 2, 3), 4)) = (%v, %v), want (2, true)", got, ok)
	}
}
