package htmlterm

import (
	"strings"
	"unicode"
	"unicode/utf8"

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

// splitAtVisualWidth splits s into chunks of at most width visible runes,
// attaching ANSI escape sequences to the preceding visible character.
// Used for break-all and break-word hard-breaking.
func splitAtVisualWidth(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{""}
	}
	var lines []string
	var cur strings.Builder
	col := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		ch := runes[i]
		if ch == '\x1b' {
			// Consume ANSI escape sequence without counting visible width.
			cur.WriteRune(ch)
			i++
			if i >= len(runes) {
				break
			}
			next := runes[i]
			cur.WriteRune(next)
			i++
			if next == '[' {
				// CSI: consume until final byte (0x40–0x7e).
				for i < len(runes) && (runes[i] < 0x40 || runes[i] > 0x7e) {
					cur.WriteRune(runes[i])
					i++
				}
				if i < len(runes) {
					cur.WriteRune(runes[i])
					i++
				}
			} else if next == ']' {
				// OSC: consume until ST (ESC \) or BEL.
				prev := next
				for i < len(runes) {
					c2 := runes[i]
					cur.WriteRune(c2)
					i++
					if (prev == '\x1b' && c2 == '\\') || c2 == '\a' {
						break
					}
					prev = c2
				}
			}
			// else: two-char escape already consumed.
		} else {
			if col >= width {
				lines = append(lines, cur.String())
				cur.Reset()
				col = 0
			}
			cur.WriteRune(ch)
			col++
			i++
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// wordWrapANSI splits text into lines of at most width visible characters.
// breakMode controls mid-word breaking: "" or "normal" = word boundaries only;
// "break-word" = also hard-break tokens that overflow the width;
// "break-all" = break at any character boundary.
func wordWrapANSI(text string, width int, breakMode string) []string {
	if width <= 0 {
		width = 10
	}
	if breakMode == "break-all" {
		return splitAtVisualWidth(text, width)
	}
	if ansiVisibleLen(text) <= width {
		return []string{text}
	}
	var lines []string
	var cur strings.Builder
	curLen := 0
	tokens := splitANSITokens(text)
	for _, tok := range tokens {
		vl := ansiVisibleLen(tok)
		space := " "
		if cur.Len() == 0 {
			space = ""
		}
		if curLen+len(space)+vl > width && cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
			curLen = 0
			space = ""
		}
		if breakMode == "break-word" && vl > width {
			// Hard-break a token that is too wide to fit on any line.
			if cur.Len() > 0 {
				lines = append(lines, cur.String())
				cur.Reset()
				curLen = 0
			}
			chunks := splitAtVisualWidth(tok, width)
			for k, chunk := range chunks {
				if k < len(chunks)-1 {
					lines = append(lines, chunk)
				} else {
					cur.WriteString(chunk)
					curLen = ansiVisibleLen(chunk)
				}
			}
		} else {
			cur.WriteString(space + tok)
			curLen += len(space) + vl
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
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

func plainInlineText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
