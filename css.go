package htmlterm

import (
	"io"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

type rule struct {
	selector string
	decls    map[string]string
}

// parseCSS parses a CSS stylesheet into a list of rules using the raw lexer.
// The high-level css.Parser discards selector tokens before BeginRulesetGrammar,
// so we drive the lexer directly and build a tiny state machine.
func parseCSS(src string) ([]rule, error) {
	l := css.NewLexer(parse.NewInputString(src))
	var rules []rule

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
		val := strings.TrimSpace(valBuf.String())
		if prop != "" && val != "" && curDecls != nil {
			curDecls[prop] = val
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
					rules = append(rules, rule{selector: s, decls: copyDecls(curDecls)})
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
			case css.LeftBraceToken:
				curDecls = make(map[string]string)
				state = inDeclarations
				inValue = false
			default:
				selBuf.Write(data)
			}

		case inDeclarations:
			switch tt {
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

// parseInlineDecls parses a CSS declaration list (the value of a style=""
// attribute) into a property→value map. No selectors or braces expected.
func parseInlineDecls(src string) map[string]string {
	l := css.NewLexer(parse.NewInputString(src))
	result := make(map[string]string)
	var propBuf, valBuf strings.Builder
	inValue := false

	commit := func() {
		prop := strings.ToLower(strings.TrimSpace(propBuf.String()))
		val := strings.TrimSpace(valBuf.String())
		if prop != "" && val != "" {
			result[prop] = val
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

func copyDecls(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
