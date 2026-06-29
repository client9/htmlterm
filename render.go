package htmlterm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"golang.org/x/net/html"
)

// declsToStyle converts a resolved CSS declaration map to a lipgloss.Style.
func declsToStyle(decls map[string]string) lipgloss.Style {
	s := lipgloss.NewStyle()
	for prop, val := range decls {
		switch prop {
		case "color":
			s = s.Foreground(lipgloss.Color(val))
		case "background-color":
			s = s.Background(lipgloss.Color(val))
		case "font-weight":
			switch val {
			case "bold":
				s = s.Bold(true)
			case "normal":
				s = s.Bold(false)
			}
		case "font-style":
			switch val {
			case "italic":
				s = s.Italic(true)
			case "normal":
				s = s.Italic(false)
			}
		case "text-align":
			switch val {
			case "right":
				s = s.Align(lipgloss.Right)
			case "center":
				s = s.Align(lipgloss.Center)
			case "left":
				s = s.Align(lipgloss.Left)
			}
		}
	}
	return s
}

// inlineStyle is the accumulated text style passed down through inline rendering.
// Bold, italic, and color are delegated to lipgloss (they produce correct
// whole-string ANSI sequences). Underline and strikethrough use raw ANSI codes
// because lipgloss v2.0.4's Underline()/Strikethrough() emit per-character
// ANSI sequences, which corrupts text and breaks OSC 8 hyperlinks.
type inlineStyle struct {
	lg        lipgloss.Style // bold, italic, foreground, background
	hasLG     bool           // true when lg has at least one property set
	underline bool
	strike    bool
}

func (s inlineStyle) has() bool { return s.hasLG || s.underline || s.strike }

// render applies the accumulated style to plain text, returning a styled string.
// lipgloss Render() is called first (on plain text only), then raw ANSI wraps
// are applied for underline/strikethrough so each produces one escape sequence
// around the entire string rather than one per character.
func (s inlineStyle) render(text string) string {
	if !s.has() {
		return text
	}
	if s.hasLG {
		text = s.lg.Render(text)
	}
	if s.underline {
		text = "\x1b[4m" + text + "\x1b[24m"
	}
	if s.strike {
		text = "\x1b[9m" + text + "\x1b[29m"
	}
	return text
}

// extractInlineStyle builds an inlineStyle from a resolved CSS declaration map.
func extractInlineStyle(decls map[string]string) inlineStyle {
	return mergeInlineStyle(inlineStyle{}, decls)
}

// mergeInlineStyle overlays the visual text properties from decls onto base.
func mergeInlineStyle(base inlineStyle, decls map[string]string) inlineStyle {
	s := base
	for prop, val := range decls {
		switch prop {
		case "color":
			s.lg = s.lg.Foreground(lipgloss.Color(val))
			s.hasLG = true
		case "background-color":
			s.lg = s.lg.Background(lipgloss.Color(val))
			s.hasLG = true
		case "font-weight":
			if val == "bold" {
				s.lg = s.lg.Bold(true)
				s.hasLG = true
			} else if val == "normal" {
				s.lg = s.lg.Bold(false)
			}
		case "font-style":
			if val == "italic" {
				s.lg = s.lg.Italic(true)
				s.hasLG = true
			} else if val == "normal" {
				s.lg = s.lg.Italic(false)
			}
		case "text-decoration":
			switch {
			case strings.Contains(val, "underline"):
				s.underline = true
			case strings.Contains(val, "line-through"):
				s.strike = true
			case val == "none" || val == "normal":
				s.underline = false
				s.strike = false
			}
		}
	}
	return s
}

// blockBorder holds the character and optional color for one edge of a block border.
type blockBorder struct {
	char  string // "" = disabled
	color string // "" = no color
}

// applyLineEdges prepends prefix and appends suffix to every line of content.
// A trailing newline is preserved. Used for margin-left / margin-right.
func applyLineEdges(content, prefix, suffix string) string {
	if prefix == "" && suffix == "" {
		return content
	}
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = prefix + line + suffix
	}
	result := strings.Join(lines, "\n")
	if trailing {
		result += "\n"
	}
	return result
}

// alignLines pads each line of content with spaces to exactly width visible
// columns according to dir: "right" pads on the left, "center" splits
// padding evenly, anything else (including "") pads on the right (left-align).
// Lines already at or beyond width are left unchanged. A trailing newline is
// preserved.
func alignLines(content, dir string, width int) string {
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		vl := ansiVisibleLen(line)
		if vl >= width {
			continue
		}
		pad := width - vl
		switch dir {
		case "right":
			lines[i] = strings.Repeat(" ", pad) + line
		case "center":
			left := pad / 2
			lines[i] = strings.Repeat(" ", left) + line + strings.Repeat(" ", pad-left)
		default:
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	result := strings.Join(lines, "\n")
	if trailing {
		result += "\n"
	}
	return result
}

// padLinesToWidth right-pads each line of content with spaces so every line
// is exactly width visible columns wide. Lines already at or beyond width are
// left unchanged. A trailing newline is preserved.
func padLinesToWidth(content string, width int) string {
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if vl := ansiVisibleLen(line); vl < width {
			lines[i] = line + strings.Repeat(" ", width-vl)
		}
	}
	result := strings.Join(lines, "\n")
	if trailing {
		result += "\n"
	}
	return result
}

