// Package textcell measures and manipulates already-rendered terminal text
// in *cells* — the unit a terminal actually charges for — rather than in
// bytes or runes.
//
// It exists because three different callers have to agree, exactly, on how
// wide a string is: internal/render, which lays content out; tui, which
// paints that layout onto a real screen and positions the hardware cursor;
// and document, which turns a click's column back into an offset in a
// value. Any disagreement between them shows up as a cursor sitting inside
// a glyph, a border landing a column past its box, or a repaint that
// smears. Keeping the measurement in one leaf package, depended on by all
// three, is what makes agreement structural instead of a convention.
//
// Everything here is a pure function of its arguments. There is deliberately
// no HTML, no CSS, and no layout in this package: callers that need to know
// what a string *means* keep that knowledge; this package only answers how
// much room it takes and where its boundaries fall.
//
// # Three units, and which one to use
//
// Mixing these up is the single most common bug this package exists to
// prevent, so they're named apart:
//
//   - bytes — Go's native string index. Never a screen position.
//   - runes — Unicode code points. What a caret offset is measured in
//     (see the caution below), and what []rune indexes.
//   - cells/columns — what a terminal charges. What Width, VisibleLen,
//     SpliceColumns and everything else here return and accept.
//
// Runes and cells coincide for ASCII and diverge everywhere else: a CJK
// ideograph is one rune and two cells; a combining accent is a second rune
// and zero extra cells; an emoji ZWJ sequence is five runes, one grapheme
// cluster, and two cells. Any arithmetic that adds a rune count to a column
// is wrong even when its tests pass, because an all-ASCII corpus can't tell
// the two apart. The repository's column-invariant suite
// (widecolumn_test.go) exists to catch exactly that.
//
// # What must NOT be folded into this package
//
// A text entry's caret and selection are rune offsets into its value, and
// they must stay that way. That is not an oversight to be tidied up: the
// DOM's selectionStart/selectionEnd/setSelectionRange are offsets into the
// value's character sequence, and document mirrors them so a caller that
// knows the web platform gets the answer it expects. Screen columns are a
// property of how that value happens to be *rendered* — they change with
// font width tables and East Asian width settings, and they don't exist at
// all for a value that was never rendered.
//
// So the conversion is a boundary, not a representation: ColumnForRuneIndex
// and RuneIndexForColumn are the only two places the units meet, and they
// are called at the edges — when placing a terminal cursor, or when turning
// a click into an offset. Everything on the document side of that boundary
// stays in runes; everything on the painting side stays in cells. "These are
// all just character counts, unify them" is the tempting wrong move, and it
// would silently break every non-ASCII selection.
package textcell

import (
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
)

// widthOptions mirrors tcell/v3's internal/widthutil.Options() byte
// for byte: tcell derives its own displaywidth.Options from the
// RUNEWIDTH_EASTASIAN env var (preserving compatibility with the old
// go-runewidth-based toggle), but that logic lives in an unimportable
// internal package. htmlterm's own width math and tcell's paint-time width
// math (tui/cellbridge.go's SetContent calls, backed by tcell's own
// displaywidth-based CellBuffer) MUST derive the same Options value under
// the same env var, or the two disagree exactly when a user sets it. If
// tcell ever changes this env var's name or semantics, this must be updated
// to match.
func widthOptions() displaywidth.Options {
	if rw := strings.ToLower(os.Getenv("RUNEWIDTH_EASTASIAN")); rw == "1" || rw == "true" || rw == "yes" {
		return displaywidth.Options{EastAsianWidth: true}
	}
	return displaywidth.Options{}
}

// maxGraphemeLookahead bounds how many runes NextGrapheme scans ahead to
// find one grapheme cluster's boundary. Real-world multi-rune clusters
// (ZWJ emoji sequences, skin-tone modifiers, regional-indicator flag pairs,
// VS16 pairs) are well under this; if a pathological input has no cluster
// boundary within the window, NextGrapheme falls back to treating the
// first rune as its own cluster rather than scanning unboundedly.
const maxGraphemeLookahead = 16

