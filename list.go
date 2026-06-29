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

	listStyleType := decls["list-style-type"]
	if listStyleType == "" {
		if ordered {
			listStyleType = "decimal"
		} else {
			listStyleType = "disc"
		}
	}

	itemCount := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			itemCount++
		}
	}

	prefixWidth := listItemPrefixWidth(listStyleType, ordered, itemCount)
	contentWidth := availWidth - indent - prefixWidth
	if contentWidth < 10 {
		contentWidth = 10
	}
	indentStr := strings.Repeat(" ", indent)
	hangStr := strings.Repeat(" ", indent+prefixWidth)

	var sb strings.Builder
	itemIdx := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "li" {
			continue
		}
		itemIdx++
		prefix := listItemPrefix(listStyleType, ordered, itemIdx, prefixWidth)
		raw := strings.TrimRight(r.renderInline(c, contentWidth), "\n ")
		lines := strings.Split(raw, "\n")
		for li, line := range lines {
			if li == 0 {
				wrapped := wordWrapANSI(line, contentWidth)
				for wi, seg := range wrapped {
					if wi == 0 {
						sb.WriteString(indentStr + prefix + seg + "\n")
					} else {
						sb.WriteString(hangStr + seg + "\n")
					}
				}
			} else {
				if strings.TrimSpace(line) == "" {
					sb.WriteByte('\n')
					continue
				}
				wrapped := wordWrapANSI(line, contentWidth)
				for _, seg := range wrapped {
					sb.WriteString(hangStr + seg + "\n")
				}
			}
		}
	}
	return sb.String()
}

// listItemPrefixWidth returns the column width of the widest prefix for the list.
func listItemPrefixWidth(style string, ordered bool, count int) int {
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
		return fmt.Sprintf("%c. ", 'a'+rune(n-1))
	case "upper-alpha", "upper-latin":
		return fmt.Sprintf("%c. ", 'A'+rune(n-1))
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
