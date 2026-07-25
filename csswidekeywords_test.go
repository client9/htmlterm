package htmlterm_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm"
)

// TestCSSWideKeywordUnsetClearsBorder is an end-to-end check that a more
// specific rule's `unset` actually clears a broader rule's declaration at
// render time (border-style is not inheritable, so unset here behaves like
// initial, not inherit) - the case that originally motivated adding general
// inherit/unset/initial support.
func TestCSSWideKeywordUnsetClearsBorder(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "unset on a non-inheritable property clears a broader rule's value",
			css:  `table { border-collapse: collapse; } td { border: solid; } td.x { border-style: unset; }`,
			html: `<table><tr><td class="x">A</td><td>B</td></tr></table>`,
			want: " ┌─┐\nA│B│\n └─┘\n",
		},
	})
}

// TestCSSWideKeywordInherit is an end-to-end check that `inherit` actually
// pulls the ancestor's rendered color, not just the cascade-level string.
func TestCSSWideKeywordInherit(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{CSS: `div { color: red; } p { color: inherit; }`, Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Render(`<div>outer<p>inner</p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "\x1b[38;2;255;0;0m") < 2 {
		t.Fatalf("expected both the div's own text and the inherit-ing <p> to render red: %q", got)
	}
}
