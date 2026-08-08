package render

import (
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
)

// resolveLength resolves a CSS <length-percentage> value against basis, in
// cells: a bare integer, its ch spelling, a percentage of basis (only when
// basisOK), or a calc()/min()/max()/clamp() expression combining any of
// those. ok is false when s isn't a recognized length at all, or contains a
// percentage while basisOK is false — the same "not a length" rule
// resolveCSSHeight already applied to a bare percentage against an
// indefinite containing block, now shared by every caller here instead of
// each reimplementing it. See docs/proposals/CSS_MATH.md's Design section
// for the per-caller table of what basis and basisOK each one passes.
//
// Does not floor, clamp to zero, or otherwise apply a property's own sign
// or zero policy. Every caller in this package decides that for itself on
// top of the raw result, the same way each already did for a plain
// percentage before this existed (see resolveCSSSize's "0 means unset" vs.
// resolveMarginSide's "0 is a real length, negative is fine" for the two
// existing policies this preserves).
//
// The non-calc path is unchanged from before this function existed — same
// parseSizeVal, same cost — guarded by a cheap prefix check
// (cssengine.IsMathFunctionToken) so a plain length, the overwhelming
// common case, never pays for a parse attempt against the math-function
// grammar.
func resolveLength(s string, basis int, basisOK bool) (int, bool) {
	s = strings.TrimSpace(s)
	if !cssengine.IsMathFunctionToken(s) {
		abs, pct, ok := parseSizeVal(s)
		if !ok {
			return 0, false
		}
		if pct > 0 {
			if !basisOK {
				return 0, false
			}
			return int(pct * float64(basis)), true
		}
		return abs, true
	}
	expr, ok := cssengine.ParseMathExpr(s)
	if !ok {
		return 0, false
	}
	v, ok := cssengine.Eval(expr, float64(basis), basisOK)
	if !ok {
		return 0, false
	}
	return int(v), true
}
