package render

import (
	"strings"
	"unicode"

	"github.com/client9/htmlterm/internal/textcell"
)

// CSS-level text semantics: what a string *means* under the cascade —
// white-space normalization, text-transform, tab expansion, and the
// terminal-escape sanitization every piece of untrusted text passes through
// on its way to output.
//
// Deliberately distinct from column/width math, which lives in
// internal/textcell: that package answers "how much room does this take",
// this file answers "what should this text be". They used to share one
// 979-line textutil.go, which is how a rune count ended up standing in for a
// column count in four separate places (see widecolumn_test.go).

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
	s = textcell.Strip(s)
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
