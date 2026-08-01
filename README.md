# htmlterm

[![Go](https://github.com/client9/htmlterm/actions/workflows/go.yml/badge.svg)](https://github.com/client9/htmlterm/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/client9/htmlterm.svg)](https://pkg.go.dev/github.com/client9/htmlterm)

`htmlterm` is a Go module that renders HTML and CSS to ANSI-styled terminal strings: layout, cascade, tables, forms, and DOM-style interactivity, built for terminal UIs and text-first applications that want more than plain text without embedding a browser engine.

## Features

- Render HTML fragments or full documents to ANSI-styled terminal output.
- Apply CSS from the renderer, `<style>` tags, and inline `style=""` attributes, with a real cascade: specificity, inheritance, `!important`, and the selector forms browsers support (classes, IDs, attributes, descendant/child/sibling combinators, `:nth-child`, `:not`/`:is`/`:where`, `::before`/`::after`, and more).
- Render the vast majority of HTML tags: headings, paragraphs, lists, blockquotes, links, tables (`<colgroup>`/`<col>`, border styles, column sizing), and forms (`<input>`, `<button>`, `<textarea>`, `<fieldset>`).
- Style with most of CSS layout: `display`, margins, padding, width, height, borders, colors, `white-space`, `overflow`, `text-overflow`, `text-align`, `text-transform`, `visibility`, and more. See [CSS.md](./CSS.md) for the full property-by-property reference.
- Emit OSC 8 hyperlinks for `<a href="...">...</a>` when supported by the terminal.
- A mutable `Document`/`Element` API for hosts that want to query, mutate, and re-render a tree instead of parsing once and discarding it.
- Native Go DOM-style events (`AddEventListener`, capture/target/bubble phases), focus management, and hit-testing (`Element.Rect`), with no scripting engine, just Go closures.
- A `Loop` that drives a `Document` against a real terminal: raw-mode keyboard/mouse input, `SetInterval`/`SetTimeout` timers, and a render loop, which is enough to build a small interactive TUI.

## Scope

`htmlterm` renders to a character grid, not pixels, and has no scripting engine, so it isn't a browser. But within that constraint it aims to behave like one: real cascade rules, real DOM/Events semantics, real layout. [CSS.md](./CSS.md) is the exhaustive per-property reference; [COMPATIBILITY.md](./COMPATIBILITY.md) is the orientation read across HTML, CSS, and the DOM/Events API: what deviates from browser behavior and why (text cells, not pixels), what's a terminal-native addition with no browser equivalent (`scrollbar-style`, `<select>` popup compositing, literal-glyph borders), and what's not implemented.

When rendering untrusted HTML or CSS, see [SECURITY.md](./SECURITY.md) for the terminal-output security model and recommended defense-in-depth settings.

## Install

```bash
go get github.com/client9/htmlterm
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/client9/htmlterm"
)

func main() {
	css := `
	.note {
		border-left: │;
		border-left-color: #555555;
		padding-left: 1;
		color: #d0d0d0;
	}
	strong { color: #ffcc66; }
	`

	r, err := htmlterm.New(htmlterm.Options{CSS: css, Width: 40})
	if err != nil {
		log.Fatal(err)
	}

	out, err := r.Render(`<p class="note">hello <strong>terminal</strong></p>`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(out)
}
```

The public API is intentionally small:

- `htmlterm.New(opts Options) (*Renderer, error)` creates a renderer from an `Options` struct
- `(*Renderer).Render(html string) (string, error)` renders an HTML fragment or document to an ANSI-styled string

```go
type Options struct {
    CSS               string               // additional stylesheet layered above built-in defaults (htmlterm.DefaultStylesheet)
    Stylesheets       []string             // additional stylesheets, layered above CSS in order, like a page's own <link> stylesheets
    Width             int                  // terminal column count; affects wrapping, tables, percentage widths
    Height            int                  // content-box line count the whole document is clipped/padded to; see "Sizing and resize" below
    IgnoreDocumentCSS bool                 // if true, <style> elements and style= attributes in HTML are ignored
    Profile           colorprofile.Profile // color profile; zero value auto-detects from environment
    NoOSC8Links       bool                 // if true, OSC 8 hyperlink sequences are not emitted for <a> elements
    MaxBlankLines     int                  // if > 0, collapses runs of blank lines to at most this many
    StripHiddenInline bool                 // if true, elements hidden via their own inline style= are removed before rendering
}
```

For untrusted documents, consider `IgnoreDocumentCSS: true` when document CSS is
not required and `NoOSC8Links: true` when terminal hyperlinks are not required.

### Stripping hidden elements

HTML email frequently hides preheader text and tracking pixels with inline
`style=""` attributes, e.g. `<span style="display:none;max-height:0;overflow:hidden;">`.
These are invisible in a real mail client but, since `IgnoreDocumentCSS` discards
`style=""` along with `<style>` blocks, would otherwise render as visible text.

Setting `StripHiddenInline: true` removes any element (and its children) whose
own inline `style=""` attribute matches one of these high-confidence patterns:

- `display: none`
- `visibility: hidden` or `visibility: collapse`
- `opacity: 0`
- `height: 0` or `max-height: 0` combined with `overflow: hidden`/`clip`

This only inspects each node's own `style=""` attribute. It does not run
selector matching, so elements hidden by a CSS class via a `<style>` rule are
not affected. It is independent of `IgnoreDocumentCSS` and safe to combine
with either setting.

## CSS Precedence

Cascade order (lowest to highest priority): built-in default stylesheet
(`htmlterm.DefaultStylesheet`) → `Options.CSS` → `Options.Stylesheets` (in
order) → `<style>` elements → inline `style=""` attributes. The last two are
suppressed by `Options.IgnoreDocumentCSS`.

`Options.Stylesheets` exists for callers assembling several independent
stylesheets (e.g. loaded from separate files or `go:embed` entries) that
should cascade in a fixed order, the way a page's own stylesheet is followed
by however many `<link>` sheets it loads. `Options.CSS` remains the place
for a single combined stylesheet. `htmlterm.DefaultStylesheet` is exported so
callers can inspect or diff their own stylesheet against the built-in
baseline.

## Interactive documents

Beyond one-shot `Renderer.Render`, `htmlterm` has a mutable `Document`/`Element`
API and an event system modeled on the browser DOM, with no scripting engine, just
native Go closures registered the way `addEventListener` would be:

```go
doc, err := htmlterm.ParseDocument(`
	<form id="f">
	  <label>Name: <input id="name"></label>
	  <button type="submit">Submit</button>
	</form>`, htmlterm.Options{Width: 40})

name := doc.GetElementByID("name")
name.Focus()

doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *htmlterm.Event) {
	fmt.Println("submitted:", name.Value())
})

doc.DispatchKey("h", document.Modifiers{})
doc.DispatchKey("i", document.Modifiers{})
doc.DispatchKey("Enter", document.Modifiers{}) // fires "submit": Enter on a focused text field is an implicit submit

out, _ := doc.Render()
fmt.Print(out)
```

- **`Document`/`Element`** (`ParseDocument`, `GetElementByID`, `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove, `ClassList`, `Value`/`SetValue`, `Checked`/`SetChecked`) let you parse once, then mutate and re-render repeatedly, instead of `Renderer.Render`'s parse-once-discard model.
- **Events.** `Document.AddEventListener(el, type, capture, fn)` / `RemoveEventListener`, with `Event.StopPropagation`/`StopImmediatePropagation`/`PreventDefault`/`DefaultPrevented`, dispatched through capture → target → bubble phases; `Event` also carries `ShiftKey`/`CtrlKey`/`AltKey`/`MetaKey`. `Document.DispatchClick(row, col, mods)` and `DispatchKey(key, mods)` hit-test/route input and run built-in default actions (checkbox/radio toggle, focus traversal, text entry, implicit form submit on Enter).
- **Focus and hit-testing.** `Element.Focus`/`Blur` (plus `Document.FocusNext`/`FocusPrev`/`FocusedElement`, which need whole-document state) manage a `:focus`-matching focused element; `Element.Rect()` returns an element's on-screen position and size (the CSS border box), recorded for free as a byproduct of rendering, which is useful for translating real mouse coordinates into `DispatchClick` calls.
- **Form controls.** `<input>` (text, checkbox, radio, submit/button/reset, hidden), `<button>`, `<textarea>`, `<form>`/`<fieldset>`/`<legend>` render with sensible terminal approximations (`[value]`, `☐`/`☑`, `○`/`●`, `[ Label ]`) driven entirely by attributes, so `Element.SetValue`/`SetChecked` are reflected on the next `Render()`. `<select>` renders as a bracketed control that opens a composited dropdown popup on a live `Document`; see [docs/SELECT.md](./docs/SELECT.md) for how it's styled.
- **`Loop`** (`NewLoop`, `Loop.Run`) drives a `Document` against a real terminal: raw mode, SGR mouse reporting, keyboard/mouse decoding into `DispatchClick`/`DispatchKey` calls, and repaint after every event, timer fire, or terminal resize. `Loop.SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout` mirror `window.setInterval`/`setTimeout` for periodic updates (a spinner, a clock) that aren't triggered by user input at all. See [`cmd/htmlterm-tui`](./cmd/htmlterm-tui) for a complete runnable example (form + spinner + clock + a long paragraph that reflows live as you resize the terminal), and `docs/INTERACTIVE.md`/`docs/REPAINT.md` for the full design history and remaining gaps.

### Sizing and resize

`Options.Width`/`Options.Height` accept either a concrete count or one of two sentinels:

- `htmlterm.SizeNatural` (`-1`, `Height` only) doesn't constrain height at all: the document renders at whatever line count its content needs, with no clipping or padding. There's no equivalent for `Width`: wrapping always needs a concrete column count.
- `htmlterm.SizeAutomatic` (`0`, the zero value for both fields) tracks the terminal's current size for that dimension. Plain `Renderer`/`Document` usage has no terminal to query, so it's inert there (behaves like `SizeNatural`); `Loop` is what actually resolves it, once at startup and again on every resize, via `Document.SetSize`.

A `Document` driven by `Loop` with `Width: htmlterm.SizeAutomatic` therefore always renders at the terminal's current column count, live: resizing the terminal sends `SIGWINCH`, which `Loop.Run` catches, re-queries the size, calls `Document.SetSize`, dispatches a `"resize"` event on `doc.DocumentElement()` (there's no separate window-level concept in this package, so the document root doubles as that event's target), and repaints. A host with no automatic axis still receives `"resize"`, which is useful for a host doing its own multi-pane layout (several `Document`s composited outside of any single `Loop`) that wants to react to a physical resize itself, e.g. by calling `Document.SetSize` on each pane with a recomputed size:

```go
doc, _ := htmlterm.ParseDocument(html, htmlterm.Options{
    Width:  htmlterm.SizeAutomatic, // track terminal width live
    Height: htmlterm.SizeNatural,   // never clip/pad height
})
loop := htmlterm.NewLoop(doc, os.Stdin, os.Stdout)
doc.AddEventListener(doc.DocumentElement(), "resize", false, func(e *htmlterm.Event) {
    w, h := doc.Size() // the size Loop just resolved and installed
    log.Printf("resized to %dx%d", w, h)
})
loop.Run()
```

`Document.SetSize(width, height)`/`Size() (width, height int)` also work outside `Loop`, for a host driving its own resize logic. `Options.Height`'s clip/pad is a plain viewport constraint, not CSS: unlike a per-element `height` (which only truncates when paired with `overflow: hidden`/`clip`), a positive `Height` always both pads short content with blank lines and truncates tall content, the behavior a non-scrolling terminal viewport needs. `SIGWINCH` handling ties this to POSIX-like platforms; see [Notes](#notes).

## CLI

The repository also includes a small CLI in [`cmd/`](./cmd):

```bash
go build -o htmlterm ./cmd/
./htmlterm -css styles.css input.html
```

If no input file is given, the CLI reads HTML from stdin. If `-width` is omitted, it auto-detects terminal width and falls back to `80`.

Use `-strip-hidden-inline` to remove elements hidden via their own inline `style=""` attribute (see [Stripping hidden elements](#stripping-hidden-elements) above):

```bash
./htmlterm -ignore-document-css -strip-hidden-inline input.html
```

Use `-dump-html` to parse input HTML and write the normalized tree instead of terminal-rendered output:

```bash
./htmlterm -dump-html input.html
```

## Development

```bash
make build    # go build ./...
make test     # go test ./...
make race     # go test -race ./...
make lint     # gofmt check + golangci-lint
make fmt      # gofmt -w + go mod tidy
make cover    # coverage report (cover.out)
make bench    # benchmarks
make vuln     # govulncheck ./...
```

## Notes

- **Not verified on Windows.** The interactive layer (`Document`/`Element`/events/`Loop`) is POSIX-oriented (raw terminal mode via `golang.org/x/term`). `Loop`'s automatic resize tracking specifically requires `syscall.SIGWINCH`, which doesn't exist on Windows at all, a compile-time constraint there, not just an unverified one.
- Unsupported HTML and CSS are ignored rather than treated as errors, matching how a real browser handles unknown markup.
