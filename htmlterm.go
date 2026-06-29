// Package htmlterm renders a restricted HTML+CSS subset to terminal strings
// via lipgloss. Supported selectors: element, .class, element.class, and
// space-separated descendant chains. See CSS.md for supported properties.
package htmlterm

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Renderer renders HTML+CSS to terminal strings.
type Renderer struct {
	rules []rule
	width int
}

// uaCSS is the built-in default stylesheet (lowest priority — user CSS overrides it).
const uaCSS = `
p, blockquote, pre, h1, h2, h3, h4, h5, h6, div, section, article, header, footer, main, nav, aside { display: block; }
dl, dt, dd, figure, figcaption  { display: block; }
p                       { margin-bottom: 1; }
h1, h2, h3, h4, h5, h6 { font-weight: bold; }
th                      { font-weight: bold; }
dt                      { font-weight: bold; }
strong, b               { font-weight: bold; }
em, i, dfn              { font-style: italic; }
samp, var, cite, figcaption { font-style: italic; }
a                       { text-decoration: underline; }
u, ins                  { text-decoration: underline; }
pre                     { white-space: pre; }
ul, ol                  { padding-left: 4; }
dd                      { padding-left: 4; }
dl                      { margin-bottom: 1; }
td, th                  { white-space: nowrap; text-overflow: ellipsis; }
blockquote              { border-left: │; border-left-color: #555555; padding-left: 1; padding-right: 2; }
s, del                  { text-decoration: line-through; }
kbd                     { font-weight: bold; }
mark                    { background-color: #cc9900; color: #000000; }
small                   { color: #888888; }
sup                     { text-transform: superscript; }
sub                     { text-transform: subscript; }
`

// New parses css and returns a Renderer. width is the terminal column count.
// uaCSS defaults are prepended so the caller's css always wins at equal specificity.
func New(css string, width int) (*Renderer, error) {
	rules, err := parseCSS(uaCSS + css)
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	return &Renderer{rules: rules, width: width}, nil
}

// Render parses htmlStr and returns a styled terminal string.
// Any <style> elements in htmlStr are parsed and their rules applied for this
// call only (appended after the base stylesheet, so they win at equal specificity).
func (r *Renderer) Render(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("htmlterm: %w", err)
	}
	rr := r
	if extra := extractStyleRules(doc); len(extra) > 0 {
		combined := make([]rule, len(r.rules)+len(extra))
		copy(combined, r.rules)
		copy(combined[len(r.rules):], extra)
		rr = &Renderer{rules: combined, width: r.width}
	}
	var sb strings.Builder
	rr.renderNode(&sb, doc)
	return sb.String(), nil
}

// extractStyleRules walks doc and parses CSS text from every <style> element.
func extractStyleRules(doc *html.Node) []rule {
	var rules []rule
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" {
			if parsed, err := parseCSS(rawContent(n)); err == nil {
				rules = append(rules, parsed...)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return rules
}

// inheritableProps is the set of CSS properties that propagate from parent to
// child when no direct declaration for that property applies to the child.
var inheritableProps = map[string]bool{
	"color":           true,
	"font-weight":     true,
	"font-style":      true,
	"text-decoration": true,
	"text-align":      true,
	"white-space":     true,
	"text-transform":  true,
	"font-variant":    true,
}

// resolveDecls returns the winning CSS declarations for node n, merging all
// matching rules by ascending specificity, then filling missing inheritable
// properties from the nearest ancestor that directly declares them.
func (r *Renderer) resolveDecls(n *html.Node) map[string]string {
	result := r.directDecls(n)
	for anc := n.Parent; anc != nil; anc = anc.Parent {
		if anc.Type != html.ElementNode {
			continue
		}
		for prop, val := range r.directDecls(anc) {
			if inheritableProps[prop] {
				if _, exists := result[prop]; !exists {
					result[prop] = val
				}
			}
		}
	}
	return result
}

// directDecls returns CSS declarations for n based only on rules that
// directly match n (no ancestor inheritance). Used by resolveDecls.
func (r *Renderer) directDecls(n *html.Node) map[string]string {
	type match struct {
		spec  int
		decls map[string]string
	}
	var matches []match
	for _, rl := range r.rules {
		parts := parseSelector(rl.selector)
		if matchSelector(n, parts) {
			matches = append(matches, match{specificity(parts), rl.decls})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].spec < matches[j].spec })
	result := make(map[string]string)
	for _, m := range matches {
		for k, v := range m.decls {
			result[k] = v
		}
	}
	// Inline style= attribute wins over all stylesheet rules.
	if s := nodeAttr(n, "style"); s != "" {
		for k, v := range parseInlineDecls(s) {
			result[k] = v
		}
	}
	return result
}

