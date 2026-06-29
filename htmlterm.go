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
p                       { margin-bottom: 1; }
h1, h2, h3, h4, h5, h6 { font-weight: bold; }
th                      { font-weight: bold; }
strong, b               { font-weight: bold; }
em, i                   { font-style: italic; }
a                       { text-decoration: underline; }
pre                     { white-space: pre; }
ul, ol                  { padding-left: 4; }
td, th                  { white-space: nowrap; text-overflow: ellipsis; }
blockquote              { border-left: │; border-left-color: #555555; padding-left: 1; padding-right: 2; }
s, del                  { text-decoration: line-through; }
u                       { text-decoration: underline; }
kbd                     { font-weight: bold; }
mark                    { background-color: #cc9900; color: #000000; }
samp, var, cite         { font-style: italic; }
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

// --- selector types and matching ---

type selectorPart struct {
	element string // tag name; "" matches any
	class   string // class attribute value; "" matches any
}

// parseSelector splits a selector string into parts for descendant matching.
// Supports: element, .class, element.class, and space-separated descendant chains.
func parseSelector(sel string) []selectorPart {
	tokens := strings.Fields(sel)
	parts := make([]selectorPart, 0, len(tokens))
	for _, tok := range tokens {
		var p selectorPart
		if i := strings.Index(tok, "."); i != -1 {
			p.element = tok[:i]
			p.class = tok[i+1:]
		} else {
			p.element = tok
		}
		parts = append(parts, p)
	}
	return parts
}

func specificity(parts []selectorPart) int {
	s := 0
	for _, p := range parts {
		if p.element != "" {
			s++
		}
		if p.class != "" {
			s += 10
		}
	}
	return s
}

func matchPart(n *html.Node, p selectorPart) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if p.element != "" && n.Data != p.element {
		return false
	}
	if p.class != "" {
		found := false
		for _, c := range strings.Fields(nodeAttr(n, "class")) {
			if c == p.class {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// matchSelector matches n against parts (rightmost part = current node,
// leftward parts must match ancestors in order).
func matchSelector(n *html.Node, parts []selectorPart) bool {
	if len(parts) == 0 || n.Type != html.ElementNode {
		return false
	}
	if !matchPart(n, parts[len(parts)-1]) {
		return false
	}
	remaining := parts[:len(parts)-1]
	for anc := n.Parent; anc != nil && len(remaining) > 0; anc = anc.Parent {
		if matchPart(anc, remaining[len(remaining)-1]) {
			remaining = remaining[:len(remaining)-1]
		}
	}
	return len(remaining) == 0
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
