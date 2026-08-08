package render

import (
	"math"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
)

// resolveNumber resolves a plain CSS <number> value or a calc()/min()/
// max()/clamp() expression built from plain numbers: opacity, flex-grow,
// and flex-shrink all use this. There is no percentage support — none of
// them accept one today — so a "%" anywhere in a math function fails to
// resolve here, the same "not a length" rule resolveLength's basis-less
// callers (parsePaddingLen and friends) already use for the same reason:
// passing basisOK=false to Eval makes that the natural outcome, with no
// separate check needed.
//
// The non-calc fast path is cssengine.ParseNumber, real CSS <number>
// syntax only — not strconv.ParseFloat, which additionally accepts things
// that aren't valid CSS numbers at all: Go literal syntax like "0x1p2" and
// "1_000", and the non-finite "Inf"/"NaN". A NaN reaching an opacity
// multiplier is worse than a dropped declaration (see ParseNumber's own
// doc comment in cssengine); switching a caller to resolveNumber closes
// that gap for it, not just the callers that already used ParseNumber
// directly.
func resolveNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !cssengine.IsMathFunctionToken(s) {
		return cssengine.ParseNumber(s)
	}
	expr, ok := cssengine.ParseMathExpr(s)
	if !ok {
		return 0, false
	}
	return cssengine.Eval(expr, 0, false)
}

// resolveCSSMathInteger is resolveNumber's <integer>-context counterpart —
// z-index and tab-size — rounded per CSS's own rule for a calc() result
// used where an integer is expected: ties round toward positive infinity,
// not away from zero the way math.Round does, so calc(1.5) is 2 and
// calc(-1.5) is -1.
//
// ok is false whenever s isn't shaped like a math function at all,
// including a plain integer literal like "3" — callers fall through to
// their own existing plain-integer parse (strconv.Atoi or parseSizeVal)
// for that case, which this function deliberately never touches: a literal
// "z-index: 1.5" is not valid CSS <integer> syntax, and calc() support
// isn't a reason to start accepting it as a literal. Only a calc() result
// is allowed to land on a fraction before rounding.
func resolveCSSMathInteger(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if !cssengine.IsMathFunctionToken(s) {
		return 0, false
	}
	expr, ok := cssengine.ParseMathExpr(s)
	if !ok {
		return 0, false
	}
	v, ok := cssengine.Eval(expr, 0, false)
	if !ok {
		return 0, false
	}
	return int(math.Floor(v + 0.5)), true
}
