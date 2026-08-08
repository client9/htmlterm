package cssengine

import "strings"

// This file parses and evaluates CSS math functions: calc(), min(), max(),
// and clamp(). Design record and rationale live in
// docs/proposals/CSS_MATH.md; this comment covers only what a reader needs
// to work on the code itself.
//
// Grammar accepted inside a math function's arguments, restricted to this
// engine's own unit vocabulary (CSS.md's Size Values: a bare number, its
// "ch" spelling, and "%"):
//
//	sum     = product (('+' | '-') product)*
//	product = atom (('*' | '/') atom)*
//	atom    = number unit?  |  '(' sum ')'  |  mathFunc '(' args ')'
//	unit    = "ch" | "%"
//
// A '+' or '-' used as a binary operator requires whitespace on both
// sides, matching real CSS's own calc() grammar; '*' and '/' don't. Both
// rules are enforced by mathParser.parseSum and parseProduct below, not by
// a separate tokenizing pass.
//
// vw/vh are deliberately not part of this grammar. They're resolved by
// internal/render's convertViewportLengths before a value ever reaches
// this parser (see CSS_MATH.md's cascade.go section), so by the time
// ParseMathExpr runs, "50vw" has already become a plain cell count. A
// vw/vh unit reaching this parser directly is treated the same as any
// other unrecognized unit: the whole expression fails to parse.

// Expr is a parsed calc()/min()/max()/clamp() expression tree. The zero
// value is a leaf equal to the constant 0; every real value comes from
// ParseMathExpr, never constructed directly by a caller outside this file.
type Expr struct {
	// Leaf value. Meaningful only when op is opLeafAbs or opLeafPct, and
	// never both at once: a single number/ch/percentage token in the
	// source text is exactly one of "N cells" (abs) or "N percent" (pct, a
	// 0.0-1.0 fraction), the same either-or distinction parseSizeVal's own
	// two-unit vocabulary already makes for a plain, non-calc length. It's
	// only a whole *expression* (an interior node combining leaves) that
	// can carry both a basis-independent part and a basis-dependent one at
	// once, which is the entire reason calc() needs this tree instead of a
	// single (abs, pct) pair.
	//
	// Which of the two a leaf is lives in op, not in "pct != 0": a literal
	// "0%" is syntactically a percentage — it still triggers the
	// percentage-against-an-indefinite-basis rule in Eval and the "*"/"/"
	// type restriction in hasPercent, the same as any other percentage —
	// even though its value is numerically indistinguishable from an
	// absent one. calc(0% + 5) against an indefinite basis has to fail the
	// same way calc(50% + 5) does; inferring "is this a percentage" from
	// the number alone would silently let it through instead.
	abs, pct float64

	// Interior node. op is opLeafAbs or opLeafPct for a leaf (args nil);
	// any other value combines args.
	op   mathOp
	args []Expr
}

type mathOp int

const (
	opLeafAbs mathOp = iota // leaf: absolute cells (a bare number or "ch")
	opLeafPct               // leaf: percentage; pct is meaningful even when it's exactly 0
	opAdd
	opSub
	opMul
	opDiv
	opMin
	opMax
	opClamp
)

// mathFuncNames is the closed set of function names this package
// recognizes as a CSS math function.
var mathFuncNames = [...]string{"calc", "min", "max", "clamp"}

// matchMathFuncName reports whether s begins with one of calc(), min(),
// max(), or clamp() (case-insensitive), returning the matched name in its
// canonical lowercase spelling. It only checks the opening "name("; a
// caller that needs the whole call well-formed still has to find the
// matching close paren itself, via findMatchingParen.
func matchMathFuncName(s string) (name string, ok bool) {
	for _, n := range mathFuncNames {
		if len(s) > len(n) && s[len(n)] == '(' && strings.EqualFold(s[:len(n)], n) {
			return n, true
		}
	}
	return "", false
}