// drawBlockHBorder returns a horizontal rule for a block element using fill as
// the repeated character, optionally colored. leftCorner and rightCorner
// replace the endpoints when non-empty. Returns "" when fill is empty or "none".
func drawBlockHBorder(fill, color, leftCorner, rightCorner string, width int) string {
	if fill == "" || fill == "none" || width <= 0 {
		return ""
	}
	lc := leftCorner
	if lc == "" {
		lc = fill
	}
	rc := rightCorner
	if rc == "" {
		rc = fill
	}
	return drawHRule([]int{max(0, width-runeLen(lc)-runeLen(rc))}, fill, color, lc, "", rc)
}

// applyBlockBorders prepends left.char and appends right.char to every line of
// content, applying ANSI color via makePainter. A trailing newline is preserved.
func applyBlockBorders(content string, left, right blockBorder) string {
	paintL := makePainter(left.color)
	paintR := makePainter(right.color)
	trailing := strings.HasSuffix(content, "\n")
	if trailing {
		content = content[:len(content)-1]
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = paintL(left.char) + line + paintR(right.char)
	}
	result := strings.Join(lines, "\n")
	if trailing {
		result += "\n"
	}
	return result
}

// superscriptMap maps runes to their Unicode superscript equivalents.
// Characters with no superscript form are passed through unchanged.
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

// subscriptMap maps runes to their Unicode subscript equivalents.
var subscriptMap = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄',
	'5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'k': 'ₖ', 'l': 'ₗ',
	'm': 'ₘ', 'n': 'ₙ', 'o': 'ₒ', 'p': 'ₚ', 's': 'ₛ',
	't': 'ₜ', 'x': 'ₓ',
}

// toSuperscript converts each rune in s to its Unicode superscript form.
// Runes with no superscript equivalent are left unchanged.
func toSuperscript(s string) string {
	return strings.Map(func(r rune) rune {
		if mapped, ok := superscriptMap[r]; ok {
			return mapped
		}
		return r
	}, s)
}

// toSubscript converts each rune in s to its Unicode subscript form.
// Runes with no subscript equivalent are left unchanged.
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
	return decls["text-transform"] // "none" or ""
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
	default: // "none", ""
		return s
	}
}

// parseMargin parses a CSS margin-top / margin-bottom value as a line count.
// Only non-negative integers are accepted; anything else returns 0.
func parseMargin(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// writeMarginNewlines ensures the builder has at least n trailing newlines,
// adding only as many as needed. This implements CSS margin collapsing: the
// larger of two adjacent margins wins rather than both being applied.
func writeMarginNewlines(sb *strings.Builder, n int) {
	s := sb.String()
	have := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
		have++
	}
	for i := have; i < n; i++ {
		sb.WriteByte('\n')
	}
}

// renderNode dispatches on node type. html.Parse wraps content in
// <html><head></head><body>...</body></html>, so those are transparent.
func (r *Renderer) renderNode(sb *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.renderNode(sb, c)
		}
	case html.TextNode:
		// Bare text at block level (direct child of <body> or <html>). Normalize
		// whitespace; skip pure-whitespace nodes that exist only between elements.
		if text := normalizeWhiteSpace(n.Data, "normal"); strings.TrimSpace(text) != "" {
			sb.WriteString(text)
		}
	case html.ElementNode:
		switch n.Data {
		case "html", "body":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				r.renderNode(sb, c)
			}
		case "head", "style", "script", "meta", "link", "noscript":
			// non-content elements — skip
		case "table":
			sb.WriteString(r.renderTable(n))
		case "ol":
			sb.WriteString(r.renderList(n, true, r.width))
		case "ul":
			sb.WriteString(r.renderList(n, false, r.width))
		case "br":
			sb.WriteByte('\n')
		case "hr":
			sb.WriteString(strings.Repeat("─", r.width))
			sb.WriteByte('\n')
		default:
			r.renderDisplayNode(sb, n)
		}
	}
}

