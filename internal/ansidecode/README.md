# `internal/ansidecode`

Decodes the ANSI string `htmlterm.Renderer.Render` produces back into
structured styled runs, and serializes those runs to SVG.

## Why it exists

GitHub's markdown renderer strips the `style` attribute from raw HTML
embedded in a `.md` file, and doesn't interpret ANSI escape codes in fenced
code blocks either. So neither an HTML `<span style="color:...">` nor the
raw ANSI string survives being pasted into a README and still showing
color. An image does. This package exists to produce that image, for a
documentation gallery of hand-picked HTML/CSS examples rendered through the
real engine, not as a test-correctness tool.

## Why it's a decoder, not a second renderer

The alternative design would have `internal/render` grow a pluggable output
format: an interface swapped in at every color and border call site,
letting the same layout pass produce ANSI, SVG, or anything else. That
would be a permanent commitment on a package this codebase otherwise keeps
narrow, and touches roughly 20 call sites across `block.go`, `table.go`,
`inline.go`, `formcontrol.go`, and `overlay_box.go`, not one.

This package instead treats the already-produced ANSI string as its input,
the same string any external consumer of the public `htmlterm` package
already gets. It's symmetric with `internal/textcell`: `ConsumeANSI` there
already recognizes exactly this codebase's escape-sequence boundaries by
delegating to `x/ansi`'s decoder, and this package reuses that recognizer
rather than tokenizing a second time. What it adds on top is turning an
opaque matched sequence into a structured `Style` (color, bold, italic,
hyperlink target), which `textcell.Carry` deliberately leaves opaque since
its own job is only to reopen a span across a wrapped line, not to know
what the span means.

Consequently this package can't drift `internal/render`'s layout or cascade
logic, because it never touches either. Worst case a decode is wrong and a
generated SVG looks off; a render is never at risk.

## What it doesn't handle

Only the vocabulary `internal/render/style.go`'s `inlineStyle.render` emits:
SGR color, bold, italic, underline, strikethrough, reverse, and OSC 8
hyperlinks, one combined sequence per styled span. htmlterm never emits
cursor movement, screen clearing, or scroll regions, so `Parse` doesn't
decode any of those either; it skips them silently rather than guessing.

## API shape

- `Parse(s string) []Line` — decode.
- `ToSVG(lines []Line, opts SVGOptions) string` — for embedding in GitHub
  markdown, where a `style=` attribute would be stripped but an image
  isn't. `SVGOptions` picks font, colors, and padding; every field has a
  usable zero value.
- `ToHTML(lines []Line, opts HTMLOptions) string` — a `<pre>` fragment with
  one `<span style="...">` per styled run, for an actual standalone
  webpage, where a real browser resolves the CSS itself and GitHub's
  sanitizer never sees it. This is the original "intermediate HTML" idea,
  corrected: not a second producer inside `internal/render` that has to be
  re-parsed to get back to a terminal string, but one more decoder backend
  over the same already-correct ANSI, same as `ToSVG`.

Cell width in the generated SVG is approximated from font size, 0.6 of it,
the usual monospace aspect ratio, rather than measured against a real font:
an SVG has no font-metrics API to query at generation time, only a
font-family name the viewer resolves later. Good enough for a documentation
snapshot; not claimed to be pixel-exact. `ToHTML` has no such approximation
to make, since a real browser lays its own `<pre>` out.
