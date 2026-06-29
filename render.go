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
		if text := normalizeWhiteSpace(n.Data, "normal"); strings.TrimSpace(text) != "" {
			sb.WriteString(text)
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				r.renderNode(sb, c)
			}
		case "head", "style", "script", "meta", "link", "noscript":
		case "table", "ol", "ul", "br", "hr":
			if r.resolveDecls(n)["display"] == "none" {
				return
			}
			switch n.Data {
			case "table":
				sb.WriteString(r.renderTable(n))
			case "ol":
				sb.WriteString(r.renderList(n, true, r.width))
			case "ul":
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
