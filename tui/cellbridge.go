package tui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/client9/htmlterm/internal/render"
	"github.com/gdamore/tcell/v3"
)

// sgrState is the running SGR/hyperlink decode state writeANSILine
// maintains while walking one already-rendered htmlterm line left to right.
type sgrState struct {
	style tcell.Style
	url   string
}

// paintLines writes lines — one already fully-rendered, self-contained ANSI
// line per Document.Render()'s output, split on "\n" — into screen via
// SetContent, one row per line. Used by Loop.paint (tcell_loop.go).
//
// nextLinkID hands out a fresh synthetic UrlId to every hyperlink opened
// anywhere in this call (see applyHyperlink), rather than reusing one id
// per distinct href. A shared id was tried first — tcell.Style.UrlId's own
// doc comment says it's meant to group a hyperlink spanning multiple lines
// under one hoverable region — but tcell v3.4.0's renderer has a confirmed
// bug where an *identical*, multi-cell style (including url+id) repeated
// across a row boundary silently fails to re-emit the OSC8 sequence for
// the second row, losing that row's hyperlink entirely, not just its
// grouping (verified directly against tcell's Screen/SetContent, with no
// htmlterm code involved). Always minting a fresh id sidesteps the bug
// category entirely (no two rows ever carry a bit-for-bit identical
// style), at the cost of the cross-line grouping tcell's own feature
// promises but doesn't reliably deliver here.
func paintLines(screen tcell.Screen, lines []string) {
	width, height := screen.Size()
	nextLinkID := 0
	for row := range height {
		var line string
		if row < len(lines) {
			line = lines[row]
		}
		writeANSILine(screen, row, line, width, &nextLinkID)
	}
}

// writeANSILine decodes one already-rendered ANSI line into cells starting
// at (0, row) via screen.SetContent, tracking SGR/hyperlink state as it
// walks left to right — reusing x/ansi's decoder (consumeANSI, below) to
// tokenize each escape sequence, the same way htmlterm's own ansiCarry
// does internally. Column position advances by render.NextRuneWidth per
// visible rune (1 normally, 2 for East Asian wide/emoji runes, matching
// vs16WidthCorrection for variation-selector pairs) — this must exactly
// match ansiVisibleLen/wordWrapTokens' own column accounting, which does
// measure double-width runes as two columns; advancing by a flat 1 here
// regardless of width (an earlier, since-corrected assumption) desynced the
// painted frame from the frame the CSS engine actually laid out and
// measured, shifting every character after a wide emoji one column left of
// where htmlterm's layout placed it — including, on lines that reached the
// pane's right edge, the scrollbar gutter itself. tcell.Screen.SetContent's
// own doc comment confirms the contract this depends on: "wide ... runes
// occupy two cells, and attempts to place a character at the next cell to
// the right will have undefined effects" — so a wide rune's second cell
// must never receive its own SetContent call, which advancing col by 2
// (skipping straight past it) guarantees.
//
// Once line's content is exhausted, every remaining column up to width is
// explicitly blanked (a space in the default style) — necessary because
// tcell's Show only redraws cells this call actually touches via
// SetContent; without this, a row that was longer on some earlier frame
// (e.g. a text input before a Backspace, or before the document reflowed
// narrower) would leave that earlier frame's trailing characters on
// screen forever, never told they're no longer part of the current line.
func writeANSILine(screen tcell.Screen, row int, line string, width int, nextLinkID *int) {
	var state sgrState
	runes := []rune(line)
	col := 0
	i := 0
	prevWidth := 0
	for i < len(runes) && col < width {
		if runes[i] == '\x1b' {
			end := consumeANSI(runes, i)
			applySequence(&state, string(runes[i:end]), nextLinkID)
			i = end
			continue
		}
		w := render.NextRuneWidth(runes[i], prevWidth)
		if w > 0 {
			screen.SetContent(col, row, runes[i], nil, state.style)
		}
		col += w
		prevWidth = w
		i++
	}
	for ; col < width; col++ {
		screen.SetContent(col, row, ' ', nil, tcell.StyleDefault)
	}
}

// consumeANSI returns the index just past the escape sequence starting at
// runes[i] (runes[i] must be '\x1b'), delegating to x/ansi's decoder rather
// than hand-rolling CSI/OSC recognition — the same approach internal/render
// takes for its own copy of this helper. runes[i:] is re-encoded to a string
// because the decoder works in bytes, and the consumed byte count is
// converted back to a rune count since writeANSILine indexes into a []rune.
func consumeANSI(runes []rune, i int) int {
	if i >= len(runes) || runes[i] != '\x1b' {
		return i + 1
	}
	s := string(runes[i:])
	_, _, n, _ := ansi.DecodeSequence(s, ansi.NormalState, nil)
	if n <= 0 {
		return i + 1
	}
	return i + utf8.RuneCountInString(s[:n])
}

// applySequence classifies one fully-extracted escape sequence (as produced
// by consumeANSI) exactly the way ansiCarry.apply does, and updates state:
// an SGR sequence updates its style, an OSC8 sequence updates its
// hyperlink. Anything else (never emitted by htmlterm's own renderer, but
// harmless if ever encountered) is left untouched.
func applySequence(state *sgrState, seq string, nextLinkID *int) {
	switch {
	case strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m"):
		applySGR(state, seq[2:len(seq)-1])
	case strings.HasPrefix(seq, "\x1b]8;"):
		applyHyperlink(state, seq, nextLinkID)
	}
}

