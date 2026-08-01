package cssengine

import (
	"io"
	"maps"
	"strconv"
	"strings"

	"github.com/mazznoer/csscolorparser"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// declValue is a single parsed declaration value plus its !important flag.
// Importance is tracked per declaration rather than per rule because a single
// rule can mix important and normal declarations, as in
// `div { color: red !important; background: blue; }`.
type declValue struct {
	value     string
	important bool
}

// Rule is one parsed CSS ruleset for one selector. Comma-separated selector
// groups are expanded into one Rule per selector so cascade matching can work
// against an already-parsed selector without reparsing on every node.
type Rule struct {
	selector string
	decls    map[string]declValue
	// parts is selector, already parsed once at construction time, here,
	// the only place a rule is ever built, rather than being re-parsed by
	// every Match attempt in cascade.go's Direct and PseudoElement paths.
	// Those used to call parseSelector(rl.selector) fresh for every rule
	// against every node checked, an O(nodes × rules) re-parse repeated on
	// every render. Rule values are never mutated after construction, so
	// parts is safe to share read-only across concurrent Render calls and
	// across every Document.Render call for the document's lifetime.
	// PseudoElement, which needs to temporarily clear the last part's
	// pseudoElem to match against a real element, copies parts rather than
	// mutating this shared slice in place.
	parts []selectorPart
}

// ParseStylesheet parses a CSS stylesheet into a list of rules using the raw lexer.
// The high-level css.Parser discards selector tokens before BeginRulesetGrammar,
// so we drive the lexer directly and build a tiny state machine.
func ParseStylesheet(src string) ([]Rule, error) {
	l := css.NewLexer(parse.NewInputString(src))
	var rules []Rule

	const (
		inSelector     = iota
		inDeclarations // inside { }
		inAtRule       // skipping an unsupported @-rule (prelude, and body if any)
	)

	state := inSelector
	var selBuf strings.Builder
	var propBuf strings.Builder
	var valBuf strings.Builder
	var curDecls map[string]declValue
	inValue := false
	atRuleDepth := 0

	commitDecl := func() {
		prop := strings.TrimSpace(propBuf.String())
		if !isCustomProp(prop) {
			prop = strings.ToLower(prop)
		}
		val, important := splitImportant(strings.TrimSpace(valBuf.String()))
		if prop != "" && val != "" && curDecls != nil {
			for k, v := range expandShorthand(prop, val) {
				curDecls[k] = declValue{value: v, important: important}
			}
		}
		propBuf.Reset()
		valBuf.Reset()
		inValue = false
	}

	commitRule := func() {
		commitDecl()
		sel := strings.TrimSpace(selBuf.String())
		if sel != "" && curDecls != nil {
			// splitSelectorList (not a naive strings.Split) so a comma
			// nested inside a functional pseudo-class argument (e.g.
			// "a:is(.x, .y), b { ... }") isn't mistaken for a rule's
			// top-level selector-group separator.
			for _, s := range splitSelectorList(sel) {
				if s = strings.TrimSpace(s); s != "" {
					rules = append(rules, Rule{selector: s, decls: copyDecls(curDecls), parts: parseSelector(s)})
				}
			}
		}
		selBuf.Reset()
		curDecls = nil
		state = inSelector
	}

	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			if err := l.Err(); err != nil && err != io.EOF {
				return nil, err
			}
			return rules, nil
		}

		switch state {
		case inSelector:
			switch tt {
			case css.CommentToken:
				continue
			case css.AtKeywordToken:
				// An @-rule: @media, @import, @font-face, @keyframes, and
				// the rest. None are supported (see CSS.md), so its prelude
				// and body, if any, are skipped as a unit rather than being
				// fed token-by-token into selBuf and curDecls. That used to
				// misinterpret a block @-rule's nested "{" and "}" as this
				// state machine's own selector and declaration boundaries,
				// silently corrupting whatever rule happened to follow it in
				// the same stylesheet. Only recognized when selBuf is
				// otherwise empty, meaning at the true start of a rule. An
				// "@" appearing mid-selector is already invalid CSS and
				// isn't specially handled.
				if strings.TrimSpace(selBuf.String()) == "" {
					selBuf.Reset()
					state = inAtRule
					atRuleDepth = 0
					continue
				}
				selBuf.Write(data)
			case css.LeftBraceToken:
				curDecls = make(map[string]declValue)
				state = inDeclarations
				inValue = false
			default:
				selBuf.Write(data)
			}

		case inAtRule:
			switch tt {
			case css.LeftBraceToken:
				atRuleDepth++
			case css.RightBraceToken:
				if atRuleDepth > 0 {
					atRuleDepth--
				}
				if atRuleDepth == 0 {
					state = inSelector
				}
			case css.SemicolonToken:
				// A statement @-rule, such as "@import url(foo.css);", ends
				// here. A block @-rule's prelude never has a top-level
				// semicolon, and any semicolon inside its body is at
				// atRuleDepth > 0 and is skipped like everything else in
				// the body.
				if atRuleDepth == 0 {
					state = inSelector
				}
			default:
				// Discard the rest of the @-rule's prelude and body tokens
				// unconditionally: identifiers, parens, nested selectors
				// and declarations, strings, and anything else.
			}

		case inDeclarations:
			switch tt {
			case css.CommentToken:
				continue
			case css.RightBraceToken:
				commitRule()
			case css.ColonToken:
				if !inValue {
					inValue = true
				} else {
					valBuf.Write(data)
				}
			case css.SemicolonToken:
				commitDecl()
			case css.WhitespaceToken:
				// Collapse whitespace in values to a single space.
				if inValue && valBuf.Len() > 0 {
					valBuf.WriteByte(' ')
				}
			default:
				if inValue {
					valBuf.Write(data)
				} else {
					propBuf.Write(data)
				}
			}
		}
	}
}

