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
	var sb strings.Builder

	if bd := r.pseudoElemDecls(n, "before"); len(bd) > 0 {
		if text := r.parseCSSContentString(bd["content"], n); text != "" {
			pseudoTT := effectiveTransform(bd)
			if pseudoTT == "" {
				pseudoTT = tt
			}
			text = applyTextTransform(text, pseudoTT)
			st := mergeInlineStyle(acc, bd)
			if st.has() {
				sb.WriteString(st.render(text, r.profile))
			} else {
				sb.WriteString(text)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			normalized := applyTextTransform(normalizeWhiteSpace(c.Data, ws, tabSize), tt)
			if normalized != "" {
				atLineStart := sb.Len() == 0 || sb.String()[sb.Len()-1] == '\n'
				if atLineStart && ws != "pre" && ws != "pre-wrap" {
					normalized = strings.TrimLeft(normalized, " ")
				}
				if acc.has() {
					sb.WriteString(acc.render(normalized, r.profile))
				} else {
					sb.WriteString(normalized)
				}
			}
		case html.ElementNode:
			childDecls := r.resolveDecls(c)
			if childDecls["display"] == "none" {
				continue
			}
			if c.Data == "br" {
				sb.WriteByte('\n')
				continue
			}
			if c.Data == "wbr" {
				continue
			}
			if c.Data == "ul" || c.Data == "ol" || c.Data == "menu" {
				if sb.Len() > 0 && sb.String()[sb.Len()-1] != '\n' {
					sb.WriteByte('\n')
				}
				sb.WriteString(r.renderList(c, c.Data == "ol", availWidth))
				continue
			}
			display := childDecls["display"]
			switch display {
			case "block":
				if sb.Len() > 0 {
					writeMarginNewlines(&sb, parseMargin(childDecls["margin-top"])+1)
				}
				sb.WriteString(r.renderBlockContent(c, childDecls, availWidth))
				sb.WriteByte('\n')
				if mb := parseMargin(childDecls["margin-bottom"]); mb > 0 {
					writeMarginNewlines(&sb, mb+1)
				}
			default:
				childAcc := mergeInlineStyle(acc, childDecls)
				inner := r.renderInlineAcc(c, childAcc, availWidth)
				if display == "inline-block" {
					if wv, ok := childDecls["width"]; ok {
						if abs, pct, ok2 := parseSizeVal(wv); ok2 {
							w := abs
							if pct > 0 {
								w = int(pct * float64(r.width))
							}
							if w > 0 {
								inner = padLinesToWidth(inner, w)
							}
						}
					}
				}
				inner = wrapInlineElement(c, inner)
				if c.Data == "a" {
					inner = wrapHyperlink(nodeAttr(c, "href"), inner)
				}
				sb.WriteString(inner)
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
				sb.WriteString(st.render(text, r.profile))
			} else {
				sb.WriteString(text)
			}
		}
	}

	return sb.String()
}