// applySGR interprets one SGR sequence's semicolon-separated numeric
// parameters and updates state.style accordingly. Covers the full standard
// vocabulary (reset, bold/dim/italic/underline/blink/reverse/strikethrough
// and their "off" codes, basic/bright/default fg+bg, 256-color and
// truecolor fg+bg) even though style.go's inlineStyle only ever emits a
// subset of it (fg/bg color, bold, italic, underline, strikethrough) —
// cheap to handle generically and avoids silently mis-rendering if that
// vocabulary ever grows.
func applySGR(state *sgrState, params string) {
	if params == "" {
		state.style = tcell.StyleDefault
		return
	}
	parts := strings.Split(params, ";")
	codes := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p) // empty/invalid segment -> 0, matching ECMA-48's "default" meaning
		codes[i] = n
	}
	for i := 0; i < len(codes); i++ {
		switch c := codes[i]; {
		case c == 0:
			state.style = tcell.StyleDefault
		case c == 1:
			state.style = state.style.Bold(true)
		case c == 2:
			state.style = state.style.Dim(true)
		case c == 3:
			state.style = state.style.Italic(true)
		case c == 4:
			state.style = state.style.Underline(true)
		case c == 5 || c == 6:
			state.style = state.style.Blink(true)
		case c == 7:
			state.style = state.style.Reverse(true)
		case c == 9:
			state.style = state.style.StrikeThrough(true)
		case c == 22:
			state.style = state.style.Bold(false).Dim(false)
		case c == 23:
			state.style = state.style.Italic(false)
		case c == 24:
			state.style = state.style.Underline(false)
		case c == 25:
			state.style = state.style.Blink(false)
		case c == 27:
			state.style = state.style.Reverse(false)
		case c == 29:
			state.style = state.style.StrikeThrough(false)
		case c >= 30 && c <= 37:
			state.style = state.style.Foreground(tcell.PaletteColor(c - 30))
		case c == 38:
			if color, consumed := parseExtendedColor(codes[i+1:]); consumed > 0 {
				state.style = state.style.Foreground(color)
				i += consumed
			}
		case c == 39:
			state.style = state.style.Foreground(tcell.ColorDefault)
		case c >= 40 && c <= 47:
			state.style = state.style.Background(tcell.PaletteColor(c - 40))
		case c == 48:
			if color, consumed := parseExtendedColor(codes[i+1:]); consumed > 0 {
				state.style = state.style.Background(color)
				i += consumed
			}
		case c == 49:
			state.style = state.style.Background(tcell.ColorDefault)
		case c >= 90 && c <= 97:
			state.style = state.style.Foreground(tcell.PaletteColor(c - 90 + 8))
		case c >= 100 && c <= 107:
			state.style = state.style.Background(tcell.PaletteColor(c - 100 + 8))
		}
	}
}

// parseExtendedColor interprets the parameters following a 38 or 48 SGR
// code: rest[0]==5 for an 8-bit palette index (rest[1]), rest[0]==2 for a
// 24-bit truecolor triple (rest[1:4]). Returns the decoded color and how
// many further entries of rest were consumed (0 if rest doesn't match
// either known form, e.g. truncated input).
func parseExtendedColor(rest []int) (color tcell.Color, consumed int) {
	if len(rest) == 0 {
		return tcell.ColorDefault, 0
	}
	switch rest[0] {
	case 5:
		if len(rest) < 2 {
			return tcell.ColorDefault, 0
		}
		return tcell.PaletteColor(rest[1]), 2
	case 2:
		if len(rest) < 4 {
			return tcell.ColorDefault, 0
		}
		return tcell.NewRGBColor(int32(rest[1]), int32(rest[2]), int32(rest[3])), 4
	default:
		return tcell.ColorDefault, 0
	}
}

// applyHyperlink parses an OSC8 sequence (as emitted by ansi.SetHyperlink/
// ResetHyperlink, block.go's wrapHyperlink/wrapHyperlinkBox — always
// "\x1b]8;;URI\x07" or "\x1b]8;;\x07" to reset, htmlterm never emits a
// params/id segment itself) and updates state.style's Url/UrlId. An empty
// URI clears the hyperlink; otherwise *nextLinkID mints a fresh id for
// this occurrence and is incremented — see paintLines' doc comment for why
// ids are never reused across occurrences.
func applyHyperlink(state *sgrState, seq string, nextLinkID *int) {
	rest := strings.TrimPrefix(seq, "\x1b]8;")
	_, uri, ok := strings.Cut(rest, ";")
	if !ok {
		return
	}
	uri = strings.TrimSuffix(uri, "\x07")
	uri = strings.TrimSuffix(uri, "\x1b\\")
	if uri == "" {
		state.url = ""
		state.style = state.style.Url("").UrlId("")
		return
	}
	id := strconv.Itoa(*nextLinkID)
	*nextLinkID++
	state.url = uri
	state.style = state.style.Url(uri).UrlId(id)
}
