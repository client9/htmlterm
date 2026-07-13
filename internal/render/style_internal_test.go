package render

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestInlineStyleOpacityZeroBlanksText(t *testing.T) {
	st := extractInlineStyle(map[string]string{"opacity": "0"})
	got := st.render("hidden", colorprofile.TrueColor)
	want := "      "
	if got != want {
		t.Errorf("render with opacity:0 = %q, want %q (blank, same width, not black)", got, want)
	}
}

func TestInlineStyleOpacityDefaultIsFullyOpaque(t *testing.T) {
	st := newInlineStyle()
	got := st.render("visible", colorprofile.TrueColor)
	if got != "visible" {
		t.Errorf("render with default (unset) inlineStyle = %q, want unchanged text", got)
	}
}

func TestInlineStyleFractionalOpacityDimsColor(t *testing.T) {
	decls := map[string]string{"opacity": "0.5", "color": "#ff0000"}
	st := extractInlineStyle(decls)
	got := st.render("dim", colorprofile.TrueColor)
	if got == "dim" {
		t.Errorf("render with opacity:0.5 and a color set should apply ANSI styling, got plain text %q", got)
	}
}