// pseudoElemDecls returns the merged CSS declarations from all rules whose
// selector targets the pseudo-element named by which ("before" or "after")
// on element n. Handles both :before/:after (CSS2) and ::before/::after (CSS3).
func (r *Renderer) pseudoElemDecls(n *html.Node, which string) map[string]string {
	type match struct {
		spec  int
		decls map[string]string
	}
	var matches []match
	for _, rl := range r.rules {
		parts := parseSelector(rl.selector)
		if len(parts) == 0 {
			continue
		}
		last := &parts[len(parts)-1]
		if last.pseudoElem != which {
			continue
		}
		last.pseudoElem = "" // clear so matchPart evaluates the element context normally
		if matchSelector(n, parts) {
			matches = append(matches, match{specificity(parts), rl.decls})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].spec < matches[j].spec })
	result := make(map[string]string)
	for _, m := range matches {
		for k, v := range m.decls {
			result[k] = v
		}
	}
	return result
}

// --- selector types and matching ---

// combinator describes how a selectorPart connects to the part to its left
// (ancestor direction) in a complex selector.
type combinator int

const (
	descendant combinator = iota // space: any ancestor
	child                        // >: immediate parent only
)

// attrOp is the match operator in an attribute selector.
type attrOp int

const (
	opExists attrOp = iota // [attr]     — attribute is present
	opEquals               // [attr=val] — attribute has exact value
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
	pseudos    []string // e.g. "first-child", "nth-child(odd)"
	attrs      []attrSel
	combo      combinator
	pseudoElem string // "before", "after", or "" — set for ::before/::after
}

