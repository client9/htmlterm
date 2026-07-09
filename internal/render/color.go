package render

import (
	"image/color"
	"strings"

	"github.com/mazznoer/csscolorparser"
)

// parseCSSColor parses a CSS color string and returns a color.Color, or nil
// if the value is unrecognized. Supports named colors, #RGB, #RRGGBB, rgb(),
// hsl(), and all other CSS Color Level 4 formats.
func parseCSSColor(s string) color.Color {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	c, err := csscolorparser.Parse(s)
	if err != nil {
		return nil
	}
	return c
}

// applyOpacity premultiplies the RGB channels by opacity (0.0–1.0).
// Used for CSS opacity support — terminals can't do alpha compositing,
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