// IsMathFunctionToken reports whether s (once trimmed) is shaped like a
// calc(), min(), max(), or clamp() call: an accepted name immediately
// followed by "(", with the trimmed string ending in ")". It does not
// validate the arguments; that's ParseMathExpr's job. Two callers rely on
// this being a cheap shape check rather than a real parse: isCSSFlexBasisToken
// (css.go), classifying a flex shorthand token without fully parsing it, and
// internal/render's resolveLength, which uses it as a fast-path guard so a
// plain length with no math function in it never pays for a parse attempt.
func IsMathFunctionToken(s string) bool {
	s = strings.TrimSpace(s)
	_, ok := matchMathFuncName(s)
	return ok && strings.HasSuffix(s, ")")
}

// ParseMathExpr parses s as a top-level calc(), min(), max(), or clamp()
// call. ok is false for anything else, including a plain "50%" — that's
// deliberately outside this function's job; callers keep using their own
// existing fast path for a plain length and only reach for this on the
// math-function path IsMathFunctionToken flags.
func ParseMathExpr(s string) (Expr, bool) {
	s = strings.TrimSpace(s)
	name, ok := matchMathFuncName(s)
	if !ok {
		return Expr{}, false
	}
	argsStart := len(name) + 1 // past the name and its opening "("
	end := findMatchingParen(s, argsStart)
	if end < 0 || end != len(s)-1 {
		// end != len(s)-1 covers both a genuinely unbalanced call and
		// trailing garbage after an otherwise well-formed one, e.g.
		// "calc(1 + 2)extra".
		return Expr{}, false
	}
	return parseMathBody(name, s[argsStart:end])
}

// parseMathBody builds the Expr for a math function already identified as
// name (one of "calc"/"min"/"max"/"clamp", case-insensitive), given the raw
// text between its own outer parens. Shared by ParseMathExpr, the top-level
// entry point, and mathParser.parseAtom's nested-call case, so a nested
// call like the min(...) inside calc(min(50%, 40) + 2) is built exactly the
// same way a top-level one is.
func parseMathBody(name, inner string) (Expr, bool) {
	switch strings.ToLower(name) {
	case "calc":
		p := &mathParser{s: inner}
		e, ok := p.parseSum()
		if !ok || !p.atEnd() {
			return Expr{}, false
		}
		return e, true
	case "min", "max":
		args, ok := parseCommaArgs(inner)
		if !ok || len(args) == 0 {
			return Expr{}, false
		}
		op := opMin
		if strings.EqualFold(name, "max") {
			op = opMax
		}
		return Expr{op: op, args: args}, true
	case "clamp":
		// clamp(<min>, <val>, <max>). Real CSS 4 also allows "none" for an
		// open-ended min or max; this pass requires exactly three
		// comma-separated expressions (see CSS_MATH.md's non-goals).
		args, ok := parseCommaArgs(inner)
		if !ok || len(args) != 3 {
			return Expr{}, false
		}
		return Expr{op: opClamp, args: args}, true
	}
	return Expr{}, false // unreachable: matchMathFuncName only matches these four names
}

// splitTopLevelCommas splits s on commas that aren't nested inside parens,
// so a nested call's own commas don't split the outer argument list —
// min(clamp(1, 2, 3), 4) has one top-level comma, not three.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// parseCommaArgs parses inner as one or more comma-separated calc-sum
// arguments, the shape min(), max(), and clamp() all share. Each argument
// gets the same grammar as a whole calc()'s own contents, including
// further nested math functions.
func parseCommaArgs(inner string) ([]Expr, bool) {
	parts := splitTopLevelCommas(inner)
	args := make([]Expr, 0, len(parts))
	for _, part := range parts {
		p := &mathParser{s: part}
		e, ok := p.parseSum()
		if !ok || !p.atEnd() {
			return nil, false
		}
		args = append(args, e)
	}
	return args, true
}

// mathParser is a small recursive-descent parser over one calc-sum: the
// text inside a single calc() call, or a single argument of min()/max()/
// clamp(). See this file's own top-of-file comment for the grammar.
type mathParser struct {
	s   string
	pos int
}