// parseSelector parses a complex CSS selector string into selectorParts.
// Combinators: space (descendant) and > (child).
// Each compound part may contain: element, #id, .class(es), :pseudo-class(es),
// and [attr] / [attr=val] attribute selectors.
func parseSelector(sel string) []selectorPart {
	var parts []selectorPart
	combo := descendant
	i, n := 0, len(sel)

	for i < n {
		// Skip leading whitespace; a plain space implies descendant combinator.
		for i < n && (sel[i] == ' ' || sel[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		// Explicit child combinator.
		if sel[i] == '>' {
			combo = child
			i++
			for i < n && (sel[i] == ' ' || sel[i] == '\t') {
				i++
			}
			continue
		}
		// Scan one compound-selector token. Stop at whitespace or '>', but
		// skip the contents of [...] so a '>' inside an attribute value is
		// not mistaken for a combinator.
		start := i
		for i < n && sel[i] != ' ' && sel[i] != '\t' && sel[i] != '>' {
			if sel[i] == '[' {
				for i < n && sel[i] != ']' {
					i++
				}
				if i < n {
					i++ // consume ']'
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
		combo = descendant // reset for the next token
	}
	return parts
}

// parseSimpleSelector parses a single compound-selector token such as
// "div#main.foo.bar:first-child[href=val]" into a selectorPart.
func parseSimpleSelector(tok string) selectorPart {
	var p selectorPart
	i, n := 0, len(tok)

	// Optional element name: leading characters before the first #, ., :, or [.
	j := i
	for j < n && tok[j] != '#' && tok[j] != '.' && tok[j] != ':' && tok[j] != '[' {
		j++
	}
	p.element = tok[i:j]
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
					// consume the functional argument, e.g. nth-child(odd)
					for j < n && tok[j] != ')' {
						j++
					}
					if j < n {
						j++ // consume ')'
					}
				} else {
					j++
				}
			}
			if ps := strings.ToLower(tok[i:j]); ps != "" {
				switch ps {
				case "before", "after":
					p.pseudoElem = ps
				default:
					p.pseudos = append(p.pseudos, ps)
				}
			}
			i = j
		case '[':
			i++ // skip '['
			j = i
			for j < n && tok[j] != ']' {
				j++
			}
			if a, ok := parseAttrSel(tok[i:j]); ok {
				p.attrs = append(p.attrs, a)
			}
			if j < n {
				i = j + 1 // skip ']'
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
// Supports [attr] (presence) and [attr=val] (exact match).
// Compound operators (~=, ^=, $=, *=) are silently rejected (return false)
// so the selector never matches, matching browser behaviour for unknown syntax.
func parseAttrSel(s string) (attrSel, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return attrSel{}, false
	}
	// Reject compound operators.
	for _, op := range []string{"~=", "^=", "$=", "*="} {
		if strings.Contains(s, op) {
			return attrSel{}, false
		}
	}
	if eq := strings.IndexByte(s, '='); eq >= 0 {
		key := strings.TrimSpace(s[:eq])
		val := strings.TrimSpace(s[eq+1:])
		// Strip optional surrounding quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key == "" {
			return attrSel{}, false
		}
		return attrSel{key: key, op: opEquals, val: val}, true
	}
	return attrSel{key: s, op: opExists}, true
}

// specificity returns the CSS specificity of a parsed selector as a flat int.
// Encoding: id×100 + (class+pseudo+attr)×10 + element×1.
func specificity(parts []selectorPart) int {
	s := 0
	for _, p := range parts {
		if p.element != "" {
			s++
		}
		if p.id != "" {
			s += 100
		}
		s += len(p.classes) * 10
		s += len(p.pseudos) * 10
		s += len(p.attrs) * 10
	}
	return s
}

// matchPart reports whether node n satisfies all simple-selector conditions in p.
func matchPart(n *html.Node, p selectorPart) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if p.pseudoElem != "" {
		return false // pseudo-element selectors never match real nodes
	}
	if p.element != "" && n.Data != p.element {
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
		if !matchPseudo(n, ps) {
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
// Supported: first-child, last-child, nth-child(odd), nth-child(even).
// Unknown pseudo-classes always return false so the selector never matches.
func matchPseudo(n *html.Node, pseudo string) bool {
	if n.Parent == nil {
		return false
	}
	switch pseudo {
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
	}
	return false
}

// matchAttr reports whether n satisfies a single attribute selector condition.
func matchAttr(n *html.Node, a attrSel) bool {
	switch a.op {
	case opExists:
		// Check presence directly in n.Attr so an attribute with an empty
		// value (e.g. <p hidden>) is correctly treated as present.
		for _, attr := range n.Attr {
			if attr.Key == a.key {
				return true
			}
		}
		return false
	case opEquals:
		return nodeAttr(n, a.key) == a.val
	}
	return false
}

// matchSelector matches n against a parsed selector. The rightmost part must
// match n; each preceding part must match an ancestor according to its
// combinator (descendant = any ancestor, child = immediate parent only).
func matchSelector(n *html.Node, parts []selectorPart) bool {
	if len(parts) == 0 || n.Type != html.ElementNode {
		return false
	}
	if !matchPart(n, parts[len(parts)-1]) {
		return false
	}
	cur := n.Parent
	for i := len(parts) - 2; i >= 0; i-- {
		// parts[i+1].combo describes how parts[i+1] connects to parts[i].
		switch parts[i+1].combo {
		case child:
			// parts[i] must be the immediate parent of where parts[i+1] matched.
			if cur == nil || cur.Type != html.ElementNode || !matchPart(cur, parts[i]) {
				return false
			}
			cur = cur.Parent
		default: // descendant — walk up until a match or root
			found := false
			for ; cur != nil; cur = cur.Parent {
				if cur.Type == html.ElementNode && matchPart(cur, parts[i]) {
					found = true
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
