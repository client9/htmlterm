package ansidecode

import (
	"fmt"
	"strings"
)

// HTMLOptions controls ToHTML's rendering. Every field's zero value is
// usable; the defaults match ToSVG's, so the two backends produce visually
// consistent output by default.
type HTMLOptions struct {
	Background string // CSS color for the <pre>'s own background; default "#1e1e1e"
	Foreground string // CSS color for the <pre>'s own text color; default "#d4d4d4"
}

func (o HTMLOptions) withDefaults() HTMLOptions {
	if o.Background == "" {
		o.Background = "#1e1e1e"
	}
	if o.Foreground == "" {
		o.Foreground = "#d4d4d4"
	}
	return o
}

// ToHTML renders lines as a single <pre> fragment: one <span style="..."> per
// run that needs one, plain escaped text where nothing does. This is the
// format the very first version of this idea reached for directly, an
// intermediate HTML representation a renderer would produce and a browser
// would show. The position was wrong, not the target: an intermediate form
// has to be re-parsed to get back to a terminal string, which means
// reimplementing a slice of internal/cssengine's cascade a second time just
// to close the loop, and it risks the HTML never quite matching what
// internal/render actually does, since nothing forces the two to agree.
// ToHTML instead decodes the same already-correct ANSI string ToSVG does,
// so there's exactly one thing to keep in sync with internal/render/style.go,
// not two.
//
// Unlike ToSVG, this fragment is not meant for GitHub markdown: GitHub
// strips the style attribute from embedded HTML, so a <span
// style="color:...."> here would render as plain text there. It's meant
// for an actual standalone webpage, where a real browser resolves the CSS
// itself, for example a generated example gallery hosted outside GitHub's
// markdown sanitizer.
func ToHTML(lines []Line, opts HTMLOptions) string {
	opts = opts.withDefaults()
	var b strings.Builder
	fmt.Fprintf(&b, "<pre style=\"background-color:%s;color:%s;margin:0\">\n", opts.Background, opts.Foreground)
	for i, ln := range lines {
		for _, run := range ln.Runs {
			writeHTMLRun(&b, run, opts)
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n</pre>\n")
	return b.String()
}

func writeHTMLRun(b *strings.Builder, run Run, opts HTMLOptions) {
	if run.Text == "" {
		return
	}
	fg, bg := effectiveColors(run.Style, opts.Foreground, opts.Background)

	var css strings.Builder
	if fg != opts.Foreground {
		fmt.Fprintf(&css, "color:%s;", fg)
	}
	if bg != opts.Background {
		fmt.Fprintf(&css, "background-color:%s;", bg)
	}
	if run.Style.Bold {
		css.WriteString("font-weight:bold;")
	}
	if run.Style.Italic {
		css.WriteString("font-style:italic;")
	}
	if deco := textDecoration(run.Style); deco != "" {
		fmt.Fprintf(&css, "text-decoration:%s;", deco)
	}

	text := escapeText(run.Text)
	linked := run.Style.Href != ""
	if linked {
		fmt.Fprintf(b, `<a href="%s">`, escapeAttr(run.Style.Href))
	}
	if css.Len() > 0 {
		fmt.Fprintf(b, `<span style="%s">%s</span>`, css.String(), text)
	} else {
		b.WriteString(text)
	}
	if linked {
		b.WriteString("</a>")
	}
}
