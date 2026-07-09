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

// ruleMatch pairs one matching rule's specificity with its declarations, for
// sorting into cascade order before merging. Shared by Direct and
// PseudoElement.
type ruleMatch struct {
	spec  specificityScore
	decls map[string]declValue
}

// mergeCascade stable-sorts matches into ascending cascade order (lowest
// specificity, then earliest source position, first) and merges their
// declarations into two tiers: normal and !important. Within each tier,
// later matches in the sorted order win, exactly as a single merged map
// would have behaved before !important existed; the two tiers are kept
// separate so a caller can layer in more declarations (e.g. an inline
// style=) at the same two priority levels before a final flatten collapses
// !important over normal.
func mergeCascade(matches []ruleMatch) (normal, important map[string]string) {
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].spec.less(matches[j].spec) })
	normal = make(map[string]string)
	important = make(map[string]string)
	for _, m := range matches {
		for k, v := range m.decls {
			if v.important {
				important[k] = v.value
			} else {
				normal[k] = v.value
			}
		}
	}
	return normal, important
}

// mergeInlineDecls layers an already-parsed style="" declaration set into
// normal/important, matching each declaration's own importance. Real CSS
// treats style="" as an author rule with specificity higher than any
// selector, so — like any other rule — its normal declarations only need to
// beat other normal declarations, and its important declarations only need
// to beat other important declarations; it does not let a normal inline
// declaration override a stylesheet !important one.
func mergeInlineDecls(normal, important map[string]string, inline map[string]declValue) {
	for k, v := range inline {
		if v.important {
			important[k] = v.value
		} else {
			normal[k] = v.value
		}
	}
}

// flattenImportant collapses normal/important into the single result map
// callers expect, with !important declarations overriding normal ones
// regardless of specificity — the one place this tier ordering is actually
// enforced.
func flattenImportant(normal, important map[string]string) map[string]string {
	for k, v := range important {
		normal[k] = v
	}
	return normal
}

// Direct returns CSS declarations for n based only on rules that directly
// match n (no ancestor inheritance). Used by Resolve.
func (c Cascade) Direct(n *html.Node) map[string]string {
	var matches []ruleMatch
	for _, rl := range c.Rules {
		if matchSelector(n, rl.parts, c.FocusAttr) {
			matches = append(matches, ruleMatch{specificity(rl.parts), rl.decls})
		}
	}
	normal, important := mergeCascade(matches)
	// Inline style= attribute: normal declarations win over all stylesheet
	// normal declarations, and important ones win over all stylesheet
	// important declarations (see mergeInlineDecls).
	if !c.IgnoreInline {
		if s := nodeAttr(n, "style"); s != "" {
			mergeInlineDecls(normal, important, parseDeclarationsWithImportance(s))
		}
	}
	return flattenImportant(normal, important)
}

// PseudoElement returns the merged CSS declarations from all rules whose
// selector targets the pseudo-element named by which ("before" or "after")
// on element n. Handles both :before/:after (CSS2) and ::before/::after (CSS3).
func (c Cascade) PseudoElement(n *html.Node, which string) map[string]string {
	var matches []ruleMatch
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
			matches = append(matches, ruleMatch{specificity(parts), rl.decls})
		}
	}
	normal, important := mergeCascade(matches)
	return flattenImportant(normal, important)
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