// NextGrapheme returns the terminal column width of the grapheme cluster
// starting at runes[i], and the index of the rune immediately following
// that cluster. A cluster is usually one rune, but can span several (VS16
// pairs, ZWJ sequences, regional-indicator flag pairs, skin-tone
// modifiers) — displaywidth's grapheme segmenter finds the boundary and
// scores the whole cluster as one unit, so callers no longer need to track
// a "previous rune's width" the way the old VS16-only correction did.
//
// The caller is responsible for not calling this with runes[i] == '\x1b';
// ANSI escape recognition is handled separately (see ConsumeANSI) by every
// caller of this function, so a cluster can never span into an escape
// sequence.
//
// Exported for htmlterm/tui's use: cellbridge.go's writeANSILine must walk
// an already-rendered ANSI line and paint it onto a real tcell.Screen one
// cluster at a time (not one rune at a time), advancing column position by
// exactly what this function returns — otherwise the painted frame desyncs
// from the frame this package's own layout/wrapping already measured (this
// is what previously caused a wide emoji to shift every subsequent
// character, including the scrollbar gutter, one column left of where
// htmlterm's own layout placed it).
func NextGrapheme(runes []rune, i int) (width int, next int) {
	end := min(i+maxGraphemeLookahead, len(runes))
	for j := i; j < end; j++ {
		if runes[j] == '\x1b' {
			end = j
			break
		}
	}
	s := string(runes[i:end])
	g := widthOptions().StringGraphemes(s)
	if !g.Next() {
		return 1, i + 1
	}
	cluster := g.Value()
	return g.Width(), i + utf8.RuneCountInString(cluster)
}

// Width is the terminal column width of an entire plain (no ANSI
// escapes) string: the sum of each grapheme cluster's display width (VS16
// pairs, ZWJ sequences, flag pairs, etc. already scored as one unit). No
// ANSI-escape interleaving to preserve here, unlike this file's other
// width-scanning functions, so this calls displaywidth directly instead of
// looping via NextGrapheme.
func Width(s string) int {
	return widthOptions().String(s)
}

// ColumnForRuneIndex returns the terminal column, relative to the start of
// plain (escape-free) text s, that the caret sits at when it's idx runes into
// s. Rune offsets and columns are not interchangeable — a CJK or emoji
// cluster is one offset but two columns — so every caret-placement site has
// to convert rather than using an offset as a column directly.
//
// An idx landing *inside* a multi-rune grapheme cluster resolves to that
// cluster's own starting column: a terminal cursor can't be placed halfway
// through a cluster, and rounding down keeps this the exact inverse of
// RuneIndexForColumn, so a click and the cursor it produces agree on a cell.
//
// Exported for htmlterm/tui's use (focusCursorPos), the same reason
// NextGrapheme is: the terminal's own hardware cursor has to land on the
// column this package's layout actually put that rune at.
func ColumnForRuneIndex(s string, idx int) int {
	runes := []rune(s)
	col := 0
	for i := 0; i < len(runes) && i < idx; {
		w, next := NextGrapheme(runes, i)
		if next > idx {
			break // idx is inside this cluster — stay at the cluster's start
		}
		col += w
		i = next
	}
	return col
}

// RuneIndexForColumn is ColumnForRuneIndex's inverse: the rune offset into
// plain (escape-free) text s that terminal column col falls in. A column
// landing on the trailing half of a double-width cluster resolves to that
// cluster's own offset (not the following one), so clicking either cell of a
// wide character puts the caret before it — and clicking a cell then drawing
// the cursor for the resulting caret lands back on the same cluster. A column
// past the end of s returns len([]rune(s)).
//
// Exported for document's use (caretIndexFromClick), the click-to-caret
// counterpart of tui's cursor placement.
func RuneIndexForColumn(s string, col int) int {
	runes := []rune(s)
	x := 0
	for i := 0; i < len(runes); {
		w, next := NextGrapheme(runes, i)
		if col < x+w {
			return i
		}
		x += w
		i = next
	}
	return len(runes)
}

// VisibleLen returns the number of visible (non-ANSI-escape) runes in s.
func VisibleLen(s string) int {
	n := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			i = ConsumeANSI(runes, i)
			continue
		}
		w, next := NextGrapheme(runes, i)
		n += w
		i = next
	}
	return n
}

