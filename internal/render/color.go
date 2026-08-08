package render

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mazznoer/csscolorparser"
)

// parseCSSColor parses a CSS color string and returns a color.Color, or nil
// if the value is unrecognized. Supports named colors, #RGB, #RRGGBB, rgb(),
// hsl(), and all other CSS Color Level 4 formats, plus the terminal-native
// ansi() function (see parseAnsiFunc).
func parseCSSColor(s string) color.Color {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if c, ok := parseAnsiFunc(s); ok {
		return c
	}
	c, err := csscolorparser.Parse(s)
	if err != nil {
		return nil
	}
	// Fully transparent colors (the "transparent" keyword, or an explicit
	// zero alpha channel) have nothing to render onto a terminal cell, which
	// has no compositing model, so treat them as unset rather than opaque
	// black, which is what a bare RGB{0,0,0} would otherwise produce.
	if _, _, _, a := c.RGBA(); a == 0 {
		return nil
	}
	return c
}

// parseAnsiFunc recognizes the terminal-native ansi(N) color function,
// N an integer 0-255, and returns nil, false for anything else, including a
// bare unwrapped number: real CSS has no such function, so this is
// deliberately spelled to be greppable and unambiguous with unitless
// lengths elsewhere in the grammar. See CSS.md's Color Values section and
// COMPATIBILITY.md's Terminal-Native Additions for the full rationale.
//
// 0-15 map to ansi.BasicColor. colorprofile.Profile.Convert (called from
// style.go's inlineStyle.render) always passes an ansi.BasicColor through
// unconverted, at every profile level, so it renders as SGR 30-37/90-97 and
// the terminal's own configured theme picks the actual RGB, the one thing a
// hex or named color can never do here since those always start from a
// fixed RGB. 16-255 map to ansi.IndexedColor, the fixed 6x6x6 cube and
// greyscale ramp: preserved exactly on a 256-color or truecolor terminal,
// downsampled to the nearest of the 16 basic colors on an ANSI-only one.
func parseAnsiFunc(s string) (color.Color, bool) {
	if len(s) < len("ansi()") || !strings.EqualFold(s[:len("ansi(")], "ansi(") || s[len(s)-1] != ')' {
		return nil, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(s[len("ansi(") : len(s)-1]))
	if err != nil || n < 0 || n > 255 {
		return nil, false
	}
	if n < 16 {
		return ansi.BasicColor(n), true
	}
	return ansi.IndexedColor(n), true
}

// applyOpacity premultiplies the RGB channels by opacity (0.0–1.0).
// Used for CSS opacity support. Terminals can't do alpha compositing,
// so dimming the color is the best approximation.
func applyOpacity(c color.Color, opacity float64) color.Color {
	if opacity <= 0 {
		return color.RGBA{A: 0xff}
	}
	if opacity >= 1 {
		return c
	}
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(float64(r>>8) * opacity),
		G: uint8(float64(g>>8) * opacity),
		B: uint8(float64(b>>8) * opacity),
		A: 0xff,
	}
}
