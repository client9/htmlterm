package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// renderInlineAcc renders the inline content of n with accumulated text
// style. It is a thin shim over renderInlineAccTokens for callers not yet
// migrated to tokens, list.go and table_render.go; see wraptoken.go.
func (r *Engine) renderInlineAcc(n *html.Node, acc inlineStyle, availWidth int) string {
	// A block or flex child's margin-top and margin-bottom, via pushBoxDirect
	// below, always show up as leading and trailing brk tokens when that
	// child is first or last, even with nothing to separate from. This shim's
	// callers, list.go and table_render.go rendering <li> and <td> content,
	// have no notion of margin collapse, so that boundary noise is discarded
	// here rather than recovered, matching this function's own pre-existing
	// contract of returning flattened content, not a margin-aware box.
	tokens, _, _ := trimBoundaryBreaks(r.renderInlineAccTokens(n, acc, availWidth))
	return tokensToString(tokens)
}

// appendText appends text, splitting any embedded "\n" into brk tokens first,
// since text tokens must never contain a literal newline. The two sources of
// one are a CSS content: "\A" pseudo-element value and a <pre> or pre-wrap
// text node, which preserves source newlines instead of collapsing them. A
// styled segment's trailing space stays inside its ANSI span: wraptoken.go's
// lastRune and block.go's own trailing-space trim are ANSI-aware, via
// textcell.Strip and textcell.TrimTrailingSpace, so they don't need it kept
// plain.
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

// restyleTrailingWhitespaceOnlyToken re-styles the last token in tokens
// using outerAcc, the style in effect just *before* this element's own
// decls were merged in, if that token's entire visible content is exactly
// one space. Such a token can only have come from a dedicated
// whitespace-only child. It is not a trailing space attached to real text
// within the same token, as in "Alternatively, ", which keeps its own
// element's styling, being real prose content rather than pure structural
// whitespace.
//
// The common source is pretty-printed HTML: a text node holding just the
// indentation and newline between a child element and its parent's closing
// tag, as in "<a href=...>\n  <em>register</em>\n</a>". That trailing
// whitespace is a real DOM child of <a>, so without this it renders styled
// with the <a>'s own underline and color, showing up as a visibly detached
// styled space glued to the link.
//
// This does NOT remove the character. An earlier version of this fix tried
// that, by symmetry with the leading-whitespace trim in the TextNode case
// above, and it was wrong: dropping a trailing space and hoping the parent's
// stream independently supplies a replacement space fails whenever there is
// no such backup source. In "<em>foo </em><b>bar</b>", written with no
// whitespace between the tags in the source, trimming foo's trailing space
// would merge it straight into "foobar". Re-styling in place can never lose
// or duplicate a character, only change its color.
func restyleTrailingWhitespaceOnlyToken(tokens []wrapToken, outerAcc inlineStyle, p colorprofile.Profile) []wrapToken {
	if len(tokens) == 0 {
		return tokens
	}
	last := len(tokens) - 1
	if tokens[last].box != nil || tokens[last].brk {
		return tokens
	}
	if textcell.Strip(tokens[last].text) != " " {
		return tokens
	}
	text := " "
	if outerAcc.has() {
		text = outerAcc.render(" ", p)
	}
	tokens[last] = wrapToken{text: text}
	return tokens
}

// renderInlineAccTokens is renderInlineAcc's token-collecting core. It walks
// n's children, accumulating a []wrapToken instead of writing into a
// cappedWriter. Text runs become text tokens. <br> becomes a brk token.
// Block children, tables, and lists become box tokens with explicit
// surrounding brk tokens, so they always occupy their own line regardless of
// their own height.
//
// Inline and inline-block children are rendered recursively, via the string
// shim, since hyperlink-wrap and inline-block width padding are still string
// operations, and folded back in as either a text token, for plain inline
// content with no embedded newline, or a box token, for inline-block content
// or plain inline content that happens to contain an embedded newline from a
// nested block-in-inline or <pre>. Box tokens alone decide whether something
// shares a line or forces a break, based purely on their own height, per
// wordWrapTokens.
func (r *Engine) renderInlineAccTokens(n *html.Node, acc inlineStyle, availWidth int) []wrapToken {
	return r.renderInlineAccTokensSeeded(n, acc, availWidth, 0, false)
}

