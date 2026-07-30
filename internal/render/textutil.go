package render

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
	"golang.org/x/net/html"
)

// displayWidthOptions mirrors tcell/v3's internal/widthutil.Options() byte
// for byte: tcell derives its own displaywidth.Options from the
// RUNEWIDTH_EASTASIAN env var (preserving compatibility with the old
// go-runewidth-based toggle), but that logic lives in an unimportable
// internal package. htmlterm's own width math and tcell's paint-time width
// math (tui/cellbridge.go's SetContent calls, backed by tcell's own
// displaywidth-based CellBuffer) MUST derive the same Options value under
// the same env var, or the two disagree exactly when a user sets it. If
// tcell ever changes this env var's name or semantics, this must be updated
// to match.
func displayWidthOptions() displaywidth.Options {
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
// ANSI escape recognition is handled separately (see consumeANSI) by every
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
	g := displayWidthOptions().StringGraphemes(s)
	if !g.Next() {
		return 1, i + 1
	}
	cluster := g.Value()
	return g.Width(), i + utf8.RuneCountInString(cluster)
}

// textVisualWidth is the terminal column width of an entire plain (no ANSI
// escapes) string: the sum of each grapheme cluster's display width (VS16
// pairs, ZWJ sequences, flag pairs, etc. already scored as one unit). No
// ANSI-escape interleaving to preserve here, unlike this file's other
// width-scanning functions, so this calls displaywidth directly instead of
// looping via NextGrapheme.
func textVisualWidth(s string) int {
	return displayWidthOptions().String(s)
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

var superscriptMap = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
	'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ',
	'f': 'ᶠ', 'g': 'ᵍ', 'h': 'ʰ', 'i': 'ⁱ', 'j': 'ʲ',
	'k': 'ᵏ', 'l': 'ˡ', 'm': 'ᵐ', 'n': 'ⁿ', 'o': 'ᵒ',
	'p': 'ᵖ', 'r': 'ʳ', 's': 'ˢ', 't': 'ᵗ', 'u': 'ᵘ',
	'v': 'ᵛ', 'w': 'ʷ', 'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
}

var subscriptMap = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄',
	'5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'k': 'ₖ', 'l': 'ₗ',
	'm': 'ₘ', 'n': 'ₙ', 'o': 'ₒ', 'p': 'ₚ', 's': 'ₛ',
	't': 'ₜ', 'x': 'ₓ',
}

func toSuperscript(s string) string {
	return strings.Map(func(r rune) rune {
		if mapped, ok := superscriptMap[r]; ok {
			return mapped
		}
		return r
	}, s)
}

func toSubscript(s string) string {
	return strings.Map(func(r rune) rune {
		if mapped, ok := subscriptMap[r]; ok {
			return mapped
		}
		return r
	}, s)
}

// effectiveTransform returns the text transform mode to use given text-transform
// and font-variant declarations. font-variant: small-caps maps to uppercase.
func effectiveTransform(decls map[string]string) string {
	if tt := decls["text-transform"]; tt != "" && tt != "none" {
		return tt
	}
	if decls["font-variant"] == "small-caps" {
		return "uppercase"
	}
	return decls["text-transform"]
}

// applyTextTransform applies a CSS text-transform value to s.
func applyTextTransform(s, mode string) string {
	switch mode {
	case "uppercase":
		return strings.ToUpper(s)
	case "lowercase":
		return strings.ToLower(s)
	case "capitalize":
		runes := []rune(s)
		atStart := true
		for i, r := range runes {
			if unicode.IsSpace(r) {
				atStart = true
			} else if atStart {
				runes[i] = unicode.ToUpper(r)
				atStart = false
			}
		}
		return string(runes)
	case "superscript":
		return toSuperscript(s)
	case "subscript":
		return toSubscript(s)
	default:
		return s
	}
}

