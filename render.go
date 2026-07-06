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

func isTableLayoutDisplay(display string) bool {
	return display == "" || display == "table"
}

// renderRootTokens builds the token stream for a whole document: doc's
// children (typically one <html> element) via renderRootNodeTokens, the
// token-based equivalent of the old cappedWriter-based renderNode walk.
func (r *Renderer) renderRootTokens(doc *html.Node) []wrapToken {
	var tokens []wrapToken
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		tokens = r.renderRootNodeTokens(tokens, c)
	}
	return tokens
}

// renderRootNodeTokens is the token-based equivalent of the old
// renderNode/renderTransparentNode/renderRootInlineNode trio: html/body are
// transparent (their children compose directly into the root token stream,
// with no box of their own); table/ol/ul/menu/br get their historical
// top-level handling; noscript re-parses its raw text content (the x/net/html
// parser hands noscript's contents over as a single raw text node when
// scripting is enabled, which it always is here, since there's no JS engine
// in a terminal); everything else dispatches through renderRootDisplayTokens.
//
// Root-level text nodes have always used a fixed white-space:normal,
// text-transform:none, tab-size:8 context — never derived from html/body's
// own (rarely set) CSS — preserved here exactly, not something this
// migration changes.
func (r *Renderer) renderRootNodeTokens(tokens []wrapToken, n *html.Node) []wrapToken {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			tokens = r.renderRootNodeTokens(tokens, c)
		}
	case html.TextNode:
		// text == "" (not strings.TrimSpace(text) != "") deliberately: a
		// standalone space between two root-level inline siblings (e.g.
		// "<span>a</span> <b>b</b>") is meaningful content to preserve, not
		// a structural artifact to discard — it's the only thing separating
		// them once whitespace-only text nodes reach this point.
		if text := normalizeWhiteSpace(sanitizeTerminalText(n.Data, true), "normal", 8); text != "" {
			if lr, ok := lastRune(tokens); !ok || lr == '\n' || lr == ' ' {
				text = strings.TrimLeft(text, " ")
			}
			if text != "" {
				tokens = append(tokens, wrapToken{text: text})
			}
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				tokens = r.renderRootNodeTokens(tokens, c)
			}
		case "style", "script", "meta", "link", "template":
		case "head":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "noscript" {
					tokens = r.renderRootNodeTokens(tokens, c)
				}
			}
		case "wbr":
			// word-break opportunity — no terminal equivalent; emit nothing
		case "noscript":
			var raw strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				switch c.Type {
				case html.TextNode:
					raw.WriteString(c.Data)
				case html.ElementNode:
					tokens = r.renderRootNodeTokens(tokens, c)
				case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode, html.RawNode:
					// nothing to render
				}
			}
			if raw.Len() > 0 {
				// inner is a full Render() output, not a box.join()'d
				// value — it has its own complete trailing-newline
				// semantics already baked in (0, 1, or more, depending on
				// its own content), so it's embedded verbatim, not trimmed.
				inner, _ := r.Render(raw.String())
				switch {
				case inner == "":
				case strings.Contains(inner, "\n"):
					bx := newBox(inner)
					tokens = append(tokens, wrapToken{box: &bx})
				default:
					tokens = append(tokens, wrapToken{text: inner})
				}
			}
		case "table", "ol", "ul", "menu", "br":
			decls := r.resolveDecls(n)
			if decls["display"] == "none" {
				return tokens
			}
			switch n.Data {
			case "table":
				if isTableLayoutDisplay(decls["display"]) {
					bx := newBox(strings.TrimSuffix(r.renderTable(n, r.width), "\n"))
					tokens = append(tokens, wrapToken{box: &bx, node: n})
					tokens = append(tokens, wrapToken{brk: true})
				} else {
					tokens = r.renderRootDisplayTokens(tokens, n)
				}
			case "ol", "ul", "menu":
				ordered := n.Data == "ol"
				if mt := parseMargin(decls["margin-top"]); mt > 0 && hasContent(tokens) {
					tokens = ensureBreaks(tokens, mt+1)
				}
				bx := newBox(strings.TrimSuffix(r.renderList(n, ordered, r.width), "\n"))
				tokens = append(tokens, wrapToken{box: &bx, node: n})
				tokens = ensureBreaks(tokens, parseMargin(decls["margin-bottom"])+1)
			case "br":
				tokens = append(tokens, wrapToken{brk: true})
			}
		default:
			tokens = r.renderRootDisplayTokens(tokens, n)
		}
	case html.ErrorNode, html.CommentNode, html.DoctypeNode, html.RawNode:
		// nothing to render
	}
	return tokens
}

