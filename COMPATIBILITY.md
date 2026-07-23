# htmlterm vs. Real CSS: Compatibility Notes

CSS.md is the exhaustive, property-by-property reference — read it when you
need the exact syntax/behavior of something. This page is the orientation
read: what's supported at a glance, what behaves differently here than in a
browser (and why — this renders to text cells, not pixels), what's
terminal-native and has no browser equivalent at all, and what simply isn't
implemented.

---

## At a Glance: What's Supported

- **Selectors:** universal, element, class (including multiple classes), ID,
  all attribute operators (`[attr]`, `=`, `~=`, `|=`, `^=`, `$=`, `*=`),
  descendant/child/adjacent-sibling/general-sibling combinators, comma
  groups, the full `:nth-*` family (`:nth-child`, `:nth-last-child`,
  `:nth-of-type`, `:nth-last-of-type`, full `An+B` syntax),
  `:first/last/only-child`, `:first/last/only-of-type`, `:empty`, `:not()`,
  `:is()`, `:where()`, `:checked`, `:disabled`, `:required`, `:focus`.
  Specificity and `!important` both work per spec.
- **Colors:** every CSS Color Level 4 format — hex (3/4/6/8-digit), named
  colors (full W3C list), `rgb()`/`rgba()`, `hsl()`/`hsla()`, `hwb()` —
  downsampled automatically to the terminal's actual color capability
  (TrueColor/256/16/none).
- **Box model:** `margin`/`padding`/`border` (shorthand and per-side
  longhands, including logical `*-block-*`/`*-inline-*` aliases),
  `width`/`min-width`/`max-width`, `height`/`min-height`/`max-height`,
  percentage and `ch` sizing, auto margins.
- **Text:** `color`, `background-color`, `font-weight`, `font-style`,
  `text-decoration`, `text-align`, `text-indent`, `text-transform` (including
  Unicode super/subscript), `font-variant: small-caps`, `white-space` (all
  five values), `overflow-wrap`/`word-break`, `tab-size`, `visibility`,
  `opacity`.
- **Generated content:** `::before`/`::after` with `content` (quoted
  strings, `attr()`, `counter()`/`counters()`, open/close-quote), CSS
  counters (`counter-reset`/`counter-increment`), `quotes`.
- **Lists:** `list-style-type` (all standard numbering systems plus custom
  string/`symbols()` bullets), `list-style-position`, nested lists, `<ol
  start>`.
- **Tables:** column sizing (fixed/percentage/min/max), multi-line cell
  wrapping, `vertical-align`, 6 named border-style presets, per-edge border
  overrides, `colspan`/`rowspan`, `<colgroup>`/`<col>`, `caption-side`.
- **Flexbox:** a deliberate single-row/single-column subset — `flex-direction`,
  `justify-content`, `align-items`/`align-self`, `order`, `gap`, `flex-grow`,
  `flex-basis`, the `flex` shorthand. See CSS.md's Flexbox section for the
  (sizeable) list of real-Flexbox features this subset excludes.
- **Forms and interactivity:** `<input>`/`<button>`/`<textarea>`/`<select>`
  (see `docs/SELECT.md`), a mutable `Document`/`Element` API, native Go DOM
  events, focus management, scrolling (`overflow: auto|scroll`, see
  `docs/SCROLLING.md`/`docs/SCROLLBARS.md`).

---

## Deviations from the W3C Spec

Real CSS properties that exist here but behave differently, because the
render target is a fixed-size character grid, not a pixel canvas:

- **All sizing is in character cells/lines, not pixels.** `width`/`height`/
  `margin`/`padding`/border thickness are all integer counts (or `ch`/`%`).
  `px`, `em`, `rem`, `vw`, `vh` are parsed as unsupported and ignored
  entirely, not converted.
- **`opacity`** can't alpha-composite against an unknown terminal
  background, so fractional values darken the foreground/background color
  channels toward black instead; `opacity: 0` blanks content to spaces
  (still occupying its layout box, matching spec) rather than true
  transparency.
- **`border-width`/`border-*-width`** parse without error but are always a
  no-op — a box-drawing character has no notion of line thickness separate
  from the glyph itself. Use `border-style: thick` or a custom glyph
  instead.
- **`border-style` values don't match real CSS's keyword set.** Real CSS
  has `solid`/`dashed`/`dotted`/`double`/`groove`/`ridge`/`inset`/`outset`/
  `none`/`hidden`; htmlterm's are named ASCII-art presets instead —
  `solid`/`rounded`/`thick`/`double`/`markdown`/`hidden`/`none` (only
  `solid`/`double`/`none`/`hidden` overlap in name with real CSS, and even
  those pick a specific box-drawing character set rather than a line style).
- **The `border`/`border-<edge>` shorthand is matched positionally, not by
  CSS value type** — this engine's own `border-style` vocabulary includes
  `thick`, which collides with real CSS's `thick` *border-width* keyword.
  A consequence: the real-CSS two-value `<width> <style>` form (no color)
  is indistinguishable from this engine's `<style> <color>` form and is
  silently dropped — use the three-value form or set `border-style`
  directly.