// Carry tracks the single currently-open SGR style span and/or OSC8
// hyperlink span while scanning already-styled/hyperlinked text left to
// right, so a line break inserted mid-span (by word-wrap or a hard break)
// can close the span before the break and reopen the identical span right
// after it, making every emitted line independently self-contained.
//
// Deliberately flat (not a stack): htmlterm's styling pipeline
// (style.go's inlineStyle.render + block.go's wrapHyperlink) only ever
// emits ONE combined SGR open + ONE full reset per span, and ONE OSC8
// open + ONE OSC8 reset per hyperlink — never partial resets, never two
// concurrently-open spans of the same kind. If that invariant changes,
// this needs to become a stack.
type Carry struct {
	sgr       string // active SGR open sequence, e.g. "\x1b[4m"; "" if none
	hyperlink string // active OSC8 open sequence; "" if none
}

func (c Carry) Empty() bool { return c.sgr == "" && c.hyperlink == "" }

// CloseSeq closes whatever is open, innermost first (SGR, then OSC8).
func (c Carry) CloseSeq() string {
	if c.Empty() {
		return ""
	}
	var b strings.Builder
	if c.sgr != "" {
		b.WriteString(ansi.ResetStyle)
	}
	if c.hyperlink != "" {
		b.WriteString(ansi.ResetHyperlink())
	}
	return b.String()
}

// OpenSeq reopens whatever is open, outermost first (OSC8, then SGR).
func (c Carry) OpenSeq() string {
	if c.Empty() {
		return ""
	}
	var b strings.Builder
	if c.hyperlink != "" {
		b.WriteString(c.hyperlink)
	}
	if c.sgr != "" {
		b.WriteString(c.sgr)
	}
	return b.String()
}

// Apply classifies one fully-extracted escape sequence (as produced by
// ConsumeANSI) and updates the carry: an SGR sequence (CSI ... 'm') with
// Empty/"0" params clears the active SGR span, any other SGR sequence
// replaces it; an OSC8 sequence with an Empty URI clears the active
// hyperlink span, any other OSC8 sequence replaces it. Anything else is
// left untouched.
func (c *Carry) Apply(seq string) {
	switch {
	case isSGRSeqStr(seq):
		if sgrIsReset(seq) {
			c.sgr = ""
		} else {
			c.sgr = seq
		}
	case isOSC8SeqStr(seq):
		if osc8IsReset(seq) {
			c.hyperlink = ""
		} else {
			c.hyperlink = seq
		}
	}
}

func isSGRSeqStr(seq string) bool {
	return strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m")
}

func isOSC8SeqStr(seq string) bool {
	return strings.HasPrefix(seq, "\x1b]8;")
}

// sgrIsReset reports whether seq (a full "\x1b[...m" sequence) is a full SGR
// reset (Empty or "0" params) as opposed to a style-setting sequence.
func sgrIsReset(seq string) bool {
	params := seq[2 : len(seq)-1]
	return params == "" || params == "0"
}

// osc8IsReset reports whether seq (a full "\x1b]8;...ST" sequence) closes a
// hyperlink (Empty URI) as opposed to opening one.
func osc8IsReset(seq string) bool {
	rest := seq[len("\x1b]8;"):]
	semi := strings.IndexByte(rest, ';')
	if semi < 0 {
		return false
	}
	uri := rest[semi+1:]
	uri = strings.TrimSuffix(uri, "\x07")
	uri = strings.TrimSuffix(uri, "\x1b\\")
	return uri == ""
}

// Scan walks s in order, updating the carry for every escape sequence it
// contains (reusing ConsumeANSI's recognizer).
func (c *Carry) Scan(s string) {
	if !strings.ContainsRune(s, '\x1b') {
		return
	}
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			end := ConsumeANSI(runes, i)
			c.Apply(string(runes[i:end]))
			i = end
			continue
		}
		i++
	}
}

