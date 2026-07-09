package cssengine

import (
	"strings"

	"golang.org/x/net/html"
)

// combinator describes how a selectorPart connects to the part to its left
// (ancestor direction) in a complex selector.
type combinator int

const (
	descendant combinator = iota // space: any ancestor
	child                        // >: immediate parent only
	adjacent                     // +: immediate preceding sibling
)

// attrOp is the match operator in an attribute selector.
type attrOp int

const (
	opExists    attrOp = iota // [attr]      — attribute is present
	opEquals                  // [attr=val]  — attribute has exact value
	opIncludes                // [attr~=val] — whitespace-separated list contains value
	opDashMatch               // [attr|=val] — exact value or value followed by "-"
	opPrefix                  // [attr^=val] — value starts with prefix
	opSuffix                  // [attr$=val] — value ends with suffix
	opSubstring               // [attr*=val] — value contains substring
)

// attrSel is a single [attr] or [attr=val] condition.
type attrSel struct {
	key string
	op  attrOp
	val string // empty for opExists
}

// selectorPart is one compound component of a CSS selector.
// combo is the combinator that connects this part to the one to its left;
// it is ignored on parts[0].
type selectorPart struct {
	element    string
	id         string
	classes    []string
	pseudos    []string
	attrs      []attrSel
	combo      combinator
	pseudoElem string
}

// SelectorGroup is a parsed comma-separated selector group.
type SelectorGroup struct {
	groups [][]selectorPart
}

// ParseSelectorGroup parses a comma-separated selector group.
func ParseSelectorGroup(sel string) SelectorGroup {
	var group SelectorGroup
	for _, s := range strings.Split(sel, ",") {
		if s = strings.TrimSpace(s); s != "" {
			group.groups = append(group.groups, parseSelector(s))
		}
	}
	return group
}

// Match reports whether n matches any selector in the group.
func (g SelectorGroup) Match(n *html.Node, focusAttr string) bool {
	for _, parts := range g.groups {
		if matchSelector(n, parts, focusAttr) {
			return true
		}
	}
	return false
}

// parseSelector parses a complex CSS selector string into selectorParts.
func parseSelector(sel string) []selectorPart {
	var parts []selectorPart
	combo := descendant
	i, n := 0, len(sel)

	isCSSSpace := func(b byte) bool {
		return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
	}
	for i < n {
		for i < n && isCSSSpace(sel[i]) {
			i++
		}
		if i >= n {
			break
		}
		if sel[i] == '>' {
			combo = child
			i++
			for i < n && isCSSSpace(sel[i]) {
				i++
			}
			continue
		}
		if sel[i] == '+' {
			combo = adjacent
			i++
			for i < n && isCSSSpace(sel[i]) {
				i++
			}
			continue
		}
		start := i
		for i < n && !isCSSSpace(sel[i]) && sel[i] != '>' && sel[i] != '+' {
			if sel[i] == '[' {
				for i < n && sel[i] != ']' {
					i++
				}
				if i < n {
					i++
				}
			} else {
				i++
			}
		}
		if tok := sel[start:i]; tok != "" {
			p := parseSimpleSelector(tok)
			p.combo = combo
			parts = append(parts, p)
		}
		combo = descendant
	}
	return parts
}