// ParseDeclarations parses a CSS declaration list, the value of a style=""
// attribute, into a property→value map, discarding any !important flags.
// Its only caller outside this package, internal/render/strip.go's
// hidden-inline check, only needs bare values. cascade.go's inline-style
// handling, which needs importance to merge correctly, calls
// parseDeclarationsWithImportance directly instead.
func ParseDeclarations(src string) map[string]string {
	parsed := parseDeclarationsWithImportance(src)
	result := make(map[string]string, len(parsed))
	for k, v := range parsed {
		result[k] = v.value
	}
	return result
}

// parseDeclarationsWithImportance is ParseDeclarations' implementation,
// preserving each declaration's !important flag. No selectors or braces
// expected.
func parseDeclarationsWithImportance(src string) map[string]declValue {
	l := css.NewLexer(parse.NewInputString(src))
	result := make(map[string]declValue)
	var propBuf, valBuf strings.Builder
	inValue := false

	commit := func() {
		prop := strings.TrimSpace(propBuf.String())
		if !isCustomProp(prop) {
			prop = strings.ToLower(prop)
		}
		val, important := splitImportant(strings.TrimSpace(valBuf.String()))
		if prop != "" && val != "" {
			for k, v := range expandShorthand(prop, val) {
				result[k] = declValue{value: v, important: important}
			}
		}
		propBuf.Reset()
		valBuf.Reset()
		inValue = false
	}

	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			break
		}
		switch tt {
		case css.CommentToken:
			continue
		case css.ColonToken:
			if !inValue {
				inValue = true
			} else {
				valBuf.Write(data)
			}
		case css.SemicolonToken:
			commit()
		case css.WhitespaceToken:
			if inValue && valBuf.Len() > 0 {
				valBuf.WriteByte(' ')
			}
		default:
			if inValue {
				valBuf.Write(data)
			} else {
				propBuf.Write(data)
			}
		}
	}
	commit()
	return result
}

// splitImportant splits a trailing case-insensitive "!important" priority
// flag off a parsed CSS declaration value, reporting it separately rather
// than discarding it. The value is otherwise treated exactly as if
// "!important" were never written, so callers that only care about the bare
// value don't need to special-case the suffix themselves: strip.go's
// isHiddenInline recognizing "display:none !important" as hidden, say, or
// parseCSSColor recognizing "red !important" as a color. cascade.go uses
// the reported flag to give !important declarations cascade priority over
// normal ones.
func splitImportant(val string) (value string, important bool) {
	trimmed := strings.TrimRight(val, " \t\n")
	const suffix = "!important"
	if len(trimmed) >= len(suffix) && strings.EqualFold(trimmed[len(trimmed)-len(suffix):], suffix) {
		return strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)]), true
	}
	return val, false
}