// CollapseDeadSpans removes SGR/OSC8 "open" escape sequences that are
// immediately (no visible character in between) superseded by a full reset
// of the same kind — i.e. spans that never got the chance to style or link
// anything. Dropping such an open and keeping only the reset is always safe
// regardless of any earlier context: a full SGR reset ("\x1b[m"/"\x1b[0m")
// or OSC8 reset (Empty-URI "\x1b]8;;...") unconditionally clears that kind
// of state no matter what, if anything, was open beforehand, so whether the
// dead open ran first makes no observable difference.
//
// This is deliberately one-directional: it does NOT collapse "open A, open
// B" (with no reset between) down to just B, even though B logically
// supersedes A — this codebase's SGR sequences are additive, not
// full-replace (see Carry's doc comment), so e.g. "\x1b[3;4m" (italic+
// underline) directly followed by "\x1b[3m" (italic) with no reset in
// between would leave underline stuck on, since there's no explicit
// "turn off underline" code involved. Only a real reset ever fully clears
// state, so only a real reset is a safe deletion trigger for what precedes it.
//
// wordWrapTokens routinely produces exactly the pattern this removes:
// coalesceTextRuns concatenates several already-independently-styled spans
// (each self-contained, from inlineStyle.render) into one string before
// SplitTokens re-tokenizes it purely on literal space characters, with
// no awareness of where one span's styling ends and the next begins. A
// closing sequence that immediately follows a space (closing the word
// before it) routinely ends up re-attached to the front of the *next*
// word's token instead, and the line-wrap carry mechanism (ensureOpen/
// closeAndPush) can then reopen a span right before that misplaced close
// cancels it out again — e.g. an <a>'s own trailing single-space text node,
// styled only with underline, sitting right at a word-wrap boundary next to
// a differently-styled neighbor. The result is well-formed but pointless
// escape sequences like "\x1b[3m\x1b[m" wrapping zero characters. This is a
// final cleanup pass, not a fix for the token misattribution itself — it
// only guarantees the misattribution can never surface as a dangling or
// Empty span in output.
func CollapseDeadSpans(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	// pendingSGROpen/pendingOSCOpen hold the out-index where a not-yet-
	// superseded SGR-open / OSC8-open sequence (respectively) starts, as
	// long as no visible character has appeared since — i.e. dropping it
	// is still safe if the very next same-kind sequence turns out to be a
	// full reset. -1 means there's no such pending, droppable open.
	pendingSGROpen, pendingOSCOpen := -1, -1
	i := 0
	for i < len(runes) {
		if runes[i] != '\x1b' {
			out = append(out, runes[i])
			pendingSGROpen, pendingOSCOpen = -1, -1
			i++
			continue
		}
		j := ConsumeANSI(runes, i)
		seq := string(runes[i:j])
		switch {
		case isSGRSeqStr(seq):
			if sgrIsReset(seq) {
				if pendingSGROpen >= 0 {
					out = out[:pendingSGROpen]
				}
				pendingSGROpen = -1
			} else {
				pendingSGROpen = len(out)
			}
			out = append(out, []rune(seq)...)
		case isOSC8SeqStr(seq):
			if osc8IsReset(seq) {
				if pendingOSCOpen >= 0 {
					out = out[:pendingOSCOpen]
				}
				pendingOSCOpen = -1
			} else {
				pendingOSCOpen = len(out)
			}
			out = append(out, []rune(seq)...)
		default:
			out = append(out, []rune(seq)...)
		}
		i = j
	}
	return string(out)
}

// SplitAtWidth is splitAtVisualWidth's carry-aware core. start is
// whatever SGR/OSC8 span is already open as of the start of s (from
// preceding text this call doesn't see); end is whatever span is left open
// at the end of s. Every internal width-driven break closes the
// currently-open span before the break and reopens the identical span
// immediately after, so every returned chunk is self-contained.
func SplitAtWidth(s string, width int, start Carry) ([]string, Carry) {
	if width <= 0 || s == "" {
		return []string{""}, start
	}
	carry := start
	var lines []string
	var cur strings.Builder
	if !carry.Empty() {
		cur.WriteString(carry.OpenSeq())
	}
	col := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		ch := runes[i]
		if ch == '\x1b' {
			j := ConsumeANSI(runes, i)
			seq := string(runes[i:j])
			cur.WriteString(seq)
			carry.Apply(seq)
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		if col+w > width && col > 0 {
			if !carry.Empty() {
				cur.WriteString(carry.CloseSeq())
			}
			lines = append(lines, cur.String())
			cur.Reset()
			col = 0
			if !carry.Empty() {
				cur.WriteString(carry.OpenSeq())
			}
		}
		cur.WriteString(string(runes[i:next]))
		col += w
		i = next
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		return []string{""}, carry
	}
	return lines, carry
}