// skipWS advances past whitespace at the current position and reports
// whether it moved at all. The bool result is what lets parseSum tell a
// real binary "+"/"-" (whitespace before and after) apart from a sign
// glued directly onto the next number, which parseNumberLeaf handles
// instead.
func (p *mathParser) skipWS() bool {
	start := p.pos
	for p.pos < len(p.s) && isCSSWhitespace(p.s[p.pos]) {
		p.pos++
	}
	return p.pos > start
}

// atEnd reports whether only trailing whitespace remains. Every entry
// point (ParseMathExpr's calc() case, each argument in parseCommaArgs)
// calls this after parsing to reject leftover, unparsed text rather than
// silently ignoring it.
func (p *mathParser) atEnd() bool {
	p.skipWS()
	return p.pos >= len(p.s)
}

// parseSum parses `product (('+' | '-') product)*`. A '+' or '-' is only
// consumed as a binary operator when it has whitespace immediately before
// and immediately after it; otherwise parsing stops there and leaves it
// unconsumed; for a "-" glued onto a following digit, parseAtom's own leaf
// parser is what picks it up as that operand's own sign. This is what
// makes "4 + -2" parse as 4 + (-2), while "4-2" and "4- 2" and "4 -2" are
// all left with unconsumed input and so fail overall, matching CSS's own
// mandatory-whitespace rule for calc()'s "+"/"-".
func (p *mathParser) parseSum() (Expr, bool) {
	left, ok := p.parseProduct()
	if !ok {
		return Expr{}, false
	}
	for {
		save := p.pos
		spaceBefore := p.skipWS()
		if p.pos >= len(p.s) {
			p.pos = save
			break
		}
		c := p.s[p.pos]
		if (c != '+' && c != '-') || !spaceBefore {
			p.pos = save
			break
		}
		opPos := p.pos
		p.pos++
		if p.pos >= len(p.s) || !isCSSWhitespace(p.s[p.pos]) {
			p.pos = opPos
			break
		}
		right, ok := p.parseProduct()
		if !ok {
			return Expr{}, false
		}
		op := opAdd
		if c == '-' {
			op = opSub
		}
		left = Expr{op: op, args: []Expr{left, right}}
	}
	return left, true
}

// parseProduct parses `atom (('*' | '/') atom)*`. Unlike parseSum, CSS
// doesn't require whitespace around '*' or '/' (neither character can
// start a number token, so there's no ambiguity to guard against), so
// whitespace here is accepted but optional.
func (p *mathParser) parseProduct() (Expr, bool) {
	left, ok := p.parseAtom()
	if !ok {
		return Expr{}, false
	}
	for {
		// save/restore around the lookahead: parseSum's own mandatory-
		// whitespace check for a binary '+'/'-' needs to see that
		// whitespace intact when this loop finds no '*'/'/' here and
		// returns left back up to it. Without restoring, this loop would
		// silently consume the space before a following '+'/'-' while
		// only peeking for a product operator, and parseSum would then
		// see spaceBefore=false and wrongly reject a well-formed "a + b".
		save := p.pos
		p.skipWS()
		if p.pos >= len(p.s) {
			p.pos = save
			break
		}
		c := p.s[p.pos]
		if c != '*' && c != '/' {
			p.pos = save
			break
		}
		p.pos++
		p.skipWS()
		right, ok := p.parseAtom()
		if !ok {
			return Expr{}, false
		}
		if c == '*' {
			// Real CSS also forbids <length> * <length>; this engine
			// accepts it (see CSS_MATH.md's documented deviation) since a
			// bare number and its "ch" spelling are already the same value
			// everywhere else here. Only the percentage restriction is
			// enforced: CSS never allows two percentage-carrying operands
			// multiplied together, regardless of what basis might later be
			// available.
			if hasPercent(left) && hasPercent(right) {
				return Expr{}, false
			}
			left = Expr{op: opMul, args: []Expr{left, right}}
		} else {
			// Dividing by a percentage is invalid regardless of value: its
			// meaning depends on a basis that a bare division has no way
			// to name. Dividing by an absolute zero is also invalid;
			// that's caught later, at Eval time, once the divisor's actual
			// value is known rather than just its shape.
			if hasPercent(right) {
				return Expr{}, false
			}
			left = Expr{op: opDiv, args: []Expr{left, right}}
		}
	}
	return left, true
}

