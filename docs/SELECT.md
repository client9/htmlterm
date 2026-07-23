# Styling `<select>`

`<select>` behaves like a normal element for its closed state, but its open
dropdown popup is a special case: it's not a real box in the layout tree (see
`docs/RENDERING.md`'s "Popups / z-order" section for why), so it has its own
rules for which CSS properties apply and how they're resolved. This page
covers both; CSS.md's own `select` entry is a one-line pointer here.

## Quick reference

| Property | Closed control | Open popup (on `<select>`) | Per `<option>` |
|---|---|---|---|
| `background-color`, `color` | ✅ normal inline-block rules | ✅ | ✅ (falls back to the `<select>`'s own value) |
| `border`, `border-style`, `border-*-color` | ✅ | ✅ (drawn around the whole option list) | not supported |
| `padding` | ✅ | ✅ (inside the border, around the whole list) | not supported |
| `margin-left`, `margin-top` | ✅ | ✅ (shifts the popup relative to the closed control) | n/a |
| `margin-right`, `margin-bottom` | ✅ | no effect (nothing follows a floating overlay to push against) | n/a |
| `width`, `min-width`, `max-width` | ✅ | ✅ (overrides the natural label-driven width) | not supported (every row shares the popup's width) |
| `:hover` | n/a | n/a | ✅ — matches the arrow-key-highlighted row (see below), not real mouse hover |
| `[selected]` | n/a | n/a | ✅ — ordinary attribute selector, no special-casing needed |

## Closed control

```html
<select style="background-color: #222; color: white; border: solid; padding-left: 1">
  <option value="a">Apple</option>
  <option value="b" selected>Banana</option>
</select>
```

Renders like any other `display: inline-block` element (the UA stylesheet
sets this): `[ Banana ▾]` bracketed with a disclosure indicator, styled by
whatever box properties you set. Nothing special here — if you already know
how `border`/`padding`/`margin`/`width` work elsewhere in htmlterm, they work
the same way on a closed `<select>`.

## Open popup

Opening the popup (click, or Enter/Space while focused — only on a live
`Document`, not plain `Renderer.Render`) composites a floating list of
`<option>`s directly beneath the closed control. It picks up CSS from two
places:

- **The `<select>` itself** supplies the popup's own box: `background-color`/
  `color` become every row's default style, and `border`/`padding`/
  `margin-left`/`margin-top`/`width` shape the popup as a whole (one border
  and one padding band around the entire list, not per-row).
- **Each `<option>`** can override `background-color`/`color` for just its
  own row, falling back to the `<select>`'s value for whichever it doesn't
  set.

```html
<style>
  select { background-color: #003366; color: white; border: solid; padding: 1; }
  option:hover { background-color: #ffcc00; color: black; }
</style>
<select>
  <option value="a">Apple</option>
  <option value="b" selected>Banana</option>
  <option value="c">Cherry</option>
</select>
```

### The highlighted row: `option:hover`

There's no mouse hover in a terminal, so `:hover` is repurposed: it matches
whichever `<option>` is currently highlighted by ArrowUp/ArrowDown while the
popup is open (a separate, browse-first-then-confirm state from `selected` —
arrowing through options doesn't commit a value until you click an option or
press Enter). `option:hover` is the only place `:hover` means anything in
htmlterm; it has no effect on any other element.

`option[selected]` needs no special CSS support — it's an ordinary attribute
selector, so `option[selected] { ... }` already works.

### Fallback styling

If none of `<select>`, `<option>`, or `option:hover` sets a color anywhere in
the chain, every row falls back to the historical reverse-video styling
(white-on-black-inverted), with a `▸` marker as the only visual distinction
for the highlighted/selected row. Set `background-color`/`color` anywhere in
that chain to opt into your own styling instead.

## Not supported

- `<option>` elements nested inside an `<optgroup>` — only a `<select>`'s
  direct `<option>` children are read.
- Per-option `border`/`padding`/`width` — every row shares the popup's own
  content width, matching a real `<select>`'s uniform-width option list.
- `margin-right`/`margin-bottom` on the popup — there's nothing after a
  floating overlay for them to push against.

## See also

- `docs/RENDERING.md`'s "Popups / z-order" section — the compositing
  mechanism (why the popup isn't a real box, how border/padding rows are
  spliced in, how hit-testing is offset to the content sub-rectangle).
- `docs/INTERACTIVE.md` — `Document`/`Element` event model background,
  including how `Element.Value()`/`SetValue()` map to `HTMLSelectElement.value`.