// renderDisplayNode renders n according to its CSS display property.
// block: margin-top + inline content + newline + margin-bottom.
// inline-block: inline content with optional fixed width, no newline.
// inline: inline content, no newline.
// none: skip entirely.
func (r *Renderer) renderDisplayNode(sb *strings.Builder, n *html.Node) {
	decls := r.resolveDecls(n)
	href := ""
	if n.Data == "a" {
		href = nodeAttr(n, "href")
	}
	switch decls["display"] {
	case "none":
		// hidden — skip
	case "block":
		if mt := parseMargin(decls["margin-top"]); mt > 0 && sb.Len() > 0 {
			writeMarginNewlines(sb, mt+1)
		}
		sb.WriteString(wrapHyperlink(href, r.renderBlockContent(n, decls, r.width)))
		sb.WriteByte('\n')
		if mb := parseMargin(decls["margin-bottom"]); mb > 0 {
			writeMarginNewlines(sb, mb+1)
		}
	case "inline-block":
		acc := extractInlineStyle(decls)
		inner := r.renderInlineAcc(n, acc, r.width)
		if wv, ok := decls["width"]; ok {
			if abs, pct, ok2 := parseSizeVal(wv); ok2 {
				w := abs
				if pct > 0 {
					w = int(pct * float64(r.width))
				}
				if w > 0 {
					inner = padLinesToWidth(inner, w)
				}
			}
		}
		sb.WriteString(wrapHyperlink(href, inner))
	default: // "inline" and unset — no newline
		acc := extractInlineStyle(decls)
		inner := r.renderInlineAcc(n, acc, r.width)
		inner = wrapInlineElement(n, inner)
		sb.WriteString(wrapHyperlink(href, inner))
	}
}