// parseAtom parses one `atom`: a parenthesized sum, a nested math function
// call, or a number leaf.
func (p *mathParser) parseAtom() (Expr, bool) {
	p.skipWS()
	if p.pos >= len(p.s) {
		return Expr{}, false
	}
	if p.s[p.pos] == '(' {
		end := findMatchingParen(p.s, p.pos+1)
		if end < 0 {
			return Expr{}, false
		}
		inner := &mathParser{s: p.s[p.pos+1 : end]}
		e, ok := inner.parseSum()
		if !ok || !inner.atEnd() {
			return Expr{}, false
		}
		p.pos = end + 1
		return e, true
	}
	if name, ok := matchMathFuncName(p.s[p.pos:]); ok {
		argsStart := p.pos + len(name) + 1
		end := findMatchingParen(p.s, argsStart)
		if end < 0 {
			return Expr{}, false
		}
		e, ok := parseMathBody(name, p.s[argsStart:end])
		if !ok {
			return Expr{}, false
		}
		p.pos = end + 1
		return e, true
	}
	return p.parseNumberLeaf()
}

// parseNumberLeaf parses a signed number, optionally immediately followed
// by "ch" or "%" with no space in between, matching CSS's own
// <dimension>/<percentage> token grammar, which never allows whitespace
// between a number and its unit. A bare number and its "ch" spelling are
// the same value here, per CSS.md's Size Values, so both land in abs. Any
// other unit glued directly onto the number — px, em, vw, vh, or anything
// else — fails the whole leaf, the same way an unrecognized unit fails a
// plain, non-calc length outright rather than being silently dropped.
func (p *mathParser) parseNumberLeaf() (Expr, bool) {
	end := ScanNumberToken(p.s, p.pos)
	if end == p.pos {
		return Expr{}, false
	}
	f, ok := ParseNumber(p.s[p.pos:end])
	if !ok {
		return Expr{}, false
	}
	rest := p.s[end:]
	switch {
	case strings.HasPrefix(rest, "%"):
		p.pos = end + 1
		return Expr{op: opLeafPct, pct: f / 100}, true
	case len(rest) >= 2 && strings.EqualFold(rest[:2], "ch") && (len(rest) == 2 || !isIdentChar(rest[2])):
		p.pos = end + 2
		return Expr{op: opLeafAbs, abs: f}, true
	default:
		if len(rest) > 0 && isASCIILetter(rest[0]) {
			return Expr{}, false
		}
		p.pos = end
		return Expr{op: opLeafAbs, abs: f}, true
	}
}