// renderRootDisplayTokens is renderDisplayNode's token-based equivalent for
// root-level content. Top-level <a> anchors get wrapHyperlink applied
// regardless of display value (block/inline-block/default alike) — this
// asymmetry with inline.go's nested "block" case (which never wraps a
// block-display anchor in a hyperlink) already existed before this
// migration and is preserved, not introduced by it.
func (r *Renderer) renderRootDisplayTokens(tokens []wrapToken, n *html.Node) []wrapToken {
	decls := r.resolveDecls(n)
	href := ""
	if n.Data == "a" {
		href = nodeAttr(n, "href")
	}
	switch decls["display"] {
	case "none":
	case "block":
		// Unlike inline.go's nested "block" case (always ensures at least 1
		// separator when there's preceding content, regardless of margin
		// value), root-level block dispatch has historically only forced
		// separation when margin-top is explicitly non-zero — preserved
		// here exactly: e.g. "before<p>paragraph</p>" (p's margin-top is 0)
		// glues directly with no separator, relying on whatever's already
		// pending from the previous sibling.
		if mt := parseMargin(decls["margin-top"]); mt > 0 && hasContent(tokens) {
			tokens = ensureBreaks(tokens, mt+1)
		}
		savedDepth := r.quoteDepth
		bx := r.renderBlockContentBox(n, decls, r.width)
		if decls["visibility"] == "hidden" {
			r.quoteDepth = savedDepth
			bx = blankVisibleContentBox(bx)
		}
		bx = r.wrapHyperlinkBox(href, bx)
		tokens = append(tokens, wrapToken{box: &bx, node: n})
		tokens = append(tokens, wrapToken{brk: true})
		tokens = ensureBreaks(tokens, parseMargin(decls["margin-bottom"])+1)
	case "inline-block":
		acc := extractInlineStyle(decls)
		savedDepth := r.quoteDepth
		inner := r.renderInlineAcc(n, acc, r.width)
		if colWidth, constrained := resolveWidthConstraints(decls, r.width, maxVisibleLineWidth(inner)); constrained && colWidth > 0 {
			inner = padLinesToWidth(inner, colWidth)
		}
		if decls["visibility"] == "hidden" {
			r.quoteDepth = savedDepth
			inner = blankVisibleContent(inner)
		}
		inner = r.wrapHyperlink(href, inner)
		switch {
		case inner == "":
		case strings.Contains(inner, "\n"):
			bx := newBox(inner)
			tokens = append(tokens, wrapToken{box: &bx, node: n})
		default:
			tokens = append(tokens, wrapToken{text: inner})
		}
	default:
		acc := extractInlineStyle(decls)
		savedDepth := r.quoteDepth
		inner := r.renderInlineAcc(n, acc, r.width)
		if decls["visibility"] == "hidden" {
			r.quoteDepth = savedDepth
			inner = blankVisibleContent(inner)
		}
		inner = r.wrapHyperlink(href, inner)
		switch {
		case inner == "":
		case strings.Contains(inner, "\n"):
			bx := newBox(inner)
			tokens = append(tokens, wrapToken{box: &bx, node: n})
		default:
			tokens = append(tokens, wrapToken{text: inner})
		}
	}
	return tokens
}
