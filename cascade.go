package htmlterm

import (
	"sort"

	"golang.org/x/net/html"
)

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
		last.pseudoElem = ""
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
