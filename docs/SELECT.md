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
| `<optgroup>` grouping | n/a | ✅ — label header row + indented options (see below) | n/a |
| `<optgroup label>` styling | n/a | ✅ via an `optgroup` selector (see below) | n/a |
| `<optgroup disabled>` | n/a | n/a | ✅ — cascades to every option inside it |

The "natural label-driven width" is measured in terminal **columns**, not
runes, so a double-width label (CJK, emoji) sizes the popup to the room it
actually needs; every row is then padded to that same column width, which is
what keeps the highlight bar rectangular rather than ragged.

## Closed control

```html
<select style="background-color: #222; color: white; border: solid; padding-left: 1">
  <option value="a">Apple</option>
  <option value="b" selected>Banana</option>
</select>
```

Renders like any other `display: inline-block` element (the UA stylesheet
sets this): `[ Banana ▾]` bracketed with a disclosure indicator, styled by
whatever box properties you set. Nothing special here: if you already know
how `border`, `padding`, `margin`, and `width` work elsewhere in htmlterm,
they work the same way on a closed `<select>`.

## Open popup

Opening the popup, by click or by Enter or Space while focused, and only on a
live `Document` rather than plain `Renderer.Render`, composites a floating
list of `<option>`s directly beneath the closed control. It picks up CSS from
two places:

- **The `<select>` itself** supplies the popup's own box. `background-color`
  and `color` become every row's default style, and `border`, `padding`,
  `margin-left`, `margin-top`, and `width` shape the popup as a whole: one
  border and one padding band around the entire list, not per row.
- **Each `<option>`** can override `background-color` and `color` for just its
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
popup is open. That is a separate, browse-first-then-confirm state from
`selected`: arrowing through options doesn't commit a value until you click an
option or press Enter. `option:hover` is the only place `:hover` means anything in
htmlterm; it has no effect on any other element.

`option[selected]` needs no special CSS support. It's an ordinary attribute
selector, so `option[selected] { ... }` already works.

### `<optgroup>`

```html
<select>
  <option value="a">Apple</option>
  <optgroup label="Citrus">
    <option value="b">Banana</option>
    <option value="c">Cherry</option>
  </optgroup>
</select>
```

An `<optgroup>`'s direct `<option>` children (one level of nesting only, same
as real HTML) are included in the popup, indented two columns so they read as
grouped under their header. The `label` attribute (if non-empty) renders as
its own non-selectable, non-clickable header row above them. Arrow-key
navigation skips over it entirely, there being nothing to highlight, and it
carries no synthetic `Rect`, so `DispatchClick` and hit-testing can never land
on one. An `<optgroup>` with no `label` still groups and indents its options,
just without a header row.

Real CSS barely styles `<optgroup>` at all (browsers just bold+indent the
label via their UA stylesheet, with no author-facing hook). htmlterm
repurposes a plain `optgroup` selector as its own styling hook instead, the
same kind of terminal-native convention `option:hover` already is:

```css
optgroup { color: gray; }
```

`optgroup`'s `background-color` and `color` style its label row the same way
an `option`'s do for an option row, falling back to the `<select>`'s own
resolved style for whichever it doesn't set. There is no `border`, `padding`,
or `width` support on the label row itself, the same restriction per-option
rows already have.

`<optgroup disabled>` cascades to every `<option>` inside it (matching
`HTMLOptionElement.disabled`'s real inherited-from-optgroup behavior): arrow
keys skip over every option in a disabled group, and clicking one in an open
popup is inert (the popup stays open, nothing is selected). This also makes
`option:disabled` match those options for styling purposes, not just ones
carrying their own `disabled` attribute directly.

### Fallback styling

If none of `<select>`, `<option>`, or `option:hover` sets a color anywhere in
the chain, every row falls back to the historical reverse-video styling
(white-on-black-inverted), with a `▸` marker as the only visual distinction
for the highlighted or selected row. Set `background-color` or `color`
anywhere in that chain to opt into your own styling instead.

## Not supported

- `<optgroup>` nested inside another `<optgroup>`. Real HTML doesn't allow
  this either.
- Per-option `border`, `padding`, and `width`, and the same on a group label
  row. Every row shares the popup's own content width, matching a real
  `<select>`'s uniform-width option list.
- `margin-right` and `margin-bottom` on the popup. There's nothing after a
  floating overlay for them to push against.

## See also

- `docs/RENDERING.md`'s "Popups / z-order" section, for the compositing
  mechanism: why the popup isn't a real box, how border and padding rows are
  spliced in, and how hit-testing is offset to the content sub-rectangle.
- `docs/INTERACTIVE.md`, for `Document` and `Element` event model background,
  including how `Element.Value()` and `SetValue()` map to
  `HTMLSelectElement.value`.
