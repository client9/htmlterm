# htmlterm

`htmlterm` is a Go module that renders a restricted subset of HTML and CSS to terminal strings using [lipgloss](https://github.com/charmbracelet/lipgloss).

It is designed for terminal UIs, CLIs, and text-first applications that want richer formatting than plain text without embedding a browser engine.

## Features

- Render HTML fragments or full documents to ANSI-styled terminal output.
- Apply CSS from the renderer, `<style>` tags, and inline `style=""` attributes.
- Support common block and inline elements including headings, paragraphs, lists, blockquotes, links, and tables.
- Support a focused CSS subset including selectors, inheritance, margins, padding, borders, width, wrapping, overflow, and text transforms.
- Emit OSC 8 hyperlinks for `<a href="...">...</a>` when supported by the terminal.

## Scope

`htmlterm` is intentionally not a browser. It supports a documented subset of HTML and CSS and silently ignores unsupported features.

See [CSS.md](./CSS.md) for the full supported surface:

- Selectors: element, class, multiple classes, ID, attributes, descendant, child (`>`), adjacent sibling (`+`), `:first-child`, `:last-child`, `:nth-child(odd|even)`, `:not(...)`, `::before`, `::after`
- Layout and styling: `display`, margins, padding, width, height, borders, colors, `white-space`, `overflow`, `text-overflow`, `text-align`, `text-transform`, `visibility`
- Tables: column sizing, wrapping, alignment, border styles, `<colgroup>` / `<col>`

## Install

```bash
go get github.com/nickg/htmlterm
```

## Library Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/nickg/htmlterm"
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

	r, err := htmlterm.New(css, 40)
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

- `htmlterm.New(css string, width int) (*Renderer, error)`
- `(*Renderer).Render(html string) (string, error)`

`width` is the available terminal width in columns. It affects wrapping, percentage widths, borders, and table layout.

## CSS Precedence

Styles are applied in this order, lowest to highest priority:

1. Built-in user-agent stylesheet
2. CSS passed to `htmlterm.New`
3. `<style>` tags in the HTML
4. Inline `style=""` attributes

Higher specificity still wins within a given layer, and later rules win on ties.

## CLI

The repository also includes a small CLI in [`cmd/`](./cmd):

```bash
go build -o htmlterm ./cmd
./htmlterm -css styles.css input.html
```

If no input file is given, the CLI reads HTML from stdin. If `-width` is omitted, it auto-detects terminal width and falls back to `80`.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

## Notes

- Unsupported HTML and CSS are ignored rather than treated as errors.
- Table cells default to `white-space: nowrap` and `text-overflow: ellipsis`.
- Blockquote, emphasis, strong text, links, and several semantic HTML elements have built-in default styling.
