# htmlterm Compatibility Notes

htmlterm reinterprets three separate web-platform surfaces for a fixed-size
character grid instead of a browser: **HTML** (parsing and per-element
rendering), **CSS** (selectors, cascade, properties), and **DOM & Events**
(the mutable `Document`/`Element` API and its native-Go event model). Each
gets the same four-part treatment below:

- **At a Glance** — what's supported, so you can tell quickly whether this
  covers your use case.
- **Deviations from Spec** — real features that exist here but behave
  differently than in a browser, and why (text cells, not pixels; no
  scripting engine; no real pointer/window).
- **Terminal-Native Additions** — things invented for this renderer with no
  browser equivalent at all.
- **Not Supported** — real features that simply aren't implemented.

This is the orientation read. For exact per-property syntax, see
**[CSS.md](./CSS.md)**, the exhaustive reference; for the design rationale
behind the DOM/Events/rendering internals, see `docs/INTERACTIVE.md`,
`docs/RENDERING.md`, `docs/REPAINT.md`, and `docs/SCROLLING.md`.

---

## HTML

### At a Glance

- **Parsing** uses `golang.org/x/net/html`, a real HTML5 tree-construction
  implementation — tag-soup recovery, implied tags, auto-closing, and
  foster parenting all work the way a browser's parser would, not a
  regex/best-effort approximation.
- **Structure:** headings, paragraphs, lists (`ul`/`ol`/`li`/`dl`/`dt`/`dd`,
  including nesting), blockquotes, tables (`thead`/`tbody`/`tfoot`/`tr`/
  `th`/`td`/`colgroup`/`col`/`caption`, `colspan`/`rowspan`), forms
  (`form`/`fieldset`/`legend`/`label`/`input`/`button`/`textarea`/`select`/
  `option`), HTML5 sectioning (`section`/`article`/`aside`/`header`/
  `footer`/`main`/`nav`/`hgroup`/`search`), inline text-level semantics
  (`a`/`span`/`strong`/`em`/`b`/`i`/`u`/`s`/`del`/`ins`/`mark`/`small`/
  `sub`/`sup`/`code`/`kbd`/`samp`/`var`/`cite`/`dfn`/`abbr`/`q`), disclosure
  (`details`/`summary`), `figure`/`figcaption`, `address`, `hr`/`br`.
