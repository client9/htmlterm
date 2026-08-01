package cssengine

import "strings"

// isCustomProp reports whether name is a CSS custom property ("--foo").
// Unlike every other property name in this engine, custom property names are
// case-sensitive, since --Foo and --foo are distinct properties per spec. See
// css.go's commitDecl/parseDeclarationsWithImportance, which special-case
// this prefix to skip their usual strings.ToLower.
func isCustomProp(name string) bool {
	return strings.HasPrefix(name, "--")
}

// customPropSubset returns just the "--*" entries of m, keyed and valued
// exactly as m has them. Used both to seed Direct()'s same-element-only var()
// resolution and to hand a caller's already-resolved custom-prop environment
// to PseudoElement (see cascade.go).
func customPropSubset(m map[string]string) map[string]string {
	sub := make(map[string]string, len(m))
	for k, v := range m {
		if isCustomProp(k) {
			sub[k] = v
		}
	}
	return sub
}

// isIdentChar reports whether c can appear inside a CSS identifier
// including the "-" and "_" this engine's own custom-property names use.
// used by isVarCallAt to make sure a "var(" match isn't actually the tail of
// a longer identifier like "--myvar(" or "somevar(".
func isIdentChar(c byte) bool {
	return c == '-' || c == '_' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isVarCallAt reports whether val[i:] begins a var(...) function call:
// a case-insensitive "var(" not itself preceded by an identifier character.
func isVarCallAt(val string, i int) bool {
	if i+4 > len(val) || !strings.EqualFold(val[i:i+4], "var(") {
		return false
	}
	return i == 0 || !isIdentChar(val[i-1])
}

// findMatchingParen returns the index of the ')' matching the '(' whose
// contents start at start (i.e. start is the position right after that '('),
// tracking nested parens and quoted strings so a fallback like
// var(--a, rgb(0, 0, 0)) or var(--a, "a, b)") isn't cut short. Returns -1 if
// there's no matching ')' (malformed input).
func findMatchingParen(val string, start int) int {
	depth := 1
	i := start
	for i < len(val) {
		switch val[i] {
		case '"', '\'':
			i = consumeCSSQuotedToken(val, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

// splitVarArgs splits a var()'s already-extracted argument text (the part
// between its outer parens) into the custom-property name and fallback text.
// Per spec, only the first top-level comma is syntactic (the name/fallback
// separator. Everything after it up to the closing paren is fallback text
// verbatim, including further commas, nested var() calls, and quoted
// strings, which is why this can't just be a naive strings.Cut(inner, ",").
func splitVarArgs(inner string) (name, fallback string, hasFallback bool) {
	depth := 0
	i := 0
	for i < len(inner) {
		switch inner[i] {
		case '"', '\'':
			i = consumeCSSQuotedToken(inner, i)
			continue
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return strings.TrimSpace(inner[:i]), strings.TrimSpace(inner[i+1:]), true
			}
		}
		i++
	}
	return strings.TrimSpace(inner), "", false
}

// scanVarCalls walks val looking for var(...) calls, handing each one's
// parsed name/fallback to resolve. Where resolve reports ok, its returned
// text replaces the whole var(...) call; where it reports !ok, the original
// var(...) text (name, fallback, and all) is preserved verbatim in the
// output. This "leave untouched" behavior on !ok is what lets
// substituteKnownVarTokens be non-destructive (see its own doc comment).
// scanVarCalls itself is agnostic to which policy a caller wants.
func scanVarCalls(val string, resolve func(name, fallback string, hasFallback bool) (string, bool)) string {
	if !strings.Contains(strings.ToLower(val), "var(") {
		return val
	}
	var b strings.Builder
	i := 0
	for i < len(val) {
		if val[i] == '"' || val[i] == '\'' {
			// A quoted string (e.g. content: "literal var(--x) text") is
			// opaque text, not a place to go looking for a var() call;
			// copy it through untouched rather than misparsing "var(--x)"
			// inside it as a real function call.
			end := consumeCSSQuotedToken(val, i)
			b.WriteString(val[i:end])
			i = end
			continue
		}
		if !isVarCallAt(val, i) {
			b.WriteByte(val[i])
			i++
			continue
		}
		argsStart := i + 4
		end := findMatchingParen(val, argsStart)
		if end < 0 {
			b.WriteString(val[i:])
			return b.String()
		}
		name, fallback, hasFallback := splitVarArgs(val[argsStart:end])
		if repl, ok := resolve(name, fallback, hasFallback); ok {
			b.WriteString(repl)
		} else {
			b.WriteString(val[i : end+1])
		}
		i = end + 1
	}
	return b.String()
}

// substituteVarTokens is the final, spec-shaped substitution pass: an
// unresolved reference (lookup reports the name isn't defined) falls back to
// its own, recursively substituted fallback text, or "" if there is none.
// Used by Resolve() (after its ancestor-inheritance loop has already
// flattened the full custom-prop environment into a single map) and by
// PseudoElement, against the caller-supplied environment. Both places
// where "unresolved" really does mean "unresolved anywhere", so collapsing
// to fallback/empty is safe. Contrast with substituteKnownVarTokens, which
// Direct() uses instead specifically because it must not make that
// assumption.
func substituteVarTokens(val string, lookup func(name string) (string, bool)) string {
	return scanVarCalls(val, func(name, fallback string, hasFallback bool) (string, bool) {
		if v, ok := lookup(name); ok {
			return v, true
		}
		if hasFallback {
			return substituteVarTokens(fallback, lookup), true
		}
		return "", true
	})
}

// substituteKnownVarTokens is Direct()'s own-element-only substitution pass
// (see docs/proposals/VARIABLES.md's Option A scope decision): it resolves
// only the var() calls whose name is present in known (n's own custom-prop
// declarations), and leaves every other var() call untouched as literal
// text, name and fallback alike.
//
// This must be non-destructive: Direct(n)'s result seeds Resolve(n)
// (`direct := c.Direct(n)`), which still needs to see any var() referencing
// an ancestor-only custom property so its own later, fully ancestor-aware
// substitution pass can resolve it. Direct() has no ancestor context of its
// own, so it cannot yet tell "genuinely undefined" apart from "defined
// further up the tree". Collapsing an unresolved reference to "" or its
// fallback here, the way substituteVarTokens does, would destroy that text
// before Resolve ever got a chance to look further, silently breaking
// inheritance for var() on every ordinary (non-custom) property.
func substituteKnownVarTokens(val string, known map[string]string) string {
	return scanVarCalls(val, func(name, _ string, _ bool) (string, bool) {
		v, ok := known[name]
		return v, ok
	})
}

// resolveCustomProps fixed-point-resolves the "--*" subset of raw against
// itself (custom properties may reference other custom properties),
// returning a map of fully-substituted custom-property values. raw may
// (and, called from Resolve, does) also contain non-custom-property keys;
// only "--*" keys are ever added to the returned map. Uses a visited-set to
// break cycles: a cyclic or otherwise unresolvable reference resolves to ""
// for that reference, approximating the spec's "guaranteed-invalid value"
// rather than implementing full invalid-at-computed-value-time
// fallback-to-inherited/initial semantics.
func resolveCustomProps(raw map[string]string) map[string]string {
	resolved := make(map[string]string)
	resolving := make(map[string]bool)

	var resolve func(name string) (string, bool)
	resolve = func(name string) (string, bool) {
		if v, ok := resolved[name]; ok {
			return v, true
		}
		rawVal, ok := raw[name]
		if !ok {
			return "", false
		}
		if resolving[name] {
			// Cyclic reference: treat as unresolved for this reference,
			// rather than recursing forever.
			return "", false
		}
		resolving[name] = true
		v := substituteVarTokens(rawVal, func(n string) (string, bool) {
			return resolve(n)
		})
		delete(resolving, name)
		resolved[name] = v
		return v, true
	}

	for name := range raw {
		if isCustomProp(name) {
			resolve(name)
		}
	}
	return resolved
}
