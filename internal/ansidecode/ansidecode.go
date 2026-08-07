// Package ansidecode turns the ANSI string htmlterm's Render produces back
// into structured styled runs, for consumers that aren't a terminal. Its
// first consumer is a documentation generator that renders curated examples
// to SVG, so a gallery of colored terminal output can be embedded in a
// GitHub-rendered markdown page. GitHub sanitizes the style attribute out of
// raw HTML in markdown, and doesn't interpret ANSI codes in fenced blocks
// either, so neither the ANSI string nor an HTML span with inline color
// survives being embedded directly; an image does.
//
// This is not a general ANSI terminal emulator. htmlterm never emits cursor
// movement, screen-clearing, or scroll-region sequences, only SGR styling
// and OSC 8 hyperlinks, self-contained per line (see
// internal/render/block.go's wrapHyperlinkBox doc comment on why every line
// gets its own open and close pair). Parse decodes exactly that vocabulary
// and silently skips anything else, rather than trying to be correct for
// arbitrary terminal output.
package ansidecode

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/textcell"
)

// Style is the fully resolved appearance of one Run: the SGR attributes and
// OSC 8 hyperlink target active when it was emitted. It mirrors
// internal/render's inlineStyle, but as a decode result rather than a
// cascade result. Nothing here is inherited; it's just what a terminal
// would have shown for these runes.
type Style struct {
	FG        color.Color // nil means the terminal's default foreground
	BG        color.Color // nil means the terminal's default background
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Reverse   bool
	Href      string // OSC 8 target; "" if the run isn't a hyperlink
}

// Run is a maximal span of text sharing one Style.
type Run struct {
	Text  string
	Width int // sum of grapheme cell widths, equal to textcell.Width(Text)
	Style Style
}

// Line is one output row. A row boundary is exactly a '\n' in the decoded
// string: htmlterm never emits cursor-addressing sequences, so there's no
// other way a new row starts.
type Line struct {
	Runs []Run
}

// Parse decodes s, a string produced by htmlterm.Renderer.Render or
// render.Engine.RenderHTML's Result.Output, into structured lines and runs.
//
// Parse mirrors strings.Split's trailing-separator behavior: a final '\n'
// ends the last line rather than starting an empty one after it, matching
// the "Hello\n" style want strings already used throughout this module's
// tests. A '\n' anywhere else, including two in a row, produces a line the
// normal way, blank or not.
func Parse(s string) []Line {
	runes := []rune(s)
	var lines []Line
	var runs []Run
	var textBuf strings.Builder
	var textWidth int
	cur := Style{}

	flush := func() {
		if textBuf.Len() == 0 {
			return
		}
		runs = append(runs, Run{Text: textBuf.String(), Width: textWidth, Style: cur})
		textBuf.Reset()
		textWidth = 0
	}
	newline := func() {
		flush()
		lines = append(lines, Line{Runs: runs})
		runs = nil
	}

	i := 0
	for i < len(runes) {
		switch runes[i] {
		case '\x1b':
			end := textcell.ConsumeANSI(runes, i)
			applySeq(string(runes[i:end]), &cur, flush)
			i = end
		case '\n':
			newline()
			i++
		default:
			width, next := textcell.NextGrapheme(runes, i)
			textBuf.WriteString(string(runes[i:next]))
			textWidth += width
			i = next
		}
	}
	flush()
	if len(runs) > 0 {
		lines = append(lines, Line{Runs: runs})
	}
	return lines
}

