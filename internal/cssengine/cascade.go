package cssengine

import (
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Cascade resolves declarations from a parsed rule set against an HTML tree.
type Cascade struct {
	Rules        []Rule
	IgnoreInline bool
	FocusAttr    string
}

// ExtractStyleRules walks doc and parses CSS text from every active <style> element.
func ExtractStyleRules(doc *html.Node) []Rule {
	var rules []Rule
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "template" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "style" {
			if parsed, err := ParseStylesheet(rawContent(n)); err == nil {
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
	"color":               true,
	"font-weight":         true,
	"font-style":          true,
	"text-decoration":     true,
	"text-align":          true,
	"white-space":         true,
	"text-transform":      true,
	"font-variant":        true,
	"overflow-wrap":       true,
	"word-break":          true,
	"text-indent":         true,
	"tab-size":            true,
	"visibility":          true,
	"list-style-position": true,
	"opacity":             true,
	"quotes":              true,
}

// Resolve returns the winning CSS declarations for node n, merging all
// matching rules by ascending specificity, then filling missing inheritable
// properties from the nearest ancestor that directly declares them.
func (c Cascade) Resolve(n *html.Node) map[string]string {
	result := c.Direct(n)
	for anc := n.Parent; anc != nil; anc = anc.Parent {
		if anc.Type != html.ElementNode {
			continue
		}
		for prop, val := range c.Direct(anc) {
			if inheritableProps[prop] {
				if _, exists := result[prop]; !exists {
					result[prop] = val
				}
			}
		}
	}
	return result
}

// Direct returns CSS declarations for n based only on rules that directly
// match n (no ancestor inheritance). Used by Resolve.
func (c Cascade) Direct(n *html.Node) map[string]string {
	type match struct {
		spec  specificityScore
		decls map[string]string
	}
	var matches []match
	for _, rl := range c.Rules {
		if matchSelector(n, rl.parts, c.FocusAttr) {
			matches = append(matches, match{specificity(rl.parts), rl.decls})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].spec.less(matches[j].spec) })
	result := make(map[string]string)
	for _, m := range matches {
		for k, v := range m.decls {
			result[k] = v
		}
	}
	// Inline style= attribute wins over all stylesheet rules.
	if !c.IgnoreInline {
		if s := nodeAttr(n, "style"); s != "" {
			for k, v := range ParseDeclarations(s) {
				result[k] = v
			}
		}
	}
	return result
}

// PseudoElement returns the merged CSS declarations from all rules whose
// selector targets the pseudo-element named by which ("before" or "after")
// on element n. Handles both :before/:after (CSS2) and ::before/::after (CSS3).
func (c Cascade) PseudoElement(n *html.Node, which string) map[string]string {
	type match struct {
		spec  specificityScore
		decls map[string]string
	}
	var matches []match
	for _, rl := range c.Rules {
		if len(rl.parts) == 0 || rl.parts[len(rl.parts)-1].pseudoElem != which {
			continue
		}
		// matchSelector needs a real element in every part's pseudoElem
		// slot (matchPart rejects any part with one set — pseudo-elements
		// aren't real DOM nodes), so the trailing ::before/::after marker
		// must be cleared before matching. rl.parts is the shared, cached
		// parse of this rule's selector (reused across every node checked
		// and every render — see rule's doc comment), so this copies rather
		// than mutating it in place the way the pre-caching code used to.
		parts := append([]selectorPart(nil), rl.parts...)
		parts[len(parts)-1].pseudoElem = ""
		if matchSelector(n, parts, c.FocusAttr) {
			matches = append(matches, match{specificity(parts), rl.decls})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].spec.less(matches[j].spec) })
	result := make(map[string]string)
	for _, m := range matches {
		for k, v := range m.decls {
			result[k] = v
		}
	}
	return result
}

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