func copyDecls(m map[string]declValue) map[string]declValue {
	cp := make(map[string]declValue, len(m))
	maps.Copy(cp, m)
	return cp
}

// shorthandLonghands maps each shorthand property name to the longhand keys
// it expands to, for expandCSSWideKeyword's short-circuiting. A property not
// listed here maps to itself, treated as an ordinary longhand, or as a
// shorthand whose own normal parsing already passes inherit, unset, and
// initial through unchanged.
var shorthandLonghands = map[string][]string{
	"margin":         {"margin-top", "margin-right", "margin-bottom", "margin-left"},
	"padding":        {"padding-top", "padding-right", "padding-bottom", "padding-left"},
	"border":         {"border-style", "border-color", "border-top-color", "border-right-color", "border-bottom-color", "border-left-color"},
	"border-color":   {"border-color", "border-top-color", "border-right-color", "border-bottom-color", "border-left-color"},
	"overflow":       {"overflow-x", "overflow-y"},
	"gap":            {"row-gap", "column-gap"},
	"flex-flow":      {"flex-direction", "flex-wrap"},
	"place-content":  {"align-content", "justify-content"},
	"place-items":    {"align-items", "justify-items"},
	"place-self":     {"align-self", "justify-self"},
	"border-spacing": {"border-spacing-x", "border-spacing-y"},
	// flex is deliberately absent: expandShorthand routes "flex" straight to
	// expandFlexShorthand without consulting this table, since that
	// function needs to keep its own literal "initial" special-casing (see
	// its doc comment) rather than the generic kw-for-every-longhand rule.
	"list-style": {"list-style-type", "list-style-position"},
	"background": {"background-color"},
}

// expandCSSWideKeyword returns the longhand expansion of val when val is
// exactly "inherit", "unset", or "initial", short-circuiting a shorthand's
// own normal value grammar. That grammar would otherwise misparse these
// keywords, as expandFlexShorthand("inherit") would by assigning only
// flex-basis and dropping flex-grow and flex-shrink, or silently drop them
// entirely, as expandListStyleShorthand and expandBackgroundShorthand do by
// returning an empty map for any unrecognized token. ok is false when val
// isn't a CSS-wide keyword, meaning the caller should fall through to its
// normal parsing.
func expandCSSWideKeyword(prop, val string) (result map[string]string, ok bool) {
	kw := cssWideKeyword(val)
	if kw == "" {
		return nil, false
	}
	longhands, known := shorthandLonghands[prop]
	if !known {
		longhands = []string{prop}
	}
	result = make(map[string]string, len(longhands))
	for _, lh := range longhands {
		result[lh] = kw
	}
	return result, true
}

