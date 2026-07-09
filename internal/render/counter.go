package render

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// counterSnapshot holds a deep copy of counter values at a point in the DOM.
// name → stack of integer values (innermost/most-recently-reset value last).
type counterSnapshot map[string][]int

// value returns the innermost counter value for name, or 0 if not set.
func (cs counterSnapshot) value(name string) int {
	stack := cs[name]
	if len(stack) == 0 {
		return 0
	}
	return stack[len(stack)-1]
}

// values returns all counter values for name (for counters() nested output).
func (cs counterSnapshot) values(name string) []int {
	return cs[name]
}

// mutableCS is the live counter state used during the DOM pre-pass.
type mutableCS map[string][]int

func (cs mutableCS) reset(name string, val int) {
	cs[name] = append(cs[name], val)
}

func (cs mutableCS) increment(name string, step int) {
	if len(cs[name]) == 0 {
		cs[name] = []int{step}
		return
	}
	cs[name][len(cs[name])-1] += step
}

func (cs mutableCS) pop(name string) {
	if stack := cs[name]; len(stack) > 0 {
		cs[name] = stack[:len(stack)-1]
	}
}

func (cs mutableCS) snapshot() counterSnapshot {
	snap := make(counterSnapshot, len(cs))
	for k, v := range cs {
		cp := make([]int, len(v))
		copy(cp, v)
		snap[k] = cp
	}
	return snap
}

// buildCounterMap walks doc in document order and returns a map from each
// element to its counter snapshot after applying that element's own
// counter-reset and counter-increment declarations (not inherited).
func (r *Engine) buildCounterMap(doc *html.Node) map[*html.Node]counterSnapshot {
	result := make(map[*html.Node]counterSnapshot)
	cs := make(mutableCS)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			return
		}
		if isSkippedContentElement(n.Data) {
			return
		}
		// counter-reset and counter-increment do not inherit; use directDecls.
		decls := r.directDecls(n)
		var resetNames []string
		if v := strings.TrimSpace(decls["counter-reset"]); v != "" && v != "none" {
			toks := strings.Fields(v)
			for i := 0; i < len(toks); {
				name := toks[i]
				i++
				val := 0
				if i < len(toks) {
					if n2, err := strconv.Atoi(toks[i]); err == nil {
						val = n2
						i++
					}
				}
				cs.reset(name, val)
				resetNames = append(resetNames, name)
			}
		}
		if v := strings.TrimSpace(decls["counter-increment"]); v != "" && v != "none" {
			toks := strings.Fields(v)
			for i := 0; i < len(toks); {
				name := toks[i]
				i++
				step := 1
				if i < len(toks) {
					if n2, err := strconv.Atoi(toks[i]); err == nil {
						step = n2
						i++
					}
				}
				cs.increment(name, step)
			}
		}
		result[n] = cs.snapshot()
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		for _, name := range resetNames {
			cs.pop(name)
		}
	}
	walk(doc)
	return result
}

// formatCounterValue formats the integer n using the given list-style-type
// style. Unlike listItemPrefix it returns just the value, no trailing ". ".
func formatCounterValue(n int, style string) string {
	switch style {
	case "", "decimal":
		return strconv.Itoa(n)
	case "lower-alpha", "lower-latin":
		return alphaSequence(n, false)
	case "upper-alpha", "upper-latin":
		return alphaSequence(n, true)
	case "lower-roman":
		return toRoman(n, false)
	case "upper-roman":
		return toRoman(n, true)
	case "none":
		return ""
	default:
		if s := parseCSSString(style); s != "" {
			return s
		}
		return strconv.Itoa(n)
	}
}

// parseCounterFnArgs parses the arg list of counter() or counters():
//
//	counter(name [, style])
//	counters(name, sep [, style])
//
// Returns (name, sep, style); sep is only set for counters().
func parseCounterFnArgs(args string) (name, sep, style string) {
	args = strings.TrimSpace(args)
	i := strings.IndexByte(args, ',')
	if i < 0 {
		return strings.TrimSpace(args), "", ""
	}
	name = strings.TrimSpace(args[:i])
	rest := strings.TrimSpace(args[i+1:])
	if rest == "" {
		return
	}
	if rest[0] == '"' || rest[0] == '\'' {
		q := rest[0]
		j := 1
		for j < len(rest) {
			if rest[j] == '\\' {
				if j+1 >= len(rest) {
					j = len(rest)
					break
				}
				j += 2
				continue
			}
			if rest[j] == q {
				j++
				break
			}
			j++
		}
		sep = parseCSSString(rest[:j])
		rest = strings.TrimSpace(rest[j:])
		if strings.HasPrefix(rest, ",") {
			rest = strings.TrimSpace(rest[1:])
		}
		style = strings.TrimSpace(rest)
	} else {
		style = rest
	}
	return
}

// defaultQuotePairs is the UA default for the `quotes` property.
var defaultQuotePairs = [][2]string{{"“", "”"}, {"‘", "’"}}

// parseQuotes parses the CSS `quotes` property value into open/close pairs.
// E.g. `'"' '"' "'" "'"` → [["\"","\""],["'","'"]].
// Returns defaultQuotePairs if v is empty or unparseable.
func parseQuotes(v string) [][2]string {
	v = strings.TrimSpace(v)
	if v == "" || v == "none" || v == "auto" {
		return defaultQuotePairs
	}
	var pairs [][2]string
	for v != "" {
		open, rest, ok := consumeQuotedToken(v)
		if !ok {
			break
		}
		v = strings.TrimSpace(rest)
		close, rest2, ok2 := consumeQuotedToken(v)
		if !ok2 {
			break
		}
		v = strings.TrimSpace(rest2)
		pairs = append(pairs, [2]string{open, close})
	}
	if len(pairs) == 0 {
		return defaultQuotePairs
	}
	return pairs
}

// consumeQuotedToken reads one CSS quoted string from the start of s,
// returning (value, remainder, ok).
func consumeQuotedToken(s string) (value, rest string, ok bool) {
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return "", s, false
	}
	q := s[0]
	i := 1
	for i < len(s) {
		if s[i] == '\\' {
			if i+1 >= len(s) {
				i = len(s)
				break
			}
			i += 2
			continue
		}
		if s[i] == q {
			i++
			break
		}
		i++
	}
	return parseCSSString(s[:i]), s[i:], true
}
