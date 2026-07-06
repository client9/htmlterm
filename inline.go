package htmlterm

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// renderInlineAcc renders the inline content of n with accumulated text
// style. Thin shim over renderInlineAccTokens for callers not yet migrated
// to tokens (list.go, table_render.go) — see wraptoken.go.
func (r *Renderer) renderInlineAcc(n *html.Node, acc inlineStyle, availWidth int) string {
	return tokensToString(r.renderInlineAccTokens(n, acc, availWidth))
}

// appendText appends text, splitting any embedded "\n" into brk tokens first
// (text tokens must never contain a literal newline — a CSS content: "\A"
// pseudo-element value or a <pre>/pre-wrap text node, which preserves source
// newlines instead of collapsing them, are the two sources of this). Each
// resulting segment is pushed as one or two tokens: st-styled core plus an
// unstyled trailing-space tail, matching the historical quirk that trailing
// spaces stay outside any ANSI span so LastByte/lastRune-style checks (and
// HasSuffix checks elsewhere) see them as plain content.
func appendText(tokens []wrapToken, st inlineStyle, text string, p colorprofile.Profile) []wrapToken {
	if text == "" {
		return tokens
	}
	if !strings.Contains(text, "\n") {
		return appendTextSegment(tokens, st, text, p)
	}
	for i, part := range strings.Split(text, "\n") {
		if i > 0 {
			tokens = append(tokens, wrapToken{brk: true})
		}
		tokens = appendTextSegment(tokens, st, part, p)
	}
	return tokens
}

func appendTextSegment(tokens []wrapToken, st inlineStyle, text string, p colorprofile.Profile) []wrapToken {
	if text == "" {
		return tokens
	}
	if !st.has() {
		return append(tokens, wrapToken{text: text})
	}
	core, trail := splitTrailingSpaces(text)
	if core != "" {
		tokens = append(tokens, wrapToken{text: st.render(core, p)})
	}
	if trail != "" {
		tokens = append(tokens, wrapToken{text: trail})
	}
	return tokens
}