// expandShorthand expands a CSS shorthand property or logical spacing alias
// into its physical longhand equivalents. Returns a map with one or more
// property→value pairs. For other properties, the map contains only the
// original prop→val pair.
//
// Supported shorthands: margin and padding, in 1–4 value syntax, background
// color extraction, and list-style. Supported logical aliases are the block
// and inline start/end forms for margin and padding.
func expandShorthand(prop, val string) map[string]string {
	// flex is excluded here. It is routed to expandFlexShorthand below, which
	// handles inherit, unset, and initial itself: its own grammar already
	// treats the literal token "initial" as a real, spec-correct shorthand
	// value, which expandCSSWideKeyword's generic "initial" handling must not
	// shadow. See expandFlexShorthand's own doc comment.
	if prop != "flex" {
		if kwResult, ok := expandCSSWideKeyword(prop, val); ok {
			return kwResult
		}
	}
	var sides [4]string // top, right, bottom, left
	switch prop {
	case "margin", "padding":
		tokens := strings.Fields(val)
		switch len(tokens) {
		case 1:
			sides = [4]string{tokens[0], tokens[0], tokens[0], tokens[0]}
		case 2:
			sides = [4]string{tokens[0], tokens[1], tokens[0], tokens[1]}
		case 3:
			sides = [4]string{tokens[0], tokens[1], tokens[2], tokens[1]}
		case 4:
			sides = [4]string{tokens[0], tokens[1], tokens[2], tokens[3]}
		default:
			return map[string]string{prop: val}
		}
		return map[string]string{
			prop + "-top":    sides[0],
			prop + "-right":  sides[1],
			prop + "-bottom": sides[2],
			prop + "-left":   sides[3],
		}
	case "border":
		// Positional, not type-detected the way expandBackgroundShorthand is.
		// Real CSS's border shorthand allows <width>, <style>, and <color> in
		// any order, and a real width keyword like "thick" has no way to be
		// distinguished from a style keyword by content alone once it's
		// sitting in an unexpected slot. In "border: thick solid red",
		// "thick" is a width keyword rather than one of our own border-style
		// preset names, but classifying tokens by content would need to know
		// that. Position sidesteps the whole question: slot 0 in a 3-token
		// value is always the ignored width, so "thick solid red" resolves
		// correctly regardless of what "thick" means to either vocabulary.
		// The one gap this leaves is the 2-token "<width> <style>" form with
		// no color, such as "border: 2px solid", which has no positional slot
		// and is silently dropped like any other unrecognized value. That is
		// documented in CSS.md.
		tokens := splitCSSComponentValues(val)
		var styleTok, colorTok string
		switch len(tokens) {
		case 1:
			styleTok = tokens[0]
		case 2:
			styleTok, colorTok = tokens[0], tokens[1]
		case 3:
			styleTok, colorTok = tokens[1], tokens[2]
		default:
			return map[string]string{prop: val}
		}
		result := map[string]string{"border-style": styleTok}
		if colorTok != "" {
			maps.Copy(result, expandShorthand("border-color", colorTok))
		}
		return result
	case "border-color":
		tokens := splitCSSComponentValues(val)
		switch len(tokens) {
		case 1:
			sides = [4]string{tokens[0], tokens[0], tokens[0], tokens[0]}
		case 2:
			sides = [4]string{tokens[0], tokens[1], tokens[0], tokens[1]}
		case 3:
			sides = [4]string{tokens[0], tokens[1], tokens[2], tokens[1]}
		case 4:
			sides = [4]string{tokens[0], tokens[1], tokens[2], tokens[3]}
		default:
			return map[string]string{prop: val}
		}
		// The bare "border-color" key is preserved alongside the four
		// per-edge longhands even though no current internal/render consumer
		// reads it directly, border coloring there being resolved entirely
		// per-edge via border-{top,right,bottom,left}-color. It is kept for
		// parity with how the "border" shorthand case above and this
		// package's own tests already expect a bare fallback key to survive
		// expansion, and as a low-cost hook for any future consumer that
		// wants a single uniform color rather than four per-edge lookups.
		return map[string]string{
			prop:                  val,
			"border-top-color":    sides[0],
			"border-right-color":  sides[1],
			"border-bottom-color": sides[2],
			"border-left-color":   sides[3],
		}
	case "border-top", "border-right", "border-bottom", "border-left":
		// Bareword versus quoted string is the dispatch. A quoted string is
		// this engine's non-standard "literal border glyph" form, as in
		// border-top: "═", and is passed through unchanged:
		// internal/render/block.go still parses it as a literal character,
		// and internal/render/table.go still reads a bare "none" value
		// directly, untouched here, since a single unquoted token is
		// returned as-is below. A bareword value is instead the standard
		// CSS border-edge shorthand grammar — <style>, <style> <color>, or
		// <width> <style> <color> with the width dropped — split out exactly
		// like the "border" shorthand above. block.go resolves the style
		// token to a glyph via the same named preset border-style uses,
		// picked for that specific edge.
		trimmed := strings.TrimSpace(val)
		if isCSSQuotedStringToken(trimmed) {
			return map[string]string{prop: val}
		}
		tokens := splitCSSComponentValues(val)
		var styleTok, colorTok string
		switch len(tokens) {
		case 1:
			styleTok = tokens[0]
		case 2:
			styleTok, colorTok = tokens[0], tokens[1]
		case 3:
			styleTok, colorTok = tokens[1], tokens[2]
		default:
			return map[string]string{prop: val}
		}
		result := map[string]string{prop: styleTok}
		if colorTok != "" {
			result[prop+"-color"] = colorTok
		}
		return result
	case "overflow":
		tokens := strings.Fields(val)
		switch len(tokens) {
		case 1:
			return map[string]string{"overflow-x": tokens[0], "overflow-y": tokens[0]}
		case 2:
			return map[string]string{"overflow-x": tokens[0], "overflow-y": tokens[1]}
		default:
			return map[string]string{prop: val}
		}
	case "list-style":
		return expandListStyleShorthand(val)
	case "background":
		return expandBackgroundShorthand(val)
	case "gap":
		tokens := strings.Fields(val)
		switch len(tokens) {
		case 1:
			return map[string]string{"row-gap": tokens[0], "column-gap": tokens[0]}
		case 2:
			return map[string]string{"row-gap": tokens[0], "column-gap": tokens[1]}
		default:
			return map[string]string{prop: val}
		}
	case "border-spacing":
		// <length> alone applies to both axes. <length> <length> is
		// horizontal then vertical, per the real CSS2 grammar, the reverse
		// order from "gap"'s row-then-column convention above.
		tokens := strings.Fields(val)
		switch len(tokens) {
		case 1:
			return map[string]string{"border-spacing-x": tokens[0], "border-spacing-y": tokens[0]}
		case 2:
			return map[string]string{"border-spacing-x": tokens[0], "border-spacing-y": tokens[1]}
		default:
			return map[string]string{prop: val}
		}
	case "flex":
		return expandFlexShorthand(val)
	case "flex-flow":
		return expandFlexFlowShorthand(val)
	case "place-content":
		return expandPlaceShorthand(prop, val, "align-content", "justify-content")
	case "place-items":
		return expandPlaceShorthand(prop, val, "align-items", "justify-items")
	case "place-self":
		return expandPlaceShorthand(prop, val, "align-self", "justify-self")
	case "margin-block-start":
		return map[string]string{"margin-top": val}
	case "margin-block-end":
		return map[string]string{"margin-bottom": val}
	case "margin-inline-start":
		return map[string]string{"margin-left": val}
	case "margin-inline-end":
		return map[string]string{"margin-right": val}
	case "padding-block-start":
		return map[string]string{"padding-top": val}
	case "padding-block-end":
		return map[string]string{"padding-bottom": val}
	case "padding-inline-start":
		return map[string]string{"padding-left": val}
	case "padding-inline-end":
		return map[string]string{"padding-right": val}
	}
	return map[string]string{prop: val}
}