// PlainText reassembles lines back into plain text, one joining '\n' per
// line and none after the last, the same convention Parse decoded from.
// Style and hyperlink information is dropped. This is Strip, restated over
// already-decoded structure instead of the ANSI string, for a caller that
// already has Lines on hand and wants a second, unstyled view of them
// rather than re-decoding.
func PlainText(lines []Line) string {
	var b strings.Builder
	for i, ln := range lines {
		for _, run := range ln.Runs {
			b.WriteString(run.Text)
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// applySeq updates cur for one fully-extracted escape sequence, as produced
// by textcell.ConsumeANSI. It flushes the run buffer first whenever the
// sequence actually changes the style, so a run never straddles a style
// change, and never flushes for a sequence that leaves cur unchanged, such
// as a redundant reset.
func applySeq(seq string, cur *Style, flush func()) {
	switch {
	case strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m"):
		next := *cur
		applySGR(seq, &next)
		if next != *cur {
			flush()
			*cur = next
		}
	case strings.HasPrefix(seq, "\x1b]8;"):
		href := parseOSC8(seq)
		if href != cur.Href {
			flush()
			cur.Href = href
		}
	default:
		// Cursor movement or another control sequence htmlterm doesn't
		// emit. Left alone rather than guessed at.
	}
}

// applySGR interprets one CSI ... 'm' sequence's parameters against s,
// covering the subset internal/render/style.go's inlineStyle.render emits:
// bold, italic, underline, strike, reverse, and basic, 256-color, or
// truecolor foreground and background. Every code render.go can produce is
// handled directly; the "no X" and default-color codes (22, 23, 24, 27, 29,
// 39, 49) are handled too even though render.go never emits them itself,
// since they're standard SGR and a hand-crafted or third-party ANSI string
// could still use them.
func applySGR(seq string, s *Style) {
	params := seq[2 : len(seq)-1] // strip the leading "\x1b[" and trailing "m"
	if params == "" {
		*s = Style{Href: s.Href}
		return
	}
	fields := strings.Split(params, ";")
	nums := make([]int, len(fields))
	for i, f := range fields {
		// An empty field, from a leading, trailing, or doubled ';', parses
		// as 0, the same as an explicit "0": both mean reset.
		n, _ := strconv.Atoi(f)
		nums[i] = n
	}
	for i := 0; i < len(nums); i++ {
		switch p := nums[i]; {
		case p == 0:
			href := s.Href
			*s = Style{Href: href}
		case p == 1:
			s.Bold = true
		case p == 3:
			s.Italic = true
		case p == 4:
			s.Underline = true
		case p == 7:
			s.Reverse = true
		case p == 9:
			s.Strike = true
		case p == 22:
			s.Bold = false
		case p == 23:
			s.Italic = false
		case p == 24:
			s.Underline = false
		case p == 27:
			s.Reverse = false
		case p == 29:
			s.Strike = false
		case p == 39:
			s.FG = nil
		case p == 49:
			s.BG = nil
		case p >= 30 && p <= 37:
			s.FG = basic16[p-30]
		case p >= 90 && p <= 97:
			s.FG = basic16[8+p-90]
		case p >= 40 && p <= 47:
			s.BG = basic16[p-40]
		case p >= 100 && p <= 107:
			s.BG = basic16[8+p-100]
		case p == 38:
			if c, consumed := parseExtendedColor(nums[i+1:]); consumed > 0 {
				s.FG = c
				i += consumed
			}
		case p == 48:
			if c, consumed := parseExtendedColor(nums[i+1:]); consumed > 0 {
				s.BG = c
				i += consumed
			}
		}
	}
}

// parseExtendedColor reads the mode selector and its color components,
// 256-color or truecolor, from the parameters following a bare 38 or 48. It
// reports how many of rest it consumed, 0 if rest doesn't hold a complete,
// recognized color.
func parseExtendedColor(rest []int) (c color.Color, consumed int) {
	if len(rest) == 0 {
		return nil, 0
	}
	switch rest[0] {
	case 5: // indexed 256-color: 5;n
		if len(rest) < 2 {
			return nil, 0
		}
		n := rest[1]
		if n < 0 || n > 255 {
			return nil, 0
		}
		return xterm256[n], 2
	case 2: // truecolor: 2;r;g;b
		if len(rest) < 4 {
			return nil, 0
		}
		return color.RGBA{R: clampByte(rest[1]), G: clampByte(rest[2]), B: clampByte(rest[3]), A: 255}, 4
	}
	return nil, 0
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// parseOSC8 extracts the URI from a "\x1b]8;<params>;<uri>" sequence,
// terminated by BEL or ST per ansi.SetHyperlink. An empty result means the
// sequence closes a hyperlink rather than opening one, matching
// textcell.Carry's osc8IsReset convention.
func parseOSC8(seq string) string {
	body := strings.TrimPrefix(seq, "\x1b]8;")
	body = strings.TrimSuffix(body, "\x1b\\")
	body = strings.TrimSuffix(body, "\x07")
	semi := strings.IndexByte(body, ';')
	if semi < 0 {
		return ""
	}
	return body[semi+1:]
}
