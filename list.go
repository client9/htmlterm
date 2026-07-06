package htmlterm

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// renderList renders <ul> or <ol> with word-wrapped items and hanging indent.
func (r *Renderer) renderList(n *html.Node, ordered bool, availWidth int) string {
	decls := r.resolveDecls(n)

	indent := 0
	if v := decls["padding-left"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok {
			indent = abs
		}
	}
	if v := decls["margin-left"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok {
			indent += abs
		}
	}

	rightIndent := 0
	if v := decls["padding-right"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok {
			rightIndent = abs
		}
	}
	if v := decls["margin-right"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok {
			rightIndent += abs
		}
	}
	availWidth -= rightIndent

	listStyleType := decls["list-style-type"]
	if listStyleType == "" {
		if ordered {
			listStyleType = "decimal"
		} else {
			listStyleType = "disc"
		}
	}

	inside := decls["list-style-position"] == "inside"

	// start is the counter value for the first <li>; default 1, overridden by start= attr.
	start := 1
	if ordered {
		if v := nodeAttr(n, "start"); v != "" {
			var s int
			if _, err := fmt.Sscanf(v, "%d", &s); err == nil {
				start = s
			}
		}
	}

	itemCount := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			itemCount++
		}
	}

	// maxVal is the largest counter value printed; used to size the prefix column.
	maxVal := start + itemCount - 1
	if maxVal < 1 {
		maxVal = 1
	}
	prefixWidth := listItemPrefixWidth(listStyleType, ordered, maxVal)
	indentStr := strings.Repeat(" ", indent)

	var contentWidth, firstLineWidth int
	var hangStr string
	if inside {
		// Prefix flows inline: continuation lines align with indent, not after prefix.
		contentWidth = availWidth - indent
		if contentWidth < 1 {
			contentWidth = 1
		}
		firstLineWidth = contentWidth - prefixWidth
		if firstLineWidth < 1 {
			firstLineWidth = 1
		}
		hangStr = indentStr
	} else {
		contentWidth = availWidth - indent - prefixWidth
		if contentWidth < 1 {
			contentWidth = 1
		}
		firstLineWidth = contentWidth
		hangStr = strings.Repeat(" ", indent+prefixWidth)
	}

	var sb strings.Builder

	if pt := parseMargin(decls["padding-top"]); pt > 0 {
		sb.WriteString(strings.Repeat("\n", pt))
	}

	itemIdx := start - 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if c.Data == "ol" || c.Data == "ul" || c.Data == "menu" {
			nested := r.renderList(c, c.Data == "ol", contentWidth)
			for _, line := range strings.Split(strings.TrimRight(nested, "\n"), "\n") {
				sb.WriteString(hangStr + line + "\n")
			}
			continue
		}
		if c.Data != "li" {
			continue
		}
		itemIdx++
		liDecls := r.resolveDecls(c)
		prefix := listItemPrefix(listStyleType, ordered, itemIdx, prefixWidth)
		if md := r.pseudoElemDecls(c, "marker"); len(md) > 0 {
			prefix = extractInlineStyle(md).render(prefix, r.profile)
		}
		savedDepth := r.quoteDepth
		tokens := r.renderInlineAccTokens(c, inlineStyle{}, contentWidth)
		tokens = trimTrailingBreaksAndSpace(tokens)
		if liDecls["visibility"] == "hidden" {
			r.quoteDepth = savedDepth
			tokens = blankVisibleContentTokens(tokens)
			prefix = blankVisibleContent(prefix)
		}
		// One call handles both the first line's narrower width (room for
		// the prefix) and the rest — eliminating the historical double-wrap
		// (wrap once via renderInline at contentWidth, split on "\n", then
		// wrap the first line again narrower and string-concatenate the
		// prefix on front).
		body, _ := wordWrapTokens(tokens, contentWidth, "", firstLineWidth)
		for i, line := range body.lines {
			switch {
			case i == 0:
				sb.WriteString(indentStr + prefix + line + "\n")
			case line == "":
				// No hanging-indent padding on a genuinely blank line (e.g.
				// from "<br><br>" inside the item) — avoids trailing
				// whitespace-only lines.
				sb.WriteByte('\n')
			default:
				sb.WriteString(hangStr + line + "\n")
			}
		}
	}
	if pb := parseMargin(decls["padding-bottom"]); pb > 0 {
		sb.WriteString(strings.Repeat("\n", pb))
	}

	return sb.String()
}

// listStyleCustomString returns the unquoted string if style is a CSS quoted
// string literal (e.g. `"→ "` or `'* '`), otherwise returns "".
func listStyleCustomString(style string) (string, bool) {
	s := strings.TrimSpace(style)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		decoded := parseCSSString(s)
		// Newlines decoded from CSS escapes (e.g. \A) break visual-width accounting.
		decoded = strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' {
				return -1
			}
			return r
		}, decoded)
		return decoded, true
	}
	return "", false
}

// listItemPrefixWidth returns the column width of the widest prefix for the list.
func listItemPrefixWidth(style string, ordered bool, count int) int {
	if custom, ok := listStyleCustomString(style); ok {
		return utf8.RuneCountInString(custom)
	}
	if !ordered || style == "none" {
		switch style {
		case "none":
			return 0
		case "circle":
			return utf8.RuneCountInString("○ ")
		case "square":
			return utf8.RuneCountInString("■ ")
		default:
			return utf8.RuneCountInString("• ")
		}
	}
	switch style {
	case "lower-roman", "upper-roman":
		return maxRomanPrefixWidth(count)
	}
	digits := len(fmt.Sprintf("%d", count))
	return digits + 2
}

// listItemPrefix returns the formatted prefix for item number n (1-based).
func listItemPrefix(style string, ordered bool, n, width int) string {
	if custom, ok := listStyleCustomString(style); ok {
		return custom
	}
	if !ordered {
		switch style {
		case "none":
			return ""
		case "circle":
			return "○ "
		case "square":
			return "■ "
		default:
			return "• "
		}
	}
	switch style {
	case "none":
		return ""
	case "lower-alpha", "lower-latin":
		return alphaSequence(n, false) + ". "
	case "upper-alpha", "upper-latin":
		return alphaSequence(n, true) + ". "
	case "lower-roman":
		numeral := toRoman(n, false)
		if width > 0 {
			return fmt.Sprintf("%*s. ", width-2, numeral)
		}
		return numeral + ". "
	case "upper-roman":
		numeral := toRoman(n, true)
		if width > 0 {
			return fmt.Sprintf("%*s. ", width-2, numeral)
		}
		return numeral + ". "
	default:
		digits := width - 2
		return fmt.Sprintf("%*d. ", digits, n)
	}
}

func alphaSequence(n int, upper bool) string {
	if n < 1 {
		return fmt.Sprintf("%d", n)
	}
	base := rune('a')
	if upper {
		base = 'A'
	}
	var chars []rune
	for n > 0 {
		n--
		chars = append(chars, base+rune(n%26))
		n /= 26
	}
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}
