# Styling Scrollbars

A scrollbar gutter appears when `overflow-y: scroll` is set on an element
with an explicit `height` (`overflow-y: auto` scrolls too, but draws no
visible indicator at all — see `docs/SCROLLING.md` for why). This page covers
everything about styling the gutter itself once you have one; CSS.md's own
`overflow`/`overflow-y` entry is where the scrolling behavior (not the
styling) is documented, with a one-line pointer here.

## Quick reference

Five pseudo-elements, all matched against the scrollable element itself
(`.log-pane::scrollbar-thumb { … }`, or left bare — e.g. a top-level
`::scrollbar-thumb { … }` — to apply to every scrollable element):

| Pseudo-element | Supported properties | Default |
|---|---|---|
| `::scrollbar` | `width` (see CSS.md's [Size Values](../CSS.md#size-values); percentages are ignored) | `width: 1ch` (UA stylesheet) |
| `::scrollbar-track` | `content`, `color`, `background-color`, `font-weight` | `content: "│"` (the `block` `scrollbar-style` preset) |
| `::scrollbar-thumb` | `content`, `color`, `background-color`, `font-weight` | `content: "█"` (same) |
| `::scrollbar-cap-start` | `content`, `color`, `background-color`, `font-weight` | `content: "▲"` (the `block` preset) |
| `::scrollbar-cap-end` | `content`, `color`, `background-color`, `font-weight` | `content: "▼"` (same) |

```css
::scrollbar       { width: 1ch; }
::scrollbar-track { content: "│"; color: gray; background: transparent; font-weight: normal; }
::scrollbar-thumb { content: "█"; color: white; background: blue; font-weight: bold; }
```

Scoping to one pane:

```css
#log::scrollbar-thumb { content: "▓"; color: #ff9e64; }
```

`content` takes the same quoted-string form `::before`/`::after` accept and
is expected to resolve to exactly one character per reserved column — there
is no re-wrap pass to correct a too-wide glyph, so a multi-column `content`
value will misalign the gutter. When `::scrollbar { width }` reserves more
than one column, the resolved track/thumb glyph is repeated across every
reserved column, not spread across them individually.

## `scrollbar-style` presets

Shorthand set on the *scrollable* element (not on `::scrollbar-track`/
`::scrollbar-thumb`/`::scrollbar-cap-*` themselves) that picks a built-in
track/thumb/cap glyph (and, for `classic`, background color) preset, without
writing out every pseudo-element rule by hand:

| Value | Track | Thumb | Cap start | Cap end |
|---|---|---|---|---|
| `block` (default) | `"│"` | `"█"` | `"▲"` | `"▼"` |
| `shaded` | `"░"` | `"█"` | `"▲"` | `"▼"` |
| `classic` | `" "` on `background-color: #444444` | `" "` on `background-color: #aaaaaa` | `"▲"` on `background-color: #444444`, `color: #ffffff` | `"▼"` (same colors) |
| `ascii` | `"\|"` | `"#"` | `"^"` | `"v"` |
| `line` | `"│"` | `"┃"` | `"▲"` | `"▼"` |

`ascii` uses only plain 7-bit ASCII characters — no box-drawing or block
glyphs — for terminals/fonts with unreliable Unicode rendering. `line` is
`block` with a thinner thumb — a bold vertical line (`"┃"`) against the same
thin track (`"│"`) rather than `block`'s full solid `"█"` thumb.

An unrecognized or unset value falls back to `block`. Any property an
`::scrollbar-track`/`::scrollbar-thumb`/`::scrollbar-cap-start`/
`::scrollbar-cap-end` rule sets directly still overrides just that one
property from the preset — the preset only fills in whatever the rule
doesn't mention:

```css
/* shaded preset's track glyph ("░"), but a custom thumb color on top */
#log { scrollbar-style: shaded; }
#log::scrollbar-thumb { color: #ff9e64; }
```

Not inherited (matches `overflow`'s own non-inherited treatment — a
`scrollbar-style` set on a non-scrollable ancestor has no scroll box of its
own to apply to).

## Cap buttons: `::scrollbar-cap-start` / `::scrollbar-cap-end`

Arrow-button cells at the very top (`start`) and bottom (`end`) of the
gutter — named `start`/`end` rather than `top`/`bottom` to stay meaningful if
a horizontal scrollbar is added later, matching WebKit's own
`::-webkit-scrollbar-button:start`/`:end` convention. **On by default
(opt-out)**: every `scrollbar-style` preset supplies an arrow glyph for both
ends, so `overflow-y: scroll` alone is enough to get cap buttons.

```css
#log::scrollbar-cap-start { content: "▲"; color: #ff9e64; }
#log::scrollbar-cap-end   { content: "▼"; color: #ff9e64; }
```

Opt back out per element with `content: none` (the same convention
`::before`/`::after` already use to suppress injection):

```css
#log::scrollbar-cap-start { content: none; }
#log::scrollbar-cap-end   { content: none; }
```

When active, a cap claims its end's row entirely (the track/thumb never
draws there), and the thumb's size/position is computed against the
remaining interior rows so it never overlaps a cap. If the gutter is too
short to spare a row per active cap, both caps are dropped for that render
(not just the one that doesn't fit) and ordinary track/thumb rendering
applies for the whole column — the same "silently drop the added chrome
when there's no room" behavior the gutter reservation itself already has.
This also means a short gutter (e.g. `height: 2`) shows plain track/thumb
with no caps, even without an explicit `content: none` — there's simply no
room.

**Clickable**, on a live `Document`: clicking a cap's cell scrolls its pane
by one line (matching `ArrowUp`/`ArrowDown`'s own step), the same way a real
scrollbar arrow button does. Not meaningful against `Renderer.Render`'s
one-shot rendering, only against a live `Document` (same restriction
`:focus` and `Element.Focus` already have). A click on a cap does not
dispatch a `"click"` `Event` on the scrollable element — a cap is rendering
chrome, not real element content, so it mirrors `Document.DispatchWheel`'s
direct-scroll behavior rather than an ordinary element click.

## Not supported

- Horizontal scrollbars — only `overflow-y` gets a gutter; there's no
  `overflow-x` equivalent.
- A `content` value wider than one character per reserved column — no
  re-wrap pass corrects a too-wide glyph, so it will misalign the gutter.
- Per-cell blending/transparency between the gutter and the pane's own
  content — the gutter is an opaque appended column, same "no per-cell
  compositing" decision the popup overlay mechanism makes (see
  `docs/RENDERING.md`).

## See also

- `docs/SCROLLING.md` — the scrolling design itself: why `auto` gets no
  visible indicator, the scroll-offset/viewport model, wheel/key/focus
  interactions.
- CSS.md's `overflow`/`overflow-x`/`overflow-y` entry — what makes an
  element scrollable in the first place, before any of this styling
  applies.
