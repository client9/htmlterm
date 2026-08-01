package render

import (
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
	return r.cascade().Resolve(n)
}

func (r *Engine) directDecls(n *html.Node) map[string]string {
	return r.cascade().Direct(n)
}

// pseudoElemDecls resolves n's ::which declarations, substituting any var()
// they contain against env, the custom-property subset of an
// already-resolved decls map the caller has on hand for n (see
// cssengine.Cascade.PseudoElement's doc comment for why this isn't resolved
// internally). Pass nil when the caller has no such map handy and doesn't
// otherwise need one; var() simply won't resolve in that pseudo-element's
// declarations.
func (r *Engine) pseudoElemDecls(n *html.Node, which string, env map[string]string) map[string]string {
	return r.cascade().PseudoElement(n, which, env)
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
