# Styling `<progress>` and `<meter>`

Both elements render as a fixed-width, one-line bar of two repeated glyphs —
a filled portion and a track (unfilled) portion — synthesized from
attributes, the same way `<input>`'s display text is synthesized. Neither
element has any interactive state: no focus behavior beyond what any other
inline-block element already gets, no click/keyboard default action, no
popup (unlike `<select>`).

## Quick reference

```css
progress, meter { display: inline-block; width: 20; height: 1; }
```

`width` is the ordinary CSS `width` property (see CSS.md's [Size
Values](../CSS.md#size-values); `ch`/bare-integer/`%` all work, same as any
other inline-block element). `20` is the UA stylesheet default — real CSS's
own default (`≈160px`) has no exact terminal equivalent; 20 columns gives a
meaningful 5%-step bar without dominating a line. `height` is always
effectively `1` — neither element has a vertical orientation (see
Non-goals).

**`color`/`background-color`/etc. set directly on `<progress>`/`<meter>`
have no visual effect** — the bar's every glyph is styled entirely through
`::progress-bar`/`::progress-value`/`::meter-*-value` (see below), the same
way `::scrollbar-thumb` never inherits its scrollable host's `color`
either. Use `progress::progress-value { color: green; }`, not `progress {
color: green; }`.

### `<progress value max>`

`value`/`max` (`max` defaults to `1`) resolve a completion fraction in
`[0,1]`, rounded to the nearest whole column. An absent `value` attribute is
the *only* indeterminate trigger (an explicitly present but unparseable
value is just clamped to `0` like any other bad numeric attribute). A
non-positive or unparseable `max` falls back to `1`.

```css
progress::progress-bar   { content: "░"; }  /* track (unfilled) portion */
progress::progress-value { content: "█"; }  /* filled portion */
```

| Pseudo-element | Supported properties | Default |
|---|---|---|
| `::progress-bar` | `content`, `color`, `background-color`, `font-weight` | `content: "░"` (the `block` `progress-style` preset) |
| `::progress-value` | `content`, `color`, `background-color`, `font-weight` | `content: "█"` (same) |

### `<meter value min max low high optimum>`

A scalar measurement within a range (disk usage, a score) — not task
completion; don't use `<meter>` for progress. Defaults: `min` → `0`, `max` →
`1`, `value` → `0`, `low` → `min`, `high` → `max`, `optimum` → the midpoint
`(min + max) / 2`. Resolution order (matching real UAs' "candidate value"
algorithm): `max` is forced up to `min` if given as less than `min`; `value`,
`low`, and `high` are each clamped into the resulting `[min,max]` (`high` is
additionally floored at `low`); `optimum` is clamped into `[min,max]` last.
If `max == min` after clamping (a zero-width range), the bar reports fully
filled when `value >= min`, empty otherwise.

`<meter>` gets three value pseudo-elements instead of `<progress>`'s one
because real UAs already color-code by region — collapsing them to a single
glyph would lose that distinction:

| Pseudo-element | Supported properties | Default |
|---|---|---|
| `::meter-bar` | `content`, `color`, `background-color`, `font-weight` | `content: "░"` |
| `::meter-optimum-value` | `content`, `color`, `background-color`, `font-weight` | `content: "█"` on `color: #2e7d32` |
| `::meter-suboptimum-value` | `content`, `color`, `background-color`, `font-weight` | `content: "█"` on `color: #f9a825` |
| `::meter-even-less-good-value` | `content`, `color`, `background-color`, `font-weight` | `content: "█"` on `color: #c62828` |

#### Region algorithm

Which region the current `value` falls in is a 3-case algorithm, not a
single formula:

1. **`optimum` is inside `[low, high]`** (inclusive both ends): `[low,
   high]` is the *optimum* region, and **both** outer ranges (`[min, low)`
   and `(high, max]`) are *suboptimum* — there is no even-less-good region
   at all in this case.
2. **`optimum < low`** (lower is better): `[min, low]` is *optimum*, `[low,
   high]` is *suboptimum*, `(high, max]` is *even-less-good*.
3. **`optimum > high`** (higher is better): `[high, max]` is *optimum*,
   `[low, high]` is *suboptimum*, `[min, low)` is *even-less-good*.

## `progress-style` / `meter-style` presets

Shorthand set on the `<progress>`/`<meter>` element itself (not on the
pseudo-elements), reusing the exact same five preset names
`scrollbar-style` already has:

| Preset | `::progress-bar`/`::meter-bar` | `::progress-value` | `::meter-optimum-value` | `::meter-suboptimum-value` | `::meter-even-less-good-value` |
|---|---|---|---|---|---|
| `block` (default) | `"░"` | `"█"` | `"█"` on `color: #2e7d32` | `"█"` on `color: #f9a825` | `"█"` on `color: #c62828` |
| `shaded` | `"░"` | `"▓"` | `"▓"` (same colors) | | |
| `classic` | `" "` on `background-color: #444444` | `" "` on `background-color: #aaaaaa` | `" "` on `background-color: #2e7d32` | `" "` on `background-color: #f9a825` | `" "` on `background-color: #c62828` |
| `ascii` | `"-"` | `"#"` | `"#"` (same colors) | | |
| `line` | `"─"` | `"━"` | `"━"` (same colors) | | |

An element's own `::progress-*`/`::meter-*` rule still overrides
property-by-property on top of the chosen preset — the same merge contract
`scrollbar-style` uses: an explicit rule always wins per-property, anything
it doesn't mention falls through to the preset.

```css
progress { progress-style: ascii; }
meter    { meter-style: classic; }
```

## Indeterminate `<progress>`

Real browsers render a value-less `<progress>` as an animated barber-pole;
this renderer has no `animation` support at all (a documented
`COMPATIBILITY.md` gap), so indeterminate rendering is **static, not
animated** — the one deliberate deviation from spec here. Real spec's own
`:indeterminate` pseudo-class is supported, matched against `<progress>`
with no `value` attribute:

```css
progress:indeterminate { color: gray; }
progress:indeterminate::progress-bar { content: "?"; }
```

Default rendering when indeterminate and no author override: the whole bar
is `::progress-bar`'s own track glyph repeated across the full width — no
`::progress-value` glyph anywhere, since there's no fraction to size a fill
from.

`:indeterminate` does not extend to `<input type=checkbox>`/`<input
type=radio>` here — this renderer's checkboxes are attribute-driven only
(`checked` is the sole source of truth), with no separate JS-settable
`indeterminate` property to reflect. Only the `<progress>` slice of
`:indeterminate` is supported.

## Non-goals

- **No vertical orientation.** Neither HTML nor CSS Basic UI define one for
  either element.
- **No built-in numeric/percentage label.** Spec defines the bar only; an
  author wanting `"70%"` next to it adds it as ordinary sibling text or an
  `::after` rule with a manually-written string.
- **No sub-character partial-fill glyphs** (Unicode eighth-block characters
  for finer-than-one-column precision) — the fraction rounds to a whole
  glyph per column.
- **No new `Document`/`Element` typed getters.** `max`/`min`/`low`/`high`/
  `optimum` are reachable via `Element.GetAttribute`/`SetAttribute`;
  `Element.Value()`/`SetValue()` already work for both elements via their
  existing generic `value`-attribute fallback.
- **No `<meter>` indeterminate state** — real spec doesn't define one.
