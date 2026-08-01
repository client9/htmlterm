package render

import (
	"strings"
	"unicode"

	"github.com/client9/htmlterm/internal/textcell"
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
	// box's own (0,0) origin: the top-left of box.lines, as returned by
	// whichever function produced it, such as renderBlockContentBox. nil if
	// box has no trackable descendants. wordWrapTokens shifts these by
	// wherever it ultimately places this token and merges them into its own
	// returned position map. This is the "propagated incrementally, one
	// level at a time" mechanism docs/RENDERING.md's Position tracking section
	// describes: nothing needs to know the absolute position of anything
	// until the walk reaches the document root.
	subPositions map[*html.Node]Rect
}

// Rect is a box's position and size relative to whatever coordinate space it
// was recorded in. See docs/RENDERING.md's "Position tracking" section. It
// approximates the CSS border box (content+padding+border): horizontal
// margin, when an element sets margin-left/right explicitly, is baked into
// its box the same way padding is (see renderBlockContentBox) and is not
// currently subtracted back out, so Rect may include a few extra columns of
// margin overlap for such elements. Vertical margin is unaffected, since it
// is injected as separate blank lines around a box, never baked into it.
// This is an accepted simplification: the primary motivating use is
// hit-testing form controls (see docs/INTERACTIVE.md), and those essentially
// never set margin on input/button/textarea.
type Rect struct {
	Row, Col      int
	Width, Height int
}

// mergePositions copies src into dst, shifting every entry by (dRow, dCol).
// It is used when a box token gets placed at (dRow, dCol) within a larger
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

// hasContent reports whether any token has been collected yet. It is the
// direct replacement for cappedWriter.Len() > 0's use inside
// renderInlineAcc/render.go for "has anything been written so far" checks,
// such as suppressing margin-top on the first child.
func hasContent(tokens []wrapToken) bool {
	return len(tokens) > 0
}

// lastRune returns the last visible rune the token stream would produce, and
// whether there's any content at all. It is the direct replacement for
// cappedWriter.LastByte()'s use for CSS whitespace-collapse decisions: is the
// next text node arriving at a line start, or right after a space. A text
// token's content may itself carry ANSI styling, since appendTextSegment no
// longer keeps trailing spaces unstyled, so this strips escape sequences
// first rather than trusting the token's raw last byte or rune.
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
		rs := []rune(textcell.Strip(last.text))
		if len(rs) == 0 {
			return 0, false
		}
		return rs[len(rs)-1], true
	}
}

// runLeadsWithSpace and runTrailsWithSpace report whether a coalesced text run
// begins or ends with whitespace in its source. wordWrapTokens needs both
// because splitting a run into words throws that whitespace away: it separates
// words, so it is re-inserted between them as a single space, but the space at
// either *edge* of the run separates the run from a neighboring box token, and
// there is no word boundary left to carry it. See wordWrapTokens' sepPending.
//
// Both strip ANSI styling first. A styled run's leading space can sit behind an
// SGR sequence, so testing the raw string's first byte reads the escape's `\x1b`
// and reports no space at all.
func runLeadsWithSpace(s string) bool {
	rs := []rune(textcell.Strip(s))
	return len(rs) > 0 && unicode.IsSpace(rs[0])
}

func runTrailsWithSpace(s string) bool {
	rs := []rune(textcell.Strip(s))
	return len(rs) > 0 && unicode.IsSpace(rs[len(rs)-1])
}

// trailingSpaceColumns counts the spaces a text run ends with, ignoring ANSI
// styling. wordWrapTokens uses it to remember how much of what it just wrote
// verbatim is whitespace it may drop at a soft wrap. See its trimTail.
func trailingSpaceColumns(s string) int {
	rs := []rune(textcell.Strip(s))
	n := 0
	for i := len(rs) - 1; i >= 0 && rs[i] == ' '; i-- {
		n++
	}
	return n
}

// trailingBreaks counts consecutive brk tokens at the end of tokens.
func trailingBreaks(tokens []wrapToken) int {
	n := 0
	for i := len(tokens) - 1; i >= 0 && tokens[i].brk; i-- {
		n++
	}
	return n
}