func expandBackgroundShorthand(val string) map[string]string {
	if kwResult, ok := expandCSSWideKeyword("background", val); ok {
		return kwResult
	}
	for _, tok := range splitCSSComponentValues(val) {
		tok = strings.TrimSpace(tok)
		if tok != "" && parseCSSColor(tok) {
			return map[string]string{"background-color": tok}
		}
	}
	return map[string]string{}
}

// expandFlexShorthand expands the `flex` shorthand into flex-grow,
// flex-shrink, and flex-basis longhands, per the CSS grammar:
//
//   - `none` is 0 0 auto, and `auto` is 1 1 auto.
//   - A single number is that number's flex-grow, with flex-shrink 1 and
//     flex-basis 0, the common `flex: 1` equal-growth pattern.
//   - A single non-numeric token is that token's flex-basis, with flex-grow
//     and flex-shrink both 1, the shorthand's own omitted-value defaults
//     rather than the longhands' initial values.
//   - A number followed by a second number is grow and shrink, with basis 0.
//   - A number followed by a basis is grow and basis, with shrink 1.
//   - The full three-token form is grow, shrink, basis.
//
// See CSS.md's Flexbox section.
//
// Every arity validates its components, and a value whose tokens don't fit the
// grammar is left unexpanded as an inert `flex` declaration rather than
// half-applying. The one-token form always did this. The two- and three-token
// forms used to assign positionally without looking, so `flex: 30% 1` became
// flex-grow: 30%, which parses to 0 and collapses the item, instead of the
// grow: 1, basis: 30% a browser resolves it to, and `flex: 1 1 junk` handed out
// the shorthand's grow and shrink defaults off a basis that would then be
// ignored. An invalid declaration must leave every longhand it covers at its
// initial value, not at part of what was written.
func expandFlexShorthand(val string) map[string]string {
	// inherit and unset only. "initial" is deliberately left to the switch
	// below, because flex's own grammar already treats the literal token
	// "initial" as a real, spec-correct shorthand value, grow:0 shrink:1
	// basis:auto, which happens to equal each longhand's own real initial
	// value. expandCSSWideKeyword's generic "initial" handling must not
	// shadow that.
	if kw := cssWideKeyword(val); kw == "inherit" || kw == "unset" {
		return map[string]string{"flex-grow": kw, "flex-shrink": kw, "flex-basis": kw}
	}
	tokens := strings.Fields(val)
	switch strings.ToLower(val) {
	case "none":
		return map[string]string{"flex-grow": "0", "flex-shrink": "0", "flex-basis": "auto"}
	case "auto":
		return map[string]string{"flex-grow": "1", "flex-shrink": "1", "flex-basis": "auto"}
	case "initial":
		return map[string]string{"flex-grow": "0", "flex-shrink": "1", "flex-basis": "auto"}
	}
	switch len(tokens) {
	case 1:
		if isCSSNumberToken(tokens[0]) {
			return map[string]string{"flex-grow": tokens[0], "flex-shrink": "1", "flex-basis": "0"}
		}
		// One-value basis form. A shorthand always resets every longhand it
		// covers, and flex's own omitted-value defaults are 1 and 1, not the
		// longhands' initial 0 and 1, so `flex: 30%` is `1 1 30%`. Leaving
		// flex-grow at its own initial 0 would make the item refuse to grow,
		// the opposite of what the shorthand asks for. The token has to be a
		// real basis for that, though: granting flex-grow: 1 off a junk value
		// would let an invalid declaration change layout, which is worse than
		// the inert flex-basis it lands as instead.
		if isCSSFlexBasisToken(tokens[0]) {
			return map[string]string{"flex-grow": "1", "flex-shrink": "1", "flex-basis": tokens[0]}
		}
		return map[string]string{"flex-basis": tokens[0]}
	case 2:
		// The grammar is `[ <'flex-grow'> <'flex-shrink'>? || <'flex-basis'> ]`,
		// and `||` means the two components may appear in either order, so
		// `flex: 30% 1` is as valid as `flex: 1 30%`, and both mean grow 1,
		// shrink 1, basis 30%. The grow-first reading is tried first, matching
		// the grammar's own operand order, which is what keeps an all-numeric
		// `flex: 1 2` a grow/shrink pair rather than a grow/basis one. A bare
		// number is a valid basis in this engine, so the two readings really
		// do overlap.
		if isCSSNumberToken(tokens[0]) {
			if isCSSNumberToken(tokens[1]) {
				return map[string]string{"flex-grow": tokens[0], "flex-shrink": tokens[1], "flex-basis": "0"}
			}
			if isCSSFlexBasisToken(tokens[1]) {
				return map[string]string{"flex-grow": tokens[0], "flex-shrink": "1", "flex-basis": tokens[1]}
			}
		} else if isCSSFlexBasisToken(tokens[0]) && isCSSNumberToken(tokens[1]) {
			return map[string]string{"flex-grow": tokens[1], "flex-shrink": "1", "flex-basis": tokens[0]}
		}
		return map[string]string{"flex": val}
	case 3:
		// Same `||` freedom as the two-token form: the basis may lead or
		// trail the grow and shrink pair. Grow-first again wins the
		// all-numeric overlap, so `flex: 1 2 3` stays grow 1, shrink 2,
		// basis 3.
		if isCSSNumberToken(tokens[0]) && isCSSNumberToken(tokens[1]) && isCSSFlexBasisToken(tokens[2]) {
			return map[string]string{"flex-grow": tokens[0], "flex-shrink": tokens[1], "flex-basis": tokens[2]}
		}
		if isCSSFlexBasisToken(tokens[0]) && isCSSNumberToken(tokens[1]) && isCSSNumberToken(tokens[2]) {
			return map[string]string{"flex-grow": tokens[1], "flex-shrink": tokens[2], "flex-basis": tokens[0]}
		}
		return map[string]string{"flex": val}
	default:
		return map[string]string{"flex": val}
	}
}