// renderBlockContent renders the styled, bordered, and margined content of a
// block element and returns it as a string (without trailing newline or vertical
// margins — those are the caller's responsibility). decls must already be
// resolved for n. availWidth is the column width the parent has allocated for
// this element (r.width at the top level, narrower for nested blocks).
func (r *Renderer) renderBlockContent(n *html.Node, decls map[string]string, availWidth int) string {
	// Read individual border properties; "none" disables that edge.
	bl := blockBorder{char: decls["border-left"], color: decls["border-left-color"]}
	br := blockBorder{char: decls["border-right"], color: decls["border-right-color"]}
	bt := blockBorder{char: decls["border-top"], color: decls["border-top-color"]}
	bb := blockBorder{char: decls["border-bottom"], color: decls["border-bottom-color"]}
	if bl.char == "none" {
		bl.char = ""
	}
	if br.char == "none" {
		br.char = ""
	}
	if bt.char == "none" {
		bt.char = ""
	}
	if bb.char == "none" {
		bb.char = ""
	}
	tlCorner := decls["border-top-left-corner"]
	trCorner := decls["border-top-right-corner"]
	blCorner := decls["border-bottom-left-corner"]
	brCorner := decls["border-bottom-right-corner"]
	// border-style applies named preset characters as defaults for any edge not
	// already set by an individual border-* property. Individual properties always
	// win; the preset fills in the gaps (same as CSS shorthand/longhand behavior).
	if styleVal := decls["border-style"]; styleVal != "" {
		if ts, ok := namedTableStyle(styleVal); ok {
			if bl.char == "" {
				bl.char = ts.left
			}
			if br.char == "" {
				br.char = ts.right
			}
			if ts.top != nil {
				if bt.char == "" {
					bt.char = ts.top.fill
				}
				if tlCorner == "" {
					tlCorner = ts.top.left
				}
				if trCorner == "" {
					trCorner = ts.top.right
				}
			}
			if ts.bottom != nil {
				if bb.char == "" {
					bb.char = ts.bottom.fill
				}
				if blCorner == "" {
					blCorner = ts.bottom.left
				}
				if brCorner == "" {
					brCorner = ts.bottom.right
				}
			}
			if ts.color != "" {
				if bl.color == "" {
					bl.color = ts.color
				}
				if br.color == "" {
					br.color = ts.color
				}
				if bt.color == "" {
					bt.color = ts.color
				}
				if bb.color == "" {
					bb.color = ts.color
				}
			}
		}
	}
	mlAuto := strings.TrimSpace(decls["margin-left"]) == "auto"
	mrAuto := strings.TrimSpace(decls["margin-right"]) == "auto"
	ml := 0
	if !mlAuto {
		if abs, pct, ok := parseSizeVal(decls["margin-left"]); ok {
			if pct > 0 {
				ml = int(pct * float64(availWidth))
			} else {
				ml = abs
			}
		}
	}
	mr := 0
	if !mrAuto {
		if abs, pct, ok := parseSizeVal(decls["margin-right"]); ok {
			if pct > 0 {
				mr = int(pct * float64(availWidth))
			} else {
				mr = abs
			}
		}
	}
	pl := 0
	if v := decls["padding-left"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			pl = abs
		}
	}
	pr := 0
	if v := decls["padding-right"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			pr = abs
		}
	}
	pt := 0
	if v := decls["padding-top"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			pt = abs
		}
	}
	pb := 0
	if v := decls["padding-bottom"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			pb = abs
		}
	}
	// hBorderWidth is the visual width of the top/bottom rule: the available
	// width minus margins (i.e. it spans the same columns as border-left through
	// border-right). Default: fill availWidth minus margins.
	hBorderWidth := availWidth - ml - mr

	// textSt is used as the initial accumulated style for inline rendering so
	// that text-visual properties (bold, italic, color, underline) are applied
	// directly to leaf text nodes rather than via an outer Render() call on
	// already-ANSI-coded content. This prevents lipgloss from emitting
	// per-character escape codes when it re-scans existing ANSI sequences.
	acc := extractInlineStyle(decls)

	textAlign := decls["text-align"] // "right", "center", "left", or ""

	heightLines := 0
	if v := decls["height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			heightLines = abs
		}
	}

	hasExplicitWidth := false
	if wv, ok := decls["width"]; ok {
		if abs, pct, ok2 := parseSizeVal(wv); ok2 {
			totalW := abs
			if pct > 0 {
				totalW = int(pct * float64(availWidth))
			}
			// Subtract margins, border chars, AND padding from the content width.
			inner := totalW - ml - runeLen(bl.char) - pl - pr - runeLen(br.char) - mr
			if inner > 0 {
				hBorderWidth = runeLen(bl.char) + pl + inner + pr + runeLen(br.char)
				hasExplicitWidth = true
			}
		}
	}

	// Resolve auto margins: distribute remaining space to left and/or right.
	// Auto margins only have effect when an explicit width is set; without one
	// the element already fills availWidth, leaving nothing to distribute.
	if (mlAuto || mrAuto) && hasExplicitWidth {
		remaining := availWidth - hBorderWidth - ml - mr
		if remaining < 0 {
			remaining = 0
		}
		switch {
		case mlAuto && mrAuto:
			ml = remaining / 2
			mr = remaining - ml
		case mlAuto:
			ml = remaining
		case mrAuto:
			mr = remaining
		}
	}

	// innerW is the text content area width: what children and word-wrap see.
	innerW := hBorderWidth - runeLen(bl.char) - pl - pr - runeLen(br.char)
	if innerW < 1 {
		innerW = 1
	}

	// Render inline content with text styles accumulated into leaves so we
	// never call Render() on a string that already contains ANSI codes.
	rawContent := strings.TrimRight(r.renderInlineAcc(n, acc, innerW), "\n")

	// Word-wrap single-line block content at the content area width.
	// Multi-line content (from lists, <br>, or block-in-inline) is already
	// laid out and must not be re-wrapped — list hanging indents would break.
	// Track whether wrapping produced multiple lines so we can later pad them
	// to a uniform width for right-edge alignment.
	wasWrapped := false
	ws := decls["white-space"]
	if ws != "pre" && ws != "nowrap" && !strings.Contains(rawContent, "\n") {
		wrapped := wordWrapANSI(rawContent, innerW)
		rawContent = strings.Join(wrapped, "\n")
		wasWrapped = len(wrapped) > 1
	}

	// Apply overflow clipping when overflow:hidden/clip and an explicit width is
	// set. Without overflow, text simply extends past the box (same as a browser
	// with overflow:visible). With it, each line is truncated to innerW columns.
	// text-overflow controls the truncation marker; the CSS default is "clip"
	// (no marker), unlike table cells whose UA default is "ellipsis".
	ov := decls["overflow"]
	if (ov == "hidden" || ov == "clip") && hasExplicitWidth {
		toVal := decls["text-overflow"]
		if toVal == "" {
			toVal = "clip"
		}
		suffix := textOverflowSuffix(toVal)
		lines := strings.Split(rawContent, "\n")
		for i, ln := range lines {
			lines[i] = truncateToWidth(ln, innerW, suffix)
		}
		rawContent = strings.Join(lines, "\n")
	}

	// Apply text-align and/or explicit-width padding via plain string ops.
	// With explicit width, pad lines to innerW so the right edge stays
	// vertically aligned. With text-align, distribute the padding according
	// to direction. With nowrap, skip padding — text overflows naturally and
	// overflow:hidden clipping handles the bounded case.
	var content string
	needsAlign := textAlign != "" || (hasExplicitWidth && ws != "nowrap")
	if needsAlign {
		content = alignLines(rawContent, textAlign, innerW)
	} else {
		content = rawContent
	}

	// When a right edge (padding-right or border-right) is present and
	// word-wrapping produced multiple lines of varying length, pad all lines to
	// the longest so the right edge stays vertically aligned. Single-line
	// content, pre-formatted content (lists, block-in-inline), and cases where
	// explicit width already padded lines are exempt.
	if !needsAlign && wasWrapped && (pr > 0 || br.char != "") {
		maxW := 0
		for _, ln := range strings.Split(content, "\n") {
			if vl := ansiVisibleLen(ln); vl > maxW {
				maxW = vl
			}
		}
		content = padLinesToWidth(content, maxW)
	}

	// Apply height (content box height in lines). Expansion pads with blank lines;
	// clipping only activates when overflow:hidden/clip is set.
	if heightLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > heightLines && (ov == "hidden" || ov == "clip") {
			lines = lines[:heightLines]
			content = strings.Join(lines, "\n")
		} else {
			blank := strings.Repeat(" ", innerW)
			for len(lines) < heightLines {
				lines = append(lines, blank)
			}
			content = strings.Join(lines, "\n")
		}
	}
	// Apply vertical padding inside the border. Blank rows are innerW wide so
	// the existing applyLineEdges / applyBlockBorders pipeline handles them
	// identically to content lines.
	if pt > 0 || pb > 0 {
		blank := strings.Repeat(" ", innerW)
		if pt > 0 {
			content = strings.Repeat(blank+"\n", pt) + content
		}
		if pb > 0 {
			content = content + "\n" + strings.Repeat(blank+"\n", pb)
			content = strings.TrimSuffix(content, "\n")
		}
	}
	// Padding is applied as string operations (inside the border) so it does
	// not cause lipgloss to normalize line widths across the entire block.
	if pl > 0 || pr > 0 {
		content = applyLineEdges(content, strings.Repeat(" ", pl), strings.Repeat(" ", pr))
	}
	if bl.char != "" || br.char != "" {
		content = applyBlockBorders(content, bl, br)
	}
	if top := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth); top != "" {
		content = top + "\n" + content
	}
	if bot := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth); bot != "" {
		content = content + "\n" + bot
	}
	if ml > 0 || mr > 0 {
		content = applyLineEdges(content, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
	}
	return content
}

