# htmlterm

[![Go](https://github.com/client9/htmlterm/actions/workflows/go.yml/badge.svg)](https://github.com/client9/htmlterm/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/client9/htmlterm.svg)](https://pkg.go.dev/github.com/client9/htmlterm)

`htmlterm` is a Go module that renders a restricted subset of HTML and CSS to ANSI-styled terminal strings.

It is designed for terminal UIs, CLIs, and text-first applications that want richer formatting than plain text without embedding a browser engine.

## Features

- Render HTML fragments or full documents to ANSI-styled terminal output.
- Apply CSS from the renderer, `<style>` tags, and inline `style=""` attributes.
- Support common block and inline elements including headings, paragraphs, lists, blockquotes, links, tables, and forms.
- Support a focused CSS subset including selectors, inheritance, margins, padding, borders, width, wrapping, overflow, and text transforms.
- Emit OSC 8 hyperlinks for `<a href="...">...</a>` when supported by the terminal.
- A mutable `Document`/`Element` API for hosts that want to query, mutate, and re-render a tree instead of parsing once and discarding it.
- Native Go DOM-style events (`AddEventListener`, capture/target/bubble phases), focus management, and hit-testing (`Document.Rect`) — no scripting engine, just Go closures.
- A `Loop` that drives a `Document` against a real terminal: raw-mode keyboard/mouse input, `SetInterval`/`SetTimeout` timers, and a render loop — enough to build a small interactive TUI.

## Scope

`htmlterm` is intentionally not a browser. It supports a documented subset of HTML and CSS and silently ignores unsupported features.
When rendering untrusted HTML or CSS, see [SECURITY.md](./SECURITY.md) for the terminal-output security model and recommended defense-in-depth settings.

See [CSS.md](./CSS.md) for the full supported surface:

- **Selectors:** universal (`*`), element, class, multiple classes, ID, attribute operators, descendant, child (`>`), adjacent sibling (`+`), `:root`, `:first-child`, `:last-child`, `:nth-child(odd|even)`, `:not(...)`, `::before`, `::after`
- **Layout and styling:** `display`, margins, padding, width, height, borders, colors, `white-space`, `overflow`, `text-overflow`, `text-align`, `text-transform`, `visibility`
- **Tables:** column sizing, wrapping, alignment, border styles, `<colgroup>` / `<col>`

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

- `htmlterm.New(opts Options) (*Renderer, error)` — create a renderer from an `Options` struct
- `(*Renderer).Render(html string) (string, error)` — render an HTML fragment or document to an ANSI-styled string

```go
type Options struct {
    CSS               string               // additional stylesheet layered above built-in UA defaults
    Width             int                  // terminal column count; affects wrapping, tables, percentage widths
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

This only inspects each node's own `style=""` attribute — it does not run
selector matching, so elements hidden by a CSS class via a `<style>` rule are
not affected. It is independent of `IgnoreDocumentCSS` and safe to combine
with either setting.

## CSS Precedence

Styles are applied in this order, lowest to highest priority:

1. Built-in user-agent stylesheet
2. `Options.CSS`
3. `<style>` elements in the HTML
4. Inline `style=""` attributes

Steps 3 and 4 are both suppressed by `Options.IgnoreDocumentCSS`.

Higher specificity wins within a given layer; later rules win on ties.

## Interactive documents

Beyond one-shot `Renderer.Render`, `htmlterm` has a mutable `Document`/`Element`
API and an event system modeled on the browser DOM — no scripting engine, just
native Go closures registered the way `addEventListener` would be:

```go
doc, err := htmlterm.ParseDocument(`
	<form id="f">
	  <label>Name: <input id="name"></label>
	  <button type="submit">Submit</button>
	</form>`, htmlterm.Options{Width: 40})

name := doc.GetElementByID("name")
doc.Focus(name)

doc.AddEventListener(doc.GetElementByID("f"), "submit", false, func(e *htmlterm.Event) {
	fmt.Println("submitted:", name.Value())
})

doc.DispatchKey("h")
doc.DispatchKey("i")
doc.DispatchKey("Enter") // fires "submit" — Enter on a focused text field is an implicit submit

out, _ := doc.Render()
fmt.Print(out)
```

- **`Document`/`Element`** (`ParseDocument`, `GetElementByID`, `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove, `ClassList`, `Value`/`SetValue`, `Checked`/`SetChecked`) — parse once, mutate and re-render repeatedly, instead of `Renderer.Render`'s parse-once-discard model.
- **Events** — `Document.AddEventListener(el, type, capture, fn)` / `RemoveEventListener`, with `Event.StopPropagation`/`StopImmediatePropagation`/`PreventDefault`/`DefaultPrevented`, dispatched through capture → target → bubble phases. `Document.DispatchClick(row, col)` and `DispatchKey(key)` hit-test/route input and run built-in default actions (checkbox/radio toggle, focus traversal, text entry, implicit form submit on Enter).
- **Focus and hit-testing** — `Document.Focus`/`Blur`/`FocusNext`/`FocusPrev` manage a `:focus`-matching focused element; `Document.Rect(el)` returns an element's on-screen position and size (the CSS border box), recorded for free as a byproduct of rendering — useful for translating real mouse coordinates into `DispatchClick` calls.
- **Form controls** — `<input>` (text, checkbox, radio, submit/button/reset, hidden), `<button>`, `<textarea>`, `<form>`/`<fieldset>`/`<legend>` render with sensible terminal approximations (`[value]`, `☐`/`☑`, `○`/`●`, `[ Label ]`) driven entirely by attributes, so `Element.SetValue`/`SetChecked` are reflected on the next `Render()`. `<select>` is not yet supported (no dropdown-rendering concept exists).
- **`Loop`** (`NewLoop`, `Loop.Run`) drives a `Document` against a real terminal: raw mode, SGR mouse reporting, keyboard/mouse decoding into `DispatchClick`/`DispatchKey` calls, and repaint after every event. `Loop.SetInterval`/`SetTimeout`/`ClearInterval`/`ClearTimeout` mirror `window.setInterval`/`setTimeout` for periodic updates (a spinner, a clock) that aren't triggered by user input at all. See [`cmd/htmlterm-tui`](./cmd/htmlterm-tui) for a complete runnable example (form + spinner + clock), and `INTERACTIVE.md`/`REPAINT.md` for the full design history, including known gaps (no `<select>` dropdown, no terminal resize handling, no line-level diff repaint yet — every paint currently redraws the whole frame).

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

- Unsupported HTML and CSS are ignored rather than treated as errors.
- Table cells default to `white-space: nowrap` and `text-overflow: ellipsis`.
- Blockquote, emphasis, strong text, links, and several semantic HTML elements have built-in default styling.
- The interactive layer (`Document`/`Element`/events/`Loop`) is POSIX-oriented (raw terminal mode via `golang.org/x/term`) and hasn't been verified on Windows.
