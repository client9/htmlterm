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
// Copy-on-write, because Cascade.Direct returns the shared cached map for a
// node and must not be mutated. The common case is no viewport unit anywhere
// in the map, which returns decls itself and allocates nothing.
func (r *Engine) substituteViewportUnits(decls map[string]string) map[string]string {
	var out map[string]string
	for prop, val := range decls {
		if strings.HasPrefix(prop, "--") || !strings.ContainsAny(val, "vV") {
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

// convertViewportLengths rewrites each whitespace-separated vh/vw token in val,
// leaving every other token alone. Shorthands are covered by that: `margin: 0
// 10vw` converts its second component and keeps its first.
//
// vpW and vpH are the viewport in columns and rows. A vh against a viewport
// with no height, which is Options.Height left at SizeAutomatic or SizeNatural,
// is left verbatim so that it falls through parseSizeVal as an unrecognized
// unit and the declaration is ignored. That matches what a percentage height
// does with no viewport to resolve against, and it is the one behavior a
// browser never shows, since a browser always has a viewport.
//
// Zero is exempt from that, and from the viewport entirely: `0vh` is `0` with
// no viewport at all, because a zero length is dimensionless in CSS. That is
// the same rule that makes `flex: 1 1 0px` work (see parseFlexSizeVal), applied
// to the two units this function owns.
//
// Fractions of a cell truncate rather than rounding, which is what every
// percentage in this engine already does, so `33vh` of 24 rows is 7 and not 8.
func convertViewportLengths(val string, vpW, vpH int) (string, bool) {
	fields := strings.Fields(val)
	changed := false
	for i, tok := range fields {
		var basis int
		switch {
		case strings.HasSuffix(tok, "vh"):
			basis = vpH
		case strings.HasSuffix(tok, "vw"):
			basis = vpW
		default:
			continue
		}
		f, err := strconv.ParseFloat(tok[:len(tok)-2], 64)
		if err != nil {
			continue
		}
		if f == 0 {
			fields[i], changed = "0", true
			continue
		}
		if basis <= 0 {
			continue
		}
		fields[i], changed = strconv.Itoa(int(f/100*float64(basis))), true
	}
	if !changed {
		return val, false
	}
	return strings.Join(fields, " "), true
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