// renderInlineAccTokensSeeded is renderInlineAccTokens's real implementation,
// with (seedLastRune, seedOK) carrying the last rune already emitted by the
// enclosing token stream. seedOK is false at true top-level and line-start
// callers. The splice branches below, display:contents and plain inline
// elements like <span>, are transparent to whitespace collapsing, their
// content being just as much "mid-line" as their parent's, so they must seed
// their recursive call with the outer stream's own trailing state rather than
// starting fresh as if their first text node were at the start of a line.
// Without this, "Google<span> search</span>" loses its leading space: the
// nested call's own tokens starts empty, so its leading-space trim below
// mistakes that emptiness for atLineStart instead of consulting what
// "Google" left behind.
func (r *Engine) renderInlineAccTokensSeeded(n *html.Node, acc inlineStyle, availWidth int, seedLastRune rune, seedOK bool) []wrapToken {
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

	if bd := r.pseudoElemDecls(n, "before", customPropSubset(nDecls)); len(bd) > 0 {
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

	// effectiveSeed reports the last-rune state a nested splice call should
	// itself be seeded with: this frame's own accumulated tokens if it has
	// emitted anything yet, otherwise this frame's own seed passed through
	// unchanged. Without falling through here, a second level of splicing,
	// such as an <em> nested inside a <span>, both transparent to whitespace
	// collapsing, would see this frame's tokens still empty and stop the
	// seed right there instead of relaying what "Google" left behind two
	// levels up.
	effectiveSeed := func() (rune, bool) {
		if lr, ok := lastRune(tokens); ok {
			return lr, ok
		}
		return seedLastRune, seedOK
	}

	// pushBoxDirect appends a box token, plus its own descendants'
	// subPositions if any, followed by one mandatory trailing brk, after
	// first ensuring at least minBreaksBefore breaks precede it. A value of 0
	// means "just make sure something separates it from any preceding
	// content", the token-domain replacement for cappedWriter's "ensure
	// newline before" and margin-top checks. This is how block children,
	// tables, and lists all force their own line regardless of their own
	// rendered height, while still carrying any trackable descendants'
	// positions along (see wraptoken.go's Rect doc comment) rather than
	// flattening to a string first and losing them.
	//
	// It is unconditional even when tokens is still empty, meaning this box
	// is the first thing here apart from n's own margin-top pseudo-content.
	// That leading separator is n's first child's margin-top, and it must
	// show up as real leading brk tokens rather than being silently skipped,
	// so that renderBlockContentBox's leading-margin collapse (block.go), the
	// mirror of its existing trailing-margin collapse, can recover it as
	// n's own collapsed margin-top. Callers with no notion of margin
	// collapse — renderInlineAcc, the string shim behind list.go and
	// table_render.go — discard this same boundary noise via
	// trimBoundaryBreaks instead of recovering it; see its doc comment.
	pushBoxDirect := func(bx box, subPositions map[*html.Node]Rect, minBreaksBefore int, node *html.Node) {
		tokens = ensureBreaks(tokens, minBreaksBefore+1)
		tokens = append(tokens, wrapToken{box: &bx, node: node, subPositions: subPositions})
		tokens = append(tokens, wrapToken{brk: true})
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			normalized := applyTextTransform(normalizeWhiteSpace(sanitizeTerminalText(c.Data, true), ws, tabSize), tt)
			if normalized != "" {
				lr, ok := lastRune(tokens)
				if !ok {
					lr, ok = seedLastRune, seedOK
				}
				atLineStart := !ok || lr == '\n'
				prevIsSpace := ok && lr == ' '
				if (atLineStart || prevIsSpace) && ws != "pre" && ws != "pre-wrap" {
					normalized = strings.TrimLeft(normalized, " ")
				}
				tokens = appendText(tokens, acc, normalized, r.profile)
			}
		case html.RawNode:
			// c.Data is inserted verbatim: no sanitizeTerminalText, no
			// whitespace normalization, and no inline styling, with acc
			// deliberately ignored unlike the TextNode case above. See
			// html.RawNode's doc comment and Document.SetPreRendered. The
			// parser, html.Parse and ParseFragment, never produces a RawNode.
			// Only application code that explicitly built one can reach
			// this branch, so c.Data is trusted by construction, by which
			// code path put it in the tree, not by any property of the
			// string itself.
			if c.Data != "" {
				for i, part := range strings.Split(c.Data, "\n") {
					if i > 0 {
						tokens = append(tokens, wrapToken{brk: true})
					}
					if part != "" {
						tokens = append(tokens, wrapToken{text: part})
					}
				}
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
			if childDecls["display"] == "none" || r.outOfFlow[c] {
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
					var bx box
					var tablePositions map[*html.Node]Rect
					if r.measuringNaturalWidth {
						// A discardable measurement trial (see
						// measureCellNaturalWidth) only ever needs this
						// nested table's own natural WIDTH, not its actual
						// rendered content. measureTableWidth computes just
						// that number via the same recursive sizing math,
						// without a full renderTable pass, which would
						// itself commit to fully rendering every cell only
						// to have the whole string thrown away here. This
						// is what keeps measuring a deeply nested document
						// linear in nesting depth instead of exponential:
						// without it, every level would both measure AND
						// fully render its descendants, and that doubling
						// compounds at every level.
						bx = newBox(strings.Repeat(" ", r.measureTableWidth(c)))
					} else {
						tableWidth := availWidth
						if r.nestedTableWidthSet {
							tableWidth = r.nestedTableWidth
						}
						tableContent, pos := r.renderTable(c, tableWidth)
						tablePositions = pos
						bx = newBox(strings.TrimSuffix(tableContent, "\n"))
					}
					if isHiddenVisibility(childDecls["visibility"]) {
						bx = blankVisibleContentBox(bx)
					}
					pushBoxDirect(bx, tablePositions, parseMargin(childDecls["margin-top"]), c)
					tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
				} else {
					savedDepth := r.quoteDepth
					bx, subPositions := r.renderBlockContentBox(c, childDecls, availWidth)
					if isHiddenVisibility(childDecls["visibility"]) {
						r.quoteDepth = savedDepth
						bx = blankVisibleContentBox(bx)
					}
					pushBoxDirect(bx, subPositions, parseMargin(childDecls["margin-top"]), c)
					tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
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
				if isHiddenVisibility(childDecls["visibility"]) {
					r.quoteDepth = savedDepth
					bx = blankVisibleContentBox(bx)
				}
				pushBoxDirect(bx, subPositions, parseMargin(childDecls["margin-top"]), c)
				tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
			case "flex":
				savedDepth := r.quoteDepth
				bx, subPositions := r.renderFlexContentBox(c, childDecls, availWidth)
				if isHiddenVisibility(childDecls["visibility"]) {
					r.quoteDepth = savedDepth
					bx = blankVisibleContentBox(bx)
				}
				pushBoxDirect(bx, subPositions, parseMargin(childDecls["margin-top"]), c)
				tokens = ensureBreaks(tokens, parseMargin(childDecls["margin-bottom"])+1)
			case "contents":
				// The element generates no box of its own (no margin/
				// padding/border/background, no forced line break); its
				// children are spliced directly into this level's stream as
				// if they were direct children of n, same as the plain
				// inline "default" case below, but skipping that case's
				// inline-block/<a> box-generating special-cases entirely -
				// contents never gets its own box or hyperlink wrap.
				childAcc := mergeContentsInlineStyle(acc, childDecls)
				savedDepth := r.quoteDepth
				childSeedLR, childSeedOK := effectiveSeed()
				childTokens := r.renderInlineAccTokensSeeded(c, childAcc, availWidth, childSeedLR, childSeedOK)
				// childTokens starts fresh from c's own first child's
				// perspective, so its leading brk tokens (that child's own
				// margin-top, always emitted now; see pushBoxDirect) don't
				// know anything about what precedes the contents wrapper
				// itself in tokens. Since a display:contents element
				// generates no box to collapse its own margin into, resolve
				// that here against the real outer context instead: nothing
				// to separate from yet (tokens empty) discards it, same as
				// any other first-content-in-the-document case; otherwise
				// re-apply it against tokens directly, exactly as if c were
				// truly n's own next child (which, per display:contents, it
				// is).
				leading := leadingBreaks(childTokens)
				childTokens = childTokens[leading:]
				if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
					childTokens = childTokens[:len(childTokens)-1]
				}
				if isHiddenVisibility(childDecls["visibility"]) {
					r.quoteDepth = savedDepth
					childTokens = blankVisibleContentTokens(childTokens)
				}
				if hasContent(tokens) {
					tokens = ensureBreaks(tokens, leading)
				}
				tokens = append(tokens, childTokens...)
			default:
				if display == "inline-block" || display == "inline-flex" || c.Data == "a" {
					// inline-block, which includes <input>, always inline-block
					// per the UA stylesheet, along with inline-flex and <a>,
					// stay string-based. An inline-block or inline-flex's
					// content is deliberately one atomic unit regardless of
					// what's inside it, and a hyperlink needs whole-string
					// OSC8 wrapping. Neither is worth the complexity of a
					// token-level equivalent, given how rarely either wraps
					// a further trackable descendant, such as a second form
					// control, in practice. This is an accepted
					// position-tracking gap for that specific, uncommon
					// combination.
					childAcc := mergeInlineStyle(acc, childDecls)
					savedDepth := r.quoteDepth
					var inner string
					switch text, synthesized := r.synthesizedControlText(c, childDecls, childAcc, availWidth); {
					case synthesized:
						// <input>, <select>, <progress>, and <meter> build their
						// content from attributes rather than from child
						// nodes; see synthesizedControlText (formcontrol.go).
						inner = text
					case display == "inline-flex":
						inner = r.renderInlineFlexContent(c, childDecls, availWidth)
					default:
						// TrimSuffix, at most one "\n". A nested recursive
						// call whose own last child was block-ish, such as an
						// implicit <tbody> wrapping <tr>, can end in a
						// trailing structural newline that was only ever
						// meaningful as pending accumulator state, never real
						// content. pushBox already trims this for its own
						// inputs, and this mirrors that for the plain-inline
						// and inline-block path. Only one is trimmed, not all
						// trailing newlines, since a further one could be a
						// real trailing blank line, from padding-bottom say.
						childToks := r.renderInlineAccTokens(c, childAcc, availWidth)
						childToks = restyleTrailingWhitespaceOnlyToken(childToks, acc, r.profile)
						inner = strings.TrimSuffix(tokensToString(childToks), "\n")
					}
					if display == "inline-block" || display == "inline-flex" {
						if colWidth, constrained := resolveWidthConstraints(childDecls, r.width, maxVisibleLineWidth(inner)); constrained && colWidth > 0 {
							inner = padLinesToWidth(inner, colWidth)
						}
					}
					if isHiddenVisibility(childDecls["visibility"]) {
						r.quoteDepth = savedDepth
						inner = blankVisibleContent(inner)
					}
					if c.Data == "a" {
						inner = r.wrapHyperlink(nodeAttr(c, "href"), inner)
					}
					switch {
					case inner == "":
						// nothing to add
					case display == "inline-block" || display == "inline-flex" || strings.Contains(inner, "\n"):
						bx := newBox(inner)
						tokens = append(tokens, wrapToken{box: &bx, node: c})
					default:
						tokens = append(tokens, wrapToken{text: inner})
					}
				} else {
					// Plain inline, meaning span, em, strong, label, and the
					// like. Splice the child's own tokens directly into this
					// level's stream instead of flattening to a string first.
					// That is the only way a trackable descendant, such as an
					// <input> inside a <label>, keeps its box-token identity,
					// and so its position, through to whichever ancestor's
					// wordWrapTokens call ultimately places it. It also
					// preserves word-wrap-ability across this element's own
					// boundary, matching docs/RENDERING.md's original
					// token-splicing intent for plain inline content
					// (findings #3 and #4) more closely than the
					// flatten-then-rebox approach the other branch uses.
					childAcc := mergeInlineStyle(acc, childDecls)
					savedDepth := r.quoteDepth
					childSeedLR, childSeedOK := effectiveSeed()
					childTokens := r.renderInlineAccTokensSeeded(c, childAcc, availWidth, childSeedLR, childSeedOK)
					// Trim one trailing brk, mirroring the TrimSuffix quirk
					// on the string-based branch above: a nested block-ish
					// descendant's own mandatory trailing brk is structural,
					// not content, and would otherwise closeAndPush a
					// spurious blank line once these tokens reach a
					// wordWrapTokens call.
					if len(childTokens) > 0 && childTokens[len(childTokens)-1].brk {
						childTokens = childTokens[:len(childTokens)-1]
					}
					childTokens = restyleTrailingWhitespaceOnlyToken(childTokens, acc, r.profile)
					if isHiddenVisibility(childDecls["visibility"]) {
						r.quoteDepth = savedDepth
						childTokens = blankVisibleContentTokens(childTokens)
					}
					tokens = append(tokens, childTokens...)
				}
			}
		case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode:
			// nothing to render
		}
	}

	if ad := r.pseudoElemDecls(n, "after", customPropSubset(nDecls)); len(ad) > 0 {
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
