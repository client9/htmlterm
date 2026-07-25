package cssengine

import (
	"maps"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Cascade resolves declarations from a parsed rule set against an HTML tree.
type Cascade struct {
	Rules        []Rule
	IgnoreInline bool
	FocusAttr    string
	// HoverAttr is a synthetic marker attribute, matched the same way
	// FocusAttr drives :focus: a node carrying HoverAttr matches :hover.
	// Real terminal output has no pointer to hover with, so this is
	// repurposed by the render engine to mean "the arrow-key-highlighted
	// <option> in an open <select> popup" (see render.Engine's
	// selectHighlightAttr) rather than true mouse hover.
	HoverAttr string

	// Cache, if non-nil, memoizes Direct's per-node result across every
	// Resolve/Direct call sharing this map. Resolve's ancestor walk calls
	// Direct(anc) once for every descendant of anc, so without a cache
	// shared across an entire render pass, resolving N nodes at average
	// depth D against R rules costs O(N*D*R) — every descendant re-running
	// the full R-rule match against the same ancestor from scratch — instead
	// of the O(N*R) achievable by matching each node against the ruleset
	// once and reusing that result for all of its descendants. Callers that
	// only resolve a handful of nodes (or the package's own tests) can leave
	// this nil; Direct/Resolve behave identically either way, just without
	// the reuse. A zero-value Cascade{Rules: ...} is safe: Direct and
	// Resolve both nil-check before touching it.
	Cache map[*html.Node]map[string]string
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

// cssWideKeyword returns the normalized keyword ("inherit", "unset", or
// "initial") if v is exactly one of them (case-insensitive), else "". These
// are CSS's own cascade-reset keywords, valid on any property:
//   - inherit: always take the parent element's own resolved value.
//   - unset: inherit if the property is normally inheritable, otherwise
//     initial.
//   - initial: revert to the property's own specified default - which, for
//     nearly every property in this engine, already just means "absent"
//     (every reader already treats a missing key as its own default).
//
// "revert" is not implemented: it requires distinguishing UA-stylesheet
// origin from author origin, which this cascade doesn't model.
func cssWideKeyword(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "inherit", "unset", "initial":
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}

// Resolve returns the winning CSS declarations for node n, merging all
// matching rules by ascending specificity, resolving any inherit/unset/
// initial keyword among n's own direct declarations, then filling missing
// inheritable properties from the nearest ancestor element's own resolved
// value.
func (c Cascade) Resolve(n *html.Node) map[string]string {
	// Copy rather than reuse c.Direct(n) directly: when c.Cache is set, that
	// map is the shared, cached result for n and must not be mutated below,
	// or a later Direct(n) call (e.g. from this same node's own use as an
	// ancestor of something else, or a caller's own directDecls-style
	// lookup) would see leaked inherited values as if they were n's own
	// direct declarations.
	direct := c.Direct(n)
	result := make(map[string]string, len(direct))
	maps.Copy(result, direct)

	// Nearest ancestor element's own fully-resolved declarations - computed
	// lazily (only if something below actually needs it) and once, shared
	// by both the keyword substitution and the inheritance fill-in. Fully
	// resolved (not just Direct) so any inherit/unset chain the ancestor
	// itself has is already reflected before n ever looks at it.
	var parentResolved map[string]string
	parentComputed := false
	getParentResolved := func() map[string]string {
		if !parentComputed {
			parentComputed = true
			for anc := n.Parent; anc != nil; anc = anc.Parent {
				if anc.Type == html.ElementNode {
					parentResolved = c.Resolve(anc)
					break
				}
			}
		}
		return parentResolved
	}

	// Resolve n's own direct declarations first, so a literal "inherit"/
	// "unset"/"initial" string never leaks out to a caller. forcedAbsent
	// tracks properties explicitly resolved to "stay absent" here (initial,
	// unset on a non-inheritable property, or inherit/unset with no
	// ancestor value to take) - the fill-in pass below must not treat these
	// as "never declared" and re-inherit them anyway.
	forcedAbsent := make(map[string]bool)
	for prop, val := range direct {
		kw := cssWideKeyword(val)
		if kw == "" {
			continue
		}
		if kw == "inherit" || (kw == "unset" && inheritableProps[prop]) {
			if pv, ok := getParentResolved()[prop]; ok {
				result[prop] = pv
				continue
			}
		}
		// unset on a non-inheritable property, initial, or inherit/unset
		// with no ancestor value to take: falls back to absent, and stays
		// absent - not eligible for the implicit inheritance fill-in below.
		delete(result, prop)
		forcedAbsent[prop] = true
	}

	// Fill in any inheritable property n doesn't declare at all, from the
	// nearest ancestor element's own resolved value - a single recursive
	// lookup, since that value already reflects everything further up the
	// tree (equivalent to the real-CSS rule that a parent's own computed
	// value is all a child ever needs to inherit from).
	if pr := getParentResolved(); pr != nil {
		for prop := range inheritableProps {
			if _, exists := result[prop]; !exists && !forcedAbsent[prop] {
				if v, ok := pr[prop]; ok {
					result[prop] = v
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
	if c.Cache != nil {
		if cached, ok := c.Cache[n]; ok {
			return cached
		}
	}
	var matches []ruleMatch
	for _, rl := range c.Rules {
		if matchSelector(n, rl.parts, c.FocusAttr, c.HoverAttr) {
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
	result := flattenImportant(normal, important)
	if c.Cache != nil {
		c.Cache[n] = result
	}
	return result
}

// PseudoElement returns the merged CSS declarations from all rules whose
// selector targets the pseudo-element named by which ("before", "after",
// "marker", "scrollbar", "scrollbar-track", "scrollbar-thumb",
// "scrollbar-cap-start", or "scrollbar-cap-end") on element n. Handles both
// :before/:after (CSS2) and ::before/::after (CSS3) syntax for all of them,
// not just before/after.
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
		if matchSelector(n, parts, c.FocusAttr, c.HoverAttr) {
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