// wrapHyperlink wraps text in an OSC 8 terminal hyperlink sequence.
// If href is empty, text is returned unchanged.
func wrapHyperlink(href, text string) string {
	if href == "" {
		return text
	}
	return "\x1b]8;;" + href + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// wrapInlineElement applies element-specific content transforms that cannot be
// expressed purely as CSS properties:
//   - q: wraps content in Unicode curly quotes (" … ")
//   - abbr: appends the title attribute expansion as " (title)" when set
func wrapInlineElement(n *html.Node, text string) string {
	switch n.Data {
	case "q":
		return "“" + text + "”"
	case "abbr":
		if title := nodeAttr(n, "title"); title != "" {
			return text + " (" + title + ")"
		}
	}
	return text
}

// renderList renders <ul> or <ol> with word-wrapped items and hanging indent.
// Each item gets a prefix ("• " or "N. "); continuation lines are indented by
// the same width so content columns align. list-style-type CSS controls the
// prefix style. padding-left / margin-left shift the entire list right.
// availWidth is the column width available from the parent block.
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

	// Count items to know how wide the numeric prefix needs to be.
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

		// renderInline may return multiple lines (e.g. nested lists). Each
		// line must be wrapped independently; nested list content is already
		// formatted and must not be re-wrapped.
		raw := strings.TrimRight(r.renderInline(c, contentWidth), "\n ")
		lines := strings.Split(raw, "\n")
		for li, line := range lines {
			if li == 0 {
				// First line: wrap with prefix on first segment, hang on rest.
				wrapped := wordWrapANSI(line, contentWidth)
				for wi, seg := range wrapped {
					if wi == 0 {
						sb.WriteString(indentStr + prefix + seg + "\n")
					} else {
						sb.WriteString(hangStr + seg + "\n")
					}
				}
			} else {
				// Subsequent lines (e.g. from nested list): re-indent under content.
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
		default: // disc
			return utf8.RuneCountInString("• ")
		}
	}
	// Numeric: width = digits in largest number + ". "
	digits := len(fmt.Sprintf("%d", count))
	return digits + 2 // "N. "
}

