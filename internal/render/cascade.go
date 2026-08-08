package render

import (
	"maps"
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
	"golang.org/x/net/html"
)

func (r *Engine) cascade() cssengine.Cascade {
	return cssengine.Cascade{
		Rules:        r.rules,
		UARules:      r.uaRules,
		IgnoreInline: r.ignoreDocumentCSS,
		FocusAttr:    r.focusAttr,
		HoverAttr:    r.selectHighlightAttr,
		Cache:        r.directCache,
	}
}

func (r *Engine) resolveDecls(n *html.Node) map[string]string {
	return r.substituteViewportUnits(r.cascade().Resolve(n))
}

func (r *Engine) directDecls(n *html.Node) map[string]string {
	return r.substituteViewportUnits(r.cascade().Direct(n))
}

// substituteViewportUnits rewrites every vh/vw length in decls to the plain
// cell count it resolves to, so nothing downstream has to know the unit exists.
//
// This is the one length unit that can be converted here rather than at the
// point of use. A percentage needs its containing block, which differs per
// property and per box, but vh and vw name their own basis: `50vh` is half the
// viewport height whatever property it lands on, `width: 50vh` included. So
// the substitution is property-independent, and doing it once as declarations
// enter the render layer beats threading the viewport through all seventeen
// parseSizeVal call sites. Synthetic decls maps built later by maps.Clone
// inherit the already-substituted values for free.
//
// "Property-independent" has one carve-out: cssengine.IsAuthorTextProp
// (content, quotes, font-family, and the three counter properties) skips
// scanning entirely. convertViewportLengths' byte-level scan, needed so a
// length glued next to a paren inside calc() still converts (see that
// function's own doc comment), has no way to tell a real dimension token
// apart from author text that merely looks like one lexically — content's
// own mini-language can legitimately contain a numeral immediately
// followed by "vw"/"vh" with no length meaning at all, e.g. the "2vw" in
// `content: counter(x, 2vw)`. These are exactly the properties
// foldKeywordValue already exempts from a different kind of textual
// scanning, for the same underlying reason: their values are text to
// preserve, not CSS syntax to parse further.
//
// Copy-on-write, because Cascade.Direct returns the shared cached map for a
// node and must not be mutated. The common case is no viewport unit anywhere
// in the map, which returns decls itself and allocates nothing.
func (r *Engine) substituteViewportUnits(decls map[string]string) map[string]string {
	var out map[string]string
	for prop, val := range decls {
		if strings.HasPrefix(prop, "--") || cssengine.IsAuthorTextProp(prop) || !strings.ContainsAny(val, "vV") {
			continue
		}
		conv, changed := convertViewportLengths(val, r.width, r.height)
		if !changed {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(decls))
			maps.Copy(out, decls)
		}
		out[prop] = conv
	}
	if out == nil {
		return decls
	}
	return out
}

