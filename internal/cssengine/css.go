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
// exactly "inherit", "unset", "initial", or "revert", short-circuiting a
// shorthand's own normal value grammar. That grammar would otherwise
// misparse these keywords, as expandFlexShorthand("inherit") would by
// assigning only flex-basis and dropping flex-grow and flex-shrink, or
// silently drop them entirely, as expandListStyleShorthand and
// expandBackgroundShorthand do by returning an empty map for any
// unrecognized token. ok is false when val isn't a CSS-wide keyword, meaning
// the caller should fall through to its normal parsing.
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
		// splitCSSComponentValues, not strings.Fields: a calc()/min()/
		// max()/clamp() value in one of these 1-4 slots has mandatory
		// internal whitespace around its own "+"/"-" (calc(1 + 2)), which
		// strings.Fields would split into extra, wrong tokens.
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
		return map[string]string{
			prop + "-top":    sides[0],
			prop + "-right":  sides[1],
			prop + "-bottom": sides[2],
			prop + "-left":   sides[3],
		}
	case "border":
		// Type-detected, matching real CSS's own border shorthand grammar:
		// <width>, <style>, and <color> may appear in any order, and each is
		// told apart by what it looks like, not by which slot it landed in.
		// See parseBorderComponents's doc comment for how a real width
		// keyword like "thick" is distinguished from this engine's own
		// border-style preset names.
		return expandBorderShorthandComponents(prop, val, "border-style", func(result map[string]string, colorTok string) {
			maps.Copy(result, expandShorthand("border-color", colorTok))
		})
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
		// CSS border-edge shorthand grammar, type-detected the same way as
		// the "border" shorthand above, just resolved to this one edge's own
		// style and color keys instead of the whole box's.
		trimmed := strings.TrimSpace(val)
		if isCSSQuotedStringToken(trimmed) {
			return map[string]string{prop: val}
		}
		return expandBorderShorthandComponents(prop, val, prop, func(result map[string]string, colorTok string) {
			result[prop+"-color"] = colorTok
		})
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
		// See the margin/padding case above for why this is
		// splitCSSComponentValues rather than strings.Fields.
		tokens := splitCSSComponentValues(val)
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
		//
		// splitCSSComponentValues, not strings.Fields: see the margin/
		// padding case above.
		tokens := splitCSSComponentValues(val)
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
	// inherit, unset, and revert only. "initial" is deliberately left to the
	// switch below, because flex's own grammar already treats the literal
	// token "initial" as a real, spec-correct shorthand value, grow:0
	// shrink:1 basis:auto, which happens to equal each longhand's own real
	// initial value. expandCSSWideKeyword's generic "initial" handling must
	// not shadow that.
	if kw := cssWideKeyword(val); kw == "inherit" || kw == "unset" || kw == "revert" {
		return map[string]string{"flex-grow": kw, "flex-shrink": kw, "flex-basis": kw}
	}
	// splitCSSComponentValues, not strings.Fields: flex-basis can be a
	// calc()/min()/max()/clamp() value (flex: 1 1 calc(50% - 4)), whose
	// mandatory internal whitespace around "+"/"-" would otherwise split
	// into extra, wrong tokens. See the margin/padding case in
	// expandShorthand above.
	tokens := splitCSSComponentValues(val)
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

// isCSSLengthLikeToken reports whether s is a bare CSS number once a
// trailing run of ASCII letters (a unit) and, when allowPercent is set, a
// trailing "%" are trimmed off. Shared by isCSSFlexBasisToken and
// isCSSBorderWidthToken, which both need to separate a real length from a
// keyword or another shorthand component without a full unit vocabulary
// check: this engine's own sizing accepts bare counts and "ch" and ignores
// any other unit, so an unrecognized one is still treated as a length and
// left for the caller to do whatever it does with an ignored unit.
func isCSSLengthLikeToken(s string, allowPercent bool) bool {
	cutset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if allowPercent {
		cutset = "%" + cutset
	}
	num := strings.TrimRight(s, cutset)
	return num != "" && isCSSNumberToken(num)
}