// expandPlaceShorthand expands one of the CSS Box Alignment `place-*`
// shorthands into its align and justify longhand pair. One value sets both
// axes. Two set the align, block or cross, axis first and the justify, inline
// or main, axis second — note the order, which is the opposite of the axis
// order most other two-value shorthands use.
//
// A `safe` or `unsafe` overflow-alignment prefix makes a single component two
// tokens, which this arity-based split can't tell from a two-component value.
// Since neither keyword does anything in this engine, such a value is left
// unexpanded, as an inert shorthand declaration, rather than misparsed into a
// wrong axis assignment.
func expandPlaceShorthand(prop, val, alignProp, justifyProp string) map[string]string {
	tokens := strings.Fields(strings.ToLower(val))
	for _, tok := range tokens {
		if tok == "safe" || tok == "unsafe" {
			return map[string]string{prop: val}
		}
	}
	switch len(tokens) {
	case 1:
		return map[string]string{alignProp: tokens[0], justifyProp: tokens[0]}
	case 2:
		return map[string]string{alignProp: tokens[0], justifyProp: tokens[1]}
	default:
		return map[string]string{prop: val}
	}
}

// expandFlexFlowShorthand expands the `flex-flow` shorthand into
// flex-direction and flex-wrap. Per the CSS grammar the two components are
// each optional and may appear in either order, so "wrap row" is as valid as
// "row wrap". Whichever is omitted is set to its own initial value, since a
// shorthand always resets every longhand it covers. Anything that isn't a
// keyword of exactly one of the two components, or that names the same
// component twice, is left unexpanded, so it lands in the cascade as an
// inert `flex-flow` declaration rather than half-applying.
func expandFlexFlowShorthand(val string) map[string]string {
	tokens := strings.Fields(strings.ToLower(val))
	if len(tokens) == 0 || len(tokens) > 2 {
		return map[string]string{"flex-flow": val}
	}
	direction, wrap := "", ""
	for _, tok := range tokens {
		switch tok {
		case "row", "row-reverse", "column", "column-reverse":
			if direction != "" {
				return map[string]string{"flex-flow": val}
			}
			direction = tok
		case "nowrap", "wrap", "wrap-reverse":
			if wrap != "" {
				return map[string]string{"flex-flow": val}
			}
			wrap = tok
		default:
			return map[string]string{"flex-flow": val}
		}
	}
	if direction == "" {
		direction = "row"
	}
	if wrap == "" {
		wrap = "nowrap"
	}
	return map[string]string{"flex-direction": direction, "flex-wrap": wrap}
}