// expandTabs replaces tab characters with spaces using tabSize-column tab stops.
func expandTabs(s string, tabSize int) string {
	if tabSize <= 0 {
		tabSize = 8
	}
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, ch := range s {
		switch ch {
		case '\t':
			spaces := tabSize - (col % tabSize)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		case '\n':
			b.WriteRune(ch)
			col = 0
		default:
			b.WriteRune(ch)
			col++
		}
	}
	return b.String()
}

// normalizeWhiteSpace applies CSS white-space rules to a raw text node string.
// tabSize controls tab expansion in pre/pre-wrap modes (0 defaults to 8).
func normalizeWhiteSpace(s, mode string, tabSize int) string {
	switch mode {
	case "pre", "pre-wrap":
		return expandTabs(s, tabSize)
	case "pre-line":
		var b strings.Builder
		lastWasSpace := false
		for _, ch := range s {
			switch ch {
			case '\n':
				b.WriteRune('\n')
				lastWasSpace = false
			case ' ', '\t', '\r':
				if !lastWasSpace {
					b.WriteRune(' ')
					lastWasSpace = true
				}
			default:
				b.WriteRune(ch)
				lastWasSpace = false
			}
		}
		return b.String()
	default:
		var b strings.Builder
		lastWasSpace := false
		for _, ch := range s {
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				if !lastWasSpace {
					b.WriteRune(' ')
					lastWasSpace = true
				}
			} else {
				b.WriteRune(ch)
				lastWasSpace = false
			}
		}
		return b.String()
	}
}

// ansiCarry tracks the single currently-open SGR style span and/or OSC8
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
type ansiCarry struct {
	sgr       string // active SGR open sequence, e.g. "\x1b[4m"; "" if none
	hyperlink string // active OSC8 open sequence; "" if none
}

func (c ansiCarry) empty() bool { return c.sgr == "" && c.hyperlink == "" }

