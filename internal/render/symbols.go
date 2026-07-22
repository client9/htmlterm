package render

import "strings"

// parseSymbolsArgs parses a CSS symbols() function token, e.g.
// `symbols("🟥" "🟨" "🟦")`, into its ordered list of decoded strings. It
// only understands the plain string-list form (no <symbols-type> keyword or
// <image> arguments, since neither has a terminal-rendering equivalent
// here). Returns ok=false if raw isn't a symbols(...) token or contains no
// quoted strings.
//
// This is a generic string -> []string parser with no list-style-specific
// knowledge, so it's reusable wherever else a "symbols(...)" value shows up.
func parseSymbolsArgs(raw string) (items []string, ok bool) {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "symbols(") || !strings.HasSuffix(s, ")") {
		return nil, false
	}
	args := strings.TrimSpace(s[len("symbols(") : len(s)-1])
	for args != "" {
		item, rest, consumed := consumeQuotedToken(args)
		if !consumed {
			break
		}
		items = append(items, item)
		args = strings.TrimSpace(rest)
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}