// listItemPrefix returns the formatted prefix for item number n (1-based).
// width is the total prefix column width so numeric prefixes can be padded
// to right-align the number (e.g. " 1." aligns with "10." when width=4).
func listItemPrefix(style string, ordered bool, n, width int) string {
	if !ordered {
		switch style {
		case "none":
			return ""
		case "circle":
			return "○ "
		case "square":
			return "■ "
		default: // disc
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
		return toRoman(n, false) + ". "
	case "upper-roman":
		return toRoman(n, true) + ". "
	default: // decimal — right-align number within available digit columns
		digits := width - 2 // subtract ". "
		return fmt.Sprintf("%*d. ", digits, n)
	}
}

// wordWrapANSI splits text into lines of at most width visible characters,
// breaking at word boundaries. ANSI escape sequences are not counted toward
// width. Returns at least one element even for empty input.
func wordWrapANSI(text string, width int) []string {
	if width <= 0 {
		width = 10
	}
	// If text fits, fast path.
	if ansiVisibleLen(text) <= width {
		return []string{text}
	}
	var lines []string
	var cur strings.Builder
	curLen := 0

	// Iterate over space-separated tokens, keeping ANSI sequences attached.
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
		cur.WriteString(space + tok)
		curLen += len(space) + vl
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
	inOSC := false
	prev := rune(0)
	for _, ch := range s {
		switch {
		case inOSC:
			// OSC ends at ST (ESC \) or BEL
			if (prev == '\x1b' && ch == '\\') || ch == '\a' {
				inOSC = false
			}
			prev = ch
		case inEsc:
			if ch == ']' {
				inOSC = true
				inEsc = false
			} else if ch >= 0x40 && ch <= 0x7e {
				inEsc = false
			}
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

// splitANSITokens splits text on whitespace but keeps ANSI sequences
// attached to the preceding visible token.
func splitANSITokens(text string) []string {
	var tokens []string
	var cur strings.Builder
	inEsc := false
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
		case inEsc:
			cur.WriteRune(ch)
			if ch == ']' {
				inOSC = true
				inEsc = false
			} else if ch >= 0x40 && ch <= 0x7e {
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

// normalizeWhiteSpace applies CSS white-space rules to a raw text node string.
//
//	normal / nowrap  — collapse all whitespace runs to a single space
//	pre / pre-wrap   — return s unchanged (all whitespace preserved)
//	pre-line         — preserve newlines; collapse spaces and tabs
func normalizeWhiteSpace(s, mode string) string {
	switch mode {
	case "pre", "pre-wrap":
		return s
	case "pre-line":
		var b strings.Builder
		lastWasSpace := false
		for _, ch := range s {
			if ch == '\n' {
				b.WriteRune('\n')
				lastWasSpace = false
			} else if ch == ' ' || ch == '\t' || ch == '\r' {
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
	default: // "normal", "nowrap"
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

// renderInline renders the inline content of n with no accumulated text style.
// availWidth is the column width available to this node for word-wrap purposes.
func (r *Renderer) renderInline(n *html.Node, availWidth int) string {
	return r.renderInlineAcc(n, inlineStyle{}, availWidth)
}

// renderInlineAcc renders the inline content of n, propagating acc (an
// accumulated text style) down to leaf text nodes so that styled Render() is
// called only on plain strings. This avoids the per-character ANSI emission
// that lipgloss produces when Render() is applied to strings that already
// contain ANSI escape sequences (including OSC 8 hyperlinks).
//
// acc is the combined text style inherited from all ancestor inline elements.
// availWidth is the column width available for any nested block children.
func (r *Renderer) renderInlineAcc(n *html.Node, acc inlineStyle, availWidth int) string {
	nDecls := r.resolveDecls(n)
	ws := "normal"
	if v := nDecls["white-space"]; v != "" {
		ws = v
	}
	tt := effectiveTransform(nDecls)
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			normalized := applyTextTransform(normalizeWhiteSpace(c.Data, ws), tt)
			// After a line break (or at start of block), the leading space that
			// whitespace-collapsing produces from a source newline is not rendered
			// by browsers. Drop it so padding-left doesn't add a double space.
			if normalized != "" {
				atLineStart := sb.Len() == 0 || sb.String()[sb.Len()-1] == '\n'
				if atLineStart {
					normalized = strings.TrimLeft(normalized, " ")
				}
				if acc.has() {
					sb.WriteString(acc.render(normalized))
				} else {
					sb.WriteString(normalized)
				}
			}
		case html.ElementNode:
			if c.Data == "br" {
				sb.WriteByte('\n')
				continue
			}
			// Lists always use the dedicated renderer regardless of CSS display,
			// so they format correctly even when nested inside block elements
			// like <blockquote>. Without this, <li><p>...</p></li> (goldmark
			// loose-list style) would inject margin newlines for each item.
			if c.Data == "ul" || c.Data == "ol" {
				if sb.Len() > 0 && sb.String()[sb.Len()-1] != '\n' {
					sb.WriteByte('\n')
				}
				sb.WriteString(r.renderList(c, c.Data == "ol", availWidth))
				continue
			}
			childDecls := r.resolveDecls(c)
			display := childDecls["display"]
			switch display {
			case "none":
				// skip
			case "block":
				// Block child inside inline flow: accumulated text style does not
				// cross block boundaries — each block is self-contained.
				if sb.Len() > 0 {
					writeMarginNewlines(&sb, parseMargin(childDecls["margin-top"])+1)
				}
				sb.WriteString(r.renderBlockContent(c, childDecls, availWidth))
				sb.WriteByte('\n')
				if mb := parseMargin(childDecls["margin-bottom"]); mb > 0 {
					writeMarginNewlines(&sb, mb+1)
				}
			default: // "inline", "inline-block", ""
				// Merge this element's text properties into the accumulated style
				// so child text nodes inherit the combined style.
				childAcc := mergeInlineStyle(acc, childDecls)
				inner := r.renderInlineAcc(c, childAcc, availWidth)
				if display == "inline-block" {
					if wv, ok := childDecls["width"]; ok {
						if abs, pct, ok2 := parseSizeVal(wv); ok2 {
							w := abs
							if pct > 0 {
								w = int(pct * float64(r.width))
							}
							if w > 0 {
								inner = padLinesToWidth(inner, w)
							}
						}
					}
				}
				inner = wrapInlineElement(c, inner)
				if c.Data == "a" {
					inner = wrapHyperlink(nodeAttr(c, "href"), inner)
				}
				sb.WriteString(inner)
			}
		}
	}
	return sb.String()
}

// renderTable renders a <table> node using the custom table engine.
// The first <tr> with <th> cells is the header. class="full-width" expands
// the table to the renderer's terminal width.
func (r *Renderer) renderTable(n *html.Node) string {
	type cell struct {
		text          string
		visualStyle   lipgloss.Style
		constraints   colConstraints
		textOverflow  string // truncation suffix from text-overflow CSS
		noWrap        bool   // true when white-space:nowrap → truncate instead of wrap
		underline     bool
		strike        bool
		paddingLeft   int
		paddingRight  int
		paddingTop    int
		paddingBottom int
		verticalAlign string   // "top", "middle", "bottom", or "" (default top)
		lines         []string // filled after column widths are known
	}

	var headers []cell
	var rows [][]cell

	var collect func(*html.Node)
	collect = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "thead", "tbody", "tfoot":
				collect(c)
			case "tr":
				var cells []cell
				isHeader := false
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode {
						continue
					}
					if td.Data != "th" && td.Data != "td" {
						continue
					}
					if td.Data == "th" {
						isHeader = true
					}
					tdDecls := r.resolveDecls(td)
					tdDeco := tdDecls["text-decoration"]
					pl, pr, pt, pb := 0, 0, 0, 0
					if v := tdDecls["padding-left"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pl = abs
						}
					}
					if v := tdDecls["padding-right"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pr = abs
						}
					}
					if v := tdDecls["padding-top"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pt = abs
						}
					}
					if v := tdDecls["padding-bottom"]; v != "" {
						if abs, _, ok := parseSizeVal(v); ok {
							pb = abs
						}
					}
					cells = append(cells, cell{
						text:          applyTextTransform(textContent(td), effectiveTransform(tdDecls)),
						visualStyle:   declsToStyle(tdDecls),
						constraints:   r.cellConstraints(td),
						textOverflow:  textOverflowSuffix(tdDecls["text-overflow"]),
						noWrap:        tdDecls["white-space"] == "nowrap",
						underline:     strings.Contains(tdDeco, "underline"),
						strike:        strings.Contains(tdDeco, "line-through"),
						paddingLeft:   pl,
						paddingRight:  pr,
						paddingTop:    pt,
						paddingBottom: pb,
						verticalAlign: tdDecls["vertical-align"],
					})
				}
				if len(cells) == 0 {
					continue
				}
				if isHeader && len(headers) == 0 {
					headers = cells
				} else {
					rows = append(rows, cells)
				}
			}
		}
	}
	collect(n)

	numCols := len(headers)
	if numCols == 0 && len(rows) > 0 {
		numCols = len(rows[0])
	}
	if numCols == 0 {
		return ""
	}

	tableDecls := r.resolveDecls(n)
	ts := applyTableCSSToStyle(namedTableStyle_default(), tableDecls)
	fullWidth := strings.TrimSpace(tableDecls["width"]) == "100%"

	// Build column constraints: start from header cells, grow natural width
	// by scanning all data cells.
	cols := make([]colConstraints, numCols)
	for i := 0; i < numCols; i++ {
		if i < len(headers) {
			cols[i] = headers[i].constraints
			cols[i].natural = runeLen(headers[i].text) + headers[i].paddingLeft + headers[i].paddingRight
		}
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= numCols {
				break
			}
			if w := runeLen(c.text) + c.paddingLeft + c.paddingRight; w > cols[i].natural {
				cols[i].natural = w
			}
			// Fill in size constraints from data cells when header didn't set them.
			dc := c.constraints
			if cols[i].fixed == 0 && dc.fixed > 0 {
				cols[i].fixed = dc.fixed
			}
			if cols[i].percent == 0 && dc.percent > 0 {
				cols[i].percent = dc.percent
			}
			if dc.minWidth > cols[i].minWidth {
				cols[i].minWidth = dc.minWidth
			}
			if dc.minPercent > cols[i].minPercent {
				cols[i].minPercent = dc.minPercent
			}
			if dc.maxWidth > 0 && (cols[i].maxWidth == 0 || dc.maxWidth < cols[i].maxWidth) {
				cols[i].maxWidth = dc.maxWidth
			}
			if dc.maxPercent > 0 && (cols[i].maxPercent == 0 || dc.maxPercent < cols[i].maxPercent) {
				cols[i].maxPercent = dc.maxPercent
			}
		}
	}

	sepW := runeLen(ts.sep)
	overhead := runeLen(ts.left) + (numCols-1)*sepW + runeLen(ts.right)
	widths := sizeColumns(cols, r.width-overhead, fullWidth)

	// Compute wrapped (or truncated) lines for every cell now that column
	// widths are final. noWrap cells use single-line truncation; all others wrap.
	fillLines := func(cells []cell) {
		for i := range cells {
			if i >= numCols {
				break
			}
			pl := cells[i].paddingLeft
			pr := cells[i].paddingRight
			contentW := widths[i] - pl - pr
			if contentW < 1 {
				contentW = 1
			}
			if cells[i].noWrap {
				cells[i].lines = []string{truncateToWidth(cells[i].text, contentW, cells[i].textOverflow)}
			} else {
				cells[i].lines = wrapToWidth(cells[i].text, contentW)
			}
			if pt := cells[i].paddingTop; pt > 0 {
				blank := make([]string, pt)
				cells[i].lines = append(blank, cells[i].lines...)
			}
			if pb := cells[i].paddingBottom; pb > 0 {
				cells[i].lines = append(cells[i].lines, make([]string, pb)...)
			}
		}
	}
	fillLines(headers)
	for i := range rows {
		fillLines(rows[i])
	}

	paint := makePainter(ts.color)

	renderRow := func(cells []cell) string {
		// Find the tallest cell in this row.
		height := 1
		for i := 0; i < numCols && i < len(cells); i++ {
			if h := len(cells[i].lines); h > height {
				height = h
			}
		}
		var sb strings.Builder
		for lineIdx := 0; lineIdx < height; lineIdx++ {
			sb.WriteString(paint(ts.left))
			for i := 0; i < numCols; i++ {
				if i > 0 {
					sb.WriteString(paint(ts.sep))
				}
				var c cell
				if i < len(cells) {
					c = cells[i]
				}
				offset := 0
				switch c.verticalAlign {
				case "bottom":
					offset = height - len(c.lines)
				case "middle":
					offset = (height - len(c.lines)) / 2
				}
				var line string
				if ci := lineIdx - offset; ci >= 0 && ci < len(c.lines) {
					line = c.lines[ci]
				}
				contentW := widths[i] - c.paddingLeft - c.paddingRight
				if contentW < 1 {
					contentW = 1
				}
				rendered := c.visualStyle.Width(contentW).Render(line)
				if c.paddingLeft > 0 {
					rendered = strings.Repeat(" ", c.paddingLeft) + rendered
				}
				if c.paddingRight > 0 {
					rendered = rendered + strings.Repeat(" ", c.paddingRight)
				}
				if c.underline {
					rendered = "\x1b[4m" + rendered + "\x1b[24m"
				}
				if c.strike {
					rendered = "\x1b[9m" + rendered + "\x1b[29m"
				}
				sb.WriteString(rendered)
			}
			sb.WriteString(paint(ts.right))
			sb.WriteByte('\n')
		}
		return sb.String()
	}

	var out strings.Builder
	out.WriteString(drawHBorder(widths, ts.top, ts.color))
	if len(headers) > 0 {
		out.WriteString(renderRow(headers))
		out.WriteString(drawHBorder(widths, ts.header, ts.color))
	}
	for i, row := range rows {
		if i > 0 {
			out.WriteString(drawHBorder(widths, ts.rowSep, ts.color))
		}
		out.WriteString(renderRow(row))
	}
	out.WriteString(drawHBorder(widths, ts.bottom, ts.color))
	return out.String()
}

func namedTableStyle_default() tableStyle {
	ts, _ := namedTableStyle("normal")
	return ts
}

// textContent returns the collapsed, trimmed text of all descendant text nodes.
// Used for table cell content where white-space: nowrap applies.
func textContent(n *html.Node) string {
	return strings.TrimSpace(normalizeWhiteSpace(rawContent(n), "normal"))
}

// rawContent returns the concatenated text of all descendant text nodes,
// preserving whitespace (used for <pre> where newlines are significant).
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
