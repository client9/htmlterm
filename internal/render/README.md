# render

`internal/render` owns `htmlterm`'s HTML/CSS layout engine and render-state production: turning a parsed HTML tree plus resolved CSS into ANSI-styled terminal text, plus the position/scroll metadata the `document` package needs to support interactivity.

It is internal so the public root package can stay a small, generic renderer facade (`Options`, `Renderer`, `New`, `SizeAutomatic`, `SizeNatural`) — all of the actual layout logic lives here instead.

## Who uses this

- `github.com/client9/htmlterm` (root package) constructs an `Engine` and calls `Render` for one-shot rendering.
- `github.com/client9/htmlterm/document` calls `RenderNode` directly (not `Render`) so it can capture the `Result`'s `Positions`/`ScrollOffsets`/`ScrollViewport`/`ContentOffsets` — the data a `Document` needs for hit-testing, focus, scrolling, and cursor placement.

## Entry points

```go
type Options struct {
    CSS, Width, Height, IgnoreDocumentCSS, Profile,
    NoOSC8Links, MaxBlankLines, StripHiddenInline, FocusAttr /* ... */
}

func New(opts Options) (*Engine, error)

func (e *Engine) Render(htmlStr string) (string, error)
func (e *Engine) RenderHTML(htmlStr string) (Result, error)
func (e *Engine) RenderNode(doc *html.Node, req Request) Result
func (e *Engine) DocumentRules(doc *html.Node) []cssengine.Rule
```

- `Render`/`RenderHTML` parse HTML themselves, for callers with no need to keep the tree around (the root package's one-shot `Renderer.Render`).
- `RenderNode` takes an already-parsed tree plus a `Request` (width/height/rules/scroll offsets for this frame) and returns a `Result` carrying both the rendered `Output` string and the layout metadata (`Positions`, `ScrollOffsets`, `ScrollViewport`, `ContentOffsets`) needed by a live `Document`. This is the seam `document.Document.Render` uses to re-render the same tree repeatedly without losing element positions between frames.
- `DocumentRules` extracts `<style>`-element rules from a tree, so a caller (again, `document.Document`) can merge them with the engine's own base rules once and cache the result instead of re-parsing on every render.

## Layout and rendering

- `engine.go`, `render.go` — `Engine`/`Options`/`Viewport`/`Request`/`Result` and root-level render dispatch.
- `block.go`, `inline.go`, `list.go`, `table.go`, `table_render.go` — block, inline, list, and table layout.
- `box.go`, `wraptoken.go`, `blankcap.go` — position tracking (`Rect`, `mergePositions`), token-level word wrapping, and blank-line-run capping.
- `cascade.go`, `style.go`, `color.go`, `strip.go`, `formcontrol.go`, `counter.go`, `textutil.go` — CSS resolution on top of `cssengine` (shorthand expansion and declaration parsing itself live in `internal/cssengine/css.go`, not here), style synthesis, color parsing, hidden-inline stripping, form control rendering, list/heading counters, and text utilities.
- `attrs.go` — small attribute-lookup helpers shared across the above.

## Key invariants

These are load-bearing enough that changing them without updating every call site will produce subtly wrong output — see the repo's [`CLAUDE.md`](../../CLAUDE.md) for the full list, but the ones specific to this package:

- **Position tracking is incremental, not a final pass.** Every box-producing call returns its own local `map[*html.Node]Rect`, relative to its own origin; a parent shifts and merges a child's map (`mergePositions`) as it embeds that child, one level at a time, until the walk reaches the document root.
- **`Rect` is the CSS border box, not the margin box.** Vertical margin is injected as blank lines around a box (already excluded from `Rect`); horizontal margin is baked into a box's own lines like padding and is not currently subtracted back out.
- **A trackable element loses its position if flattened to a string before its wrapping box's `wordWrapTokens`/root dispatch call runs.** Plain inline dispatch cases must splice a child's own `[]wrapToken` rather than rendering it to a string first; `<a>` and `display:inline-block` children are the accepted exceptions.
- **Column alignment:** pad plain text to fixed width first, then apply color — color escape codes break `fmt.Printf` width specifiers.
- **Block border box model:** rendering order is `margin-left | border-left | padding-left | content | padding-right | border-right | margin-right`; `width: 100%` subtracts margins and border characters so the total visual line equals exactly the specified width.

## Testing

- `box_internal_test.go`, `wrap_token_internal_test.go`, `blank_cap_internal_test.go` — unit tests for `box.go`'s primitives, `wraptoken.go`'s token wrapping/position tracking, and `blankcap.go`'s blank-run capping/`rowRemap`.
- `helpers_internal_test.go`, `color_test.go`, `list_test.go`, `wrap_ansi_carry_internal_test.go` — counters, roman numerals, column sizing, color parsing, list rendering, and ANSI-carry-aware splitting.
- `scrollbar_internal_test.go`, `strip_internal_test.go` — `block.go`'s `appendScrollbarColumn` and `strip.go`'s hidden-inline stripping.
- Higher-level layout behavior (full HTML documents through to rendered output) is covered by the root package's black-box tests instead (`htmlterm_test.go`, `layout_test.go`, `table_test.go`, `text_test.go`, `formcontrol_test.go`), since that's the level at which "does this render correctly" is meaningful.

## See also

- [`internal/cssengine`](../cssengine) — CSS parsing, selector matching, and cascade resolution this package builds on.
- [`RENDERING.md`](../../RENDERING.md) — design history and rationale for the layout engine.
- [`CSS.md`](../../CSS.md) — the full supported CSS surface as seen by users of `htmlterm`.
