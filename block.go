package htmlterm

import (
	"strconv"
	"strings"

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

// parseMargin parses a CSS margin-top / margin-bottom value as a line count.
func parseMargin(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// writeMarginNewlines ensures the builder has at least n trailing newlines.
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

// renderDisplayNode renders n according to its CSS display property.
func (r *Renderer) renderDisplayNode(sb *strings.Builder, n *html.Node) {
	decls := r.resolveDecls(n)
	href := ""
	if n.Data == "a" {
		href = nodeAttr(n, "href")
	}
	switch decls["display"] {
	case "none":
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
	default:
		acc := extractInlineStyle(decls)
		inner := r.renderInlineAcc(n, acc, r.width)
		if decls["visibility"] == "hidden" {
			inner = blankVisibleContent(inner)
		}
		inner = wrapInlineElement(n, inner)
		sb.WriteString(wrapHyperlink(href, inner))
	}
}

// renderBlockContent renders the styled, bordered, and margined content of a block element.
func (r *Renderer) renderBlockContent(n *html.Node, decls map[string]string, availWidth int) string {
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

// parseCSSContentString extracts the string value from a CSS content property.
func parseCSSContentString(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "none" || v == "normal" {
		return ""
	}
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return ""
}

// wrapHyperlink wraps text in an OSC 8 terminal hyperlink sequence.
func wrapHyperlink(href, text string) string {
	if href == "" {
		return text
	}
	return "\x1b]8;;" + href + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// wrapInlineElement applies element-specific content transforms.
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
