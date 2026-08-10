# Architecture

Orientation for anyone working in this repository, human or AI. It covers the
package layering, the rules that span files, and the invariants that aren't
derivable from reading any single file. Paths below are relative to the
repository root.

For what's supported see `CSS.md`; for how it differs from a browser see
`COMPATIBILITY.md`; for a package's internals see that package's `README.md`.

## Commands

```bash
go build ./...                              # build
go build -o cmd/htmlterm ./cmd/             # build htmlterm CLI
go vet ./...                                # lint
go test ./...                               # run tests
make fmt                                    # reformat (gofmt + go mod tidy)
make lint                                   # lint with golangci-lint
make cover                                  # coverage report (cover.out)
make bench                                  # benchmarks
```

## What this is

`htmlterm` is a Go module that renders HTML and CSS to styled terminal
strings. It supports a subset of both; see `COMPATIBILITY.md` for the gap
from real browser behavior.

- `github.com/client9/htmlterm` is the renderer facade only: `Options`,
  `Renderer`, `New`, `SizeAutomatic`, `SizeNatural`
- `github.com/client9/htmlterm/document` is the mutable DOM and event layer
  for interactive use
- `github.com/client9/htmlterm/tui` is the `tcell`-backed event loop and
  terminal bridge
- `internal/render` owns layout, box composition, and render-state production
- `internal/cssengine` owns CSS parsing, selector matching, and cascade
  resolution
- `internal/textcell` measures rendered text in terminal cells

The split keeps dependencies one-directional. A consumer that only wants
one-shot rendering needs neither the DOM nor `tcell`. A consumer that wants
to mutate and re-render uses `document` without the terminal loop. Each
package has its own `README.md` covering its internals; start there rather
than here.

---

## Writing style (comments, docs, and this file)

Prose here has drifted toward one habit: the em-dash appositive as the
default connective, stacked two and three deep in a single sentence. A
codebase-wide copy-edit removed ~600 of them. Don't reintroduce the pattern.

- **One idea per sentence.** If a comment needs a dash, a semicolon, and a
  parenthetical to hold itself together, it's three sentences.
- **Dashes are not the default joint.** Use a period, colon, or comma.
  Reserve a dash pair for a genuine aside you'd say in a different tone of
  voice, and at most one per paragraph.
- **Spell out slash-compounds in prose.** `foo/bar/baz` reads as "and" or
  "or" and should be written that way. Keep the slashes only when naming an
  exact value set a reader will match against CSS.md
  (`solid`/`rounded`/`heavy`), or a real path.
- **Promote asides that carry the argument.** A parenthetical holding the
  reason something works belongs in a sentence. Keep parentheses for short
  glosses and cross-references.
- **Cut intensifiers that change nothing**: "actually", "simply",
  "completely", "really", "the very first". Keep one only when it's doing
  contrastive work against a stated alternative.

