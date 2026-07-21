package render

import (
	"strings"

	"golang.org/x/net/html"
)

// wrapToken is one atomic unit of inline content for wordWrapTokens: either a
// run of plain text, a forced structural line break (<br>), or a fully
// rendered, already-positioned sub-box (a block-in-inline or inline-block
// child) that gets placed whole rather than re-flowed.
//
// Exactly one of text/brk/box is set per token.
type wrapToken struct {
	text string     // plain text token (word-level tokenization happens in wordWrapTokens)
	brk  bool       // forced line break, e.g. <br>
	box  *box       // pre-rendered sub-box; nil unless this token is a box
	node *html.Node // originating node, for position tracking (nil if not needed)

	// subPositions carries box's own descendants' positions, relative to
	// box's own (0,0) origin (top-left of box.lines, as returned by
	// whichever function produced it — e.g. renderBlockContentBox). nil if
	// box has no trackable descendants. wordWrapTokens shifts these by
	// wherever it ultimately places this token and merges them into its own
	// returned position map — this is the "propagated incrementally, one
	// level at a time" mechanism docs/RENDERING.md's Position tracking section
	// describes: nothing needs to know the absolute position of anything
	// until the walk reaches the document root.
	subPositions map[*html.Node]Rect
}

// Rect is a box's position and size relative to whatever coordinate space it
// was recorded in — see docs/RENDERING.md's "Position tracking" section. It
// approximates the CSS border box (content+padding+border): horizontal
// margin, when an element sets margin-left/right explicitly, is baked into
// its box the same way padding is (see renderBlockContentBox) and is not
// currently subtracted back out, so Rect may include a few extra columns of
// margin overlap for such elements. Vertical margin is unaffected (it's
// injected as separate blank lines around a box, never baked into it). This
// is an accepted simplification: the primary motivating use (hit-testing
// form controls — see docs/INTERACTIVE.md) essentially never sets margin on
// input/button/textarea.
type Rect struct {
	Row, Col      int
	Width, Height int
}

// mergePositions copies src into dst, shifting every entry by (dRow, dCol) —
// used when a box token gets placed at (dRow, dCol) within a larger
// composition, to carry its own descendants' positions along with it.
func mergePositions(dst map[*html.Node]Rect, src map[*html.Node]Rect, dRow, dCol int) map[*html.Node]Rect {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[*html.Node]Rect, len(src))
	}
	for n, rect := range src {
		rect.Row += dRow
		rect.Col += dCol
		dst[n] = rect
	}
	return dst
}

// hasContent reports whether any token has been collected yet — the direct
// replacement for cappedWriter.Len() > 0's use inside renderInlineAcc/render.go
// for "has anything been written so far" checks (e.g. suppressing margin-top
// on the very first child).
func hasContent(tokens []wrapToken) bool {
	return len(tokens) > 0
}

// lastRune returns the last visible rune the token stream would produce, and
// whether there's any content at all — the direct replacement for
// cappedWriter.LastByte()'s use for CSS whitespace-collapse decisions (is the
// next text node arriving at a line start, or right after a space). A text
// token's content may itself carry ANSI styling (appendTextSegment no longer
// keeps trailing spaces unstyled), so this strips escape sequences first
// rather than trusting the token's raw last byte/rune.
func lastRune(tokens []wrapToken) (r rune, ok bool) {
	if len(tokens) == 0 {
		return 0, false
	}
	last := tokens[len(tokens)-1]
	switch {
	case last.brk:
		return '\n', true
	case last.box != nil:
		// Opaque: a box token is never itself a space or a line-start marker.
		return 0, true
	default:
		rs := []rune(stripANSI(last.text))
		if len(rs) == 0 {
			return 0, false
		}
		return rs[len(rs)-1], true
	}
}

// trailingBreaks counts consecutive brk tokens at the end of tokens.
func trailingBreaks(tokens []wrapToken) int {
	n := 0
	for i := len(tokens) - 1; i >= 0 && tokens[i].brk; i-- {
		n++
	}
	return n
}

// leadingBreaks counts consecutive brk tokens at the start of tokens — the
// mirror image of trailingBreaks, used the same way on the other edge.
func leadingBreaks(tokens []wrapToken) int {
	n := 0
	for n < len(tokens) && tokens[n].brk {
		n++
	}
	return n
}

