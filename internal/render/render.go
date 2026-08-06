package render

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

// isHiddenVisibility reports whether a computed visibility value hides the
// element's content while keeping its layout space. `collapse` is included
// because on anything that isn't a flex item (or a table row/column, which
// this engine doesn't collapse either) the spec says it simply computes to
// `hidden`. A *flex* item declaring it is dropped from layout entirely, and
// never reaches any of these call sites (see appendFlexItems).
func isHiddenVisibility(v string) bool {
	return v == "hidden" || v == "collapse"
}

// renderRootTokens builds the token stream for a whole document: doc's
// children (typically one <html> element) via renderRootNodeTokens, the
// token-based equivalent of the old cappedWriter-based renderNode walk.
func (r *Engine) renderRootTokens(doc *html.Node) []wrapToken {
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
// text-transform:none, tab-size:8 context, never derived from html or body's
// own rarely-set CSS, preserved here exactly, not something this
// migration changes.
func (r *Engine) renderRootNodeTokens(tokens []wrapToken, n *html.Node) []wrapToken {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			tokens = r.renderRootNodeTokens(tokens, c)
		}
	case html.TextNode:
		// text == "" (not strings.TrimSpace(text) != "") deliberately: a
		// standalone space between two root-level inline siblings (e.g.
		// "<span>a</span> <b>b</b>") is meaningful content to preserve, not
		// a structural artifact to discard: it's the only thing separating
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
			// word-break opportunity, with no terminal equivalent. Emit nothing
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
				// value, which has its own complete trailing-newline
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
					if hasContent(tokens) {
						tokens = ensureBreaks(tokens, parseMargin(decls["margin-top"])+1)
					}
					tableContent, tablePositions := r.renderTable(n, r.width)
					bx := newBox(strings.TrimSuffix(tableContent, "\n"))
					tokens = append(tokens, wrapToken{box: &bx, node: n, subPositions: tablePositions})
					tokens = append(tokens, wrapToken{brk: true})
					tokens = ensureBreaks(tokens, parseMargin(decls["margin-bottom"])+1)
				} else {
					tokens = r.renderRootDisplayTokens(tokens, n)
				}
			case "ol", "ul", "menu":
				ordered := n.Data == "ol"
				if hasContent(tokens) {
					tokens = ensureBreaks(tokens, parseMargin(decls["margin-top"])+1)
				}
				listContent, listPositions := r.renderList(n, ordered, r.width)
				bx := newBox(strings.TrimSuffix(listContent, "\n"))
				tokens = append(tokens, wrapToken{box: &bx, node: n, subPositions: listPositions})
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
// regardless of display value, whether block, inline-block, or default. This
// asymmetry with inline.go's nested "block" case (which never wraps a
// block-display anchor in a hyperlink) already existed before this
// migration and is preserved, not introduced by it.
func (r *Engine) renderRootDisplayTokens(tokens []wrapToken, n *html.Node) []wrapToken {
	if r.outOfFlow[n] {
		// position: absolute/fixed elements reserve no space in normal
		// flow. applyOutOfFlow (outofflow.go) positions and paints them
		// after layout finishes, same as compositeOpenSelects does for an
		// open <select> popup, not here.
		return tokens
	}
	decls := r.resolveDecls(n)
	href := ""
	if n.Data == "a" {
		href = nodeAttr(n, "href")
	}
	switch decls["display"] {
	case "none":
	case "block":
		savedDepth := r.quoteDepth
		// renderBlockContentBox runs before consulting decls["margin-top"]:
		// it may raise that value itself, when n's own first child's
		// margin-top collapses through n's open top edge (block.go's
		// leading-trim), the pre-box separator below must see that
		// collapsed value, not the one resolveDecls produced.
		bx, subPositions := r.renderBlockContentBox(n, decls, r.width)
		if isHiddenVisibility(decls["visibility"]) {
			r.quoteDepth = savedDepth
			bx = blankVisibleContentBox(bx)
		}
		bx = r.wrapHyperlinkBox(href, bx)
		// A block box always starts its own line regardless of margin-top, so
		// matches inline.go's nested "block" case (pushBoxDirect always
		// ensures at least 1 separator when there's preceding content); a
		// non-zero margin-top only raises that minimum further.
		if hasContent(tokens) {
			tokens = ensureBreaks(tokens, parseMargin(decls["margin-top"])+1)
		}
		tokens = append(tokens, wrapToken{box: &bx, node: n, subPositions: subPositions})
		tokens = append(tokens, wrapToken{brk: true})
		tokens = ensureBreaks(tokens, parseMargin(decls["margin-bottom"])+1)
	case "flex":
		savedDepth := r.quoteDepth
		bx, subPositions := r.renderFlexContentBox(n, decls, r.width)
		if isHiddenVisibility(decls["visibility"]) {
			r.quoteDepth = savedDepth
			bx = blankVisibleContentBox(bx)
		}
		bx = r.wrapHyperlinkBox(href, bx)
		if hasContent(tokens) {
			tokens = ensureBreaks(tokens, parseMargin(decls["margin-top"])+1)
		}
		tokens = append(tokens, wrapToken{box: &bx, node: n, subPositions: subPositions})
		tokens = append(tokens, wrapToken{brk: true})
		tokens = ensureBreaks(tokens, parseMargin(decls["margin-bottom"])+1)
	case "inline-block", "inline-flex":
		acc := extractInlineStyle(decls)
		savedDepth := r.quoteDepth
		var inner string
		switch text, synthesized := r.synthesizedControlText(n, decls, acc, r.width); {
		case synthesized:
			// <input>/<select>/<progress>/<meter> build their content from
			// attributes rather than from child nodes; see
			// synthesizedControlText (formcontrol.go).
			inner = text
		case decls["display"] == "inline-flex":
			inner = r.renderInlineFlexContent(n, decls, r.width)
		default:
			inner = r.renderInlineAcc(n, acc, r.width)
		}
		if colWidth, constrained := resolveWidthConstraints(decls, r.width, maxVisibleLineWidth(inner)); constrained && colWidth > 0 {
			inner = padLinesToWidth(inner, colWidth)
		}
		if isHiddenVisibility(decls["visibility"]) {
			r.quoteDepth = savedDepth
			inner = blankVisibleContent(inner)
		}
		inner = r.wrapHyperlink(href, inner)
		if inner != "" {
			// Unconditionally box (not just when inner contains "\n"),
			// matching inline.go's nested inline-block case: inline-block
			// content is deliberately one atomic unit regardless of height,
			// and boxing it is also what makes it trackable via
			// Document.Rect. A single-line <button> or <input> would
			// otherwise become a plain text token with no position at all.
			bx := newBox(inner)
			tokens = append(tokens, wrapToken{box: &bx, node: n})
		}
	case "contents":
		// No box, no hyperlink wrap, regardless of n.Data. n's children
		// splice directly into the root token stream as if they were root
		// siblings themselves. Mirrors inline.go's nested "contents" case.
		acc := mergeContentsInlineStyle(newInlineStyle(), decls)
		savedDepth := r.quoteDepth
		childTokens := r.renderInlineAccTokens(n, acc, r.width)
		// See inline.go's nested "contents" case for why the leading brk
		// tokens (n's first child's own margin-top) need resolving against
		// the real outer tokens here rather than passing through as-is.
		leading := leadingBreaks(childTokens)
		childTokens = childTokens[leading:]
		if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
			childTokens = childTokens[:len(childTokens)-1]
		}
		if isHiddenVisibility(decls["visibility"]) {
			r.quoteDepth = savedDepth
			childTokens = blankVisibleContentTokens(childTokens)
		}
		if hasContent(tokens) {
			tokens = ensureBreaks(tokens, leading)
		}
		tokens = append(tokens, childTokens...)
	default:
		switch {
		case n.Data == "a":
			// <a> stays string-based, matching inline.go's nested "default"
			// case: a hyperlink needs whole-string OSC8 wrapping, and a
			// token-level equivalent isn't worth the complexity given how
			// rarely a root-level anchor wraps further trackable
			// descendants, such as a form control. An accepted
			// position-tracking gap for that specific, uncommon case.
			acc := extractInlineStyle(decls)
			savedDepth := r.quoteDepth
			inner := r.renderInlineAcc(n, acc, r.width)
			if isHiddenVisibility(decls["visibility"]) {
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
		case n.Data == "label":
			// A root-level <label> mirrors inline.go's nested "label" case:
			// it needs its own trackable Rect (for DispatchClick's
			// label-to-control redirect; see COMPATIBILITY.md's <label>
			// click-redirect deviation, in the HTML "Deviations from Spec"
			// section) while still forwarding a nested trackable
			// descendant's own position, via wordWrapTokens directly rather
			// than the flatten-to-string r.renderInlineAcc uses for <a>
			// above. See inline.go's nested case for the full rationale,
			// including the accepted display:inline-block-like reflow
			// tradeoff.
			acc := extractInlineStyle(decls)
			savedDepth := r.quoteDepth
			childTokens := r.renderInlineAccTokens(n, acc, r.width)
			// Trim one trailing brk, matching the plain-inline default
			// case below and inline.go's nested label case: a nested
			// block-ish descendant's own mandatory trailing brk is
			// structural, not content, and would otherwise closeAndPush a
			// spurious blank line once wordWrapTokens processes it.
			if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
				childTokens = childTokens[:len(childTokens)-1]
			}
			if isHiddenVisibility(decls["visibility"]) {
				r.quoteDepth = savedDepth
				childTokens = blankVisibleContentTokens(childTokens)
			}
			breakMode := decls["overflow-wrap"]
			if breakMode == "" {
				breakMode = decls["word-break"]
			}
			bx, subPositions := wordWrapTokens(childTokens, r.width, breakMode, 0, false)
			if !(len(bx.lines) <= 1 && bx.width == 0) {
				tokens = append(tokens, wrapToken{box: &bx, node: n, subPositions: subPositions})
			}
		default:
			// Plain inline root content (a root-level <span>, <em>, and the
			// like): splice its own tokens directly instead of flattening
			// to a string first, so a trackable descendant (e.g. an
			// <input> inside a root-level <span>) keeps its box-token
			// identity through to the root wordWrapTokens call; see
			// inline.go's matching nested case for the full rationale.
			// <label> is the one plain-inline element that opts out of this
			// path; see its own case above.
			acc := extractInlineStyle(decls)
			savedDepth := r.quoteDepth
			childTokens := r.renderInlineAccTokens(n, acc, r.width)
			if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
				childTokens = childTokens[:len(childTokens)-1]
			}
			if isHiddenVisibility(decls["visibility"]) {
				r.quoteDepth = savedDepth
				childTokens = blankVisibleContentTokens(childTokens)
			}
			tokens = append(tokens, childTokens...)
		}
	}
	return tokens
}