Two things this does *not* mean. Don't shorten by deleting content: the long
"why" comments are the point of this codebase, and every technical claim,
example, spec citation, and documented gap stays. And don't reformat aligned
tables or term-then-gloss lists, where a dash is a column separator rather
than punctuation (see `selector.go`'s `attrOp` block, `docs/DOM_API.md`).

Rule of thumb: a comment with three dashes in it is a comment that hasn't
decided what it's saying.

---

## Cross-cutting rules

These span files, so no single file's own comments will tell you about them.

- **`tui` must not import `internal/render`.** Layout, styling, and DOM state
  stay behind `document`'s exported API. `tui` may import
  `internal/textcell`, and must: anything painting or positioning a cursor on
  top of a rendered frame has to measure columns by the exact rules layout
  used, including `RUNEWIDTH_EASTASIAN`, or the terminal desyncs by a column
  per wide character.
- **Adding an element that renders specially means touching three dispatches**,
  not one: root level (`render.go`), nested inline (`inline.go`), and flex
  item (`flex.go`'s `renderFlexItemLeaf`). Blockifying changes only an
  element's *outer* display type, so a `<table>` or `<ul>` flex item still
  needs its own renderer. Miss one and the element silently renders as its
  children.
- **A new widget with non-W3C styling gets a short `CSS.md` entry plus its
  own `docs/<WIDGET>.md`**, not a long inline entry. See `docs/SELECT.md`,
  `docs/SCROLLBARS.md`, `docs/TABLES.md`.
- **A new deviation or terminal-native feature updates two docs**: the
  detailed one (`CSS.md`, or the relevant `docs/*.md`) and
  `COMPATIBILITY.md`'s matching section. They serve different readers, not
  different facts.
- **Read the design docs before extending those surfaces.** `git log` and
  code comments do not carry the "why" the way `docs/INTERACTIVE.md`,
  `docs/RENDERING.md`, `docs/REPAINT.md`, and `docs/SCROLLING.md` do.

## Key invariants

- **Column alignment:** pad plain text to fixed width first, then apply
  color. Color escape codes break `fmt.Printf` width specifiers.
- **Rune offsets are not columns, and must not be unified.**
  `textcell.ColumnForRuneIndex` and `RuneIndexForColumn` are the only two
  places the units meet, called at the edges: placing the terminal cursor,
  turning a click into a caret. A text entry's caret and selection stay in
  rune offsets for DOM parity. See `internal/textcell/README.md`.
- **Block border box model:** rendering order is
  `margin-left | border-left | padding-left | content | padding-right | border-right | margin-right`.
  `width: 100%` subtracts margins and border chars, so the total visual line
  equals exactly the specified width.
- **`box-sizing` defaults to `border-box`, via the UA stylesheet, not a
  hardcoded default.** `width` and `height` both read it and both behave
  like `border-box` unless a selector overrides it to `content-box`, real
  CSS's own initial value. A flex item's main-axis size is the one
  exception: it always behaves like `border-box`, regardless of what the
  item's own `box-sizing` resolves to. See `COMPATIBILITY.md`.
- **Whitespace at line starts:** leading space produced by collapsing a
  source newline is trimmed when the builder is empty or the previous char
  was `\n`, matching browser `white-space: normal` behavior.
- **CSS cascade order:** UA stylesheet (`render.DefaultStylesheet`) →
  `Options.CSS` → `Options.Stylesheets` in order → `<style>` elements →
  inline `style=` attribute. The last two are skipped when
  `Options.IgnoreDocumentCSS` is set.
- **Keyword values arrive lowercased.** `cssengine`'s `keywordcase.go` folds
  declaration values once, at the end of the cascade, which is why nothing in
  `internal/render` compares case-insensitively. Strings, custom identifiers,
  custom properties, and unresolved `var(` values are exempt.
- **Position tracking is incremental, not a final pass:** every box-producing
  call returns its own local position map (`map[*html.Node]Rect`, relative to
  its own origin). A parent shifts and merges a child's map
  (`mergePositions`) as it embeds that child, one level at a time, until the
  walk reaches the document root.
- **`Rect` is the CSS border box, not the margin box.** Vertical margin is
  injected as blank lines around a box, so it's already excluded. Horizontal
  margin is baked into a box's own lines like padding and is not currently
  subtracted back out.
- **A trackable element loses its position if flattened to a string** before
  its wrapping box's `wordWrapTokens` or root dispatch call runs. That is why
  plain inline dispatch cases splice a child's own `[]wrapToken` rather than
  rendering it to a string first. `<a>` and `display:inline-block` children
  are the accepted exceptions.
- **No locking in the interactive layer:** `Loop.Run`'s main goroutine is the
  only place that ever mutates a `Document` or calls `paint()`. Timer
  callbacks and dispatched event handlers all run there too, reached only via
  `tcell.Screen`'s single `EventQ()` channel.
- **A cell-based renderer's `Show` diffing only redraws what a call touched.**
  `cellbridge.go`'s `paintLines` must explicitly blank every column past a
  line's own content, and every row past the frame's own line count, up to the
  screen's full size.

## Where to look

Use glob and grep for the file map; it changes faster than any list here can.
These are the entry points worth knowing:

| Looking for | Start at |
|---|---|
| What CSS is supported, property by property | `CSS.md` |
| The default (user-agent) stylesheet itself | `internal/render/defaultstylesheet.go` |
| What differs from a browser (not how it's implemented) | `COMPATIBILITY.md` |
| DOM/Element API against the real spec | `docs/DOM_API.md` |
| A package's internals and layering | that package's `README.md` |
| Layout, box composition, wrapping | `internal/render/` |
| Parsing, selectors, cascade | `internal/cssengine/` |
| Column measurement and ANSI slicing | `internal/textcell/` |
| Design history and rationale | `docs/INTERACTIVE.md`, `docs/RENDERING.md`, `docs/REPAINT.md`, `docs/SCROLLING.md` |
| Per-widget styling behavior | `docs/SELECT.md`, `docs/SCROLLBARS.md`, `docs/TABLES.md`, `docs/DIALOG.md` |
| Not-yet-built designs | `docs/proposals/` |
| A runnable interactive example | `cmd/htmlterm-tui/` |