// parseSimpleSelector parses a single compound-selector token such as
// "div#main.foo.bar:first-child[href=val]" into a selectorPart.
func parseSimpleSelector(tok string) selectorPart {
	var p selectorPart
	i, n := 0, len(tok)

	j := i
	for j < n && tok[j] != '#' && tok[j] != '.' && tok[j] != ':' && tok[j] != '[' {
		j++
	}
	// CSS type selectors match case-insensitively, and golang.org/x/net/html
	// always lowercases parsed HTML tag names — lowercase here so matchPart's
	// n.Data != p.element comparison actually matches an uppercase/mixed-case
	// selector like "DIV { ... }" against a real <div>.
	p.element = strings.ToLower(tok[i:j])
	i = j

	for i < n {
		switch tok[i] {
		case '#':
			i++
			j = i
			for j < n && tok[j] != '#' && tok[j] != '.' && tok[j] != ':' && tok[j] != '[' {
				j++
			}
			p.id = tok[i:j]
			i = j
		case '.':
			i++
			j = i
			for j < n && tok[j] != '#' && tok[j] != '.' && tok[j] != ':' && tok[j] != '[' {
				j++
			}
			if cls := tok[i:j]; cls != "" {
				p.classes = append(p.classes, cls)
			}
			i = j
		case ':':
			i++
			j = i
			for j < n && tok[j] != '#' && tok[j] != '.' && tok[j] != ':' && tok[j] != '[' {
				if tok[j] == '(' {
					for j < n && tok[j] != ')' {
						j++
					}
					if j < n {
						j++
					}
				} else {
					j++
				}
			}
			if ps := strings.ToLower(tok[i:j]); ps != "" {
				switch ps {
				case "before", "after", "marker":
					p.pseudoElem = ps
				default:
					p.pseudos = append(p.pseudos, ps)
				}
			}
			i = j
		case '[':
			i++
			j = i
			for j < n && tok[j] != ']' {
				j++
			}
			if a, ok := parseAttrSel(tok[i:j]); ok {
				p.attrs = append(p.attrs, a)
			}
			if j < n {
				i = j + 1
			} else {
				i = j
			}
		default:
			i++
		}
	}
	return p
}

// parseAttrSel parses the content of [...] into an attrSel.
func parseAttrSel(s string) (attrSel, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return attrSel{}, false
	}

	if idx, tokenLen, op, ok := findAttrSelectorOp(s); ok {
		key := strings.TrimSpace(s[:idx])
		val := unquoteAttrSelectorValue(strings.TrimSpace(s[idx+tokenLen:]))
		if key == "" {
			return attrSel{}, false
		}
		return attrSel{key: key, op: op, val: val}, true
	}
	return attrSel{key: s, op: opExists}, true
}

func findAttrSelectorOp(s string) (idx, tokenLen int, op attrOp, ok bool) {
	var quote byte
	for i := 0; i < len(s); i++ {
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
			continue
		}
		if s[i] == '"' || s[i] == '\'' {
			quote = s[i]
			continue
		}
		if i+1 < len(s) && s[i+1] == '=' {
			switch s[i] {
			case '~':
				return i, 2, opIncludes, true
			case '|':
				return i, 2, opDashMatch, true
			case '^':
				return i, 2, opPrefix, true
			case '$':
				return i, 2, opSuffix, true
			case '*':
				return i, 2, opSubstring, true
			}
		}
		if s[i] == '=' {
			return i, 1, opEquals, true
		}
	}
	return 0, 0, opExists, false
}

func unquoteAttrSelectorValue(val string) string {
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		return val[1 : len(val)-1]
	}
	return val
}

type specificityScore struct {
	ids      int
	classes  int
	elements int
}

func (s specificityScore) less(other specificityScore) bool {
	if s.ids != other.ids {
		return s.ids < other.ids
	}
	if s.classes != other.classes {
		return s.classes < other.classes
	}
	return s.elements < other.elements
}

// specificity returns the CSS specificity of a parsed selector.
func specificity(parts []selectorPart) specificityScore {
	var s specificityScore
	for _, p := range parts {
		if p.element != "" && p.element != "*" {
			s.elements++
		}
		if p.id != "" {
			s.ids++
		}
		if p.pseudoElem != "" {
			s.elements++
		}
		s.classes += len(p.classes)
		s.classes += len(p.attrs)
		for _, ps := range p.pseudos {
			if strings.HasPrefix(ps, "not(") && strings.HasSuffix(ps, ")") {
				inner := ps[4 : len(ps)-1]
				inner = strings.TrimSpace(inner)
				innerSpec := specificity([]selectorPart{parseSimpleSelector(inner)})
				s.ids += innerSpec.ids
				s.classes += innerSpec.classes
				s.elements += innerSpec.elements
			} else {
				s.classes++
			}
		}
	}
	return s
}

