package cssengine

import "strings"

// CSS keyword values are ASCII case-insensitive: `display: FLEX` and
// `display: flex` are the same declaration, the way `DISPLAY:` and `display:`
// are the same property. Property names have always been folded here (see
// css.go's commitDecl), but values were stored verbatim, so every keyword
// comparison in internal/render — all of which spell their keywords in
// lowercase — silently missed an author who didn't. `display: FLEX` laid out as
// a plain block, `overflow: HIDDEN` clipped nothing, `border-collapse: COLLAPSE`
// rendered as `separate`.
//
// Folding happens at the cascade layer rather than at those ~200 comparison
// sites, which need no changes: no comparison target in this engine is spelled
// with an uppercase letter.

// caseSensitiveValueProps are the properties whose values carry a payload that
// is *not* a keyword and must survive verbatim. Two kinds:
//
//   - author text, emitted as-is — `content` (which also carries counter()/
//     attr()/var() references and open-quote-style idents, all interleaved with
//     strings, so it's excluded wholesale rather than picked apart) and `quotes`;
//   - custom identifiers, which CSS defines as case-sensitive — the three
//     counter properties, whose names really do distinguish `Xy` from `xy`
//     (counter-reset: Xy then counter(xy) legitimately reads a different,
//     unset counter).
//
// `font-family` is here for the same reason as the first group: family names
// are author text. Nothing in this engine reads it today, but folding it would
// be wrong the moment something does.
//
// Custom properties (`--*`) are excluded separately, via isCustomProp: both
// their names and their values are case-sensitive per spec.
//
// Every *other* case-sensitive payload in this engine is a quoted string —
// `border-left: "▌"`, `border-top-left-corner: 'Z'`, `list-style-type: "→ "`,
// `text-overflow: '…'`, `list-style-type: symbols('Ab' 'Cd')` — and
// parseCSSString (render/block.go) requires the quotes, so those need no
// per-property entry: the quote check in foldKeywordValue covers them all,
// while still folding the same properties' unquoted keyword forms
// (`border-left: SOLID`, `text-overflow: ELLIPSIS`, `list-style-type: SQUARE`).
var caseSensitiveValueProps = map[string]bool{
	"content":           true,
	"quotes":            true,
	"font-family":       true,
	"counter-reset":     true,
	"counter-increment": true,
	"counter-set":       true,
}

// foldKeywordValue lowercases a declaration value, unless the value is one this
// engine must preserve exactly. It is a no-op for values with no uppercase
// letters, which is nearly all of them.
//
// A value still containing an unresolved `var(` is left alone: custom property
// names are case-sensitive, so folding `var(--Foo)` would break the reference.
// Such a value is folded later, once substitution has replaced it with real
// text — see foldDeclValues' call sites in cascade.go.
func foldKeywordValue(prop, val string) string {
	if caseSensitiveValueProps[prop] || isCustomProp(prop) {
		return val
	}
	if strings.ContainsAny(val, `"'`) || containsVarCall(val) {
		return val
	}
	return strings.ToLower(val)
}

// foldDeclValues applies foldKeywordValue to every entry of decls in place.
func foldDeclValues(decls map[string]string) {
	for prop, val := range decls {
		if folded := foldKeywordValue(prop, val); folded != val {
			decls[prop] = folded
		}
	}
}

// containsVarCall reports whether val holds a real var() reference, reusing
// isVarCallAt so a property named e.g. "--myvar(x)" or a token merely ending in
// "var" isn't mistaken for one (see customprops.go).
func containsVarCall(val string) bool {
	for i := range val {
		if isVarCallAt(val, i) {
			return true
		}
	}
	return false
}
