package cssengine

import (
	"maps"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Cascade resolves declarations from a parsed rule set against an HTML tree.
type Cascade struct {
	Rules []Rule
	// UARules is the subset of Rules that came from the user-agent
	// stylesheet specifically, rather than from Options.CSS, an
	// Options.Stylesheets entry, a document <style> element, or an inline
	// style= attribute. It exists only to give the "revert" cascade-wide
	// keyword something to revert to: real CSS defines revert as rolling a
	// property back to the value the cascade origin *before* the one that
	// set it would have produced, and this engine has only two origins, UA
	// and author, with no separate user-origin stylesheet of its own. A nil
	// or empty UARules makes every revert behave as unset, the same
	// fallback real CSS uses when the UA stylesheet has no matching rule
	// either.
	UARules      []Rule
	IgnoreInline bool
	FocusAttr    string
	// HoverAttr is a synthetic marker attribute, matched the same way
	// FocusAttr drives :focus: a node carrying HoverAttr matches :hover.
	// Real terminal output has no pointer to hover with, so this is
	// repurposed by the render engine to mean "the arrow-key-highlighted
	// <option> in an open <select> popup" (see render.Engine's
	// selectHighlightAttr) rather than true mouse hover.
	HoverAttr string
	// ModalAttr is a synthetic marker attribute, matched the same way
	// FocusAttr drives :focus: a node carrying ModalAttr matches :modal.
	// The render engine sets it on a <dialog> opened as a modal (see
	// render.Engine's dialogModalAttr), which is what lets the UA
	// stylesheet's own `dialog:modal` rule give such a dialog the
	// position/z-index/centering that puts it in the top layer.
	ModalAttr string

	// Cache, if non-nil, memoizes Direct's per-node result across every
	// Resolve and Direct call sharing this map. Resolve's ancestor walk calls
	// Direct(anc) once for every descendant of anc, so without a cache
	// shared across an entire render pass, resolving N nodes at average
	// depth D against R rules costs O(N*D*R), with every descendant re-running
	// the full R-rule match against the same ancestor from scratch, instead
	// of the O(N*R) achievable by matching each node against the ruleset
	// once and reusing that result for all of its descendants. Callers that
	// only resolve a handful of nodes, and the package's own tests, can leave
	// this nil. Direct and Resolve behave identically either way, just without
	// the reuse. A zero-value Cascade{Rules: ...} is safe, since Direct and
	// Resolve both nil-check before touching it.
	Cache map[*html.Node]map[string]string
}

// markers bundles this Cascade's three synthetic marker attributes into the
// form every selector-matching call takes. See Markers (selector.go).
func (c Cascade) markers() Markers {
	return Markers{Focus: c.FocusAttr, Hover: c.HoverAttr, Modal: c.ModalAttr}
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

// cssWideKeyword returns the normalized keyword "inherit", "unset",
// "initial", or "revert" if v is exactly one of them, case-insensitively,
// and otherwise "". These are CSS's own cascade-reset keywords, valid on any
// property:
//   - inherit: always take the parent element's own resolved value.
//   - unset: inherit if the property is normally inheritable, otherwise
//     initial.
//   - initial: revert to the property's own specified default. For nearly
//     every property in this engine that already means "absent", since every
//     reader treats a missing key as its own default.
//   - revert: take the value the user-agent stylesheet establishes for this
//     element, ignoring every author declaration regardless of specificity.
//     Falls back to unset's own behavior wherever the UA stylesheet doesn't
//     set the property either. See Cascade.UARules's doc comment for why
//     this always lands on UA origin specifically, with no separate
//     user-origin stylesheet to prefer first.
func cssWideKeyword(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "inherit", "unset", "initial", "revert":
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}

// Resolve returns the winning CSS declarations for node n, merging all
// matching rules by ascending specificity, resolving any inherit, unset,
// initial, or revert keyword among n's own direct declarations, then filling
// missing inheritable properties from the nearest ancestor element's own
// resolved value.
func (c Cascade) Resolve(n *html.Node) map[string]string {
	// Copy rather than reuse c.Direct(n) directly. When c.Cache is set, that
	// map is the shared, cached result for n and must not be mutated below,
	// or a later Direct(n) call would see leaked inherited values as if they
	// were n's own direct declarations. Such a call could come from this same
	// node's use as an ancestor of something else, or from a caller's own
	// directDecls-style lookup.
	direct := c.Direct(n)
	result := make(map[string]string, len(direct))
	maps.Copy(result, direct)

	// Nearest ancestor element's own fully-resolved declarations, computed
	// lazily, only if something below needs it, and computed once, shared
	// by both the keyword substitution and the inheritance fill-in. Fully
	// resolved rather than just Direct, so any inherit or unset chain the
	// ancestor itself has is already reflected before n ever looks at it.
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

	// n's own declarations from the user-agent stylesheet alone, computed
	// lazily and once, shared across every property that resolves to
	// "revert". uaDirect has no ancestor context of its own, which matches
	// real CSS: revert only rolls back the winning declaration's origin on
	// this element, it doesn't re-derive the whole ancestor chain under a
	// UA-only cascade, so a revert that misses falls through to the same
	// inherited value (possibly author-styled on the parent) that unset
	// already uses below.
	var uaVals map[string]string
	uaComputed := false
	getUAVals := func() map[string]string {
		if !uaComputed {
			uaComputed = true
			uaVals = c.uaDirect(n)
		}
		return uaVals
	}

	// Resolve n's own direct declarations first, so a literal "inherit",
	// "unset", "initial", or "revert" string never leaks out to a caller.
	// forcedAbsent tracks properties explicitly resolved to "stay absent"
	// here: initial, unset on a non-inheritable property, or inherit or
	// unset with no ancestor value to take. The fill-in pass below must not
	// treat these as "never declared" and re-inherit them anyway.
	forcedAbsent := make(map[string]bool)
	for prop, val := range direct {
		kw := cssWideKeyword(val)
		if kw == "" {
			continue
		}
		if kw == "revert" {
			// revert behaves identically for a custom property and an
			// ordinary one: if the UA stylesheet has a matching declaration,
			// by name, that wins, custom property or not. In practice this
			// engine's own UA stylesheet never declares a "--*" name, so a
			// custom property's revert always falls through to the unset
			// case below, but that's a fact about DefaultStylesheet's
			// current contents, not a rule revert itself carves out for
			// custom properties, so this doesn't special-case isCustomProp.
			if uv, ok := getUAVals()[prop]; ok {
				result[prop] = uv
				continue
			}
			// No UA-origin declaration for this property on this element:
			// revert behaves exactly like unset (CSS Cascade §7.4).
			kw = "unset"
		}
		if kw == "inherit" || (kw == "unset" && (inheritableProps[prop] || isCustomProp(prop))) {
			if pv, ok := getParentResolved()[prop]; ok {
				result[prop] = pv
				continue
			}
		}
		// unset on a non-inheritable property, initial, or inherit or unset
		// with no ancestor value to take. Each falls back to absent and stays
		// absent, not eligible for the implicit inheritance fill-in below.
		delete(result, prop)
		forcedAbsent[prop] = true
	}

	// Fill in any inheritable property n doesn't declare at all, from the
	// nearest ancestor element's own resolved value. That is a single
	// recursive lookup, since the value already reflects everything further
	// up the tree, equivalent to the real-CSS rule that a parent's own
	// computed value is all a child ever needs to inherit from.
	if pr := getParentResolved(); pr != nil {
		for prop := range inheritableProps {
			if _, exists := result[prop]; !exists && !forcedAbsent[prop] {
				if v, ok := pr[prop]; ok {
					result[prop] = v
				}
			}
		}
		// Custom properties inherit unconditionally, by name, unlike the
		// fixed inheritableProps whitelist above, so this can't just widen
		// that loop's condition: prop there is only ever drawn from
		// inheritableProps's own keys, never a "--*" name. This second loop
		// walks the parent's own resolved keys instead, which is where any
		// custom-property names live. pr is already computed above, so this
		// reuses it rather than triggering a new ancestor walk.
		for prop, v := range pr {
			if !isCustomProp(prop) {
				continue
			}
			if _, exists := result[prop]; !exists && !forcedAbsent[prop] {
				result[prop] = v
			}
		}
	}

	// Final var() substitution. By this point result holds n's own and
	// inherited custom properties, via the loop above and via Direct(n)'s
	// own same-element substitution already folded into `direct`, alongside
	// every other cascaded property, any of which may still contain literal
	// var(...) text referencing a custom property that's only defined on an
	// ancestor. resolveCustomProps flattens result's own "--*" subset into
	// fully-resolved values first, since custom properties may reference other
	// custom properties, and then every property's value, custom or not, is
	// substituted against that flattened environment.
	//
	// This is the "final" pass, substituteVarTokens rather than
	// substituteKnownVarTokens. By now "unresolved" really does mean
	// unresolved anywhere in the tree, so an unresolvable reference collapsing
	// to its fallback, or to "", is correct here, unlike inside Direct().
	resolvedVars := resolveCustomProps(result)
	for prop, val := range result {
		sub := substituteVarTokens(val, func(name string) (string, bool) {
			v, ok := resolvedVars[name]
			return v, ok
		})
		// Only a value this pass rewrote can still need keyword folding.
		// Everything else in result came either from Direct(n) or from the
		// parent's own Resolve, both of which folded already. Testing for
		// that keeps this off the hot path: Resolve isn't memoized the way
		// Direct is (see PseudoElement's doc comment), so it runs far more
		// often than once per element, and var() is rare in any real
		// stylesheet.
		if sub != val {
			sub = foldKeywordValue(prop, sub)
		}
		result[prop] = sub
	}
	return result
}

// InheritedDecls returns the subset of an already-resolved declaration map
// that a child would inherit from it: the inheritable properties plus every
// custom property. It is the same filter Resolve's own fill-in pass applies,
// exposed for a caller that has no element to resolve.
//
// The caller with no element is an *anonymous box*: a box CSS generates around
// content that has no element of its own, such as the anonymous flex item
// wrapping a run of loose text inside a flex container. Such a box has no
// declarations of its own and cannot be selected, so its entire computed style
// is what it inherits from its parent, which is exactly this map. Resolving a
// synthetic element through the cascade instead would let a `div > *` rule
// style a box the author has no way to name.
func InheritedDecls(parent map[string]string) map[string]string {
	out := make(map[string]string, len(parent))
	for prop, v := range parent {
		if inheritableProps[prop] || isCustomProp(prop) {
			out[prop] = v
		}
	}
	return out
}

// ruleMatch pairs one matching rule's specificity with its declarations, for
// sorting into cascade order before merging. Shared by Direct and
// PseudoElement.
type ruleMatch struct {
	spec  specificityScore
	decls map[string]declValue
}

// mergeCascade stable-sorts matches into ascending cascade order, lowest
// specificity first and then earliest source position, and merges their
// declarations into two tiers: normal and !important. Within each tier,
// later matches in the sorted order win, exactly as a single merged map
// would have behaved before !important existed. The two tiers are kept
// separate so a caller can layer in more declarations, such as an inline
// style=, at the same two priority levels before a final flatten collapses
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
// normal and important, matching each declaration's own importance. Real CSS
// treats style="" as an author rule with specificity higher than any
// selector, so, like any other rule, its normal declarations only need to
// beat other normal declarations, and its important declarations only need
// to beat other important declarations. It does not let a normal inline
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

// flattenImportant collapses normal and important into the single result map
// callers expect, with !important declarations overriding normal ones
// regardless of specificity. It is the one place this tier ordering is
// enforced.
func flattenImportant(normal, important map[string]string) map[string]string {
	for k, v := range important {
		normal[k] = v
	}
	return normal
}

// matchRules matches every rule in rules against n and merges the matches
// into cascade order, exactly as Direct's own first step does. It's the part
// of Direct shared with uaDirect, which runs the same match-and-merge
// against Rules's UA-only subset instead of the full rule set.
func (c Cascade) matchRules(rules []Rule, n *html.Node) (normal, important map[string]string) {
	var matches []ruleMatch
	for _, rl := range rules {
		if matchSelector(n, rl.parts, c.markers()) {
			matches = append(matches, ruleMatch{specificity(rl.parts), rl.decls})
		}
	}
	return mergeCascade(matches)
}

// uaDirect returns n's declarations from c.UARules alone, the fallback value
// "revert" reads (see Cascade.UARules's doc comment). Unlike Direct, this
// has no inline style= to layer in, since style= is always author origin,
// and no var() substitution pass, since the UA stylesheet declares no custom
// properties for one to resolve against. It also isn't memoized in c.Cache:
// revert is rare in real stylesheets, so paying the match cost only when
// Resolve actually hits one is cheaper than growing the per-node cache for
// every render, including the overwhelming majority that never use revert
// at all.
func (c Cascade) uaDirect(n *html.Node) map[string]string {
	normal, important := c.matchRules(c.UARules, n)
	result := flattenImportant(normal, important)
	foldDeclValues(result)
	return result
}

// Direct returns CSS declarations for n based only on rules that directly
// match n, with no ancestor inheritance. Used by Resolve.
func (c Cascade) Direct(n *html.Node) map[string]string {
	if c.Cache != nil {
		if cached, ok := c.Cache[n]; ok {
			return cached
		}
	}
	normal, important := c.matchRules(c.Rules, n)
	// Inline style= attribute: normal declarations win over all stylesheet
	// normal declarations, and important ones win over all stylesheet
	// important declarations (see mergeInlineDecls).
	if !c.IgnoreInline {
		if s := nodeAttr(n, "style"); s != "" {
			mergeInlineDecls(normal, important, parseDeclarationsWithImportance(s))
		}
	}
	result := flattenImportant(normal, important)

	// Same-element-only var() resolution, the Option A scope cut described in
	// docs/proposals/VARIABLES.md. Direct() has no ancestor context, so this
	// only resolves references to n's own custom-property declarations via
	// substituteKnownVarTokens, which leaves anything else untouched rather
	// than collapsing it to "" or a fallback. See that function's doc comment
	// for why that distinction matters here.
	//
	// A custom property whose own raw value is a bare CSS-wide keyword, as in
	// "--x: unset", "inherit", or "initial", must be excluded from `own`. That
	// keyword is only ever resolved by Resolve()'s own keyword-handling loop,
	// just after `direct := c.Direct(n)`, which needs ancestor context
	// Direct() doesn't have. Treating the literal text "unset" as if it
	// were --x's real value here would let it leak into any other
	// declaration that references --x via var(), so that "color: var(--x)"
	// would resolve to the literal string "unset", before Resolve() ever
	// gets a chance to resolve --x itself.
	own := customPropSubset(result)
	for prop, val := range own {
		if cssWideKeyword(val) != "" {
			delete(own, prop)
		}
	}
	if len(own) > 0 {
		resolvedOwn := resolveCustomProps(own)
		for prop, val := range result {
			result[prop] = substituteKnownVarTokens(val, resolvedOwn)
		}
	}

	// Keyword values are ASCII case-insensitive (see keywordcase.go). Folding
	// here rather than at parse time is what lets the same-element var()
	// substitution just above feed it: `display: var(--d)` with `--d: FLEX`
	// arrives as literal "FLEX" and folds like any other keyword. A value
	// still holding a var() reference, one pointing at an *ancestor's* custom
	// property, which Direct has no context to resolve, is deliberately left
	// alone by foldKeywordValue and folded by Resolve instead, once its final
	// substitution pass has turned it into real text.
	//
	// Direct's own result is folded, not just Resolve's copy of it, because
	// callers read it straight: render's directDecls serves counter.go and
	// table_render.go's col and colgroup styling, neither of which goes
	// through Resolve. Folding before the cache write also means this runs
	// once per element per render, not once per lookup.
	foldDeclValues(result)

	if c.Cache != nil {
		c.Cache[n] = result
	}
	return result
}

// PseudoElement returns the merged CSS declarations from all rules whose
// selector targets the pseudo-element named by which on element n. Recognized
// names are "before", "after", "marker", "scrollbar", "scrollbar-track",
// "scrollbar-thumb", "scrollbar-cap-start", and "scrollbar-cap-end". It
// handles both the CSS2 :before and :after syntax and the CSS3 ::before and
// ::after syntax for all of them, not just before and after.
//
// env is the custom-property environment to resolve any var() found in
// these declarations against, typically customPropSubset over the caller's
// own already-resolved declarations for n. PseudoElement deliberately does
// not call c.Resolve(n) itself to build this. Every real caller already
// resolves n's own declarations for its own purposes right next to its
// PseudoElement call (see internal/render/cascade.go's pseudoElemDecls and
// its call sites), and Resolve is not memoized the way Direct is, so an
// internal Resolve(n) here would silently double that cost for every element
// with a pseudo-element. env's values are assumed already fully resolved,
// with no leftover var() text, which holds for anything produced by Resolve,
// so this is a single substitution pass with no fixed-point or cycle logic of
// its own.
func (c Cascade) PseudoElement(n *html.Node, which string, env map[string]string) map[string]string {
	var matches []ruleMatch
	for _, rl := range c.Rules {
		if len(rl.parts) == 0 || rl.parts[len(rl.parts)-1].pseudoElem != which {
			continue
		}
		// matchSelector needs a real element in every part's pseudoElem
		// slot, since matchPart rejects any part with one set because
		// pseudo-elements aren't real DOM nodes. So the trailing ::before or
		// ::after marker must be cleared before matching. rl.parts is the
		// shared, cached parse of this rule's selector, reused across every
		// node checked and every render (see rule's doc comment), so this
		// copies rather than mutating it in place the way the pre-caching
		// code used to.
		parts := append([]selectorPart(nil), rl.parts...)
		parts[len(parts)-1].pseudoElem = ""
		if matchSelector(n, parts, c.markers()) {
			matches = append(matches, ruleMatch{specificity(parts), rl.decls})
		}
	}
	normal, important := mergeCascade(matches)
	decls := flattenImportant(normal, important)
	// env may be nil, for a caller with nothing resolved yet or one that
	// doesn't otherwise need it. Reading a nil map is safe and means every
	// lookup misses, so a fallback, or "", is used, exactly as if no custom
	// properties were in scope at all.
	for prop, val := range decls {
		decls[prop] = substituteVarTokens(val, func(name string) (string, bool) {
			v, ok := env[name]
			return v, ok
		})
	}
	// This is the pseudo-element's own final substitution pass, and nothing
	// downstream folds these the way Resolve does for an element's own
	// declarations, so fold the whole map here rather than only the entries
	// var() touched.
	foldDeclValues(decls)
	return decls
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
