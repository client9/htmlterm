package ansidecode

import (
	"fmt"
	"strings"
)

// SVGOptions controls ToSVG's rendering. Every field's zero value is
// usable; the defaults approximate a dark terminal theme.
type SVGOptions struct {
	FontFamily string  // CSS font-family list; default is a common monospace stack
	FontSize   float64 // px; default 14
	Background string  // CSS color for the default terminal background; default "#1e1e1e"
	Foreground string  // CSS color used where a run's Style.FG is nil; default "#d4d4d4"
	Padding    float64 // px around the text grid; default 12
}

func (o SVGOptions) withDefaults() SVGOptions {
	if o.FontFamily == "" {
		o.FontFamily = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
	}
	if o.FontSize == 0 {
		o.FontSize = 14
	}
	if o.Background == "" {
		o.Background = "#1e1e1e"
	}
	if o.Foreground == "" {
		o.Foreground = "#d4d4d4"
	}
	if o.Padding == 0 {
		o.Padding = 12
	}
	return o
}

// ToSVG renders lines as a self-contained SVG document: one background
// <rect> per run that has a background color, and one <text> per run,
// positioned on a fixed monospace grid. It's built for documentation, a
// static snapshot GitHub's markdown renderer can embed with
// ![](example.svg), not for pixel-exact terminal emulation. Cell width is
// approximated as 0.6 of FontSize, the usual aspect ratio for monospace
// fonts, rather than measured against the real font: an SVG has no font
// metrics API to query, only a font-family name the viewer resolves later.
func ToSVG(lines []Line, opts SVGOptions) string {
	opts = opts.withDefaults()
	cellW := opts.FontSize * 0.6
	lineH := opts.FontSize * 1.4

	cols := 0
	for _, ln := range lines {
		w := 0
		for _, r := range ln.Runs {
			w += r.Width
		}
		if w > cols {
			cols = w
		}
	}
	width := opts.Padding*2 + float64(cols)*cellW
	height := opts.Padding*2 + float64(len(lines))*lineH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 %.2f %.2f" width="%.2f" height="%.2f" font-family=%q font-size="%v">`+"\n",
		width, height, width, height, opts.FontFamily, opts.FontSize)
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%.2f" height="%.2f" fill=%q/>`+"\n", width, height, opts.Background)

	for row, ln := range lines {
		y := opts.Padding + float64(row)*lineH
		col := 0
		for _, run := range ln.Runs {
			x := opts.Padding + float64(col)*cellW
			runW := float64(run.Width) * cellW
			fg, bg := effectiveColors(run.Style, opts.Foreground, opts.Background)
			if bg != opts.Background {
				fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill=%q/>`+"\n",
					x, y, runW, lineH, bg)
			}
			writeSVGRun(&b, x, y+lineH*0.78, run, fg)
			col += run.Width
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func writeSVGRun(b *strings.Builder, x, yBaseline float64, run Run, fg string) {
	if run.Text == "" {
		return
	}
	var attrs strings.Builder
	if run.Style.Bold {
		attrs.WriteString(` font-weight="bold"`)
	}
	if run.Style.Italic {
		attrs.WriteString(` font-style="italic"`)
	}
	if deco := textDecoration(run.Style); deco != "" {
		fmt.Fprintf(&attrs, ` text-decoration="%s"`, deco)
	}

	linked := run.Style.Href != ""
	if linked {
		// escapeAttr, not %q: Href comes from arbitrary HTML href= content,
		// and needs XML entity escaping, not Go string-literal escaping.
		// The two coincide for a plain URL but diverge on a literal
		// backslash or non-ASCII byte.
		fmt.Fprintf(b, `<a xlink:href="%s">`, escapeAttr(run.Style.Href))
	}
	fmt.Fprintf(b, `<text x="%.2f" y="%.2f" xml:space="preserve" fill=%q%s>%s</text>`+"\n",
		x, yBaseline, fg, attrs.String(), escapeText(run.Text))
	if linked {
		b.WriteString("</a>\n")
	}
}
