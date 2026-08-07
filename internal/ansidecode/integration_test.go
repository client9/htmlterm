package ansidecode_test

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/internal/ansidecode"
)

// TestParseDecodesRealRendererOutput runs actual HTML through the public
// Renderer, the same path every other test in this module exercises, and
// checks Parse recovers the styling instead of asserting against a
// hand-crafted ANSI string. internal/render/style.go is free to change how
// it composes SGR codes as long as it stays spec-compliant; this test
// catches ansidecode falling out of sync with that without needing to
// import internal/render itself.
func TestParseDecodesRealRendererOutput(t *testing.T) {
	r, err := htmlterm.New(htmlterm.Options{Width: 20, Profile: colorprofile.TrueColor, NoOSC8Links: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(`<b style="color:#ff0000">Red</b> plain`)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	lines := ansidecode.Parse(out)
	if len(lines) != 1 {
		t.Fatalf("Parse(%q) has %d lines, want 1", out, len(lines))
	}
	runs := lines[0].Runs
	if len(runs) != 2 {
		t.Fatalf("Parse(%q) has %d runs, want 2 (styled + plain): %+v", out, len(runs), runs)
	}

	red := runs[0]
	if red.Text != "Red" || !red.Style.Bold {
		t.Errorf("first run = %+v, want text %q and Bold", red, "Red")
	}
	if r, g, b, _ := red.Style.FG.RGBA(); r>>8 != 0xff || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("first run FG = %#v, want pure red", red.Style.FG)
	}

	plain := runs[1]
	if plain.Style.Bold || plain.Style.FG != nil {
		t.Errorf("second run = %+v, want no style at all", plain)
	}
}