func isCSSNumberToken(s string) bool {
	_, ok := ParseNumber(s)
	return ok
}

// isCSSFlexBasisToken reports whether s is usable as flex's <'flex-basis'>
// component: the auto and content keywords, the intrinsic sizing keywords, or
// a <length-percentage>, meaning a CSS number with an optional "%" or unit
// suffix. This is deliberately not a full unit vocabulary check, since this
// engine's own sizing accepts bare counts and "ch" and ignores any other unit.
// It only has to separate a real basis from a junk token, so that
// `flex: <junk>` stays inert instead of picking up the shorthand's grow and
// shrink defaults. Callers that need the value itself parse it later, and an
// unrecognized unit lands as an ignored flex-basis exactly as it would from
// the longhand.
func isCSSFlexBasisToken(s string) bool {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "auto", "content", "min-content", "max-content", "fit-content":
		return true
	}
	num := strings.TrimRight(s, "%abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	return num != "" && isCSSNumberToken(num)
}

// ParseNumber parses a CSS <number> token: an optional sign, digits with an
// optional fractional part, where either side may be omitted but not both,
// and an optional e-notation exponent. Surrounding whitespace is ignored.
//
// This is deliberately stricter than strconv.ParseFloat, which is a Go literal
// parser and also accepts "NaN", "Inf", "Infinity", hex-float forms like
// "0x1p-2", and digit-separating underscores. None of those are CSS numbers,
// and a NaN reaching layout is worse than a dropped declaration: it compares
// false against every bound, so it slips past validity checks and then
// poisons any total it's summed into.
func ParseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	rest := strings.TrimLeft(s, "+-")
	if len(s)-len(rest) > 1 {
		return 0, false
	}
	mantissa, exponent, hasExponent := rest, "", false
	if i := strings.IndexAny(rest, "eE"); i >= 0 {
		mantissa, exponent, hasExponent = rest[:i], rest[i+1:], true
	}
	intPart, fracPart, _ := strings.Cut(mantissa, ".")
	if !isASCIIDigits(intPart) || !isASCIIDigits(fracPart) || intPart+fracPart == "" {
		return 0, false
	}
	if hasExponent {
		digits := strings.TrimLeft(exponent, "+-")
		if len(exponent)-len(digits) > 1 || digits == "" || !isASCIIDigits(digits) {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Syntax is already known-good, so this is out-of-range only: a
		// value like 1e400, whose ParseFloat result is an infinity. Rejecting
		// it keeps the guarantee callers rely on, that a successful parse
		// is always a finite number.
		return 0, false
	}
	return f, true
}

