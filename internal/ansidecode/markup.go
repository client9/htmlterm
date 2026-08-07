package ansidecode

import (
	"fmt"
	"image/color"
	"strings"
)

// effectiveColors resolves a run's foreground and background against page
// defaults, applying SGR 7 (reverse) the way a real terminal would: swap
// whatever's resolved, author-set or default, not just whatever the run
// happened to set explicitly. Both return values are always concrete hex
// colors, never "": ToSVG needs fg for a mandatory fill attribute, and
// ToHTML compares both against the page defaults itself to decide whether
// they're worth stating at all.
func effectiveColors(s Style, defaultFG, defaultBG string) (fg, bg string) {
	fg = colorToHex(s.FG, defaultFG)
	bg = colorToHex(s.BG, defaultBG)
	if s.Reverse {
		fg, bg = bg, fg
	}
	return fg, bg
}

func colorToHex(c color.Color, fallback string) string {
	if c == nil {
		return fallback
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// textDecoration builds an SVG/CSS text-decoration value from the two
// properties that map onto it. "" means neither is set, the same value
// CSS's own initial value normalizes to.
func textDecoration(s Style) string {
	var decos []string
	if s.Underline {
		decos = append(decos, "underline")
	}
	if s.Strike {
		decos = append(decos, "line-through")
	}
	return strings.Join(decos, " ")
}

// escapeText escapes the three characters that are special in both HTML
// and XML text content. The two markup languages agree on this subset,
// which is why ToSVG and ToHTML share it instead of each carrying its own
// copy.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// escapeAttr is escapeText plus the double quote, for use inside a
// "-delimited attribute value in either markup language.
func escapeAttr(s string) string {
	return strings.ReplaceAll(escapeText(s), `"`, "&quot;")
}
