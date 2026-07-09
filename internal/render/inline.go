package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// renderInlineAcc renders the inline content of n with accumulated text
// style. Thin shim over renderInlineAccTokens for callers not yet migrated
// to tokens (list.go, table_render.go) — see wraptoken.go.
func (r *Engine) renderInlineAcc(n *html.Node, acc inlineStyle, availWidth int) string {
	return tokensToString(r.renderInlineAccTokens(n, acc, availWidth))
}

// appendText appends text, splitting any embedded "\n" into brk tokens first
// (text tokens must never contain a literal newline — a CSS content: "\A"
// pseudo-element value or a <pre>/pre-wrap text node, which preserves source
// newlines instead of collapsing them, are the two sources of this). A
// styled segment's trailing space stays inside its ANSI span (lastRune,
// wraptoken.go, and block.go's own trailing-space trim are ANSI-aware, via
// stripANSI/trimOneTrailingVisibleSpace, so they don't need it kept plain).
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
	return append(tokens, wrapToken{text: st.render(text, p)})
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
func (r *Engine) renderInlineAccTokens(n *html.Node, acc inlineStyle, availWidth int) []wrapToken {
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

	// pushBoxDirect appends a box token (plus its own descendants'
	// subPositions, if any) followed by one mandatory trailing brk, after
	// first ensuring at least n breaks precede it (n==0 means "just make
	// sure something separates it from any preceding content" — the
	// token-domain replacement for cappedWriter's "ensure newline before"/
	// margin-top checks). This is how block children, tables, and lists all
	// force their own line regardless of their own rendered height, while
	// still carrying any trackable descendants' positions along (see
	// wraptoken.go's Rect doc comment) rather than flattening to a string
	// first and losing them.
	pushBoxDirect := func(bx box, subPositions map[*html.Node]Rect, minBreaksBefore int, node *html.Node) {
		if hasContent(tokens) {
			tokens = ensureBreaks(tokens, minBreaksBefore+1)
		}
		tokens = append(tokens, wrapToken{box: &bx, node: node, subPositions: subPositions})
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
					tableContent, tablePositions := r.renderTable(c, tableWidth)
					bx := newBox(strings.TrimSuffix(tableContent, "\n"))
					if childDecls["visibility"] == "hidden" {
						bx = blankVisibleContentBox(bx)
					}
					pushBoxDirect(bx, tablePositions, 0, c)
				} else {
					savedDepth := r.quoteDepth
					bx, subPositions := r.renderBlockContentBox(c, childDecls, availWidth)
					if childDecls["visibility"] == "hidden" {
						r.quoteDepth = savedDepth
						bx = blankVisibleContentBox(bx)
					}
					pushBoxDirect(bx, subPositions, 0, c)
				}
				continue
			}
			if c.Data == "ul" || c.Data == "ol" || c.Data == "menu" {
				listContent, listPositions := r.renderList(c, c.Data == "ol", availWidth)
				bx := newBox(strings.TrimSuffix(listContent, "\n"))
				pushBoxDirect(bx, listPositions, 0, c)
				continue
			}
			display := childDecls["display"]
			switch display {
			case "block":
				savedDepth := r.quoteDepth
				bx, subPositions := r.renderBlockContentBox(c, childDecls, availWidth)
				if childDecls["visibility"] == "hidden" {
					r.quoteDepth = savedDepth
					bx = blankVisibleContentBox(bx)
				}
				pushBoxDirect(bx, subPositions, parseMargin(childDecls["margin-top"]), c)
				tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
			default:
				if display == "inline-block" || c.Data == "a" {
					// inline-block (including <input>, always inline-block
					// per the UA stylesheet) and <a> stay string-based: an
					// inline-block's content is deliberately one atomic
					// unit regardless of what's inside it, and a hyperlink
					// needs whole-string OSC8 wrapping — neither is worth
					// the complexity of a token-level equivalent given
					// how rarely either wraps further trackable
					// descendants (e.g. a second form control) in
					// practice. This is an accepted position-tracking gap
					// for that specific, uncommon combination.
					childAcc := mergeInlineStyle(acc, childDecls)
					savedDepth := r.quoteDepth
					var inner string
					if c.Data == "input" {
						// <input> has no children — its visual content is
						// synthesized from attributes (type/value/placeholder/
						// checked), not rendered from child nodes.
						inner = inputDisplayText(c)
					} else {
						// TrimSuffix (at most one "\n"): a nested recursive
						// call whose own last child was block-ish (e.g. an
						// implicit <tbody> wrapping <tr>) can end in a
						// trailing structural newline that was only ever
						// meaningful as pending accumulator state, never real
						// content — pushBox already trims this for its own
						// inputs; this mirrors that for the plain-inline/
						// inline-block path. Only one is trimmed, not all
						// trailing newlines, since a further one could be a
						// real trailing blank line (e.g. from padding-bottom).
						inner = strings.TrimSuffix(r.renderInlineAcc(c, childAcc, availWidth), "\n")
					}
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
				} else {
					// Plain inline (span, em, strong, label, etc.): splice
					// the child's own tokens directly into this level's
					// stream instead of flattening to a string first — the
					// only way a trackable descendant (e.g. an <input>
					// inside a <label>) keeps its box-token identity (and
					// so its position) through to whichever ancestor's
					// wordWrapTokens call ultimately places it. It also
					// naturally preserves word-wrap-ability across this
					// element's own boundary, matching RENDERING.md's
					// original token-splicing intent for plain inline
					// content (findings #3/#4) more closely than the
					// flatten-then-rebox approach the other branch uses.
					childAcc := mergeInlineStyle(acc, childDecls)
					savedDepth := r.quoteDepth
					childTokens := r.renderInlineAccTokens(c, childAcc, availWidth)
					// Trim one trailing brk, mirroring the TrimSuffix quirk
					// on the string-based branch above: a nested block-ish
					// descendant's own mandatory trailing brk is structural,
					// not content, and would otherwise closeAndPush a
					// spurious blank line once these tokens reach a
					// wordWrapTokens call.
					if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
						childTokens = childTokens[:len(childTokens)-1]
					}
					if childDecls["visibility"] == "hidden" {
						r.quoteDepth = savedDepth
						childTokens = blankVisibleContentTokens(childTokens)
					}
					tokens = append(tokens, childTokens...)
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
