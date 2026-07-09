package render

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

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
			} else {
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
// ConsumeANSI) and updates the carry: an SGR sequence (CSI ... 'm') with
// empty/"0" params clears the active SGR span, any other SGR sequence
// replaces it; an OSC8 sequence with an empty URI clears the active
// hyperlink span, any other OSC8 sequence replaces it. Anything else is
// left untouched.
func (c *ansiCarry) apply(seq string) {
	switch {
	case strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m"):
		params := seq[2 : len(seq)-1]
		if params == "" || params == "0" {
			c.sgr = ""
		} else {
			c.sgr = seq
		}
	case strings.HasPrefix(seq, "\x1b]8;"):
		rest := seq[len("\x1b]8;"):]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			return
		}
		uri := rest[semi+1:]
		uri = strings.TrimSuffix(uri, "\x07")
		uri = strings.TrimSuffix(uri, "\x1b\\")
		if uri == "" {
			c.hyperlink = ""
		} else {
			c.hyperlink = seq
		}
	}
}

// scan walks s in order, updating the carry for every escape sequence it
// contains (reusing ConsumeANSI's recognizer).
func (c *ansiCarry) scan(s string) {
	if !strings.ContainsRune(s, '\x1b') {
		return
	}
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			end := ConsumeANSI(runes, i)
			c.apply(string(runes[i:end]))
			i = end
			continue
		}
		i++
	}
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
			j := ConsumeANSI(runes, i)
			seq := string(runes[i:j])
			cur.WriteString(seq)
			carry.apply(seq)
			i = j
			continue
		}
		if col >= width {
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
		cur.WriteRune(ch)
		col++
		i++
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		return []string{""}, carry
	}
	return lines, carry
}

// spliceColumns overwrites visible columns [col, col+width) of line with
// replacement, preserving line's ANSI styling for the untouched prefix/
// suffix and re-carrying any open span across the splice boundary — the
// primitive popup/overlay rendering needs (see RENDERING.md's popup
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
			j := ConsumeANSI(runes, i)
			seq := string(runes[i:j])
			out.WriteString(seq)
			carry.apply(seq)
			i = j
			continue
		}
		out.WriteRune(runes[i])
		i++
		visCol++
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
			j := ConsumeANSI(runes, i)
			carry.apply(string(runes[i:j]))
			i = j
			continue
		}
		i++
		visCol++
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
	inEsc := false
	inCSI := false
	inOSC := false
	prev := rune(0)
	for _, ch := range s {
		switch {
		case inOSC:
			if (prev == '\x1b' && ch == '\\') || ch == '\a' {
				inOSC = false
			}
			prev = ch
		case inCSI:
			if ch >= 0x40 && ch <= 0x7e {
				inCSI = false
			}
		case inEsc:
			switch {
			case ch == '[':
				inCSI = true
				inEsc = false
			case ch == ']':
				inOSC = true
				inEsc = false
			case ch >= 0x40 && ch <= 0x7e:
				// Two-character escape sequence (Fs); consume final byte.
				inEsc = false
			}
			// Intermediate bytes (0x20–0x3F) keep inEsc=true to consume the
			// following final byte as part of the same sequence.
			prev = ch
		case ch == '\x1b':
			inEsc = true
			prev = ch
		default:
			n++
			prev = ch
		}
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
	var b strings.Builder
	inEsc := false
	inCSI := false
	inOSC := false
	prev := rune(0)
	for _, ch := range s {
		switch {
		case inOSC:
			if (prev == '\x1b' && ch == '\\') || ch == '\a' {
				inOSC = false
			}
			prev = ch
		case inCSI:
			if ch >= 0x40 && ch <= 0x7e {
				inCSI = false
			}
		case inEsc:
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
			inEsc = true
			prev = ch
		default:
			b.WriteRune(ch)
			prev = ch
		}
	}
	return b.String()
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

// sanitizeTerminalText removes terminal escape sequences and control
// characters from untrusted HTML/CSS text before it reaches terminal output.
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
			next := ConsumeANSI(runes, i)
			b.WriteString(string(runes[i:next]))
			i = next
			continue
		}
		b.WriteRune(runes[i])
		visible++
		i++
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
// runes[i] (runes[i] must be '\x1b'), recognizing CSI ("\x1b[...letter") and
// OSC ("\x1b]...BEL or ST") forms — the same recognizer every ANSI-aware
// helper in this package (wordWrapANSI, ansiVisibleLen, stripANSI, the
// ansiCarry state machine) tokenizes escape sequences with. Exported so a
// consumer decoding htmlterm's own rendered ANSI output back into another
// representation (e.g. tui's cell-bridge, painting cells into a
// tcell.Screen) can tokenize with the exact same rules the renderer used to
// produce it, rather than maintaining a second, potentially drifting
// implementation.
func ConsumeANSI(runes []rune, i int) int {
	if i >= len(runes) || runes[i] != '\x1b' {
		return i + 1
	}
	i++
	if i >= len(runes) {
		return i
	}
	next := runes[i]
	i++
	switch next {
	case '[':
		for i < len(runes) && (runes[i] < 0x40 || runes[i] > 0x7e) {
			i++
		}
		if i < len(runes) {
			i++
		}
	case ']':
		prev := next
		for i < len(runes) {
			ch := runes[i]
			i++
			if (prev == '\x1b' && ch == '\\') || ch == '\a' {
				break
			}
			prev = ch
		}
	default:
		for i < len(runes) && next < 0x40 {
			next = runes[i]
			i++
		}
	}
	return i
}

func plainInlineText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