- **`:hover` has no real pointer-hover meaning** — the only place it
  matches anything is `option:hover` inside an open `<select>` popup,
  repurposed to mean "the arrow-key-highlighted option" (see
  `docs/SELECT.md`). **`:focus`** likewise matches a synthetic marker
  `Element.Focus` sets, not real window/pointer focus, and only means
  anything against a live `Document`, not one-shot `Renderer.Render`.
- **`text-transform: superscript`/`subscript`** substitutes each character
  for its Unicode superscript/subscript code point where one exists
  (there's no real script/font rendering) — characters with no Unicode
  equivalent pass through unchanged. **`font-variant: small-caps`**
  similarly just uppercases everything; terminals can't render true small
  caps.
- **`background` shorthand** only extracts the color component — `image`,
  `repeat`, `attachment`, `position`, `size`, `origin`, and `clip` tokens
  are recognized as present (so they don't break parsing) but otherwise
  ignored.
- **Bare ANSI color index numbers** (e.g. a raw `"214"`) are not accepted
  as a color value, even though they'd be meaningful to this renderer
  specifically — use `#rrggbb` or a named color and let automatic
  downsampling handle the terminal's actual palette.

---

## Terminal-Native Additions (no browser equivalent)

Features invented for this renderer, with no corresponding real-CSS or
real-DOM concept:

- **Literal-glyph border values** — `border-left: "▌"`, `border-top: "═"` —
  a quoted string sets that exact character directly, this engine's
  original (and still primary) way to use arbitrary box-drawing characters
  that have no named-preset equivalent.
- **`border-*-mid`, `border-center`, `border-*-corner`** — junction and
  corner glyph overrides for tables and boxes; real CSS has no per-junction
  border styling concept at all.
- **`scrollbar-style: block|shaded|classic|ascii|line`** and
  **`::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`/
  `::scrollbar-cap-start`/`::scrollbar-cap-end`** — real CSS has no
  standardized scrollbar-styling API (only nonstandard, prefixed
  `::-webkit-scrollbar-*` in Chromium); this is htmlterm's own equivalent,
  including clickable cap buttons with no real precedent at all. See
  `docs/SCROLLBARS.md`.
- **`list-style-type: symbols("a" "b" ...)`** — cycles a custom bullet list
  per item; a simplified take on the real CSS Counter Styles spec's
  `symbols()` function (no `<symbols-type>` keyword or image arguments).
- **OSC 8 terminal hyperlinks** for `<a href>` — an actual terminal escape
  sequence, emitted automatically, not a CSS feature at all.
- **`img::before { content: attr(alt) }`** UA-stylesheet default — since no
  image can render, alt text is shown inline by default (a real browser
  shows a broken-image icon plus the alt text in the image's own box
  instead).
- **`abbr[title]::after { content: " (" attr(title) ")" }`** UA default —
  since there's no hover-tooltip concept in a terminal, the title
  expansion is appended inline instead.
- **`<hr>` renders via `border-top`** (`hr { display: block; border-top: "─"; }`)
  rather than any pixel-drawn rule.

---

## Not Supported

- **CSS units:** `px`, `em`, `rem`, `vw`, `vh`, and friends (ignored; use
  bare integers, `ch`, or `%`).
- **CSS math/variables:** `calc()`, `min()`, `max()`, `clamp()`, custom
  properties (`--foo`).
- **At-rules:** `@media`, `@font-face`, `@keyframes`, `@import`,
  `@charset`, `@supports`, `@page`, etc. — the parser recognizes any
  `@`-rule and skips it as a unit rather than erroring or corrupting the
  rest of the stylesheet, but none of them do anything.
- **Pseudo-classes/elements beyond the supported list** — notably
  `:active` and any real mouse-hover semantics.
- **Layout models:** `display: grid`, `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none`; positioned layout
  (`position: absolute/relative/fixed`, `z-index` as a general mechanism —
  see `docs/RENDERING.md` for the one special-cased exception, `<select>`'s
  popup); `float`/`clear`.
- **Visual effects:** `box-shadow`, gradients, `background-image`,
  `transform`, `transition`/`animation`, `filter`.
- **Flexbox gaps:** `flex-wrap`, `align-content`, applied `flex-shrink`
  (parsed, never applied), main-axis distribution in `column` direction,
  `baseline` alignment, `margin: auto` on a flex item — see CSS.md's
  Flexbox section for the full reasoning per gap.
- **Table gaps:** `border-spacing`/cell padding (column separators are
  always exactly one character), multi-line cell content combined with
  `white-space: nowrap`.
- **`<select>` gaps:** `<optgroup>`, per-`<option>` border/padding/width —
  see `docs/SELECT.md`.
- **Scrollbar gaps:** horizontal scrollbars (only `overflow-y` gets a
  gutter) — see `docs/SCROLLBARS.md`.
- **`font-size`** — there is no concept of font size at all; terminal
  glyphs are a fixed cell size.

---

## See Also

- **CSS.md** — the exhaustive property-by-property reference.
- **docs/SELECT.md** — `<select>` popup styling in full.
- **docs/SCROLLBARS.md** — scrollbar gutter styling in full.
- **docs/SCROLLING.md** — the scrolling/viewport design.
- **docs/RENDERING.md** — the rendering engine's own internals, including
  why popups (`<select>`'s dropdown) are the one exception to "no
  positioned layout."
