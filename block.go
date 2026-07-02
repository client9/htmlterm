package htmlterm

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/net/html"
)

// blockBorder holds the character and optional color for one edge of a block border.
type blockBorder struct {
	char  string
	color string
}

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

func drawBlockHBorder(fill, color, leftCorner, rightCorner string, width int, p colorprofile.Profile) string {
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
	return drawHRule([]int{max(0, width-runeLen(lc)-runeLen(rc))}, fill, color, lc, "", rc, p)
}

func applyBlockBorders(content string, left, right blockBorder, p colorprofile.Profile) string {
	paintL := makePainter(left.color, p)
	paintR := makePainter(right.color, p)
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

// parseMargin parses a CSS margin-top / margin-bottom value as a line count.
func parseMargin(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// renderDisplayNode renders n according to its CSS display property.
func (r *Renderer) renderDisplayNode(w *cappedWriter, n *html.Node) {
	decls := r.resolveDecls(n)
	href := ""
	if n.Data == "a" {
		href = nodeAttr(n, "href")
	}
	switch decls["display"] {
	case "none":
	case "block":
		if mt := parseMargin(decls["margin-top"]); mt > 0 && w.Len() > 0 {
			w.WriteAtLeastNewlines(mt + 1)
		}
		content := r.wrapHyperlink(href, r.renderBlockContent(n, decls, r.width))
		ws := decls["white-space"]
		if ws == "pre" || ws == "pre-wrap" {
			w.EnterPre()
		}
		w.WriteString(content)
		if ws == "pre" || ws == "pre-wrap" {
			w.ExitPre()
		}
		w.writeNewline()
		w.WriteAtLeastNewlines(parseMargin(decls["margin-bottom"]) + 1)
	case "inline-block":
		acc := extractInlineStyle(decls)
		inner := r.renderInlineAcc(n, acc, r.width)
		if wv, ok := decls["width"]; ok {
			if abs, pct, ok2 := parseSizeVal(wv); ok2 {
				colWidth := abs
				if pct > 0 {
					colWidth = int(pct * float64(r.width))
				}
				if colWidth > 0 {
					inner = padLinesToWidth(inner, colWidth)
				}
			}
		}
		w.WriteString(r.wrapHyperlink(href, inner))
	default:
		acc := extractInlineStyle(decls)
		inner := r.renderInlineAcc(n, acc, r.width)
		if decls["visibility"] == "hidden" {
			inner = blankVisibleContent(inner)
		}
		w.WriteString(r.wrapHyperlink(href, inner))
	}
}

// renderBlockContent renders the styled, bordered, and margined content of a block element.
func (r *Renderer) renderBlockContent(n *html.Node, decls map[string]string, availWidth int) string {
	bl := blockBorder{char: parseCSSString(decls["border-left"]), color: decls["border-left-color"]}
	br := blockBorder{char: parseCSSString(decls["border-right"]), color: decls["border-right-color"]}
	bt := blockBorder{char: parseCSSString(decls["border-top"]), color: decls["border-top-color"]}
	bb := blockBorder{char: parseCSSString(decls["border-bottom"]), color: decls["border-bottom-color"]}
	tlCorner := parseCSSString(decls["border-top-left-corner"])
	trCorner := parseCSSString(decls["border-top-right-corner"])
	blCorner := parseCSSString(decls["border-bottom-left-corner"])
	brCorner := parseCSSString(decls["border-bottom-right-corner"])
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
	hBorderWidth := availWidth - ml - mr
	acc := extractInlineStyle(decls)
	textAlign := decls["text-align"]
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
			inner := totalW - ml - runeLen(bl.char) - pl - pr - runeLen(br.char) - mr
			if inner > 0 {
				hBorderWidth = runeLen(bl.char) + pl + inner + pr + runeLen(br.char)
				hasExplicitWidth = true
			}
		}
	}
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

	innerW := hBorderWidth - runeLen(bl.char) - pl - pr - runeLen(br.char)
	if innerW < 1 {
		innerW = 1
	}
	rawContent := strings.TrimRight(r.renderInlineAcc(n, acc, innerW), "\n")
	wasWrapped := false
	ws := decls["white-space"]
	if ws != "pre" && ws != "pre-wrap" && strings.HasSuffix(rawContent, " ") && !strings.HasSuffix(rawContent, "  ") {
		rawContent = rawContent[:len(rawContent)-1]
	}
	if ws != "pre" && ws != "nowrap" && !strings.Contains(rawContent, "\n") {
		breakMode := decls["overflow-wrap"]
		if breakMode == "" {
			breakMode = decls["word-break"]
		}
		wrapped := wordWrapANSI(rawContent, innerW, breakMode)
		rawContent = strings.Join(wrapped, "\n")
		wasWrapped = len(wrapped) > 1
	}

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

	var content string
	needsAlign := textAlign != "" || (hasExplicitWidth && ws != "nowrap")
	if needsAlign {
		content = alignLines(rawContent, textAlign, innerW)
	} else {
		content = rawContent
	}

	if !needsAlign && wasWrapped && (pr > 0 || br.char != "") {
		maxW := 0
		for _, ln := range strings.Split(content, "\n") {
			if vl := ansiVisibleLen(ln); vl > maxW {
				maxW = vl
			}
		}
		content = padLinesToWidth(content, maxW)
	}

	minH := 0
	if v := decls["min-height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			minH = abs
		}
	}
	maxH := 0
	if v := decls["max-height"]; v != "" {
		if abs, _, ok := parseSizeVal(v); ok && abs > 0 {
			maxH = abs
		}
	}
	if heightLines > 0 || minH > 0 || maxH > 0 {
		lines := strings.Split(content, "\n")
		blank := strings.Repeat(" ", innerW)
		if heightLines > 0 {
			// Fixed height takes priority over min/max.
			if len(lines) > heightLines && (ov == "hidden" || ov == "clip") {
				lines = lines[:heightLines]
			} else {
				for len(lines) < heightLines {
					lines = append(lines, blank)
				}
			}
		} else {
			// max-height clips (requires overflow: hidden/clip).
			if maxH > 0 && len(lines) > maxH && (ov == "hidden" || ov == "clip") {
				lines = lines[:maxH]
			}
			// min-height always pads.
			if minH > 0 {
				for len(lines) < minH {
					lines = append(lines, blank)
				}
			}
		}
		content = strings.Join(lines, "\n")
	}
	// text-indent: apply only when this element's first rendered content is
	// direct inline text (not a child block that will apply its own indent).
	if v := decls["text-indent"]; v != "" && r.firstContentIsInline(n) {
		indent := 0
		if abs, pct, ok := parseSizeVal(v); ok {
			if pct > 0 {
				indent = int(pct * float64(innerW))
			} else {
				indent = abs
			}
		}
		if indent > 0 {
			lines := strings.SplitN(content, "\n", 2)
			if len(lines) >= 1 {
				lines[0] = strings.Repeat(" ", indent) + lines[0]
			}
			content = strings.Join(lines, "\n")
		}
	}
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
	if pl > 0 || pr > 0 {
		content = applyLineEdges(content, strings.Repeat(" ", pl), strings.Repeat(" ", pr))
	}
	if bl.char != "" || br.char != "" {
		content = applyBlockBorders(content, bl, br, r.profile)
	}
	if top := drawBlockHBorder(bt.char, bt.color, tlCorner, trCorner, hBorderWidth, r.profile); top != "" {
		if content != "" {
			content = top + "\n" + content
		} else {
			content = top
		}
	}
	if bot := drawBlockHBorder(bb.char, bb.color, blCorner, brCorner, hBorderWidth, r.profile); bot != "" {
		if content != "" {
			content = content + "\n" + bot
		} else {
			content = bot
		}
	}
	if ml > 0 || mr > 0 {
		content = applyLineEdges(content, strings.Repeat(" ", ml), strings.Repeat(" ", mr))
	}
	if decls["visibility"] == "hidden" {
		content = blankVisibleContent(content)
	}
	return content
}

