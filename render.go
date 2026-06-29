package htmlterm

import (
	"strings"

	"golang.org/x/net/html"
)

// renderNode dispatches on node type. html.Parse wraps content in
// <html><head></head><body>...</body></html>, so those are transparent.
func (r *Renderer) renderNode(sb *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.renderNode(sb, c)
		}
	case html.TextNode:
		if text := normalizeWhiteSpace(n.Data, "normal", 8); strings.TrimSpace(text) != "" {
			sb.WriteString(text)
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				r.renderNode(sb, c)
			}
		case "style", "script", "meta", "link":
		case "head":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "noscript" {
					r.renderNode(sb, c)
				}
			}
		case "img":
			alt := nodeAttr(n, "alt")
			if alt != "" {
				sb.WriteString("[" + alt + "]")
			}
		case "wbr":
			// word-break opportunity — no terminal equivalent; emit nothing
		case "noscript":
			// x/net/html parses noscript with scripting enabled, so content
			// arrives as a raw text node. Re-parse and render it as HTML so
			// noscript content always displays in a terminal (no JS anyway).
			var raw strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				switch c.Type {
				case html.TextNode:
					raw.WriteString(c.Data)
				case html.ElementNode:
					r.renderNode(sb, c)
				case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode, html.RawNode:
					// nothing to render
				}
			}
			if raw.Len() > 0 {
				inner, _ := r.Render(raw.String())
				sb.WriteString(inner)
			}
		case "table", "ol", "ul", "menu", "br", "hr":
			if r.resolveDecls(n)["display"] == "none" {
				return
			}
			switch n.Data {
			case "table":
				sb.WriteString(r.renderTable(n))
			case "ol":
				sb.WriteString(r.renderList(n, true, r.width))
			case "ul", "menu":
				sb.WriteString(r.renderList(n, false, r.width))
			case "br":
				sb.WriteByte('\n')
			case "hr":
				sb.WriteString(strings.Repeat("─", r.width))
				sb.WriteByte('\n')
			}
		default:
			r.renderDisplayNode(sb, n)
		}
	case html.ErrorNode, html.CommentNode, html.DoctypeNode, html.RawNode:
		// nothing to render
	}
}
