package cssengine

import (
	"io"
	"strings"

	"github.com/mazznoer/csscolorparser"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// Rule is one parsed CSS ruleset for one selector. Comma-separated selector
// groups are expanded into one Rule per selector so cascade matching can work
// against an already-parsed selector without reparsing on every node.
type Rule struct {
	selector string
	decls    map[string]string
	// parts is selector, already parsed once at construction time (here,
	// the only place a rule is ever built) rather than being re-parsed by
	// every Match attempt in cascade.go's Direct/PseudoElement paths. Those
	// used to call parseSelector(rl.selector) fresh
	// for every rule against every node checked, an O(nodes × rules)
	// re-parse repeated on every single render. rule values are never
	// mutated after construction, so parts is safe to share/reuse read-only
	// across concurrent Render calls and across every Document.Render call
	// for the document's lifetime. PseudoElement, which needs to temporarily
	// clear the last part's
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
	)

	state := inSelector
	var selBuf strings.Builder
	var propBuf strings.Builder
	var valBuf strings.Builder
	var curDecls map[string]string
	inValue := false

	commitDecl := func() {
		prop := strings.ToLower(strings.TrimSpace(propBuf.String()))
		val := stripImportant(strings.TrimSpace(valBuf.String()))
		if prop != "" && val != "" && curDecls != nil {
			for k, v := range expandShorthand(prop, val) {
				curDecls[k] = v
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
			for _, s := range strings.Split(sel, ",") {
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
			case css.LeftBraceToken:
				curDecls = make(map[string]string)
				state = inDeclarations
				inValue = false
			default:
				selBuf.Write(data)
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

// ParseDeclarations parses a CSS declaration list (the value of a style=""
// attribute) into a property→value map. No selectors or braces expected.
func ParseDeclarations(src string) map[string]string {
	l := css.NewLexer(parse.NewInputString(src))
	result := make(map[string]string)
	var propBuf, valBuf strings.Builder
	inValue := false

	commit := func() {
		prop := strings.ToLower(strings.TrimSpace(propBuf.String()))
		val := stripImportant(strings.TrimSpace(valBuf.String()))
		if prop != "" && val != "" {
			for k, v := range expandShorthand(prop, val) {
				result[k] = v
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

// stripImportant removes a trailing "!important" priority flag (case-
// insensitive) from a parsed CSS declaration value. htmlterm's cascade has
// no !important-priority concept (cascade.go resolves purely by source
// order/specificity), so the value is treated exactly as if "!important"
// were never written, rather than left attached to the value string — which
// silently broke every exact-string check against that value anywhere in
// the package (e.g. strip.go's isHiddenInline never recognizing
// "display:none !important" as hidden — the motivating "hidden preheader"
// case its own doc comment names — and parseCSSColor failing to recognize
// "red !important" as a color at all).
func stripImportant(val string) string {
	trimmed := strings.TrimRight(val, " \t\n")
	const suffix = "!important"
	if len(trimmed) >= len(suffix) && strings.EqualFold(trimmed[len(trimmed)-len(suffix):], suffix) {
		return strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)])
	}
	return val
}

func copyDecls(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// expandShorthand expands a CSS shorthand property or logical spacing alias
// into its physical longhand equivalents. Returns a map with one or more
// property→value pairs. For other properties, the map contains only the
// original prop→val pair.
//
// Supported shorthands: margin, padding (1–4 value syntax), background color
// extraction, list-style. Supported logical aliases are the block/inline
// start/end forms for margin and padding.
func expandShorthand(prop, val string) map[string]string {
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
	for _, tok := range splitCSSComponentValues(val) {
		tok = strings.TrimSpace(tok)
		if tok != "" && parseCSSColor(tok) {
			return map[string]string{"background-color": tok}
		}
	}
	return map[string]string{}
}

func expandListStyleShorthand(val string) map[string]string {
	decls := make(map[string]string)
	for _, tok := range splitCSSComponentValues(val) {
		tok = strings.TrimSpace(tok)
		lower := strings.ToLower(tok)
		switch {
		case lower == "inside" || lower == "outside":
			decls["list-style-position"] = lower
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