// firstContentIsInline reports whether n's first non-whitespace content is
// inline (a text node or inline element). Returns false when the first child
// is a block-level element, meaning text-indent should not be applied here —
// the block child will apply its own inherited value on its own first line.
func (r *Renderer) firstContentIsInline(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			if strings.TrimSpace(c.Data) != "" {
				return true
			}
			continue
		}
		if c.Type == html.ElementNode {
			return r.resolveDecls(c)["display"] != "block"
		}
	}
	return false
}

// blankVisibleContent removes all ANSI escapes and replaces every non-newline
// character with a space, preserving line structure for layout purposes.
func blankVisibleContent(s string) string {
	plain := stripANSI(s)
	var b strings.Builder
	b.Grow(len(plain))
	for _, ch := range plain {
		if ch == '\n' {
			b.WriteRune('\n')
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// parseCSSString unquotes a CSS quoted string token (e.g. `"│"` → `│`).
// Returns "" for unquoted values, keywords, or empty input.
func parseCSSString(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return ""
	}
	if (v[0] != '"' || v[len(v)-1] != '"') && (v[0] != '\'' || v[len(v)-1] != '\'') {
		return ""
	}
	inner := v[1 : len(v)-1]
	if !strings.ContainsRune(inner, '\\') {
		return sanitizeTerminalText(inner, true)
	}
	// Decode CSS string escape sequences.
	var b strings.Builder
	runes := []rune(inner)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i+1 >= len(runes) {
			b.WriteRune(runes[i])
			continue
		}
		i++
		next := runes[i]
		// \<newline> — line continuation, consume and skip the newline
		if next == '\n' {
			continue
		}
		// \<hex>{1,6}<optional-space> — Unicode code point
		if isHexRune(next) {
			hexStart := i
			for i+1 < len(runes) && isHexRune(runes[i+1]) && i-hexStart < 5 {
				i++
			}
			cp, _ := strconv.ParseInt(string(runes[hexStart:i+1]), 16, 32)
			b.WriteRune(rune(cp))
			// consume optional single whitespace after hex escape
			if i+1 < len(runes) && isSpaceRune(runes[i+1]) {
				i++
			}
			continue
		}
		// \<other> — the character itself
		b.WriteRune(next)
	}
	return sanitizeTerminalText(b.String(), true)
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

// parseCSSContentString extracts the text from a CSS content property value.
// Supports: quoted strings, attr(), counter(), counters(), open-quote,
// close-quote, no-open-quote, no-close-quote. Returns "" for none/normal.
func (r *Renderer) parseCSSContentString(v string, n *html.Node) string {
	v = strings.TrimSpace(v)
	if v == "none" || v == "normal" || v == "" {
		return ""
	}
	cs := r.counterMap[n]
	var b strings.Builder
	for v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			break
		}
		switch {
		case strings.HasPrefix(v, "attr("):
			end := strings.Index(v, ")")
			if end < 0 {
				return b.String()
			}
			attrName := strings.TrimSpace(v[5:end])
			v = v[end+1:]
			if n != nil {
				b.WriteString(sanitizeTerminalText(nodeAttr(n, attrName), true))
			}

		case strings.HasPrefix(v, "counters("):
			end := strings.Index(v, ")")
			if end < 0 {
				return b.String()
			}
			name, sep, style := parseCounterFnArgs(v[9:end])
			v = v[end+1:]
			vals := cs.values(name)
			parts := make([]string, len(vals))
			for i, val := range vals {
				parts[i] = formatCounterValue(val, style)
			}
			b.WriteString(strings.Join(parts, sep))

		case strings.HasPrefix(v, "counter("):
			end := strings.Index(v, ")")
			if end < 0 {
				return b.String()
			}
			name, _, style := parseCounterFnArgs(v[8:end])
			v = v[end+1:]
			b.WriteString(formatCounterValue(cs.value(name), style))

		case strings.HasPrefix(v, "no-open-quote"):
			v = v[len("no-open-quote"):]
			r.quoteDepth++

		case strings.HasPrefix(v, "no-close-quote"):
			v = v[len("no-close-quote"):]
			if r.quoteDepth > 0 {
				r.quoteDepth--
			}

		case strings.HasPrefix(v, "open-quote"):
			v = v[len("open-quote"):]
			pairs := parseQuotes(r.resolveDecls(n)["quotes"])
			depth := r.quoteDepth
			if depth >= len(pairs) {
				depth = len(pairs) - 1
			}
			b.WriteString(pairs[depth][0])
			r.quoteDepth++

		case strings.HasPrefix(v, "close-quote"):
			v = v[len("close-quote"):]
			if r.quoteDepth > 0 {
				r.quoteDepth--
			}
			pairs := parseQuotes(r.resolveDecls(n)["quotes"])
			depth := r.quoteDepth
			if depth >= len(pairs) {
				depth = len(pairs) - 1
			}
			b.WriteString(pairs[depth][1])

		case v[0] == '"' || v[0] == '\'':
			// quoted string — find the closing quote
			q := v[0]
			i := 1
			for i < len(v) {
				if v[i] == '\\' {
					i += 2
					continue
				}
				if v[i] == q {
					i++
					break
				}
				i++
			}
			b.WriteString(parseCSSString(v[:i]))
			v = v[i:]

		default:
			// unrecognised token — skip one word
			i := strings.IndexAny(v, " \t\n\r")
			if i < 0 {
				return b.String()
			}
			v = v[i:]
		}
	}
	return b.String()
}

// wrapHyperlink wraps text in an OSC 8 terminal hyperlink sequence.
func (r *Renderer) wrapHyperlink(href, text string) string {
	href = sanitizeTerminalText(href, false)
	if href == "" || r.noOSC8Links || r.profile <= colorprofile.Ascii {
		return text
	}
	return "\x1b]8;;" + href + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
