package ansidecode

import "image/color"

// basic16 is the conventional xterm default 16-color palette, used to
// resolve SGR 30-37, 90-97, 40-47, and 100-107. htmlterm's own tests always
// run with colorprofile.TrueColor, so production output never actually
// exercises these codes; they're here so Parse gives a reasonable answer
// for hand-crafted or third-party ANSI input too, rather than silently
// dropping the color. Real terminals let a user's theme override these
// exact values, so treat them as a display convention, not a guarantee.
var basic16 = [16]color.Color{
	color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, // black
	color.RGBA{R: 0xcd, G: 0x00, B: 0x00, A: 0xff}, // red
	color.RGBA{R: 0x00, G: 0xcd, B: 0x00, A: 0xff}, // green
	color.RGBA{R: 0xcd, G: 0xcd, B: 0x00, A: 0xff}, // yellow
	color.RGBA{R: 0x00, G: 0x00, B: 0xee, A: 0xff}, // blue
	color.RGBA{R: 0xcd, G: 0x00, B: 0xcd, A: 0xff}, // magenta
	color.RGBA{R: 0x00, G: 0xcd, B: 0xcd, A: 0xff}, // cyan
	color.RGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff}, // white
	color.RGBA{R: 0x7f, G: 0x7f, B: 0x7f, A: 0xff}, // bright black
	color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}, // bright red
	color.RGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}, // bright green
	color.RGBA{R: 0xff, G: 0xff, B: 0x00, A: 0xff}, // bright yellow
	color.RGBA{R: 0x5c, G: 0x5c, B: 0xff, A: 0xff}, // bright blue
	color.RGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}, // bright magenta
	color.RGBA{R: 0x00, G: 0xff, B: 0xff, A: 0xff}, // bright cyan
	color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // bright white
}

// xterm256 is the standard xterm 256-color palette: indices 0-15 are
// basic16, 16-231 are a 6x6x6 color cube, and 232-255 are a 24-step
// grayscale ramp. It's computed once from the well-known formula rather
// than transcribed as 256 literal values.
var xterm256 [256]color.Color

func init() {
	copy(xterm256[:16], basic16[:])

	// cubeStep is the standard xterm mapping from a 0-5 cube coordinate to
	// an 8-bit channel value: 0 stays 0, 1-5 step evenly from 95 to 255.
	cubeStep := func(n int) uint8 {
		if n == 0 {
			return 0
		}
		return uint8(55 + 40*n)
	}
	i := 16
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				xterm256[i] = color.RGBA{R: cubeStep(r), G: cubeStep(g), B: cubeStep(b), A: 0xff}
				i++
			}
		}
	}

	for n := 0; n < 24; n++ {
		v := uint8(8 + 10*n)
		xterm256[232+n] = color.RGBA{R: v, G: v, B: v, A: 0xff}
	}
}