// ScanNumberToken returns the end index of the longest prefix of s[i:]
// that looks like a CSS <number> token: an optional sign, digits, an
// optional fractional part, and an optional exponent. It doesn't validate
// the result — "-" alone "matches" as a bare sign with no digits — a
// caller that needs the real validation hands the substring to ParseNumber,
// which does that and rejects anything that isn't a well-formed number.
//
// Exported for internal/render's convertViewportLengths (cascade.go),
// which needs the same "where does a number token end" boundary to find a
// vw/vh length glued next to a paren or operator inside a math function,
// e.g. the "2vw" in calc(2vw + 4). See that function's own doc comment.
func ScanNumberToken(s string, i int) int {
	j := i
	if j < len(s) && (s[j] == '+' || s[j] == '-') {
		j++
	}
	for j < len(s) && isASCIIDigitByte(s[j]) {
		j++
	}
	if j < len(s) && s[j] == '.' {
		k := j + 1
		for k < len(s) && isASCIIDigitByte(s[k]) {
			k++
		}
		if k > j+1 {
			j = k
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		digitsStart := k
		for k < len(s) && isASCIIDigitByte(s[k]) {
			k++
		}
		if k > digitsStart {
			j = k
		}
	}
	return j
}

func isASCIIDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// hasPercent reports whether e is, or contains anywhere in its tree, a
// percentage leaf — including a literal "0%", which is still a percentage
// syntactically (see Expr's own doc comment on why this checks op rather
// than "pct != 0"). Used only for calc()'s parse-time type restriction on
// '*' and '/' (CSS forbids multiplying two percentage-carrying operands
// together, or dividing by one at all, regardless of whether a basis will
// ever be available to resolve it). Eval's own basis resolution below
// doesn't need this: a percentage leaf that can't resolve fails on its
// own, and that failure propagates through every ancestor node during
// evaluation without a separate pre-scan.
func hasPercent(e Expr) bool {
	switch e.op {
	case opLeafPct:
		return true
	case opLeafAbs:
		return false
	}
	for _, a := range e.args {
		if hasPercent(a) {
			return true
		}
	}
	return false
}

// Eval resolves e to a single number of cells, given basis (the containing
// block's size on the relevant axis, or whatever else the calling
// property's own resolver uses as its basis) and basisOK (whether that
// basis is itself definite). The result is not floored, clamped to zero,
// or rounded; every caller in internal/render applies its own property's
// sign and zero policy on top of the raw number, the same way it already
// does for a plain percentage today.
//
// A percentage leaf fails to resolve when basisOK is false, and that
// failure propagates up through every enclosing node, so the whole
// expression fails rather than falling back to whichever branch didn't
// need a basis. This matches CSS2.1 §10.5's own worked example for a plain
// calc() sum — `calc(100% - 100% + 1em)` computes to `calc(0% + 1em)`, not
// `calc(1em)`, and because it still contains a percentage relative to an
// indefinite containing-block size, the whole declaration resolves as
// auto, not 1em. calc() doesn't pre-simplify a percentage away before
// deciding whether a value depends on the basis, and neither does this.
// min()/max()/clamp() get no separate rule: CSS Values 4 §10 defines them
// as sugar over the same resolution algorithm calc() uses, so
// `max-height: min(50%, 20)` on an auto-height container is unconstrained,
// not 20 — the absolute-length branch does not rescue the value.
//
// Division by zero also fails here rather than at parse time, since the
// divisor may be an expression (e.g. `calc(4 / (2 - 2))`) whose value
// isn't known until basis is supplied.
func Eval(e Expr, basis float64, basisOK bool) (float64, bool) {
	switch e.op {
	case opLeafAbs:
		return e.abs, true
	case opLeafPct:
		// basisOK gates this leaf regardless of pct's value: a literal
		// "0%" is still a percentage (see Expr's own doc comment) and
		// still fails against an indefinite basis, the same as any other
		// percentage.
		if !basisOK {
			return 0, false
		}
		return e.pct * basis, true
	}
	vals := make([]float64, len(e.args))
	for i, a := range e.args {
		v, ok := Eval(a, basis, basisOK)
		if !ok {
			return 0, false
		}
		vals[i] = v
	}
	switch e.op {
	case opAdd:
		return vals[0] + vals[1], true
	case opSub:
		return vals[0] - vals[1], true
	case opMul:
		return vals[0] * vals[1], true
	case opDiv:
		if vals[1] == 0 {
			return 0, false
		}
		return vals[0] / vals[1], true
	case opMin:
		m := vals[0]
		for _, v := range vals[1:] {
			m = min(m, v)
		}
		return m, true
	case opMax:
		m := vals[0]
		for _, v := range vals[1:] {
			m = max(m, v)
		}
		return m, true
	case opClamp:
		return max(vals[0], min(vals[1], vals[2])), true
	default:
		return 0, false // unreachable: every non-leaf Expr this package builds sets a known op
	}
}