// leadingBreaks counts consecutive brk tokens at the start of tokens. It is
// the mirror image of trailingBreaks, used the same way on the other edge.
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
// leading/trailing brk tokens when that child is first or last, even though
// no preceding or following content exists yet to separate from. Every
// consumer of renderInlineAccTokens's result needs to either discard that
// boundary noise (renderInlineAcc, the string shim used by
// list.go/table_render.go, which has no notion of margin collapse) or recover
// it as a collapsed margin on its own container (renderBlockContentBox,
// block.go). The leading/trailing counts returned here are what let the
// latter do that.
func trimBoundaryBreaks(tokens []wrapToken) (trimmed []wrapToken, leading, trailing int) {
	leading = leadingBreaks(tokens)
	tokens = tokens[leading:]
	trailing = trailingBreaks(tokens)
	tokens = tokens[:len(tokens)-trailing]
	return tokens, leading, trailing
}

// ensureBreaks appends brk tokens so at least n consecutive breaks trail the
// stream, raising (never lowering) whatever's already there. It is the direct
// token-domain replacement for cappedWriter.WriteAtLeastNewlines(n), used by
// renderInlineAcc's block-child margin-collapse handling. Margin-top/bottom
// is real per-call arithmetic here rather than deferred buffering, since
// renderInlineAcc no longer has a cappedWriter to defer it to.
func ensureBreaks(tokens []wrapToken, n int) []wrapToken {
	for trailingBreaks(tokens) < n {
		tokens = append(tokens, wrapToken{brk: true})
	}
	return tokens
}

// trimTrailingBreaksAndSpace drops trailing brk tokens and trims trailing
// spaces from the last text token, repeating until neither applies. It is
// the token-domain equivalent of strings.TrimRight(s, "\n "). A trailing box
// token stops the trim; its content is never touched.
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
// text tokens must be merged before word-tokenization, not tokenized
// independently, since two sibling inline elements with no whitespace
// between them, such as "<b>foo</b>bar", must wrap as a single word "foobar".
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
// tokens verbatim, brk as "\n", box tokens as their own lines joined by
// "\n". This is a faithful reconstruction of what the pre-token
// cappedWriter-based renderInlineAcc used to produce. No width-based wrapping
// is applied here; that only happens via wordWrapTokens, at whichever call
// site is ready to consume tokens directly. Used by renderInlineAcc's
// string-signature shim for callers not yet migrated to tokens (list.go,
// table_render.go).
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
// natural (unwrapped) width. It is large enough that no text-driven wrapping
// ever occurs, so the only line breaks that happen are the structural ones
// brk and multi-line-box tokens force regardless of width. That is exactly
// what "natural width" means: the widest of whatever lines already exist from
// explicit structure, matching maxVisibleLineWidth's string-domain
// equivalent.
const naturalWidthCap = 1 << 30

// measureBlockWidthCap bounds the availWidth a block/flex container ever
// resolves its own width against while measuringNaturalWidth is set (see
// measureCellNaturalWidth). For inline text, handing wordWrapTokens
// naturalWidthCap as a wrap budget is free, since it only ever suppresses
// width-driven line breaks. A block is different: its default unconstrained
// width is its container's width, real CSS block behavior where width:auto
// fills the containing block, so renderBlockContentBox and
// renderFlexContentBox resolve their own hBorderWidth directly from
// availWidth. Handed naturalWidthCap (1<<30) as that availWidth, they'd size
// themselves to roughly a billion columns and then materialize that many
// characters of padding the moment anything needs aligning, for text-align or
// a closed border box. That is not an infinite loop, but it is a
// multi-second-to-multi-minute hang from sheer string size. 1<<16 is still
// far larger than any real cell's content could plausibly need, so it
// essentially never changes a genuine measurement, while keeping every string
// operation derived from it cheap.
const measureBlockWidthCap = 1 << 16