// matchPart reports whether node n satisfies all simple-selector conditions in p.
func matchPart(n *html.Node, p selectorPart, focusAttr string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if p.pseudoElem != "" {
		return false
	}
	if p.element != "" && p.element != "*" && n.Data != p.element {
		return false
	}
	if p.id != "" && nodeAttr(n, "id") != p.id {
		return false
	}
	for _, cls := range p.classes {
		found := false
		for _, c := range strings.Fields(nodeAttr(n, "class")) {
			if c == cls {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, ps := range p.pseudos {
		if !matchPseudo(n, ps, focusAttr) {
			return false
		}
	}
	for _, a := range p.attrs {
		if !matchAttr(n, a) {
			return false
		}
	}
	return true
}

// matchPseudo reports whether n satisfies a single pseudo-class condition.
func matchPseudo(n *html.Node, pseudo, focusAttr string) bool {
	// :not(<selector>) — negation pseudo-class (simple selector argument only).
	if strings.HasPrefix(pseudo, "not(") && strings.HasSuffix(pseudo, ")") {
		inner := pseudo[4 : len(pseudo)-1]
		inner = strings.TrimSpace(inner)
		p := parseSimpleSelector(inner)
		return !matchPart(n, p, focusAttr)
	}

	if n.Parent == nil {
		return false
	}
	switch pseudo {
	case "root":
		return n.Parent.Type == html.DocumentNode
	case "first-child":
		for s := n.Parent.FirstChild; s != nil; s = s.NextSibling {
			if s.Type == html.ElementNode {
				return s == n
			}
		}
	case "last-child":
		for s := n.Parent.LastChild; s != nil; s = s.PrevSibling {
			if s.Type == html.ElementNode {
				return s == n
			}
		}
	case "nth-child(odd)", "nth-child(even)":
		wantOdd := pseudo == "nth-child(odd)"
		idx := 0
		for s := n.Parent.FirstChild; s != nil; s = s.NextSibling {
			if s.Type == html.ElementNode {
				idx++
				if s == n {
					return (idx%2 == 1) == wantOdd
				}
			}
		}
	case "checked":
		return nodeHasAttr(n, "checked")
	case "disabled":
		return nodeHasAttr(n, "disabled")
	case "required":
		return nodeHasAttr(n, "required")
	case "focus":
		return focusAttr != "" && nodeHasAttr(n, focusAttr)
	}
	return false
}

// matchAttr reports whether n satisfies a single attribute selector condition.
func matchAttr(n *html.Node, a attrSel) bool {
	switch a.op {
	case opExists:
		for _, attr := range n.Attr {
			if attr.Key == a.key {
				return true
			}
		}
		return false
	case opEquals:
		return nodeAttr(n, a.key) == a.val
	case opIncludes:
		if a.val == "" {
			return false
		}
		for _, field := range strings.Fields(nodeAttr(n, a.key)) {
			if field == a.val {
				return true
			}
		}
		return false
	case opDashMatch:
		if a.val == "" {
			return false
		}
		val := nodeAttr(n, a.key)
		return val == a.val || strings.HasPrefix(val, a.val+"-")
	case opPrefix:
		return a.val != "" && strings.HasPrefix(nodeAttr(n, a.key), a.val)
	case opSuffix:
		return a.val != "" && strings.HasSuffix(nodeAttr(n, a.key), a.val)
	case opSubstring:
		return a.val != "" && strings.Contains(nodeAttr(n, a.key), a.val)
	}
	return false
}

// matchSelector matches n against a parsed selector.
func matchSelector(n *html.Node, parts []selectorPart, focusAttr string) bool {
	if len(parts) == 0 || n.Type != html.ElementNode {
		return false
	}
	if !matchPart(n, parts[len(parts)-1], focusAttr) {
		return false
	}
	cur := n.Parent
	// curNode tracks the current element for adjacent-sibling matching.
	curNode := n
	for i := len(parts) - 2; i >= 0; i-- {
		switch parts[i+1].combo {
		case child:
			if cur == nil || cur.Type != html.ElementNode || !matchPart(cur, parts[i], focusAttr) {
				return false
			}
			curNode = cur
			cur = cur.Parent
		case adjacent:
			// Find the immediately preceding element sibling of curNode.
			var prev *html.Node
			for s := curNode.PrevSibling; s != nil; s = s.PrevSibling {
				if s.Type == html.ElementNode {
					prev = s
					break
				}
			}
			if prev == nil || !matchPart(prev, parts[i], focusAttr) {
				return false
			}
			curNode = prev
			cur = prev.Parent
		default:
			found := false
			for ; cur != nil; cur = cur.Parent {
				if cur.Type == html.ElementNode && matchPart(cur, parts[i], focusAttr) {
					found = true
					curNode = cur
					cur = cur.Parent
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeHasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