// trimBoundaryBreaks strips leading and trailing brk tokens from tokens,
// returning the trimmed slice along with how many were stripped from each
// end. A block/flex child's margin-top/margin-bottom (pushBoxDirect,
// renderInlineAccTokens's nested "block"/"flex" cases) always shows up as
// leading/trailing brk tokens when that child is first/last, even though no
// preceding/following content exists yet to separate from — every consumer
// of renderInlineAccTokens's result needs to either discard that boundary
// noise (renderInlineAcc, the string shim used by list.go/table_render.go,
// which has no notion of margin collapse) or recover it as a collapsed
// margin on its own container (renderBlockContentBox, block.go) — the
// leading/trailing counts returned here are what let the latter do that.
func trimBoundaryBreaks(tokens []wrapToken) (trimmed []wrapToken, leading, trailing int) {
	leading = leadingBreaks(tokens)
	tokens = tokens[leading:]
	trailing = trailingBreaks(tokens)
	tokens = tokens[:len(tokens)-trailing]
	return tokens, leading, trailing
}

// ensureBreaks appends brk tokens so at least n consecutive breaks trail the
// stream, raising (never lowering) whatever's already there — the direct
// token-domain replacement for cappedWriter.WriteAtLeastNewlines(n), used by
// renderInlineAcc's block-child margin-collapse handling (margin-top/bottom
// arithmetic is real per-call arithmetic here rather than deferred buffering,
// since renderInlineAcc no longer has a cappedWriter to defer it to).
func ensureBreaks(tokens []wrapToken, n int) []wrapToken {
	for trailingBreaks(tokens) < n {
		tokens = append(tokens, wrapToken{brk: true})
	}
	return tokens
}

// trimTrailingBreaksAndSpace drops trailing brk tokens and trims trailing
// spaces from the last text token, repeating until neither applies — the
// token-domain equivalent of strings.TrimRight(s, "\n "). A trailing box
// token stops the trim (its content is never touched).
func trimTrailingBreaksAndSpace(tokens []wrapToken) []wrapToken {
	for len(tokens) > 0 {
		last := len(tokens) - 1
		switch {
		case tokens[last].brk:
			tokens = tokens[:last]
		case tokens[last].box == nil:
			trimmed := strings.TrimRight(tokens[last].text, " ")
			if trimmed == tokens[last].text {
				return tokens
			}
			if trimmed == "" {
				tokens = tokens[:last]
			} else {
				tokens[last] = wrapToken{text: trimmed}
			}
		default:
			return tokens
		}
	}
	return tokens
}

// coalesceTextRuns merges consecutive text tokens into one combined text
// token each, leaving brk/box tokens as their own atomic entries. Adjacent
// text tokens must be merged before word-tokenization (not tokenized
// independently), since two sibling inline elements with no whitespace
// between them (e.g. "<b>foo</b>bar") must wrap as a single word "foobar".
func coalesceTextRuns(tokens []wrapToken) []wrapToken {
	out := make([]wrapToken, 0, len(tokens))
	i := 0
	for i < len(tokens) {
		if tokens[i].brk || tokens[i].box != nil {
			out = append(out, tokens[i])
			i++
			continue
		}
		var sb strings.Builder
		for i < len(tokens) && !tokens[i].brk && tokens[i].box == nil {
			sb.WriteString(tokens[i].text)
			i++
		}
		out = append(out, wrapToken{text: sb.String()})
	}
	return out
}

// tokensToString flattens tokens back to a raw, unwrapped string: text
// tokens verbatim, brk as "\n", box tokens as their own lines joined by "\n"
// — a faithful reconstruction of what the pre-token cappedWriter-based
// renderInlineAcc used to produce (no width-based wrapping is applied here;
// that only happens via wordWrapTokens, at whichever call site is ready to
// consume tokens directly). Used by renderInlineAcc's string-signature shim
// for callers not yet migrated to tokens (list.go, table_render.go).
func tokensToString(tokens []wrapToken) string {
	var sb strings.Builder
	for _, t := range tokens {
		switch {
		case t.brk:
			sb.WriteByte('\n')
		case t.box != nil:
			sb.WriteString(t.box.join())
		default:
			sb.WriteString(t.text)
		}
	}
	return sb.String()
}