// VisibleWindow returns the [offset, offset+width) visible-column
// window of s — wide-rune/ANSI-aware, and, like SplitAtWidth's
// internal chunk boundaries, self-contained: an SGR/OSC8 span still open at
// offset is reopened at the window's start and closed at its end. Used for
// overflow-x: scroll|auto's horizontal scroll-offset slicing (see
// docs/SCROLLING.md), where offset can land anywhere within a line — unlike
// SplitAtWidth, which only ever chops a string into sequential
// chunks from column 0. A wide rune straddling either boundary is dropped
// entirely rather than split, the same rule VisiblePrefix
// already applies at its own single (right) boundary.
func VisibleWindow(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	carry := Carry{}
	col := 0
	i := 0
	for i < len(runes) && col < offset {
		ch := runes[i]
		if ch == '\x1b' {
			j := ConsumeANSI(runes, i)
			carry.Apply(string(runes[i:j]))
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		if col+w > offset {
			i = next
			col += w
			break
		}
		col += w
		i = next
	}
	var out strings.Builder
	if !carry.Empty() {
		out.WriteString(carry.OpenSeq())
	}
	visible := 0
	for i < len(runes) && visible < width {
		ch := runes[i]
		if ch == '\x1b' {
			j := ConsumeANSI(runes, i)
			seq := string(runes[i:j])
			out.WriteString(seq)
			carry.Apply(seq)
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		if visible+w > width {
			break
		}
		out.WriteString(string(runes[i:next]))
		visible += w
		i = next
	}
	if !carry.Empty() {
		out.WriteString(carry.CloseSeq())
	}
	return out.String()
}

// SpliceColumns overwrites visible columns [col, col+width) of line with
// replacement, preserving line's ANSI styling for the untouched prefix/
// suffix and re-carrying any open span across the splice boundary — the
// primitive popup/overlay rendering needs (see docs/RENDERING.md's popup
// section) to punch replacement content into an already-composed line
// without disturbing anything outside the overwritten range, unlike
// splitAtVisualWidth/SplitAtWidth, which only chop a string into
// sequential chunks from column 0 and have no notion of an interior
// overwrite. replacement is inserted verbatim — the caller is responsible
// for it being exactly width visible columns wide (e.g. a popup's own
// already-rendered box.width) so the result's total visible width matches
// line's; SpliceColumns does not itself pad or truncate it.
//
// If line is narrower than col, it's padded with spaces so replacement
// lands at the requested column. If line is narrower than col+width,
// there's nothing past its own end to preserve, so the result simply ends
// after replacement.
func SpliceColumns(line string, col, width int, replacement string) string {
	if col < 0 || width <= 0 {
		return line
	}
	var out strings.Builder
	carry := Carry{}
	runes := []rune(line)
	i := 0
	visCol := 0
	// Copy the untouched prefix, tracking carry state as we go so it can be
	// closed cleanly right before replacement.
	for i < len(runes) && visCol < col {
		if runes[i] == '\x1b' {
			j := ConsumeANSI(runes, i)
			seq := string(runes[i:j])
			out.WriteString(seq)
			carry.Apply(seq)
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		out.WriteString(string(runes[i:next]))
		i = next
		visCol += w
	}
	for visCol < col {
		out.WriteByte(' ')
		visCol++
	}
	if !carry.Empty() {
		out.WriteString(carry.CloseSeq())
	}
	out.WriteString(replacement)
	// Skip the overwritten region without emitting it, still tracking
	// carry state so the resuming suffix below can reopen whatever span
	// was active there — the same span, if any, that spanned across the
	// entire cut region and would otherwise be orphaned (its opening
	// sequence discarded, only its eventual reset surviving in the
	// suffix).
	skipEnd := col + width
	for i < len(runes) && visCol < skipEnd {
		if runes[i] == '\x1b' {
			j := ConsumeANSI(runes, i)
			carry.Apply(string(runes[i:j]))
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		i = next
		visCol += w
	}
	if i < len(runes) {
		if !carry.Empty() {
			out.WriteString(carry.OpenSeq())
		}
		out.WriteString(string(runes[i:]))
	}
	return out.String()
}

// SplitTokens splits text on whitespace but keeps ANSI sequences attached.
func SplitTokens(text string) []string {
	var tokens []string
	var cur strings.Builder
	inEsc := false
	inCSI := false
	inOSC := false
	prev := rune(0)

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for _, ch := range text {
		switch {
		case inOSC:
			cur.WriteRune(ch)
			if (prev == '\x1b' && ch == '\\') || ch == '\a' {
				inOSC = false
			}
			prev = ch
		case inCSI:
			cur.WriteRune(ch)
			if ch >= 0x40 && ch <= 0x7e {
				inCSI = false
			}
		case inEsc:
			cur.WriteRune(ch)
			switch {
			case ch == '[':
				inCSI = true
				inEsc = false
			case ch == ']':
				inOSC = true
				inEsc = false
			case ch >= 0x40 && ch <= 0x7e:
				inEsc = false
			}
			prev = ch
		case ch == '\x1b':
			cur.WriteRune(ch)
			inEsc = true
			prev = ch
		case ch == ' ' || ch == '\t':
			flush()
			prev = ch
		default:
			cur.WriteRune(ch)
			prev = ch
		}
	}
	flush()
	return tokens
}

// Strip removes CSI and OSC terminal escape sequences while preserving content.
func Strip(s string) string {
	return ansi.Strip(s)
}

// TrimTrailingSpace removes s's last visible (non-ANSI-escape)
// rune if it's a space, preserving every escape sequence exactly where it
// was — including ones immediately before or after the removed rune, so a
// styled span's opening/closing codes stay correctly paired around the
// shortened content. ok is false, and s is returned unchanged, if s has no
// visible content or its last visible rune isn't a space.
func TrimTrailingSpace(s string) (trimmed string, ok bool) {
	runes := []rune(s)
	lastVisible := -1
	for i := 0; i < len(runes); {
		if runes[i] == '\x1b' {
			i = ConsumeANSI(runes, i)
			continue
		}
		lastVisible = i
		i++
	}
	if lastVisible == -1 || runes[lastVisible] != ' ' {
		return s, false
	}
	return string(runes[:lastVisible]) + string(runes[lastVisible+1:]), true
}

func VisiblePrefix(s string, width int) string {
	runes := []rune(s)
	var b strings.Builder
	visible := 0
	i := 0
	for i < len(runes) && visible < width {
		if runes[i] == '\x1b' {
			next := ConsumeANSI(runes, i)
			b.WriteString(string(runes[i:next]))
			i = next
			continue
		}
		rw, next := NextGrapheme(runes, i)
		// A wide (multi-column) grapheme cluster that would land exactly on
		// the boundary must not be included — doing so overshoots width by
		// 1 (or more), which is exactly what produced the scrollbar-gutter
		// off-by-one for lines ending in an emoji. Drop it and stop; the
		// caller pads the resulting gap instead.
		if visible+rw > width {
			break
		}
		b.WriteString(string(runes[i:next]))
		visible += rw
		i = next
	}
	for i < len(runes) {
		if runes[i] != '\x1b' {
			i++
			continue
		}
		next := ConsumeANSI(runes, i)
		b.WriteString(string(runes[i:next]))
		i = next
	}
	return b.String()
}

// ConsumeANSI returns the index just past the escape sequence starting at
// runes[i] (runes[i] must be '\x1b') — the same recognizer every ANSI-aware
// helper in this package (wordWrapTokens, VisibleLen, the Carry state
// machine) tokenizes escape sequences with. Delegates to x/ansi's decoder
// rather than hand-rolling CSI/OSC recognition a second time; runes[i:] is
// re-encoded to a string because the decoder works in bytes, and the
// consumed byte count is converted back to a rune count since every caller
// here indexes into a []rune.
func ConsumeANSI(runes []rune, i int) int {
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

// TruncateToWidth truncates s to at most width terminal *columns* — not
// runes, which is what its name and this comment used to say: it measures
// with VisibleLen, so ANSI escapes are free and a double-width cluster
// costs two. suffix is appended when content is clipped (use "" for clip/no
// indicator, "…" for ellipsis, etc.), and is itself measured the same way.
func TruncateToWidth(s string, width int, suffix string) string {
	if width <= 0 {
		return ""
	}
	if VisibleLen(s) <= width {
		return s
	}
	cut := width - VisibleLen(suffix)
	if cut <= 0 {
		return VisiblePrefix(s, width)
	}
	return VisiblePrefix(s, cut) + suffix
}
