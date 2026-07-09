package render

import (
	"image/color"
	"testing"
)

// rgba255 extracts non-premultiplied 0–255 channel values from any color.Color.
// The Go color.Color interface returns premultiplied alpha, so we unpremultiply
// using floating-point to avoid truncation errors.
func rgba255(c color.Color) (r, g, b, a uint8) {
	ri, gi, bi, ai := c.RGBA() // 0–65535, premultiplied
	a = uint8(ai >> 8)
	if ai == 0 {
		return 0, 0, 0, 0
	}
	fa := float64(ai)
	r = uint8(float64(ri)/fa*255.0 + 0.5)
	g = uint8(float64(gi)/fa*255.0 + 0.5)
	b = uint8(float64(bi)/fa*255.0 + 0.5)
	return
}

func TestParseCSSColor(t *testing.T) {
	tests := []struct {
		in         string
		r, g, b, a uint8 // expected non-premultiplied 0–255 values; all zero means expect nil
		wantNil    bool
	}{
		// hex 6-digit
		{in: "#ff0000", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "#000000", r: 0x00, g: 0x00, b: 0x00, a: 0xff},
		{in: "#ffffff", r: 0xff, g: 0xff, b: 0xff, a: 0xff},
		{in: "#1a2b3c", r: 0x1a, g: 0x2b, b: 0x3c, a: 0xff},
		{in: "#888888", r: 0x88, g: 0x88, b: 0x88, a: 0xff},

		// hex 3-digit (each nibble doubled)
		{in: "#f00", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "#0f0", r: 0x00, g: 0xff, b: 0x00, a: 0xff},
		{in: "#00f", r: 0x00, g: 0x00, b: 0xff, a: 0xff},
		{in: "#abc", r: 0xaa, g: 0xbb, b: 0xcc, a: 0xff},
		{in: "#123", r: 0x11, g: 0x22, b: 0x33, a: 0xff},

		// uppercase hex
		{in: "#FF0000", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "#ABC", r: 0xaa, g: 0xbb, b: 0xcc, a: 0xff},

		// hex with alpha (#RGBA and #RRGGBBAA — CSS Color Level 4)
		{in: "#1234", r: 0x11, g: 0x22, b: 0x33, a: 0x44},
		{in: "#11223344", r: 0x11, g: 0x22, b: 0x33, a: 0x44},

		// named colors
		{in: "red", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "blue", r: 0x00, g: 0x00, b: 0xff, a: 0xff},
		{in: "green", r: 0x00, g: 0x80, b: 0x00, a: 0xff},
		{in: "white", r: 0xff, g: 0xff, b: 0xff, a: 0xff},
		{in: "black", r: 0x00, g: 0x00, b: 0x00, a: 0xff},
		{in: "cornflowerblue", r: 0x64, g: 0x95, b: 0xed, a: 0xff},
		{in: "rebeccapurple", r: 0x66, g: 0x33, b: 0x99, a: 0xff},

		// named color case-insensitive
		{in: "Red", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "BLUE", r: 0x00, g: 0x00, b: 0xff, a: 0xff},
		{in: "CornflowerBlue", r: 0x64, g: 0x95, b: 0xed, a: 0xff},

		// whitespace trimmed
		{in: "  red  ", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "  #ff0000  ", r: 0xff, g: 0x00, b: 0x00, a: 0xff},

		// grey/gray aliases
		{in: "grey", r: 0x80, g: 0x80, b: 0x80, a: 0xff},
		{in: "gray", r: 0x80, g: 0x80, b: 0x80, a: 0xff},
		{in: "darkgrey", r: 0xa9, g: 0xa9, b: 0xa9, a: 0xff},
		{in: "darkgray", r: 0xa9, g: 0xa9, b: 0xa9, a: 0xff},

		// functional notation
		{in: "rgb(255,0,0)", r: 0xff, g: 0x00, b: 0x00, a: 0xff},
		{in: "rgb(0 128 0)", r: 0x00, g: 0x80, b: 0x00, a: 0xff},
		{in: "rgba(0,0,255,1)", r: 0x00, g: 0x00, b: 0xff, a: 0xff},
		{in: "rgba(255,0,0,0.5)", r: 0xff, g: 0x00, b: 0x00, a: 0x80},
		{in: "hsl(0,100%,50%)", r: 0xff, g: 0x00, b: 0x00, a: 0xff},

		// unrecognized → nil
		{in: "", wantNil: true},
		{in: "notacolor", wantNil: true},
		{in: "21", wantNil: true},       // bare ANSI index not supported
		{in: "#gg0000", wantNil: true},  // invalid hex digit
		{in: "#12345", wantNil: true},   // wrong hex length (5 digits)
		{in: "#1234567", wantNil: true}, // wrong hex length (7 digits)
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parseCSSColor(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("parseCSSColor(%q) = %v, want nil", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseCSSColor(%q) = nil, want rgba(%d,%d,%d,%d)", tc.in, tc.r, tc.g, tc.b, tc.a)
			}
			gr, gg, gb, ga := rgba255(got)
			if gr != tc.r || gg != tc.g || gb != tc.b || ga != tc.a {
				t.Fatalf("parseCSSColor(%q) = rgba(%d,%d,%d,%d), want rgba(%d,%d,%d,%d)",
					tc.in, gr, gg, gb, ga, tc.r, tc.g, tc.b, tc.a)
			}
		})
	}
}

func TestApplyOpacity(t *testing.T) {
	red := color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}

	tests := []struct {
		name    string
		c       color.Color
		opacity float64
		wantR   uint8
		wantG   uint8
		wantB   uint8
	}{
		{"opacity 1.0 is passthrough", red, 1.0, 0xff, 0x00, 0x00},
		{"opacity 0.5 halves channels", red, 0.5, 0x7f, 0x00, 0x00},
		{"opacity 0.0 gives black", red, 0.0, 0x00, 0x00, 0x00},
		{"opacity 0.25", color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, 0.25, 0x3f, 0x3f, 0x3f},
		{"opacity > 1 clamped to passthrough", red, 2.0, 0xff, 0x00, 0x00},
		{"opacity < 0 gives black", red, -1.0, 0x00, 0x00, 0x00},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyOpacity(tc.c, tc.opacity)
			r, g, b, _ := got.RGBA()
			gotR, gotG, gotB := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			if gotR != tc.wantR || gotG != tc.wantG || gotB != tc.wantB {
				t.Fatalf("applyOpacity(red, %v) = RGB(%d,%d,%d), want RGB(%d,%d,%d)",
					tc.opacity, gotR, gotG, gotB, tc.wantR, tc.wantG, tc.wantB)
			}
		})
	}
}