- **Global attributes:** `hidden` and `aria-hidden="true"` both hide an
  element and its subtree (via the UA stylesheet's `display: none`).
- Unrecognized elements, attributes, comments, and doctypes are silently
  ignored rather than raising an error — see "Not Supported" below for what
  falls in that bucket.

### Deviations from Spec

- **Form controls are attribute-driven, not stateful.** `<input>`'s
  `value`/`checked`/`type` attributes (and `<select>`'s selected `<option>`)
  are the only source of truth — `Element.Value()`/`SetValue()`/
  `Checked()`/`SetChecked()` read and write those same attributes directly.
  There's no separate "live DOM property vs. reflected HTML attribute"
  distinction the way real browsers maintain for form controls.
- **Typing has no cursor.** `DispatchKey` on a focused text `<input>`/
  `<textarea>` always appends to, or trims from, the *end* of `value` —
  there's no caret position, no text selection, and no Home/End/ArrowLeft/
  ArrowRight-driven insertion point within the field itself (those arrow
  keys are reserved for select-popup/scroll navigation instead — see
  `docs/SELECT.md`).
- **A header row's `rowspan` is clamped to 1** — this renderer recognizes
  only a single header row, so a header cell can never merge down into data
  rows the way a real `<table>` could.
- **`<noscript>` content always renders** — there's no scripting engine to
  disable it for, so (unlike a browser, which only shows `noscript` content
  when JavaScript is off) it's unconditionally treated as regular markup.
- **`<details>`/`<summary>` always render fully expanded** — no collapse/
  expand interactivity exists; a real browser's native disclosure widget
  has no terminal equivalent here.

### Terminal-Native Additions

- **`img::before { content: attr(alt) }`** (UA stylesheet default) — since
  no image can ever actually render, alt text is shown inline directly,
  rather than in a broken-image icon's box the way a browser shows it.
- **`abbr[title]::after`** appends the title attribute's expansion inline —
  there's no hover-tooltip concept in a terminal to show it in instead.
- **`<hr>`** renders as a text rule line via `border-top`, not a pixel-drawn
  line.

### Not Supported

- **`<script>`, `<canvas>`, `<video>`, `<audio>`, `<iframe>`, `<embed>`,
  `<object>`, `<svg>`** — no scripting, media playback, embedding, or
  vector-graphics rendering exists. `<script>`/`<meta>`/`<link>` content is
  explicitly skipped during rendering; other unhandled elements aren't
  specifically stripped, they just fall back to generic inline treatment of
  whatever text content they happen to contain (usually none).
- **`<optgroup>`** inside `<select>` — only a `<select>`'s direct `<option>`
  children are read (see `docs/SELECT.md`).
- **The legacy `width` HTML attribute** on table cells/columns — ignored in
  favor of CSS `width`; in real-world markup (especially HTML email) it's
  almost always a pixel value with no reliable pixel-to-column conversion.
- **`tabindex`** — not read at all; keyboard focus order is a fixed
  document-order walk (see DOM & Events below), not customizable per
  element.

---

## CSS

### At a Glance

- **Selectors:** universal, element, class (including multiple classes),
  ID, all attribute operators (`[attr]`, `=`, `~=`, `|=`, `^=`, `$=`, `*=`),
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
- **Wide characters:** column widths (wrapping, alignment, padding, table
  sizing) account for double-width CJK and emoji glyphs via
  `go-runewidth`, not a naive one-rune-one-column assumption.
- **Generated content:** `::before`/`::after` with `content` (quoted
  strings, `attr()`, `counter()`/`counters()`, open/close-quote), CSS
  counters (`counter-reset`/`counter-increment`), `quotes`.
- **Lists:** `list-style-type` (all standard numbering systems plus custom
  string/`symbols()` bullets), `list-style-position`, nested lists, `<ol
  start>`.
- **Tables:** column sizing (fixed/percentage/min/max), multi-line cell
  wrapping, `vertical-align`, 6 named border-style presets, per-edge border
  overrides, junction/corner glyph overrides, `colspan`/`rowspan`,
  `<colgroup>`/`<col>`, `caption-side`, and (surprisingly, given real
  browsers rarely use it) `padding`/`margin` on `<table>` itself — see
  `docs/TABLES.md`.
- **Flexbox:** a deliberate single-row/single-column subset — `flex-direction`,
  `justify-content`, `align-items`/`align-self`, `order`, `gap`, `flex-grow`,
  `flex-basis`, the `flex` shorthand. See CSS.md's Flexbox section for the
  (sizeable) list of real-Flexbox features this subset excludes.
- **Forms and interactivity:** `<input>`/`<button>`/`<textarea>`/`<select>`
  (see `docs/SELECT.md`), scrolling (`overflow: auto|scroll`, see
  `docs/SCROLLING.md`/`docs/SCROLLBARS.md`).

### Deviations from Spec

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
  from the glyph itself. Use `border-style: heavy` or a custom glyph
  instead.
- **`border-style` values don't match real CSS's keyword set.** Real CSS
  has `solid`/`dashed`/`dotted`/`double`/`groove`/`ridge`/`inset`/`outset`/
  `none`/`hidden`; htmlterm's are named ASCII-art presets instead —
  `solid`/`rounded`/`heavy`/`double`/`markdown`/`hidden`/`none` (only
  `solid`/`double`/`none`/`hidden` overlap in name with real CSS, and even
  those pick a specific box-drawing character set rather than a line style).
  `heavy` is deliberately not named `thick`, to avoid colliding with real
  CSS's `border-width: thick` keyword.
- **The `border`/`border-<edge>` shorthand is matched positionally, not by
  CSS value type** — real CSS's border shorthand allows `<width>`/
  `<style>`/`<color>` in any order, and a real width keyword like `thick`
  can't be distinguished from a style keyword by content alone once it's in
  an unexpected slot. A consequence: the real-CSS two-value `<width>
  <style>` form (no color) is indistinguishable from this engine's
  `<style> <color>` form and is silently dropped — use the three-value form
  or set `border-style` directly.
- **`<table>`'s `border-left: none`/`border-right: none` remove more than
  the one rule they name** — every corner and internal junction glyph
  (header divider, row dividers) on that side, across every horizontal
  line, not just the outer frame's own corner. Real CSS's `border-left:
  none` never touches unrelated border segments; this is a deliberate
  choice for ASCII-art rendering specifically (a dangling corner glyph with
  no vertical rule to connect to would look broken, not correct) — see
  `docs/TABLES.md` for a rendered example and how to get a literal
  one-sided-rule look instead.
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
- **`list-style-type`/`symbols()` implement only a small slice of the real
  spec's predefined counter styles** — `disc`/`circle`/`square`/`decimal`/
  `lower-alpha`/`upper-alpha`/`lower-roman`/`upper-roman`/a quoted
  string/`symbols()`, versus the full
  [CSS Counter Styles](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/list-style-type)
  list (`armenian`, `georgian`, the CJK/Japanese/Korean variants, `hebrew`,
  `devanagari`, `disclosure-open`/`disclosure-closed`, etc. — see "Not
  Supported" below). `symbols()` itself is a real CSS Counter Styles
  function, just without its `<symbols-type>` keyword or image arguments.

### Terminal-Native Additions

- **Literal-glyph border values** — `border-left: "▌"`, `border-top: "═"` —
  a quoted string sets that exact character directly, this engine's
  original (and still primary) way to use arbitrary box-drawing characters
  that have no named-preset equivalent.
- **`border-*-mid`, `border-center`, `border-*-corner`** — junction and
  corner glyph overrides for tables and boxes; real CSS has no per-junction
  border styling concept at all. **`border-style` on `<table>`** (a
  whole-frame preset — `solid`/`rounded`/`heavy`/`double`/`markdown`/
  `standard`/`hidden`/`none`) and the **`border-header`/`border-columns`/
  `border-rows` edge toggles** are the same kind of addition, applied to
  tables specifically — see `docs/TABLES.md` for examples of every preset.
- **`scrollbar-style: block|shaded|classic|ascii|line`** and
  **`::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`/
  `::scrollbar-cap-start`/`::scrollbar-cap-end`** — real CSS has no
  standardized scrollbar-styling API (only nonstandard, prefixed
  `::-webkit-scrollbar-*` in Chromium); this is htmlterm's own equivalent,
  including clickable cap buttons with no real precedent at all. See
  `docs/SCROLLBARS.md`.
- **OSC 8 terminal hyperlinks** for `<a href>` — an actual terminal escape
  sequence, emitted automatically, not a CSS feature at all.

### Not Supported

- **CSS units:** `px`, `em`, `rem`, `vw`, `vh`, and friends (ignored; use
  bare integers, `ch`, or `%`).
- **CSS math/variables:** `calc()`, `min()`, `max()`, `clamp()`, custom
  properties (`--foo`).
- **At-rules:** `@media`, `@font-face`, `@keyframes`, `@import`,
  `@charset`, `@supports`, `@page`, `@counter-style` (no custom counter
  styles), etc. — the parser recognizes any `@`-rule and skips it as a unit
  rather than erroring or corrupting the rest of the stylesheet, but none
  of them do anything.
- **Pseudo-classes/elements beyond the supported list** — notably
  `:active` and any real mouse-hover semantics.
- **Layout models:** `display: grid`, `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none`; `float`/`clear`.
- **Positioned layout:** `position: absolute/relative/fixed`, `z-index`, or
  any CSS property surface for overlays in general — `<select>`'s dropdown
  is the only overlay that exists today, and it's driven by Go code
  (`select_popup.go`), not a general CSS positioning mechanism a document
  author could invoke on arbitrary elements. The underlying compositing
  primitives it's built on (line-splicing already-rendered output, shifting
  a sub-rendered box's positions by an offset) are general-purpose and
  reused elsewhere in the render engine, not select-specific internals — so
  this is a "not built yet" gap, not an architectural wall. See
  `docs/RENDERING.md`.
- **Visual effects:** `box-shadow`, gradients, `background-image`,
  `transform`, `transition`/`animation`, `filter`.
- **Flexbox gaps:** `flex-wrap`, `align-content`, applied `flex-shrink`
  (parsed, never applied), main-axis distribution in `column` direction,
  `baseline` alignment, `margin: auto` on a flex item — see CSS.md's
  Flexbox section for the full reasoning per gap.
- **Table gaps:** no `border-collapse`/`border-spacing` at all — the gap
  between columns is always exactly the frame's own separator character (0
  or 1 columns, toggled by `border-columns`, never a configurable width),
  and there's no equivalent spacing control between rows either; multi-line
  cell content combined with `white-space: nowrap` — see `docs/TABLES.md`
  for the full reasoning (including why table-level `padding` working, per
  above, means a configurable `border-spacing` isn't architecturally out of
  reach if ever needed — it just isn't exposed as a property today).
- **List gaps:** `list-style-image`; most of the real spec's predefined
  `list-style-type` counter styles — `armenian`/`lower-armenian`/
  `upper-armenian`, `georgian`, the CJK/Japanese/Korean variants
  (`cjk-decimal`, `cjk-ideographic`, `japanese-formal`/`informal`,
  `korean-hangul-formal`, etc.), `hebrew`, `devanagari` and the other
  script-specific systems, `disclosure-open`/`disclosure-closed`,
  `decimal-leading-zero`, `lower-greek` — only the small Western subset
  listed above under "At a Glance" is implemented.
- **`<select>` gaps:** `<optgroup>`, per-`<option>` border/padding/width —
  see `docs/SELECT.md`.
- **Horizontal scrolling doesn't exist at any layer, not just the
  scrollbar widget.** `overflow-x: scroll`/`auto` create no scrollable
  viewport at all — silently identical to `visible` (only `overflow-x:
  hidden`/`clip` do anything, and that's a one-time truncation, not
  scrolling); `overflow-y` is the only axis with real scroll-offset
  behavior. There is correspondingly no horizontal scrollbar gutter/
  gutter styling (see `docs/SCROLLBARS.md`) and, on the DOM/Events side, no
  `ScrollLeft`/`SetScrollLeft` and no horizontal wheel delta (see DOM &
  Events below).
- **`font-size`** — there is no concept of font size at all; terminal
  glyphs are a fixed cell size.

---

## DOM & Events

### At a Glance

- **`Document`/`Element`:** `ParseDocument`, `GetElementByID`,
  `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove, `ClassList`,
  `Value`/`SetValue`, `Checked`/`SetChecked` — parse once, mutate and
  re-render repeatedly, instead of `Renderer.Render`'s parse-once-discard
  model.
- **Events:** `AddEventListener`/`RemoveEventListener` with capture/target/
  bubble dispatch order, `StopPropagation`/`StopImmediatePropagation`/
  `PreventDefault`/`DefaultPrevented` — `"click"`, `"keydown"`, `"focus"`,
  `"blur"`, `"submit"`, `"change"`, `"resize"` event types, each with real
  default actions (checkbox/radio toggle, focus traversal, text entry,
  implicit form submit on Enter).
- **Focus:** `Element.Focus`/`Blur`, `Document.FocusNext`/`FocusPrev`/
  `FocusedElement`, matching `:focus` in CSS.
- **Hit-testing:** `Element.Rect()` returns the on-screen box (row/column/
  width/height in terminal cells) as a byproduct of rendering, for
  translating real input coordinates into `DispatchClick` calls.
- **Scrolling:** `Document.ScrollTop`/`SetScrollTop`, `DispatchWheel`,
  `PageUp`/`PageDown`/arrow-key scrolling on a focused descendant.
- **`Loop`:** drives a `Document` against a real terminal — raw mode, SGR
  mouse decoding, `SetInterval`/`SetTimeout` timers, repaint on every
  event/timer/resize.

### Deviations from Spec

- **Coordinates are terminal cells, not pixels.** `DispatchClick(row, col)`
  hit-tests against `Element.Rect()`'s row/column grid, not a
  `MouseEvent`'s `clientX`/`clientY`.
- **`Event` carries far less than a real DOM event** — no modifier keys
  (`ctrlKey`/`shiftKey`/`altKey`/`metaKey`), no `relatedTarget`, no
  `button`, no `detail` (click count). `Event.Key` is a single UTF-8 rune
  or one name from a fixed vocabulary (`"Enter"`, `"Backspace"`, `"Tab"`,
  `"Escape"`, arrow keys) — the host translates raw terminal input into
  these strings itself; htmlterm never reads a terminal directly outside
  of `Loop`.
- **Only one click "kind" exists** — there's no `mousedown`/`mouseup`/
  `dblclick`/`contextmenu`/drag events, just a single synthesized
  `"click"` that hit-tests and runs default actions atomically.
- **`DispatchWheel` mutates scroll position directly** and returns whether
  anything scrolled — unlike every other `Dispatch*` method, it does not
  dispatch a `"wheel"` `Event` a listener could observe or prevent.
- **Tab order is a fixed document-order walk** over form controls
  (`input`/`button`/`textarea`/`select`, skipping `disabled`) plus
  focusable scroll containers — there is no `tabindex` to reorder or add
  to it, and plain `<a>` links are never tab stops (real browsers make
  links focusable by default).

### Terminal-Native Additions

- **Scroll containers as tab stops:** an `overflow: auto|scroll` element
  with a resolved height and no other focusable descendant automatically
  becomes a keyboard tab stop, purely so `PageUp`/`PageDown`/arrow keys
  have something to scroll once focused.
- **Scrollbar cap buttons** (`::scrollbar-cap-start`/`-end`) are clickable
  via `DispatchClick`, but deliberately do *not* dispatch a `"click"`
  `Event` on the scrollable element — they're rendering chrome, not real
  element content (see `docs/SCROLLBARS.md`).
- **`<select>`'s dropdown popup** is a synthesized text overlay with its
  own synthetic `Rect` per `<option>`, composited on top of already-
  rendered output — real DOM has no equivalent to this text-based popup
  compositing step (see `docs/RENDERING.md`).

### Not Supported

- **`mousemove`, `mouseover`/`mouseout`, `dblclick`, `contextmenu`,
  drag-and-drop events** — no continuous hover tracking exists in a
  terminal, and none of these are wired up.
- **Text selection/clipboard events** (`select`, `copy`, `cut`, `paste`).
- **`input` events distinct from `change`** — typing doesn't fire a
  per-keystroke `"input"` event, only `"change"` on commit (blur, Enter, or
  selecting an option).
- **Custom events / arbitrary `dispatchEvent`** — only the fixed built-in
  event names above are ever dispatched.
- **Shadow DOM, custom elements, `MutationObserver`** — there is no tree-
  change observation API; a host must re-render after mutating.
- **Horizontal scrolling** — `Document.ScrollTop`/`SetScrollTop` have no
  `ScrollLeft`/`SetScrollLeft` counterpart, and `DispatchWheel(row, col,
  delta int)` takes a single (vertical) delta with no horizontal axis at
  all — matching `overflow-x` never creating a scrollable viewport in the
  first place (see CSS above).
- **Windows.** `Loop`'s automatic resize tracking requires `syscall.SIGWINCH`,
  which doesn't exist on Windows at all — a compile-time constraint there,
  not just an unverified one. The rest of the interactive layer (raw
  terminal mode via `golang.org/x/term`) is POSIX-oriented and hasn't been
  verified on Windows either.
- **Concurrent access.** A `Document` has no internal locking — `Loop.Run`'s
  own goroutine is the only place that may ever mutate a `Document` or
  trigger a repaint; timer callbacks and dispatched event handlers all run
  there too. There's no DOM-level concurrency model to compare this against
  (browsers are single-threaded per document too), but it's worth stating
  plainly for Go callers: calling `Document` methods from multiple
  goroutines is not safe.

---

## See Also

- **CSS.md** — the exhaustive property-by-property reference.
- **docs/SELECT.md** — `<select>` popup styling in full.
- **docs/SCROLLBARS.md** — scrollbar gutter styling in full.
- **docs/TABLES.md** — table border/margin/padding styling in full,
  including rendered examples of every `border-style` preset.
- **docs/SCROLLING.md** — the scrolling/viewport design.
- **docs/INTERACTIVE.md** — the `Document`/`Element`/events design history.
- **SECURITY.md** — a different axis from this doc (safety against
  untrusted HTML/CSS, not fidelity to spec): terminal escape-sequence
  stripping, and `StripHiddenInline`/`IgnoreDocumentCSS` for content a
  browser would render invisibly but htmlterm can remove structurally.
- **docs/RENDERING.md** — the rendering engine's own internals, including
  why popups (`<select>`'s dropdown) are the one exception to "no
  positioned layout."
