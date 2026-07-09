package render

import (
	"github.com/client9/htmlterm/internal/cssengine"
	"golang.org/x/net/html"
)

func (r *Engine) cascade() cssengine.Cascade {
	return cssengine.Cascade{
		Rules:        r.rules,
		IgnoreInline: r.ignoreDocumentCSS,
		FocusAttr:    r.focusAttr,
	}
}

func (r *Engine) resolveDecls(n *html.Node) map[string]string {
	return r.cascade().Resolve(n)
}

func (r *Engine) directDecls(n *html.Node) map[string]string {
	return r.cascade().Direct(n)
}

func (r *Engine) pseudoElemDecls(n *html.Node, which string) map[string]string {
	return r.cascade().PseudoElement(n, which)
}
