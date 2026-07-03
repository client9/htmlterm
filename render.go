package htmlterm

import (
	"strings"

	"golang.org/x/net/html"
)

func isSkippedContentElement(name string) bool {
	switch name {
	case "style", "script", "meta", "link", "template":
		return true
	default:
		return false
	}
}

func (r *Renderer) renderTransparentNode(w *cappedWriter, n *html.Node) {
	inlineW := cappedWriter{maxBlanks: r.maxBlankLines}
	flushInline := func() {
		if inlineW.Len() == 0 {
			return
		}
		content := inlineW.String()
		if b, ok := w.LastByte(); strings.TrimSpace(content) != "" && (!ok || b == '\n' || b == ' ') {
			content = strings.TrimLeft(content, " ")
		}
		if r.width > 0 && !strings.Contains(content, "\n") {
			content = strings.Join(wordWrapANSI(content, r.width, ""), "\n")
		}
		w.WriteString(content)
		inlineW = cappedWriter{maxBlanks: r.maxBlankLines}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r.isRootInlineContent(c) {
			r.renderRootInlineNode(&inlineW, c)
			continue
		}
		flushInline()
		r.renderNode(w, c)
	}
	flushInline()
}

func (r *Renderer) renderRootInlineNode(w *cappedWriter, n *html.Node) {
	if n.Type != html.TextNode {
		r.renderNode(w, n)
		return
	}
	text := normalizeWhiteSpace(sanitizeTerminalText(n.Data, true), "normal", 8)
	if text == "" {
		return
	}
	b, ok := w.LastByte()
	if !ok || b == '\n' || b == ' ' {
		text = strings.TrimLeft(text, " ")
	}
	if text != "" {
		w.WriteString(text)
	}
}

func (r *Renderer) isRootInlineContent(n *html.Node) bool {
	switch n.Type {
	case html.TextNode:
		return true
	case html.ElementNode:
		if n.Data == "html" || n.Data == "body" {
			return false
		}
		if isSkippedContentElement(n.Data) || n.Data == "head" || n.Data == "wbr" || n.Data == "noscript" {
			return true
		}
		decls := r.resolveDecls(n)
		if decls["display"] == "none" {
			return true
		}
		switch n.Data {
		case "table", "ol", "ul", "menu", "br":
			return false
		}
		return decls["display"] != "block"
	default:
		return true
	}
}

// renderNode dispatches on node type. html.Parse wraps content in
// <html><head></head><body>...</body></html>, so those are transparent.
func (r *Renderer) renderNode(w *cappedWriter, n *html.Node) {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.renderNode(w, c)
		}
	case html.TextNode:
		if text := normalizeWhiteSpace(sanitizeTerminalText(n.Data, true), "normal", 8); strings.TrimSpace(text) != "" {
			b, ok := w.LastByte()
			if !ok || b == '\n' || b == ' ' {
				text = strings.TrimLeft(text, " ")
			}
			w.WriteString(text)
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body":
			r.renderTransparentNode(w, n)
		case "style", "script", "meta", "link", "template":
		case "head":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "noscript" {
					r.renderNode(w, c)
				}
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
					r.renderNode(w, c)
				case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode, html.RawNode:
					// nothing to render
				}
			}
			if raw.Len() > 0 {
				inner, _ := r.Render(raw.String())
				w.WriteString(inner)
			}
		case "table", "ol", "ul", "menu", "br":
			decls := r.resolveDecls(n)
			if decls["display"] == "none" {
				return
			}
			switch n.Data {
			case "table":
				w.WriteString(r.renderTable(n))
			case "ol", "ul", "menu":
				ordered := n.Data == "ol"
				if mt := parseMargin(decls["margin-top"]); mt > 0 && w.Len() > 0 {
					w.WriteAtLeastNewlines(mt + 1)
				}
				w.WriteString(r.renderList(n, ordered, r.width))
				w.WriteAtLeastNewlines(parseMargin(decls["margin-bottom"]) + 1)
			case "br":
				w.writeNewline()
			}
		default:
			r.renderDisplayNode(w, n)
		}
	case html.ErrorNode, html.CommentNode, html.DoctypeNode, html.RawNode:
		// nothing to render
	}
}