// isCSSFlexBasisToken reports whether s is usable as flex's <'flex-basis'>
// component: the auto and content keywords, the intrinsic sizing keywords, or
// a <length-percentage>, meaning a CSS number with an optional "%" or unit
// suffix. It only has to separate a real basis from a junk token, so that
// `flex: <junk>` stays inert instead of picking up the shorthand's grow and
// shrink defaults. Callers that need the value itself parse it later.
func isCSSFlexBasisToken(s string) bool {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "auto", "content", "min-content", "max-content", "fit-content":
		return true
	}
	if IsMathFunctionToken(s) {
		return true
	}
	return isCSSLengthLikeToken(s, true)
}

// BorderStyleNames is the closed vocabulary border-style, and by extension
// the border and border-<edge> shorthands, accept: htmlterm's own named
// presets, not real CSS's solid/dashed/dotted/double/groove/ridge/inset/
// outset/none/hidden keyword set (see CSS.md's border-style entry for which
// names overlap and which don't).
//
// This is the canonical copy. internal/render owns what each name actually
// draws (internal/render/table.go's borderStylePresets), since glyphs are a
// rendering concern and cssengine sits below render in the package layering,
// unable to import it to share that mapping directly. But render already
// imports cssengine, so borderStylePresets keys off this map instead of
// carrying a second hand-written name list: rather than "kept in sync by
// hand" with a chance to silently drift, TestNamedTableStyleMatchesBorderStyleVocabulary
// (internal/render) asserts the two sets are identical on every test run, so
// adding a name in only one place fails a test immediately instead of
// quietly misclassifying a shorthand value or leaving a preset undrawable.
var BorderStyleNames = map[string]bool{
	"solid": true, "rounded": true, "heavy": true, "double": true,
	"markdown": true, "standard": true, "hidden": true, "none": true,
}

// isCSSBorderWidthToken reports whether s is usable as a border shorthand's
// <line-width> component: one of CSS's three width keywords, or a bare
// <length> (border-width has no percentage form, unlike flex-basis, so
// isCSSLengthLikeToken's allowPercent is off here). border-width is always a
// no-op in this engine (a box-drawing character has no line-thickness notion
// separate from the glyph itself, see CSS.md's border-width entry), so this
// only has to tell a real width apart from a style keyword or a color, not
// interpret the value. s must already be lowercase; callers compare against
// the lowercased token, the same convention expandFlexFlowShorthand uses,
// since expandShorthand runs ahead of the cascade's own keyword folding and
// can't rely on it yet.
func isCSSBorderWidthToken(s string) bool {
	switch s {
	case "thin", "medium", "thick":
		return true
	}
	return isCSSLengthLikeToken(s, false)
}