// naturalWidthCap is a sentinel width used to measure a token stream's
// natural (unwrapped) width: large enough that no text-driven wrapping ever
// occurs, so the only line breaks that happen are the structural ones brk/
// multi-line-box tokens force regardless of width — exactly what "natural
// width" means (the widest of whatever lines already exist from explicit
// structure, matching maxVisibleLineWidth's string-domain equivalent).
const naturalWidthCap = 1 << 30

// measureBlockWidthCap bounds the availWidth a block/flex container ever
// resolves its own width against while measuringNaturalWidth is set (see
// measureCellNaturalWidth). Unlike inline text - where handing wordWrapTokens
// naturalWidthCap as a wrap budget is free, since it only ever suppresses
// width-driven line breaks - a block's default (unconstrained) width is
// literally its container's width (real CSS block behavior: width:auto
// fills the containing block), so renderBlockContentBox/renderFlexContentBox
// resolve their own hBorderWidth directly from availWidth. Handed
// naturalWidthCap (1<<30) as that availWidth, they'd size themselves to
// roughly a billion columns and then materialize that many characters of
// padding the moment anything needs aligning (text-align, a closed border
// box, etc.) - not an infinite loop, but a multi-second-to-multi-minute hang
// from sheer string size. 1<<16 is still far larger than any real cell's
// content could plausibly need (so it essentially never changes a genuine
// measurement), while keeping every string operation derived from it cheap.
const measureBlockWidthCap = 1 << 16

// tokensNaturalWidth returns the width tokens would need if never
// text-wrapped — the token-domain equivalent of maxVisibleLineWidth(text).
func tokensNaturalWidth(tokens []wrapToken) int {
	b, _ := wordWrapTokens(tokens, naturalWidthCap, "", 0)
	return b.width
}

// blankVisibleContentTokens is blankVisibleContent's token-domain
// equivalent: text tokens are blanked (ANSI stripped, every rune replaced
// with a space), box tokens have blankVisibleContentBox applied, and brk
// tokens are left untouched (they carry no visible content to blank).
func blankVisibleContentTokens(tokens []wrapToken) []wrapToken {
	out := make([]wrapToken, len(tokens))
	for i, t := range tokens {
		switch {
		case t.brk:
			out[i] = t
		case t.box != nil:
			bx := blankVisibleContentBox(*t.box)
			out[i] = wrapToken{box: &bx, node: t.node}
		default:
			out[i] = wrapToken{text: blankLineVisible(t.text)}
		}
	}
	return out
}

