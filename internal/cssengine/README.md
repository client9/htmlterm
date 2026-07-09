# cssengine

`internal/cssengine` owns `htmlterm`'s CSS parsing, selector matching, and cascade resolution.

It is internal deliberately: the boundary keeps CSS/selector machinery out of the renderer and DOM packages without committing `htmlterm` to a public CSS engine API. Only `internal/render` imports it.

## What it does

- **Parses stylesheets and inline styles** (`css.go`) — `ParseStylesheet` turns CSS source into an ordered list of `Rule`s (selector group + declarations + specificity + source order); `ParseDeclarations` parses a bare declaration block, the form used for `<style>` attributes.
- **Parses and matches selectors** (`selector.go`) — `ParseSelectorGroup`/`SelectorGroup.Match` support the selector subset documented in [`CSS.md`](../../CSS.md): universal (`*`), element, class, multiple classes, ID, attribute operators, descendant, child (`>`), adjacent sibling (`+`), `:root`, `:first-child`, `:last-child`, `:nth-child(odd|even)`, `:not(...)`, and the state-driven pseudo-classes (`:focus`, `:checked`, `:disabled`, `:required`) the `document` package's live event/focus model needs.
- **Resolves the cascade** (`cascade.go`) — `ExtractStyleRules` pulls rules out of `<style>` elements in a parsed document; `Cascade.Resolve` computes an element's final property map by applying inheritance, specificity, and source order across all applicable rules; `Cascade.Direct` returns only the declarations that apply to a node directly (no inheritance), and `Cascade.PseudoElement` resolves `::before`/`::after` declarations.

## Key types

```go
type Rule struct { /* selector group, declarations, specificity, source order */ }

func ParseStylesheet(src string) ([]Rule, error)
func ParseDeclarations(src string) map[string]string

type SelectorGroup struct { /* ... */ }
func ParseSelectorGroup(sel string) SelectorGroup
func (g SelectorGroup) Match(n *html.Node, focusAttr string) bool

type Cascade struct { /* ordered rules */ }
func (c Cascade) Resolve(n *html.Node) map[string]string
func (c Cascade) Direct(n *html.Node) map[string]string
func (c Cascade) PseudoElement(n *html.Node, which string) map[string]string
```

`SelectorGroup.Match`'s `focusAttr` parameter is the reserved marker attribute the `document` package uses to track the currently focused node (see `document`'s `focusAttr`), so `:focus` matching doesn't require this package to know anything about the DOM/event layer above it.

## Testing

`cssengine_internal_test.go` covers parsing, selector matching, and cascade resolution from inside the package. Higher-level behavior (how cascade output turns into terminal styling) is covered by `internal/render`'s and the root package's tests instead.

## See also

- [`CSS.md`](../../CSS.md) — the full supported CSS surface as seen by users of `htmlterm`.
- [`internal/render`](../render) — the only consumer of this package; turns resolved cascade output into layout and ANSI styling.
