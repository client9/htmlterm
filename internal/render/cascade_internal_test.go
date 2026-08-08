package render

import "testing"

func TestConvertViewportLengths(t *testing.T) {
	tests := []struct {
		name       string
		val        string
		vpW, vpH   int
		want       string
		wantChange bool
	}{
		{name: "plain vw token", val: "50vw", vpW: 80, vpH: 24, want: "40", wantChange: true},
		{name: "plain vh token", val: "50vh", vpW: 80, vpH: 24, want: "12", wantChange: true},
		{name: "shorthand converts only the vw component", val: "0 10vw", vpW: 80, vpH: 24, want: "0 8", wantChange: true},
		{name: "no viewport unit is a no-op", val: "40ch", vpW: 80, vpH: 24, want: "40ch", wantChange: false},
		{name: "zero vh needs no viewport at all", val: "0vh", vpW: 80, vpH: 0, want: "0", wantChange: true},
		{name: "nonzero vh with no viewport height is left unconverted", val: "50vh", vpW: 80, vpH: 0, want: "50vh", wantChange: false},
		{name: "vw glued inside calc converts", val: "calc(2vw + 4)", vpW: 100, vpH: 24, want: "calc(2 + 4)", wantChange: true},
		{name: "vw glued to a closing paren converts", val: "calc(4 + 2vw)", vpW: 100, vpH: 24, want: "calc(4 + 2)", wantChange: true},
		{name: "vh inside a nested min() converts", val: "calc(min(50vh, 40) + 2)", vpW: 100, vpH: 24, want: "calc(min(12, 40) + 2)", wantChange: true},
		{name: "unit-like suffix past vw is left alone", val: "10vwx", vpW: 100, vpH: 24, want: "10vwx", wantChange: false},
		{name: "negative vw", val: "-50vw", vpW: 100, vpH: 24, want: "-50", wantChange: true},
		{name: "decimal vw truncates toward zero", val: "33vh", vpW: 100, vpH: 24, want: "7", wantChange: true},
		{name: "no vw/vh at all", val: "1 + 2", vpW: 100, vpH: 24, want: "1 + 2", wantChange: false},
		// Regression: the byte-level scan needed to find vw/vh glued inside
		// calc()'s parens has no token boundaries to rely on the way the
		// old whitespace-split implementation incidentally did, so it must
		// explicitly skip over quoted spans (content, a custom border
		// glyph, a quoted list-style-type, ...) rather than mistake a
		// literal digit-then-"vw" substring inside one for a real length.
		{name: "vw-looking digits inside a quoted string are left alone", val: `"3vw call"`, vpW: 80, vpH: 24, want: `"3vw call"`, wantChange: false},
		{name: "a real vw outside the quotes still converts", val: `"3vw call" 10vw`, vpW: 80, vpH: 24, want: `"3vw call" 8`, wantChange: true},
		{name: "single-quoted string is also protected", val: `'2vh'`, vpW: 80, vpH: 24, want: `'2vh'`, wantChange: false},
		// Regression: the byte-level scan started matching a number
		// anywhere it found one, including glued directly onto a
		// preceding identifier character, which the old whitespace-split
		// implementation never did (a non-numeric prefix like "logo" or
		// "Georgia" broke strconv.ParseFloat on the whole token). A digit
		// run immediately preceded by a letter, digit, "-", or "_" is part
		// of that identifier, not a standalone dimension token.
		{name: "digits glued onto a preceding identifier are left alone", val: "url(logo10vh.png)", vpW: 100, vpH: 50, want: "url(logo10vh.png)", wantChange: false},
		{name: "a font-family-shaped identifier is left alone", val: "Georgia5vw", vpW: 100, vpH: 50, want: "Georgia5vw", wantChange: false},
		{name: "a real dimension right after a word boundary still converts", val: "url(x) 10vw", vpW: 100, vpH: 50, want: "url(x) 10", wantChange: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := convertViewportLengths(tt.val, tt.vpW, tt.vpH)
			if got != tt.want || changed != tt.wantChange {
				t.Errorf("convertViewportLengths(%q, %d, %d) = (%q, %v), want (%q, %v)",
					tt.val, tt.vpW, tt.vpH, got, changed, tt.want, tt.wantChange)
			}
		})
	}
}

// TestSubstituteViewportUnitsExemptsAuthorTextProps is a regression test:
// convertViewportLengths' byte-level scan (needed to find a vw/vh length
// glued next to a paren inside calc()) has no way to tell a real dimension
// token apart from author text that merely looks like one lexically —
// content's own mini-language can legitimately contain a numeral
// immediately followed by "vw"/"vh" with no length meaning at all, e.g.
// the "2vw" in `content: counter(x, 2vw)`. substituteViewportUnits must
// skip these properties (cssengine.IsAuthorTextProp) before ever calling
// convertViewportLengths on their value, the same set foldKeywordValue
// already exempts from case folding for the same underlying reason.
func TestSubstituteViewportUnitsExemptsAuthorTextProps(t *testing.T) {
	e := &Engine{width: 100, height: 50}
	out := e.substituteViewportUnits(map[string]string{
		"content":     "counter(x, 2vw)",
		"quotes":      `"2vw" "3vh"`,
		"font-family": "2vw-sans",
		"width":       "10vw",
	})
	for _, prop := range []string{"content", "quotes", "font-family"} {
		if out[prop] == "" {
			t.Fatalf("missing %q in output", prop)
		}
	}
	if out["content"] != "counter(x, 2vw)" {
		t.Errorf(`content = %q, want unchanged`, out["content"])
	}
	if out["quotes"] != `"2vw" "3vh"` {
		t.Errorf(`quotes = %q, want unchanged`, out["quotes"])
	}
	if out["font-family"] != "2vw-sans" {
		t.Errorf(`font-family = %q, want unchanged`, out["font-family"])
	}
	// A real Size Value property in the same decls map must still convert
	// — the exemption is per-property, not global.
	if out["width"] != "10" {
		t.Errorf(`width = %q, want "10"`, out["width"])
	}
}