// wordWrapTokens is wordWrapANSI's generalization: it greedily fills lines of
// at most width visible columns from a mixed stream of text/brk/box tokens.
// text tokens are word-wrapped exactly as wordWrapANSI does (reusing
// splitANSITokens + the same fill/break-word/break-all logic); a brk token
// always ends the current line; a box token is placed whole — single-line
// boxes behave like an atomic word (can share a line with surrounding text),
// multi-line boxes force a line break before and after themselves and
// contribute their own lines verbatim (no reflow), per docs/RENDERING.md's stated
// scope of not flowing text around a tall embedded object. A box wider than
// width is clipped (overflow:hidden semantics), matching truncateToWidth's
// use elsewhere for explicit-width overflow.
//
// firstLineWidth, when > 0, constrains fill width until the first
// structural break (a brk token, or a multi-line box) is reached — not just
// the first output line, since a width-driven wrap within that first
// segment still needs the narrower width (e.g. a list item's prefix narrows
// every wrapped line of the item's first paragraph, but a second paragraph
// after a nested block/<br> is unconstrained by the prefix). 0 means "same
// as width". It affects only the text/box fit-checks, not break-word/
// break-all's internal hard-split width, which is never combined with a
// first-line width by any current caller.
//
// positions records each box token's placement (Row/Col/Width/Height)
// relative to this call's own output — not yet absolute document
// coordinates; callers up the composition chain shift these by their own
// offset as they embed this result into a parent (see docs/RENDERING.md's
// "Position tracking" section).
func wordWrapTokens(tokens []wrapToken, width int, breakMode string, firstLineWidth int) (box, map[*html.Node]Rect) {
	if width <= 0 {
		width = 10
	}
	positions := map[*html.Node]Rect{}

	coalesced := coalesceTextRuns(tokens)

	// firstSegmentDone flips true the first time a structural break (brk, or
	// a multi-line box) is processed — until then, curWidth reports
	// firstLineWidth for every line, not just the first.
	firstSegmentDone := false
	curWidth := func() int {
		if !firstSegmentDone && firstLineWidth > 0 {
			return firstLineWidth
		}
		return width
	}

	// Fast path: a single text-only run (no brk/box at all) that fits within
	// its line's width needs no tokenizing at all — mirrors wordWrapANSI's
	// own fits-on-one-line early exit. Only valid with no brk/box tokens
	// present, since those always force a break regardless of width.
	if len(coalesced) == 1 && coalesced[0].box == nil && !coalesced[0].brk {
		if ansiVisibleLen(coalesced[0].text) <= curWidth() {
			return box{lines: []string{coalesced[0].text}, width: ansiVisibleLen(coalesced[0].text)}, positions
		}
	}

	var outLines []string
	var outPre []bool
	anyPre := false
	var cur strings.Builder
	curLen := 0
	curPre := false
	var carry ansiCarry
	freshLine := true

	// pushLine appends both a line and its pre-flag in lockstep — every
	// outLines append in this function goes through here so the two slices
	// never drift out of alignment.
	pushLine := func(line string, isPre bool) {
		outLines = append(outLines, line)
		outPre = append(outPre, isPre)
		if isPre {
			anyPre = true
		}
	}

	// boxJustClosed is true immediately after a multi-line box's lines were
	// appended directly (bypassing cur entirely, unlike a single-line glued
	// box which accumulates into cur). The mandatory brk that always follows
	// a box token (renderInlineAccTokens's pushBox, root-level table/list/
	// block handling) exists to guarantee "something separates this from
	// what follows" — for a single-line glued box that's cur's own pending
	// content, finalized into a real line by the first closeAndPush; for a
	// multi-line box, that separation is already structurally established
	// (its own last line already ended it, and freshLine is already true),
	// so that first closeAndPush must be a no-op, not push a spurious blank
	// line. A *second* brk in the same run still pushes a real blank line —
	// consecutive <br><br> after a box is exactly as intentional as after
	// any other content.
	boxJustClosed := false
	closeAndPush := func() {
		if cur.Len() == 0 && boxJustClosed {
			boxJustClosed = false
			return
		}
		boxJustClosed = false
		if !carry.empty() {
			cur.WriteString(carry.closeSeq())
		}
		pushLine(cur.String(), curPre)
		cur.Reset()
		curLen = 0
		curPre = false
		freshLine = true
	}
	ensureOpen := func() {
		if freshLine {
			if !carry.empty() {
				cur.WriteString(carry.openSeq())
			}
			freshLine = false
		}
	}
	// placeWord places tok on the fill (no position is recorded for plain
	// text words — only box tokens carry a node worth tracking a Rect for).
	// glueLeft suppresses the automatic word-separator space: valid only
	// between two words split from the same coalesced text run (where a real
	// source space justified it) — the first word of a run immediately
	// following a box token has no such justification and must glue to it.
	placeWord := func(tok string, glueLeft bool) {
		vl := ansiVisibleLen(tok)
		space := " "
		if curLen == 0 || glueLeft {
			space = ""
		}
		if curLen+len(space)+vl > curWidth() && curLen > 0 {
			closeAndPush()
			space = ""
		}
		if breakMode == "break-word" && vl > width {
			if curLen > 0 {
				closeAndPush()
			}
			chunks, endCarry := splitAtVisualWidthCarry(tok, width, carry)
			carry = endCarry
			for k, chunk := range chunks {
				if k < len(chunks)-1 {
					pushLine(chunk, false)
				} else {
					cur.WriteString(chunk)
					curLen = ansiVisibleLen(chunk)
					freshLine = false
				}
			}
			return
		}
		ensureOpen()
		cur.WriteString(space + tok)
		curLen += len(space) + vl
		carry.scan(tok)
	}

	// placeGlued places a single-line box token directly adjacent to
	// whatever precedes/follows it — unlike placeWord, it never inserts an
	// automatic space, since a box token boundary (an element boundary)
	// doesn't imply source whitespace the way a splitANSITokens word
	// boundary does; any real whitespace there is already its own preceding
	// or following text token. Its content is already fully self-styled, so
	// it neither reopens the surrounding carry nor scans into it. isPre
	// marks the whole resulting line pre if this glued box itself was pre.
	placeGlued := func(line string, w int, isPre bool) (row, col int) {
		if curLen+w > curWidth() && curLen > 0 {
			closeAndPush()
		}
		row, col = len(outLines), curLen
		cur.WriteString(line)
		curLen += w
		if isPre {
			curPre = true
		}
		freshLine = false
		return row, col
	}

	placeBreakAll := func(text string) {
		if curLen > 0 {
			closeAndPush()
		}
		chunks, endCarry := splitAtVisualWidthCarry(text, width, carry)
		carry = endCarry
		for k, chunk := range chunks {
			if k < len(chunks)-1 {
				pushLine(chunk, false)
			} else {
				cur.WriteString(chunk)
				curLen = ansiVisibleLen(chunk)
				freshLine = false
			}
		}
	}

	for _, t := range coalesced {
		switch {
		case t.brk:
			closeAndPush()
			firstSegmentDone = true
		case t.box != nil:
			bx := *t.box
			lines := bx.lines
			w := bx.width
			// A box wider than width is embedded as-is, not clipped: it has
			// already made its own overflow decision (e.g. renderBlockContentBox
			// only clips when overflow:hidden and an explicit width are both
			// set); a box that's simply wider because its content couldn't or
			// didn't need to break (overflow-wrap:normal and an unbreakable
			// word, for instance) must be allowed to overflow its container
			// the same way any other CSS content does by default.
			if len(lines) > 1 {
				if curLen > 0 {
					closeAndPush()
				}
				startRow := len(outLines)
				for i, ln := range lines {
					pushLine(ln, len(bx.pre) > i && bx.pre[i])
				}
				if t.node != nil {
					positions[t.node] = Rect{Row: startRow, Col: 0, Width: w, Height: len(lines)}
				}
				positions = mergePositions(positions, t.subPositions, startRow, 0)
				// Force a break after: next content must start a fresh line.
				freshLine = true
				curLen = 0
				firstSegmentDone = true
				boxJustClosed = true
			} else {
				line := ""
				isPre := false
				if len(lines) == 1 {
					line = lines[0]
					isPre = len(bx.pre) > 0 && bx.pre[0]
				}
				row, col := placeGlued(line, w, isPre)
				if t.node != nil {
					positions[t.node] = Rect{Row: row, Col: col, Width: w, Height: 1}
				}
				positions = mergePositions(positions, t.subPositions, row, col)
			}
		default:
			if t.text == "" {
				continue
			}
			if breakMode == "break-all" {
				placeBreakAll(t.text)
				continue
			}
			// A run starting fresh (nothing pending on the current line)
			// that fits verbatim is placed as-is, whitespace untouched —
			// mirrors wordWrapANSI's own fits-on-one-line early exit, applied
			// per coalesced run rather than only when there's a single run
			// overall. This matters for exact-whitespace content (pre-line/
			// pre-wrap runs, or blanked visibility:hidden runs) whose
			// interior/leading/trailing spaces splitANSITokens would
			// otherwise collapse or drop entirely, since it tokenizes on
			// whitespace as pure word-separators.
			if curLen == 0 {
				if vl := ansiVisibleLen(t.text); vl <= curWidth() {
					ensureOpen()
					cur.WriteString(t.text)
					curLen = vl
					carry.scan(t.text)
					continue
				}
			}
			toks := splitANSITokens(t.text)
			if len(toks) == 0 {
				// t.text is pure whitespace too wide to fit verbatim above
				// (rare) — place it as one glued unit rather than silently
				// dropping it, which splitANSITokens would otherwise do.
				placeWord(t.text, true)
				continue
			}
			first := true
			for _, tok := range toks {
				placeWord(tok, first)
				first = false
			}
		}
	}
	if cur.Len() > 0 || len(outLines) == 0 {
		pushLine(cur.String(), curPre)
	}
	// Word-splitting an already-ANSI-styled, multi-span coalesced run (see
	// coalesceTextRuns) can leave dead, pointless escape sequences behind at
	// span/word boundaries — harmless to a compliant terminal but visible
	// noise; strip them per line before returning. See
	// collapseDeadANSISpans's own doc comment for why this happens.
	for i, ln := range outLines {
		outLines[i] = collapseDeadANSISpans(ln)
	}
	var pre []bool
	if anyPre {
		pre = outPre
	}
	return box{lines: outLines, width: linesWidth(outLines), pre: pre}, positions
}