// renderInlineAccTokens is renderInlineAcc's token-collecting core. It walks
// n's children, accumulating a []wrapToken instead of writing into a
// cappedWriter: text runs become text tokens; <br> becomes a brk token;
// block children, tables, and lists become box tokens with explicit
// surrounding brk tokens (so they always occupy their own line regardless of
// their own height); inline/inline-block children are rendered recursively
// (via the string shim, since hyperlink-wrap and inline-block width padding
// are still string operations) and folded back in as either a text token
// (plain inline, no embedded newline) or a box token (inline-block, or plain
// inline content that happens to contain an embedded newline from a nested
// block-in-inline/pre — box tokens alone decide single-line-shares-a-line
// vs. multi-line-forces-a-break based purely on their own height, per
// wordWrapTokens).
func (r *Renderer) renderInlineAccTokens(n *html.Node, acc inlineStyle, availWidth int) []wrapToken {
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

	var tokens []wrapToken

	if bd := r.pseudoElemDecls(n, "before"); len(bd) > 0 {
		if text := r.parseCSSContentString(bd["content"], n); text != "" {
			pseudoTT := effectiveTransform(bd)
			if pseudoTT == "" {
				pseudoTT = tt
			}
			text = applyTextTransform(text, pseudoTT)
			st := mergeInlineStyle(acc, bd)
			tokens = appendText(tokens, st, text, r.profile)
		}
	}

	// pushBox appends a box token followed by one mandatory trailing brk,
	// after first ensuring at least n breaks precede it (n==0 means "just
	// make sure something separates it from any preceding content" — the
	// token-domain replacement for cappedWriter's "ensure newline before"/
	// margin-top checks). This is how block children, tables, and lists all
	// force their own line regardless of their own rendered height.
	pushBox := func(content string, minBreaksBefore int, node *html.Node) {
		if hasContent(tokens) {
			tokens = ensureBreaks(tokens, minBreaksBefore+1)
		}
		bx := newBox(strings.TrimRight(content, "\n"))
		tokens = append(tokens, wrapToken{box: &bx, node: node})
		tokens = append(tokens, wrapToken{brk: true})
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			normalized := applyTextTransform(normalizeWhiteSpace(sanitizeTerminalText(c.Data, true), ws, tabSize), tt)
			if normalized != "" {
				lr, ok := lastRune(tokens)
				atLineStart := !ok || lr == '\n'
				prevIsSpace := ok && lr == ' '
				if (atLineStart || prevIsSpace) && ws != "pre" && ws != "pre-wrap" {
					normalized = strings.TrimLeft(normalized, " ")
				}
				tokens = appendText(tokens, acc, normalized, r.profile)
			}
		case html.ElementNode:
			switch c.Data {
			case "head":
				continue
			}
			if isSkippedContentElement(c.Data) {
				continue
			}
			childDecls := r.resolveDecls(c)
			if childDecls["display"] == "none" {
				continue
			}
			if c.Data == "br" {
				tokens = append(tokens, wrapToken{brk: true})
				continue
			}
			if c.Data == "wbr" {
				continue
			}
			if c.Data == "table" {
				if isTableLayoutDisplay(childDecls["display"]) {
					tableWidth := availWidth
					if r.nestedTableWidthSet {
						tableWidth = r.nestedTableWidth
					}
					tableContent := r.renderTable(c, tableWidth)
					if childDecls["visibility"] == "hidden" {
						tableContent = blankVisibleContent(tableContent)
					}
					pushBox(tableContent, 0, c)
				} else {
					savedDepth := r.quoteDepth
					tableContent := r.renderBlockContent(c, childDecls, availWidth)
					if childDecls["visibility"] == "hidden" {
						r.quoteDepth = savedDepth
						tableContent = blankVisibleContent(tableContent)
					}
					pushBox(tableContent, 0, c)
				}
				continue
			}
			if c.Data == "ul" || c.Data == "ol" || c.Data == "menu" {
				pushBox(r.renderList(c, c.Data == "ol", availWidth), 0, c)
				continue
			}
			display := childDecls["display"]
			switch display {
			case "block":
				savedDepth := r.quoteDepth
				blockContent := r.renderBlockContent(c, childDecls, availWidth)
				if childDecls["visibility"] == "hidden" {
					r.quoteDepth = savedDepth
					blockContent = blankVisibleContent(blockContent)
				}
				pushBox(blockContent, parseMargin(childDecls["margin-top"]), c)
				tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
			default:
				childAcc := mergeInlineStyle(acc, childDecls)
				savedDepth := r.quoteDepth
				// TrimRight: a nested recursive call whose own last child
				// was block-ish (e.g. an implicit <tbody> wrapping <tr>)
				// can end in a trailing structural newline that was only
				// ever meaningful as cappedWriter pending state, never real
				// content — pushBox already trims this for its own inputs;
				// this mirrors that for the plain-inline/inline-block path.
				inner := strings.TrimRight(r.renderInlineAcc(c, childAcc, availWidth), "\n")
				if display == "inline-block" {
					if colWidth, constrained := resolveWidthConstraints(childDecls, r.width, maxVisibleLineWidth(inner)); constrained && colWidth > 0 {
						inner = padLinesToWidth(inner, colWidth)
					}
				}
				if childDecls["visibility"] == "hidden" {
					r.quoteDepth = savedDepth
					inner = blankVisibleContent(inner)
				}
				if c.Data == "a" {
					inner = r.wrapHyperlink(nodeAttr(c, "href"), inner)
				}
				switch {
				case inner == "":
					// nothing to add
				case display == "inline-block" || strings.Contains(inner, "\n"):
					bx := newBox(inner)
					tokens = append(tokens, wrapToken{box: &bx, node: c})
				default:
					tokens = append(tokens, wrapToken{text: inner})
				}
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
			tokens = appendText(tokens, st, text, r.profile)
		}
	}

	return tokens
}