// tokensNaturalWidth returns the width tokens would need if never
// text-wrapped: the token-domain equivalent of maxVisibleLineWidth(text).
// Measured as non-preformatted, matching how the same tokens are rendered by
// every caller that measures them: table_render.go wraps a cell's content in
// the same mode, and its nowrap path trims each rendered line's trailing
// spaces outright. Measuring them in would size a column for columns the cell
// then doesn't paint.
func tokensNaturalWidth(tokens []wrapToken) int {
	b, _ := wordWrapTokens(tokens, naturalWidthCap, "", 0, false)
	return b.width
}

// blankVisibleContentTokens is blankVisibleContent's token-domain
// equivalent: text tokens are blanked (ANSI stripped, every rune replaced
// with a space), box tokens have blankVisibleContentBox applied, and brk
// tokens are left untouched, since they carry no visible content to blank.
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

// wordWrapTokens greedily fills lines of at most width visible columns from
// a mixed stream of text/brk/box tokens. Each token kind is handled its own
// way. text tokens are word-wrapped via textcell.SplitTokens plus the
// fill/break-word/break-all logic below; breakMode "" or "normal" means word
// boundaries only, "break-word" also hard-breaks tokens that overflow the
// width, and "break-all" breaks at any character boundary. A brk token always
// ends the current line. A box token is placed whole: single-line boxes
// behave like an atomic word and can share a line with surrounding text,
// while multi-line boxes force a line break before and after themselves and
// contribute their own lines verbatim, with no reflow, per
// docs/RENDERING.md's stated scope of not flowing text around a tall embedded
// object. A box wider than width is clipped, overflow:hidden semantics,
// matching textcell.TruncateToWidth's use elsewhere for explicit-width
// overflow.
//
// firstLineWidth, when > 0, constrains fill width until the first
// structural break (a brk token, or a multi-line box) is reached. That is
// not just the first output line, since a width-driven wrap within that
// first segment still needs the narrower width: a list item's prefix narrows
// every wrapped line of the item's first paragraph, but a second paragraph
// after a nested block or <br> is unconstrained by the prefix. 0 means "same
// as width". It affects only the text/box fit-checks, not break-word's or
// break-all's internal hard-split width, which is never combined with a
// first-line width by any current caller.
//
// Any SGR style or OSC8 hyperlink span that gets split across an inserted
// line break is closed before the break and reopened at the start of the
// next line (see textcell.Carry), so a line's own trailing padding or margin
// never inherits a style left open by a wrapped span, and every wrapped line
// of a styled or linked run remains independently styled.
//
// positions records each box token's placement (Row/Col/Width/Height)
// relative to this call's own output, not yet as absolute document
// coordinates. Callers up the composition chain shift these by their own
// offset as they embed this result into a parent. See docs/RENDERING.md's
// "Position tracking" section.
//
// preformatted marks content in a white-space mode that preserves its own
// spacing, meaning `pre` or `pre-wrap`. It changes one thing: the spaces in
// front of a brk token survive. CSS removes a line's trailing spaces at a
// soft wrap in every mode, this function's own default (see trimTail), but a
// forced break is content rather than a fit decision, and pre-wrap keeps what
// sits before it. Only renderBlockContentBox passes true. Everywhere else the
// stream is a container's own inline content, where preformatted descendants
// arrive as already-rendered box tokens, not as text tokens in a preserving
// mode.
//
// A `<td>` or `<li>` declaring pre-wrap on *itself* is the gap that leaves:
// its own inline content is wrapped as ordinary text, so the spaces before an
// explicit break in it are dropped. Sizing is why. A table column's width
// comes from tokensNaturalWidth, which measures the same tokens with no
// element to read a white-space mode from, and a column measured narrower
// than its cell then paints is worse than a lost trailing space that only a
// background-color could reveal.
func wordWrapTokens(tokens []wrapToken, width int, breakMode string, firstLineWidth int, preformatted bool) (box, map[*html.Node]Rect) {
	if width <= 0 {
		width = 10
	}
	positions := map[*html.Node]Rect{}

	coalesced := coalesceTextRuns(tokens)

	// firstSegmentDone flips true the first time a structural break (brk, or
	// a multi-line box) is processed. Until then, curWidth reports
	// firstLineWidth for every line, not just the first.
	firstSegmentDone := false
	curWidth := func() int {
		if !firstSegmentDone && firstLineWidth > 0 {
			return firstLineWidth
		}
		return width
	}

	// Fast path: a single text-only run (no brk/box at all) that fits within
	// its line's width needs no tokenizing at all. This is the same
	// fits-on-one-line early exit the per-run path below applies. It is only
	// valid with no brk/box tokens present, since those always force a break
	// regardless of width.
	if len(coalesced) == 1 && coalesced[0].box == nil && !coalesced[0].brk {
		if textcell.VisibleLen(coalesced[0].text) <= curWidth() {
			return box{lines: []string{coalesced[0].text}, width: textcell.VisibleLen(coalesced[0].text)}, positions
		}
	}

	var outLines []string
	var outPre []bool
	anyPre := false
	var cur strings.Builder
	curLen := 0
	curPre := false
	var carry textcell.Carry
	freshLine := true

	// pushLine appends both a line and its pre-flag in lockstep. Every
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
	// appended directly, bypassing cur entirely, unlike a single-line glued
	// box which accumulates into cur. The mandatory brk that always follows
	// a box token (renderInlineAccTokens's pushBox, root-level table/list/
	// block handling) exists to guarantee "something separates this from
	// what follows". For a single-line glued box that separator is cur's own
	// pending content, finalized into a real line by the first closeAndPush.
	// For a multi-line box, the separation is already structurally
	// established: its own last line already ended it, and freshLine is
	// already true. So that first closeAndPush must be a no-op, not push a
	// spurious blank line. A *second* brk in the same run still pushes a real
	// blank line, since consecutive <br><br> after a box is as intentional as
	// after any other content.
	boxJustClosed := false
	// trimTail is how many trailing space columns sitting at the end of cur
	// were written by the verbatim-run path below, which copies a text run out
	// with its own leading and trailing whitespace intact. Every other writer
	// sets it back to 0, so it never describes a box token's own content: an
	// inline-block padded out to a declared width ends in real spaces that are
	// the box, not whitespace around it, and a pre box's trailing spaces are
	// its text. Closing a line clears it along with everything else pending.
	//
	// softClose consumes it. CSS removes a line's trailing spaces at a soft
	// wrap, in every white-space mode that wraps at all, so they neither paint
	// a background nor count toward the line's width. Without this, a run short
	// enough to be copied verbatim, followed by a box too wide to fit beside
	// it, left its space stranded at the end of the line: `aaaaaa <span
	// style="display:inline-block">XX</span>` in eight columns pushed the box
	// to the next line and kept "aaaaaa " on the first, one column wider than
	// its own text, with any background-color painting that extra cell.
	// Word-split text never had the problem, since placeWord writes each
	// separator in front of the word that follows it rather than behind the
	// word before it.
	//
	// A structural break is deliberately not a soft wrap. `white-space:
	// pre-wrap` preserves the spaces before an explicit <br>, and the ones at
	// the very end of a block are already trimmed by renderBlockContentBox,
	// which knows the white-space mode this function is not told.
	trimTail := 0
	closeAndPush := func() {
		if cur.Len() == 0 && boxJustClosed {
			boxJustClosed = false
			return
		}
		boxJustClosed = false
		if !carry.Empty() {
			cur.WriteString(carry.CloseSeq())
		}
		pushLine(cur.String(), curPre)
		cur.Reset()
		curLen = 0
		curPre = false
		freshLine = true
		trimTail = 0
	}
	// softClose ends the current line at a soft wrap, a break taken to make
	// room for something rather than one the content asked for. See trimTail.
	softClose := func() {
		if trimTail > 0 {
			s := cur.String()
			trimmed := s
			for range trimTail {
				t, ok := textcell.TrimTrailingSpace(trimmed)
				if !ok {
					break
				}
				trimmed = t
			}
			if trimmed != s {
				cur.Reset()
				cur.WriteString(trimmed)
				curLen -= textcell.VisibleLen(s) - textcell.VisibleLen(trimmed)
			}
		}
		closeAndPush()
	}
	ensureOpen := func() {
		if freshLine {
			if !carry.Empty() {
				cur.WriteString(carry.OpenSeq())
			}
			freshLine = false
		}
	}
	// placeWord places tok on the fill. No position is recorded for plain
	// text words; only box tokens carry a node worth tracking a Rect for.
	// glueLeft suppresses the automatic word-separator space. It is valid
	// only between two words split from the same coalesced text run, where a
	// real source space justified it. The first word of a run immediately
	// following a box token has no such justification and must glue to it.
	placeWord := func(tok string, glueLeft bool) {
		vl := textcell.VisibleLen(tok)
		space := " "
		if curLen == 0 || glueLeft {
			space = ""
		}
		if curLen+len(space)+vl > curWidth() && curLen > 0 {
			softClose()
			space = ""
		}
		trimTail = 0
		if breakMode == "break-word" && vl > width {
			if curLen > 0 {
				softClose()
			}
			chunks, endCarry := textcell.SplitAtWidth(tok, width, carry)
			carry = endCarry
			for k, chunk := range chunks {
				if k < len(chunks)-1 {
					pushLine(chunk, false)
				} else {
					cur.WriteString(chunk)
					curLen = textcell.VisibleLen(chunk)
					freshLine = false
				}
			}
			return
		}
		ensureOpen()
		cur.WriteString(space + tok)
		curLen += len(space) + vl
		carry.Scan(tok)
	}

	// placeGlued places a single-line box token directly adjacent to
	// whatever precedes or follows it. Unlike placeWord, it never inserts an
	// automatic space, since a box token boundary is an element boundary and
	// doesn't imply source whitespace the way a textcell.SplitTokens word
	// boundary does. Its content is already fully self-styled, so it neither
	// reopens the surrounding carry nor scans into it. isPre marks the whole
	// resulting line pre if this glued box itself was pre.
	//
	// sep asks for one separating space first, for the one case where real
	// source whitespace before the box has no other way to reach the output:
	// the preceding text run ended with a space and was split into words,
	// which drops it. A space at a line start is not written, matching CSS's
	// own collapsing of one there. When the box no longer fits once the space
	// is counted, the line breaks instead: that space is a wrap opportunity,
	// and dropping it to squeeze the box onto the line would run the box into
	// the word before it.
	placeGlued := func(line string, w int, isPre bool, sep bool) (row, col int) {
		if sep && curLen > 0 {
			if curLen+1+w <= curWidth() {
				cur.WriteString(" ")
				curLen++
			} else {
				softClose()
			}
		}
		if curLen+w > curWidth() && curLen > 0 {
			softClose()
		}
		trimTail = 0
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
			softClose()
		}
		trimTail = 0
		chunks, endCarry := textcell.SplitAtWidth(text, width, carry)
		carry = endCarry
		for k, chunk := range chunks {
			if k < len(chunks)-1 {
				pushLine(chunk, false)
			} else {
				cur.WriteString(chunk)
				curLen = textcell.VisibleLen(chunk)
				freshLine = false
			}
		}
	}

	// sepPending records that the run just placed ended with source whitespace
	// that word-splitting discarded, so the next thing placed on this line has
	// to be separated from it by a space. Only a box token consults it: two
	// words always get placeWord's own separator, and a run boundary only ever
	// falls next to a brk or a box (coalesceTextRuns merges everything else).
	//
	// Without it, whitespace on either side of an atomic inline box was lost
	// whenever the neighboring text run was tokenized rather than placed
	// verbatim: `x <input> y` came out as `x ☐y`, and a run long enough to wrap
	// before an inline-block lost the space in front of it as well. The
	// verbatim path needs no flag, since it writes the run's own spaces out
	// as they stand.
	sepPending := false
	for _, t := range coalesced {
		switch {
		case t.brk:
			// A forced break is not a soft wrap, so the trailing spaces before
			// it are only dropped where the white-space mode collapses them
			// anyway. See wordWrapTokens' preformatted parameter.
			if preformatted {
				closeAndPush()
			} else {
				softClose()
			}
			firstSegmentDone = true
			// The break absorbs it: a line's own trailing space is trimmed,
			// and a leading one at the start of the next line collapses away.
			sepPending = false
		case t.box != nil:
			bx := *t.box
			lines := bx.lines
			w := bx.width
			// A box wider than width is embedded as-is, not clipped: it has
			// already made its own overflow decision. renderBlockContentBox,
			// for instance, only clips when overflow:hidden and an explicit
			// width are both set. A box that's wider because its content
			// couldn't or didn't need to break, say overflow-wrap:normal and
			// an unbreakable word, must be allowed to overflow its container
			// the same way any other CSS content does by default.
			if len(lines) > 1 {
				if curLen > 0 {
					softClose()
				}
				trimTail = 0
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
				// Same as a brk: the box ended the line it was on, so a
				// space pending in front of it has nowhere left to go.
				sepPending = false
			} else {
				line := ""
				isPre := false
				if len(lines) == 1 {
					line = lines[0]
					isPre = len(bx.pre) > 0 && bx.pre[0]
				}
				row, col := placeGlued(line, w, isPre, sepPending)
				sepPending = false
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
				// SplitAtWidth keeps the run's own characters, spaces
				// included, so nothing is left pending.
				placeBreakAll(t.text)
				sepPending = false
				continue
			}
			// A run starting fresh, with nothing pending on the current
			// line, that fits verbatim is placed as-is, whitespace
			// untouched. This is the same fits-on-one-line early exit as
			// above, applied per coalesced run rather than only when there's
			// a single run overall. It matters for exact-whitespace content,
			// such as pre-line/pre-wrap runs or blanked visibility:hidden
			// runs, whose interior, leading, and trailing spaces
			// textcell.SplitTokens would otherwise collapse or drop
			// entirely, since it tokenizes on whitespace as pure
			// word-separators.
			if curLen == 0 {
				if vl := textcell.VisibleLen(t.text); vl <= curWidth() {
					ensureOpen()
					cur.WriteString(t.text)
					curLen = vl
					carry.Scan(t.text)
					// Written out as it stands, trailing space included, so
					// there is nothing left pending. Those spaces are the
					// ones softClose drops if this line turns out to end
					// here.
					sepPending = false
					trimTail = trailingSpaceColumns(t.text)
					continue
				}
			}
			toks := textcell.SplitTokens(t.text)
			if len(toks) == 0 {
				// t.text is pure whitespace too wide to fit verbatim above,
				// which is rare. Place it as one glued unit rather than
				// silently dropping it, as textcell.SplitTokens would.
				placeWord(t.text, true)
				sepPending = false
				trimTail = trailingSpaceColumns(t.text)
				continue
			}
			// The first word of a run glues to whatever precedes it: a run
			// boundary is an element boundary, which implies no whitespace of
			// its own. Real whitespace at that boundary is the exception, and it
			// lives at the edge of the run's own text, exactly where word
			// splitting is about to discard it. So a run that starts with a
			// space takes placeWord's separator instead of gluing, and one that
			// ends with a space leaves that separator pending for whatever box
			// comes next.
			glue := !runLeadsWithSpace(t.text) && !sepPending
			for _, tok := range toks {
				placeWord(tok, glue)
				glue = false
			}
			sepPending = runTrailsWithSpace(t.text)
		}
	}
	if cur.Len() > 0 || len(outLines) == 0 {
		pushLine(cur.String(), curPre)
	}
	// Word-splitting an already-ANSI-styled, multi-span coalesced run (see
	// coalesceTextRuns) can leave dead escape sequences behind at span and
	// word boundaries. They are harmless to a compliant terminal but visible
	// noise, so strip them per line before returning. See
	// textcell.CollapseDeadSpans's own doc comment for why this happens.
	for i, ln := range outLines {
		outLines[i] = textcell.CollapseDeadSpans(ln)
	}
	var pre []bool
	if anyPre {
		pre = outPre
	}
	return box{lines: outLines, width: linesWidth(outLines), pre: pre}, positions
}