// closeSeq closes whatever is open, innermost first (SGR, then OSC8).
func (c ansiCarry) closeSeq() string {
	if c.empty() {
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

// openSeq reopens whatever is open, outermost first (OSC8, then SGR).
func (c ansiCarry) openSeq() string {
	if c.empty() {
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

// apply classifies one fully-extracted escape sequence (as produced by
// consumeANSI) and updates the carry: an SGR sequence (CSI ... 'm') with
// empty/"0" params clears the active SGR span, any other SGR sequence
// replaces it; an OSC8 sequence with an empty URI clears the active
// hyperlink span, any other OSC8 sequence replaces it. Anything else is
// left untouched.
func (c *ansiCarry) apply(seq string) {
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
// reset (empty or "0" params) as opposed to a style-setting sequence.
func sgrIsReset(seq string) bool {
	params := seq[2 : len(seq)-1]
	return params == "" || params == "0"
}

// osc8IsReset reports whether seq (a full "\x1b]8;...ST" sequence) closes a
// hyperlink (empty URI) as opposed to opening one.
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

// scan walks s in order, updating the carry for every escape sequence it
// contains (reusing consumeANSI's recognizer).
func (c *ansiCarry) scan(s string) {
	if !strings.ContainsRune(s, '\x1b') {
		return
	}
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			end := consumeANSI(runes, i)
			c.apply(string(runes[i:end]))
			i = end
			continue
		}
		i++
	}
}

// collapseDeadANSISpans removes SGR/OSC8 "open" escape sequences that are
// immediately (no visible character in between) superseded by a full reset
// of the same kind — i.e. spans that never got the chance to style or link
// anything. Dropping such an open and keeping only the reset is always safe
// regardless of any earlier context: a full SGR reset ("\x1b[m"/"\x1b[0m")
// or OSC8 reset (empty-URI "\x1b]8;;...") unconditionally clears that kind
// of state no matter what, if anything, was open beforehand, so whether the
// dead open ran first makes no observable difference.
//
// This is deliberately one-directional: it does NOT collapse "open A, open
// B" (with no reset between) down to just B, even though B logically
// supersedes A — this codebase's SGR sequences are additive, not
// full-replace (see ansiCarry's doc comment), so e.g. "\x1b[3;4m" (italic+
// underline) directly followed by "\x1b[3m" (italic) with no reset in
// between would leave underline stuck on, since there's no explicit
// "turn off underline" code involved. Only a real reset ever fully clears
// state, so only a real reset is a safe deletion trigger for what precedes it.
//
// wordWrapTokens routinely produces exactly the pattern this removes:
// coalesceTextRuns concatenates several already-independently-styled spans
// (each self-contained, from inlineStyle.render) into one string before
// splitANSITokens re-tokenizes it purely on literal space characters, with
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
// empty span in output.
func collapseDeadANSISpans(s string) string {
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
		j := consumeANSI(runes, i)
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

// splitAtVisualWidth splits s into chunks of at most width visible runes,
// attaching ANSI escape sequences to the preceding visible character.
// Used for break-all and break-word hard-breaking.
func splitAtVisualWidth(s string, width int) []string {
	lines, _ := splitAtVisualWidthCarry(s, width, ansiCarry{})
	return lines
}

// splitAtVisualWidthCarry is splitAtVisualWidth's carry-aware core. start is
// whatever SGR/OSC8 span is already open as of the start of s (from
// preceding text this call doesn't see); end is whatever span is left open
// at the end of s. Every internal width-driven break closes the
// currently-open span before the break and reopens the identical span
// immediately after, so every returned chunk is self-contained.
func splitAtVisualWidthCarry(s string, width int, start ansiCarry) ([]string, ansiCarry) {
	if width <= 0 || s == "" {
		return []string{""}, start
	}
	carry := start
	var lines []string
	var cur strings.Builder
	if !carry.empty() {
		cur.WriteString(carry.openSeq())
	}
	col := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		ch := runes[i]
		if ch == '\x1b' {
			j := consumeANSI(runes, i)
			seq := string(runes[i:j])
			cur.WriteString(seq)
			carry.apply(seq)
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		if col+w > width && col > 0 {
			if !carry.empty() {
				cur.WriteString(carry.closeSeq())
			}
			lines = append(lines, cur.String())
			cur.Reset()
			col = 0
			if !carry.empty() {
				cur.WriteString(carry.openSeq())
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

// visibleWindowCarry returns the [offset, offset+width) visible-column
// window of s — wide-rune/ANSI-aware, and, like splitAtVisualWidthCarry's
// internal chunk boundaries, self-contained: an SGR/OSC8 span still open at
// offset is reopened at the window's start and closed at its end. Used for
// overflow-x: scroll|auto's horizontal scroll-offset slicing (see
// docs/SCROLLING.md), where offset can land anywhere within a line — unlike
// splitAtVisualWidthCarry, which only ever chops a string into sequential
// chunks from column 0. A wide rune straddling either boundary is dropped
// entirely rather than split, the same rule visiblePrefixWithTrailingEscapes
// already applies at its own single (right) boundary.
func visibleWindowCarry(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	carry := ansiCarry{}
	col := 0
	i := 0
	for i < len(runes) && col < offset {
		ch := runes[i]
		if ch == '\x1b' {
			j := consumeANSI(runes, i)
			carry.apply(string(runes[i:j]))
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
	if !carry.empty() {
		out.WriteString(carry.openSeq())
	}
	visible := 0
	for i < len(runes) && visible < width {
		ch := runes[i]
		if ch == '\x1b' {
			j := consumeANSI(runes, i)
			seq := string(runes[i:j])
			out.WriteString(seq)
			carry.apply(seq)
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
	if !carry.empty() {
		out.WriteString(carry.closeSeq())
	}
	return out.String()
}

// spliceColumns overwrites visible columns [col, col+width) of line with
// replacement, preserving line's ANSI styling for the untouched prefix/
// suffix and re-carrying any open span across the splice boundary — the
// primitive popup/overlay rendering needs (see docs/RENDERING.md's popup
// section) to punch replacement content into an already-composed line
// without disturbing anything outside the overwritten range, unlike
// splitAtVisualWidth/splitAtVisualWidthCarry, which only chop a string into
// sequential chunks from column 0 and have no notion of an interior
// overwrite. replacement is inserted verbatim — the caller is responsible
// for it being exactly width visible columns wide (e.g. a popup's own
// already-rendered box.width) so the result's total visible width matches
// line's; spliceColumns does not itself pad or truncate it.
//
// If line is narrower than col, it's padded with spaces so replacement
// lands at the requested column. If line is narrower than col+width,
// there's nothing past its own end to preserve, so the result simply ends
// after replacement.
func spliceColumns(line string, col, width int, replacement string) string {
	if col < 0 || width <= 0 {
		return line
	}
	var out strings.Builder
	carry := ansiCarry{}
	runes := []rune(line)
	i := 0
	visCol := 0
	// Copy the untouched prefix, tracking carry state as we go so it can be
	// closed cleanly right before replacement.
	for i < len(runes) && visCol < col {
		if runes[i] == '\x1b' {
			j := consumeANSI(runes, i)
			seq := string(runes[i:j])
			out.WriteString(seq)
			carry.apply(seq)
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
	if !carry.empty() {
		out.WriteString(carry.closeSeq())
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
			j := consumeANSI(runes, i)
			carry.apply(string(runes[i:j]))
			i = j
			continue
		}
		w, next := NextGrapheme(runes, i)
		i = next
		visCol += w
	}
	if i < len(runes) {
		if !carry.empty() {
			out.WriteString(carry.openSeq())
		}
		out.WriteString(string(runes[i:]))
	}
	return out.String()
}

// wordWrapANSI splits text into lines of at most width visible characters.
// breakMode controls mid-word breaking: "" or "normal" = word boundaries only;
// "break-word" = also hard-break tokens that overflow the width;
// "break-all" = break at any character boundary.
//
// Any SGR style or OSC8 hyperlink span that gets split across an inserted
// line break is closed before the break and reopened at the start of the
// next line (see ansiCarry), so a line's own trailing padding/margin never
// inherits a style left open by a wrapped span, and every wrapped line of a
// styled/linked run remains independently styled.
//
// Thin shim over wordWrapTokens with a single text token — see wraptoken.go.
func wordWrapANSI(text string, width int, breakMode string) []string {
	b, _ := wordWrapTokens([]wrapToken{{text: text}}, width, breakMode, 0)
	return b.lines
}

// ansiVisibleLen returns the number of visible (non-ANSI-escape) runes in s.
func ansiVisibleLen(s string) int {
	n := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			i = consumeANSI(runes, i)
			continue
		}
		w, next := NextGrapheme(runes, i)
		n += w
		i = next
	}
	return n
}

// splitANSITokens splits text on whitespace but keeps ANSI sequences attached.
func splitANSITokens(text string) []string {
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

// toRoman converts n to a Roman numeral string (upper or lower case).
func toRoman(n int, upper bool) string {
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			if upper {
				b.WriteString(strings.ToUpper(syms[i]))
			} else {
				b.WriteString(syms[i])
			}
			n -= v
		}
	}
	return b.String()
}

// maxRomanPrefixWidth returns the maximum roman numeral prefix width
// (numeral + ". ") across all items 1..count. Instead of scanning every item,
// it checks the ~15 threshold numbers where the maximum roman numeral length
// grows (3, 8, 18, 28, …, 3888).
func maxRomanPrefixWidth(count int) int {
	// Each entry is the first N for which that numeral becomes the widest in 1..N.
	thresholds := []int{1, 2, 3, 8, 18, 28, 38, 88, 188, 288, 388, 888, 1888, 2888, 3888}
	widest := 0
	for _, n := range thresholds {
		if n > count {
			break
		}
		if w := utf8.RuneCountInString(toRoman(n, false)); w > widest {
			widest = w
		}
	}
	return widest + 2 // +2 for ". "
}

// rawContent returns the concatenated text of all descendant text nodes.
func rawContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// stripANSI removes CSI and OSC terminal escape sequences while preserving content.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

// trimOneTrailingVisibleSpace removes s's last visible (non-ANSI-escape)
// rune if it's a space, preserving every escape sequence exactly where it
// was — including ones immediately before or after the removed rune, so a
// styled span's opening/closing codes stay correctly paired around the
// shortened content. ok is false, and s is returned unchanged, if s has no
// visible content or its last visible rune isn't a space.
func trimOneTrailingVisibleSpace(s string) (trimmed string, ok bool) {
	runes := []rune(s)
	lastVisible := -1
	for i := 0; i < len(runes); {
		if runes[i] == '\x1b' {
			i = consumeANSI(runes, i)
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

// isInvisibleFormatChar reports whether r is a Unicode format character that
// is never meant to be seen (zero-width joiners/spaces, bidi controls, the
// BOM). Browsers render these at true zero width with no glyph; terminals
// are inconsistent about that — many fall back to drawing a tofu/notdef
// glyph at width 1 when the character isn't adjacent to the joining-script
// context it's meant for (e.g. a bare ZWNJ used as HTML-email filler, not
// next to Arabic/Indic letterforms). Rather than rely on the terminal to
// agree with our own zero-width table, these are dropped from text content
// entirely before they reach layout.
func isInvisibleFormatChar(r rune) bool {
	switch r {
	case '\u200b', // ZERO WIDTH SPACE
		'\u200c',                                         // ZERO WIDTH NON-JOINER
		'\u200d',                                         // ZERO WIDTH JOINER
		'\u2060',                                         // WORD JOINER
		'\ufeff',                                         // ZERO WIDTH NO-BREAK SPACE / BOM
		'\u200e',                                         // LEFT-TO-RIGHT MARK
		'\u200f',                                         // RIGHT-TO-LEFT MARK
		'\u202a', '\u202b', '\u202c', '\u202d', '\u202e', // bidi embedding/override controls
		'\u2066', '\u2067', '\u2068', '\u2069': // bidi isolate controls
		return true
	}
	return false
}

// sanitizeTerminalText removes terminal escape sequences, control
// characters, and invisible Unicode format characters from untrusted
// HTML/CSS text before it reaches terminal output.
func sanitizeTerminalText(s string, allowNewline bool) string {
	s = stripANSI(s)
	var b strings.Builder
	for _, ch := range s {
		switch {
		case ch == '\n' && allowNewline:
			b.WriteRune(ch)
		case ch == '\t' && allowNewline:
			b.WriteRune(ch)
		case ch < 0x20 || ch == 0x7f || (ch >= 0x80 && ch <= 0x9f):
			// Drop remaining C0/C1 controls, including BEL and ESC.
		case isInvisibleFormatChar(ch):
			// Drop zero-width/bidi format characters; see isInvisibleFormatChar.
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func visiblePrefixWithTrailingEscapes(s string, width int) string {
	runes := []rune(s)
	var b strings.Builder
	visible := 0
	i := 0
	for i < len(runes) && visible < width {
		if runes[i] == '\x1b' {
			next := consumeANSI(runes, i)
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
		next := consumeANSI(runes, i)
		b.WriteString(string(runes[i:next]))
		i = next
	}
	return b.String()
}

// consumeANSI returns the index just past the escape sequence starting at
// runes[i] (runes[i] must be '\x1b') — the same recognizer every ANSI-aware
// helper in this package (wordWrapANSI, ansiVisibleLen, the ansiCarry state
// machine) tokenizes escape sequences with. Delegates to x/ansi's decoder
// rather than hand-rolling CSI/OSC recognition a second time; runes[i:] is
// re-encoded to a string because the decoder works in bytes, and the
// consumed byte count is converted back to a rune count since every caller
// here indexes into a []rune.
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

func plainInlineText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