// convertViewportLengths rewrites each vh/vw length anywhere in val, leaving
// everything else alone. That includes a length glued next to a paren or
// operator inside a math function: `calc(2vw + 4)`'s `2vw` converts the same
// way a whitespace-delimited `10vw` in a plain shorthand does (`margin: 0
// 10vw`), because this scans for a number-token-immediately-followed-by-
// "vw"/"vh" anywhere in the string rather than only within whitespace-split
// tokens. cssengine's calc parser is deliberately not given its own vw/vh
// unit at all (see calc.go's own top-of-file comment); this is the one place
// that unit is ever resolved, calc()-nested or not, so calc()'s own grammar
// only needs to understand "ch" and "%", the same two units a plain,
// non-calc length already does.
//
// vpW and vpH are the viewport in columns and rows. A vh against a viewport
// with no height, which is Options.Height left at SizeAutomatic or SizeNatural,
// is left verbatim so that it falls through parseSizeVal (or, inside a math
// function, ParseMathExpr) as an unrecognized unit and the declaration is
// ignored. That matches what a percentage height does with no viewport to
// resolve against, and it is the one behavior a browser never shows, since a
// browser always has a viewport.
//
// Zero is exempt from that, and from the viewport entirely: `0vh` is `0` with
// no viewport at all, because a zero length is dimensionless in CSS. That is
// the same rule that makes `flex: 1 1 0px` work (see parseFlexSizeVal), applied
// to the two units this function owns.
//
// Fractions of a cell truncate rather than rounding, which is what every
// percentage in this engine already does, so `33vh` of 24 rows is 7 and not 8.
func convertViewportLengths(val string, vpW, vpH int) (string, bool) {
	type span struct {
		start, end int
		repl       string
	}
	var spans []span
	i := 0
	for i < len(val) {
		if val[i] == '"' || val[i] == '\'' {
			// A quoted string (content: "3vw wide", a custom border glyph,
			// a quoted list-style-type, ...) is opaque literal text, not a
			// place to go looking for a vw/vh length. Skip over it whole,
			// the same protection consumeCSSQuotedToken's other callers
			// already give a value's quoted spans elsewhere in this
			// engine; scanning into one here would rewrite a literal "3vw"
			// substring into a resolved cell count.
			i = cssengine.ConsumeQuotedToken(val, i)
			continue
		}
		if i > 0 && isIdentContinuationByte(val[i-1]) {
			// The digit run starting here is glued onto whatever
			// identifier or word ends at val[i-1] — the "10" in
			// "logo10vh.png" or the "5" in "Georgia5vw" — not a
			// standalone dimension token starting fresh at this position.
			// A real CSS <dimension-token> can't begin mid-identifier;
			// skip this one byte so the loop resumes scanning right after
			// it, walking the rest of the identifier one byte at a time
			// without ever proposing a match inside it.
			i++
			continue
		}
		numEnd := cssengine.ScanNumberToken(val, i)
		if numEnd == i {
			i++
			continue
		}
		var basis int
		unitEnd := numEnd
		switch {
		case strings.HasPrefix(val[numEnd:], "vh"):
			basis, unitEnd = vpH, numEnd+2
		case strings.HasPrefix(val[numEnd:], "vw"):
			basis, unitEnd = vpW, numEnd+2
		default:
			i = numEnd
			continue
		}
		if unitEnd < len(val) && isIdentContinuationByte(val[unitEnd]) {
			// Something like "10vwx" isn't a real vw dimension. Leave it
			// alone; resume scanning right after the number so "vwx" is
			// walked one byte at a time next, matching (never finding a
			// number to anchor on).
			i = numEnd
			continue
		}
		f, err := strconv.ParseFloat(val[i:numEnd], 64)
		if err != nil {
			i = unitEnd
			continue
		}
		switch {
		case f == 0:
			spans = append(spans, span{i, unitEnd, "0"})
		case basis > 0:
			spans = append(spans, span{i, unitEnd, strconv.Itoa(int(f / 100 * float64(basis)))})
		}
		i = unitEnd
	}
	if len(spans) == 0 {
		return val, false
	}
	var b strings.Builder
	prev := 0
	for _, sp := range spans {
		b.WriteString(val[prev:sp.start])
		b.WriteString(sp.repl)
		prev = sp.end
	}
	b.WriteString(val[prev:])
	return b.String(), true
}

// isIdentContinuationByte reports whether c could continue a CSS identifier
// (a unit name, in convertViewportLengths' case). Used only to reject a
// "vw"/"vh" match that's actually a prefix of a longer, unrecognized
// identifier, e.g. the "vw" inside "10vwx".
func isIdentContinuationByte(c byte) bool {
	return c == '-' || c == '_' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// pseudoElemDecls resolves n's ::which declarations, substituting any var()
// they contain against env, the custom-property subset of an
// already-resolved decls map the caller has on hand for n (see
// cssengine.Cascade.PseudoElement's doc comment for why this isn't resolved
// internally). Pass nil when the caller has no such map handy and doesn't
// otherwise need one; var() simply won't resolve in that pseudo-element's
// declarations.
func (r *Engine) pseudoElemDecls(n *html.Node, which string, env map[string]string) map[string]string {
	return r.substituteViewportUnits(r.cascade().PseudoElement(n, which, env))
}

// customPropSubset returns just the "--*" entries of decls: the
// custom-property environment pseudoElemDecls needs for var() resolution
// (see its own doc comment for why the caller supplies this instead of
// pseudoElemDecls deriving it internally).
func customPropSubset(decls map[string]string) map[string]string {
	sub := make(map[string]string, len(decls))
	for k, v := range decls {
		if strings.HasPrefix(k, "--") {
			sub[k] = v
		}
	}
	return sub
}
