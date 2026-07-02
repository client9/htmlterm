package htmlterm

import (
	"strings"

	"golang.org/x/net/html"
)

// renderInline renders the inline content of n with no accumulated text style.
func (r *Renderer) renderInline(n *html.Node, availWidth int) string {
	return r.renderInlineAcc(n, inlineStyle{}, availWidth)
}

// renderInlineAcc renders the inline content of n with accumulated text style.
func (r *Renderer) renderInlineAcc(n *html.Node, acc inlineStyle, availWidth int) string {
	nDecls := r.resolveDecls(n)
	ws := "normal"
	if v := nDecls["white-space"]; v != "" {
		ws = v
	}
	tabSize := 8
	if abs, _, ok := parseSizeVal(nDecls["tab-size"]); ok && abs > 0 {
		tabSize = abs
	}
	tt := effectiveTransform(nDecls)
	w := cappedWriter{maxBlanks: r.maxBlankLines}
	if ws == "pre" || ws == "pre-wrap" {
		w.EnterPre()
	}

	if bd := r.pseudoElemDecls(n, "before"); len(bd) > 0 {
		if text := r.parseCSSContentString(bd["content"], n); text != "" {
			pseudoTT := effectiveTransform(bd)
			if pseudoTT == "" {
				pseudoTT = tt
			}
			text = applyTextTransform(text, pseudoTT)
			st := mergeInlineStyle(acc, bd)
			if st.has() {
				core, trail := splitTrailingSpaces(text)
				if core != "" {
					w.WriteString(st.render(core, r.profile))
				}
				w.WriteString(trail)
			} else {
				w.WriteString(text)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			normalized := applyTextTransform(normalizeWhiteSpace(sanitizeTerminalText(c.Data, true), ws, tabSize), tt)
			if normalized != "" {
				b, ok := w.LastByte()
				atLineStart := !ok || b == '\n'
				prevIsSpace := ok && b == ' '
				if (atLineStart || prevIsSpace) && ws != "pre" && ws != "pre-wrap" {
					normalized = strings.TrimLeft(normalized, " ")
				}
				if acc.has() {
					core, trail := splitTrailingSpaces(normalized)
					if core != "" {
						w.WriteString(acc.render(core, r.profile))
					}
					w.WriteString(trail)
				} else {
					w.WriteString(normalized)
				}
			}
		case html.ElementNode:
			switch c.Data {
			case "style", "script", "meta", "link", "head":
				continue
			}
			childDecls := r.resolveDecls(c)
			if childDecls["display"] == "none" {
				continue
			}
			if c.Data == "br" {
				w.writeNewline()
				continue
			}
			if c.Data == "wbr" {
				continue
			}
			if c.Data == "ul" || c.Data == "ol" || c.Data == "menu" {
				if b, ok := w.LastByte(); ok && b != '\n' {
					w.writeNewline()
				}
				w.WriteString(r.renderList(c, c.Data == "ol", availWidth))
				continue
			}
			display := childDecls["display"]
			switch display {
			case "block":
				if w.Len() > 0 {
					w.WriteAtLeastNewlines(parseMargin(childDecls["margin-top"]) + 1)
				}
				childWS := childDecls["white-space"]
				if childWS == "pre" || childWS == "pre-wrap" {
					w.EnterPre()
				}
				w.WriteString(r.renderBlockContent(c, childDecls, availWidth))
				if childWS == "pre" || childWS == "pre-wrap" {
					w.ExitPre()
				}
				w.writeNewline()
				w.WriteAtLeastNewlines(parseMargin(childDecls["margin-bottom"]) + 1)
			default:
				childAcc := mergeInlineStyle(acc, childDecls)
				inner := r.renderInlineAcc(c, childAcc, availWidth)
				if display == "inline-block" {
					if wv, ok := childDecls["width"]; ok {
						if abs, pct, ok2 := parseSizeVal(wv); ok2 {
							colWidth := abs
							if pct > 0 {
								colWidth = int(pct * float64(r.width))
							}
							if colWidth > 0 {
								inner = padLinesToWidth(inner, colWidth)
							}
						}
					}
				}
				if c.Data == "a" {
					inner = r.wrapHyperlink(nodeAttr(c, "href"), inner)
				}
				w.WriteString(inner)
			}
		case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode, html.RawNode:
			// nothing to render
		}
	}

	if ad := r.pseudoElemDecls(n, "after"); len(ad) > 0 {
		if text := r.parseCSSContentString(ad["content"], n); text != "" {
			pseudoTT := effectiveTransform(ad)
			if pseudoTT == "" {
				pseudoTT = tt
			}
			text = applyTextTransform(text, pseudoTT)
			st := mergeInlineStyle(acc, ad)
			if st.has() {
				core, trail := splitTrailingSpaces(text)
				if core != "" {
					w.WriteString(st.render(core, r.profile))
				}
				w.WriteString(trail)
			} else {
				w.WriteString(text)
			}
		}
	}

	return w.String()
}
