package htmlterm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// renderTrueColor creates a Renderer with forced TrueColor and calls Render.
func renderTrueColor(t *testing.T, css, htmlStr string) string {
	t.Helper()
	r, err := New(Options{CSS: css, Width: 40, Profile: colorprofile.TrueColor})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out, err := r.Render(htmlStr)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

func TestListMarkerColor(t *testing.T) {
	t.Run("li::marker emits ANSI color on prefix", func(t *testing.T) {
		out := renderTrueColor(t, `li::marker { color: #ff0000; }`, `<ul><li>hello</li></ul>`)
		// Red TrueColor foreground sequence
		if !strings.Contains(out, "\x1b[38;2;255;0;0m") {
			t.Errorf("expected red ANSI sequence in output, got: %q", out)
		}
		// Bullet and content must still be present
		if !strings.Contains(out, "•") {
			t.Errorf("expected bullet in output, got: %q", out)
		}
		if !strings.Contains(out, "hello") {
			t.Errorf("expected item text in output, got: %q", out)
		}
		// Content "hello" must NOT be wrapped in the marker color
		// (color applies to prefix only, not to item text)
		idx := strings.Index(out, "hello")
		if idx > 0 && strings.Contains(out[:idx], "\x1b[38;2;255;0;0m") {
			// There must be a reset between the colored prefix and "hello"
			prefixSection := out[:idx]
			if !strings.Contains(prefixSection, "\x1b[m") && !strings.Contains(prefixSection, "\x1b[0m") {
				t.Errorf("color reset expected between marker and item text, got prefix: %q", prefixSection)
			}
		}
	})

	t.Run("li::marker on ol emits ANSI color on prefix", func(t *testing.T) {
		out := renderTrueColor(t, `li::marker { color: #0000ff; }`, `<ol><li>x</li></ol>`)
		if !strings.Contains(out, "\x1b[38;2;0;0;255m") {
			t.Errorf("expected blue ANSI sequence in output, got: %q", out)
		}
		if !strings.Contains(out, "1.") {
			t.Errorf("expected decimal prefix in output, got: %q", out)
		}
	})

	t.Run("li::marker with font-weight bold on prefix", func(t *testing.T) {
		out := renderTrueColor(t, `li::marker { font-weight: bold; }`, `<ul><li>item</li></ul>`)
		// Bold ANSI sequence
		if !strings.Contains(out, "\x1b[1m") {
			t.Errorf("expected bold ANSI sequence in output, got: %q", out)
		}
	})

	t.Run("no marker rule emits no extra ANSI", func(t *testing.T) {
		out := renderTrueColor(t, ``, `<ul><li>a</li></ul>`)
		// Without a marker rule there should be no ANSI at all in this simple case
		if strings.Contains(out, "\x1b[") {
			t.Errorf("expected no ANSI in plain list, got: %q", out)
		}
	})
}