// parseBorderComponents parses the border or border-<edge> shorthand's value
// into a style token and a color token, type-detecting each of up to three
// space-separated components the way real CSS's own grammar does: a
// component is classified as <line-width>, <line-style>, or <color> by what
// it looks like, not by which slot it landed in. Real CSS's border shorthand
// allows those three in any order, and a real width keyword like "thick" has
// no way to be told apart from a style keyword by content alone once it sits
// in an unexpected slot, which is exactly why position can't be the
// dispatch. Type detection sidesteps that: "thick solid red" resolves
// correctly regardless of which slot "thick" is in, because it is
// recognized as a width keyword on its own merits, and so does the
// previously unsupported two-value "<width> <style>" form with no color,
// such as "2px solid", because a missing third component no longer needs a
// fixed slot to be detected as absent. The returned tokens keep their
// original casing; only the classification decision is made against a
// lowercased copy of each token, the cascade's own keyword folding not
// having run yet at this point in parsing.
//
// A token holding an unresolved var() reference, such as
// "border: var(--bw) solid red", can't be classified by content at all: it
// might expand to any of the three kinds once substituted, and substitution
// happens later, per element, well after this shorthand has already been
// expanded once at parse time (see CSS.md's "Shorthand fan-out" gap). Such a
// token is classified as color if the color slot is still open, matching the
// common "style var(--color)" pattern, and otherwise treated as the (always
// discarded) width, on the working assumption that an author combining a
// var() with both an explicit style and an explicit color most likely meant
// the var() for the width. That assumption can be wrong, but the alternative,
// two same-kind components rejecting the whole declaration, is worse for the
// common case this is guessing about.
//
// ok is false when val doesn't fit the grammar: more than three components,
// or two literal (non-var()) components that classify as the same kind (CSS
// permits each of width, style, and color at most once). A false ok leaves
// the shorthand's own declaration in place, unexpanded, the same "invalid
// value stays inert" handling expandFlexShorthand uses. The returned style
// token is "" when val had no style component at all, such as "border: red"
// setting only a color; expandBorderShorthandComponents is what turns that
// "" into an explicit reset to "none", matching real CSS's own rule that a
// shorthand resets every longhand it covers, not just the ones it writes.
func parseBorderComponents(val string) (styleTok, colorTok string, ok bool) {
	tokens := splitCSSComponentValues(val)
	if len(tokens) == 0 || len(tokens) > 3 {
		return "", "", false
	}
	var haveWidth bool
	var varTok string
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		switch {
		case !isCSSQuotedStringToken(tok) && containsVarCall(tok):
			if varTok != "" {
				return "", "", false
			}
			varTok = tok
		case BorderStyleNames[lower]:
			if styleTok != "" {
				return "", "", false
			}
			styleTok = tok
		case isCSSBorderWidthToken(lower):
			if haveWidth {
				return "", "", false
			}
			haveWidth = true
		default:
			if colorTok != "" {
				return "", "", false
			}
			colorTok = tok
		}
	}
	if varTok != "" && colorTok == "" {
		colorTok = varTok
	}
	// varTok != "" && colorTok != "": the color slot is already taken by a
	// literal token, so the var() is presumed to be the (always discarded)
	// width, and simply dropped.
	return styleTok, colorTok, true
}

// expandBorderShorthandComponents finishes what parseBorderComponents
// starts, shared by the "border" and "border-top"/"border-right"/
// "border-bottom"/"border-left" cases in expandShorthand: styleKey is the
// longhand the style component is written to ("border-style" for the
// whole-box shorthand, prop itself for an edge one), and applyColor writes
// whatever the color component maps to for that caller (border-color's own
// four-side expansion for the whole box, a single prop+"-color" for an
// edge).
//
// styleKey is always present in the returned map when val parses at all,
// even when val has no style component of its own: real CSS's border
// shorthand resets every longhand it covers, style included, not just the
// ones it happens to write, and an absent key here would instead let a
// separately set border-style (or border-top, etc.) survive the cascade
// merge untouched, since that merge is per-key. "none" is the reset value,
// matching real CSS's own border-style initial value and this engine's
// existing "hidden"/"none" no-border presets.
func expandBorderShorthandComponents(prop, val, styleKey string, applyColor func(result map[string]string, colorTok string)) map[string]string {
	styleTok, colorTok, ok := parseBorderComponents(val)
	if !ok {
		return map[string]string{prop: val}
	}
	if styleTok == "" {
		styleTok = "none"
	}
	result := map[string]string{styleKey: styleTok}
	if colorTok != "" {
		applyColor(result, colorTok)
	}
	return result
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

// ConsumeQuotedToken exports consumeCSSQuotedToken for internal/render's
// convertViewportLengths (cascade.go), which needs to skip a quoted CSS
// string span entirely — `content: "3vw wide"` must not have its literal
// "3vw" mistaken for a length and rewritten — rather than scan into it
// looking for a vw/vh length the way it does everywhere else in a value.
// s[i] must be '"' or '\”; the caller checks that before calling.
func ConsumeQuotedToken(s string, i int) int {
	return consumeCSSQuotedToken(s, i)
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