// isASCIIDigits reports whether s is entirely ASCII digits. An empty string
// qualifies: ParseNumber's caller checks separately that at least one of the
// integer and fractional parts is non-empty.
func isASCIIDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func expandListStyleShorthand(val string) map[string]string {
	if kwResult, ok := expandCSSWideKeyword("list-style", val); ok {
		return kwResult
	}
	decls := make(map[string]string)
	for _, tok := range splitCSSComponentValues(val) {
		tok = strings.TrimSpace(tok)
		lower := strings.ToLower(tok)
		switch {
		case lower == "inside" || lower == "outside":
			decls["list-style-position"] = lower
		case strings.HasPrefix(lower, "symbols("):
			// symbols(...) is a supported list-style-type value (see
			// internal/render/symbols.go). It is kept verbatim, case as
			// written, so the quoted strings inside aren't lowercased.
			decls["list-style-type"] = tok
		case isCSSFunctionToken(lower):
			// list-style-image is not supported. Ignore url(...) and other
			// function-valued image tokens in the shorthand.
			continue
		case isCSSQuotedStringToken(tok):
			decls["list-style-type"] = tok
		case isSupportedListStyleType(lower):
			decls["list-style-type"] = lower
		}
	}
	return decls
}

func isCSSQuotedStringToken(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return false
	}
	return (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')
}

func isSupportedListStyleType(v string) bool {
	switch v {
	case "disc", "circle", "square", "none", "decimal",
		"lower-alpha", "lower-latin", "upper-alpha", "upper-latin",
		"lower-roman", "upper-roman":
		return true
	default:
		return false
	}
}

func isCSSFunctionToken(v string) bool {
	i := strings.IndexByte(v, '(')
	return i > 0 && strings.HasSuffix(strings.TrimSpace(v), ")")
}

func splitCSSComponentValues(s string) []string {
	var toks []string
	for i := 0; i < len(s); {
		for i < len(s) && isCSSWhitespace(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		depth := 0
		for i < len(s) {
			c := s[i]
			if c == '"' || c == '\'' {
				i = consumeCSSQuotedToken(s, i)
				continue
			}
			switch c {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			default:
				if depth == 0 && isCSSWhitespace(c) {
					toks = append(toks, s[start:i])
					i++
					goto nextToken
				}
			}
			i++
		}
		toks = append(toks, s[start:i])
	nextToken:
	}
	return toks
}

func consumeCSSQuotedToken(s string, i int) int {
	quote := s[i]
	i++
	for i < len(s) {
		if s[i] == '\\' {
			if i+1 >= len(s) {
				return len(s)
			}
			i += 2
			continue
		}
		i++
		if s[i-1] == quote {
			break
		}
	}
	return i
}

func isCSSWhitespace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

func parseCSSColor(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := csscolorparser.Parse(s)
	return err == nil
}
