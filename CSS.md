# htmlterm CSS Reference

htmlterm renders a restricted subset of HTML+CSS to terminal strings.
This document lists every selector form, HTML element, and CSS property
that is recognized. Anything not listed here is silently ignored.

---

## Selectors

| Form | Example |
|------|---------|
| Universal | `* { }`, `*.warn { }` |
| Element | `th { }` |
| Class | `.num { }` |
| Multiple classes | `.warn.big { }` |
| Element + class(es) | `tr.unseen { }`, `p.a.b { }` |
| ID | `#intro { }` |
| Element + ID | `h1#title { }` |
| Attribute presence | `a[href] { }` |
| Attribute value | `td[data-role=header] { }` |
| Attribute word contains | `p[data-tags~=beta] { }` |
| Attribute dash-prefix | `p[lang|=en] { }` |
| Attribute prefix | `a[href^=https://] { }` |
| Attribute suffix | `a[href$=.pdf] { }` |
| Attribute substring | `a[href*=example] { }` |
| Descendant (space) | `tr.unseen td { }` |
| Child (`>`) | `div > p { }` |
| Pseudo-class | `:root { }`, `li:first-child { }`, `tr:nth-child(odd) { }` |
| Pseudo-element | `p::before { content: "→ "; }`, `p::after { content: " ←"; }` |
| Adjacent sibling (`+`) | `h2 + p { }` |
| Comma-separated (any of the above) | `h1, h2, h3 { }` |

**Specificity** follows CSS rules: ID = 100, class / pseudo-class / attribute = 10,
element / pseudo-element = 1, universal selector = 0. Higher specificity wins;
equal specificity last-write wins.

**`!important`** on a declaration (e.g. `color: red !important;`) lifts it into
a separate, higher-priority tier that always wins over any normal declaration,
regardless of specificity. Specificity (and last-write-wins for ties) still
applies *within* the `!important` tier, so an `!important` declaration with
higher specificity beats another `!important` declaration with lower
specificity. `!important` applies per declaration, not per rule, so a single
rule may mix `!important` and normal declarations. See also [Inline `style`
Attribute](#inline-style-attribute) for how `!important` interacts with inline
styles.

**Supported pseudo-classes:** `:root`, `:first-child`, `:last-child`,
`:nth-child(odd)`, `:nth-child(even)`, `:not(<simple-selector>)`, `:checked`,
`:disabled`, `:required`, `:focus`. `:root` matches the document element
(`html` for parsed HTML documents/fragments). Full `An+B` expressions are not
supported. `:not()` accepts a single compound selector (element, universal
selector, class, id, attribute, or combinations thereof) as its argument;
nested combinators inside `:not()` are not supported. `:checked`/`:disabled`/
`:required` match the real HTML `checked`/`disabled`/`required` attributes'
presence. `:focus` matches whichever element `Document.Focus` (see the
package godoc's events section) most recently marked focused; it has no
meaning against `Renderer.Render`'s one-shot rendering, only against a live
`Document`.

**Supported pseudo-elements:** `::before`, `::after`, and `::marker` (all also accepted
with a single colon). `::before`/`::after` inject inline text at the start or end of an
element's content; they require the `content` property. `::marker` styles the list
prefix (bullet or number) of an `<li>` element; supported properties are `color`,
`background-color`, `font-weight`, `font-style`, and `text-decoration`.
All combinator and element-matching forms work: `div p::before`, `.warn::after`,
`li::marker`, `ul.fancy li::marker`, etc.

**Supported attribute operators:** `[attr]` (presence), `[attr=val]` (exact
match), `[attr~=val]` (whitespace-separated word), `[attr|=val]` (exact value
or value followed by `-`), `[attr^=val]` (prefix), `[attr$=val]` (suffix), and
`[attr*=val]` (substring).

**Not supported:** `:hover`, `:active`, and other pseudo-classes beyond those
listed above; general sibling (`~`) combinator.

---

## Inheritance

The following properties inherit from parent to child when no direct rule
applies to the child element:

`color` · `font-weight` · `font-style` · `font-variant` · `text-decoration` · `text-align` · `white-space` · `text-transform` · `overflow-wrap` · `word-break` · `text-indent` · `tab-size` · `visibility` · `opacity` · `quotes`

Inheritance is resolved by walking up the ancestor chain and taking the value
from the nearest ancestor that sets the property directly. For example,
`tr.unseen { font-weight: bold }` causes all `<td>` children to be bold
without needing `tr.unseen td { font-weight: bold }`.

To explicitly cancel an inherited value, set the property to its `normal` (or
`none`) reset on the child element.

---

## HTML Elements

| Element | Notes |
|---------|-------|
| `h1`–`h6` | Rendered as a single styled line |
| `p` | Inline content followed by a newline |
| `blockquote` | Inline content followed by a newline; default `border-left: "│"; border-left-color: #555555; padding-left: 1; padding-right: 2` |
| `a` | Hyperlink; `href` attribute becomes an OSC 8 terminal hyperlink (default: `text-decoration: underline`) |
| `span` | Inline styled text; at block level, followed by a newline |
| `s`, `del` | Inline strikethrough text (default: `text-decoration: line-through`) |
| `u`, `ins` | Inline underlined text (default: `text-decoration: underline`). `ins` represents inserted/added content. |
| `b` | Inline bold text (default: `font-weight: bold`; alias for `<strong>`) |
| `kbd` | Keyboard input (default: `font-weight: bold`) |
| `mark` | Highlighted text (default: `background-color: #cc9900; color: #000000`) |
| `samp` | Sample program output (default: `font-style: italic`) |
| `var` | Variable name (default: `font-style: italic`) |
| `cite` | Citation / title of work (default: `font-style: italic`) |
| `sup` | Superscript text (default: `text-transform: superscript`) |
| `sub` | Subscript text (default: `text-transform: subscript`) |
| `strong` | Inline bold |
| `em`, `i` | Inline italic (default: `font-style: italic`) |
| `dfn` | Definition term; inline italic (default: `font-style: italic`) |
| `abbr` | Abbreviation. The UA stylesheet provides `abbr[title]::after { content: " (" attr(title) ")"; }`, so the expansion is appended inline when `title` is present — e.g. `<abbr title="HyperText Markup Language">HTML</abbr>` renders as `HTML (HyperText Markup Language)`. Override with user CSS to change the format. |
| `small` | Fine print / secondary text (default: `color: #888888`). No font-size reduction is possible in terminals. |
| `q` | Inline quotation; the UA stylesheet injects `open-quote` before and `close-quote` after the content. The characters used depend on the inherited `quotes` property (default `"…"` / `'…'` for nested). |
| `code` | Inline styled span |
| `pre` | Raw text block; `white-space: pre` by default; child elements are styled |
| `br` | Line break (inline or block) |
| `hr` | Horizontal rule. The UA stylesheet provides `hr { display: block; border-top: "─"; }`, drawing a full-width line. Override `border-top` to change the character and `border-top-color` to change the color. |
| `ul` | Unordered list; `• ` prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `ol` | Ordered list; decimal prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `li` | List item; content word-wraps with a hanging indent aligned to the prefix. Style the bullet/number with `li::marker { color: …; font-weight: bold; … }`. |
| `dl` | Definition list block (default: `display: block; margin-bottom: 1`) |
| `dt` | Definition term; rendered as a bold block (default: `display: block; font-weight: bold`) |
| `dd` | Definition description; rendered as an indented block (default: `display: block; padding-left: 4`) |
| `figure` | Block container for self-contained content such as illustrations or code (default: `display: block`). Style with `margin-left`/`margin-right` to indent. |
| `figcaption` | Caption for the nearest `<figure>` ancestor (default: `display: block; font-style: italic`) |
| `img` | Inline image. The UA stylesheet provides `img::before { content: attr(alt); }`, so alt text is shown by default and nothing is emitted when `alt` is absent. Override with user CSS to change the format — for example, to produce Markdown-style output: `img::before { content: "![" attr(alt) "](" attr(src) ")"; }` |
| `address` | Contact/attribution block (default: `display: block; font-style: italic`). |
| `details` | Disclosure container (default: `display: block`). Always rendered fully expanded — no interactivity. Content is preserved and displayed. |
| `summary` | Disclosure summary; the visible heading of a `<details>` block (default: `display: block; font-weight: bold`). |
| `noscript` | Content is always rendered (no JavaScript in terminal). The HTML5 parser may deliver noscript content as raw text; it is re-parsed and rendered as HTML automatically. |
| `template` | Inert template content. The element and all descendants are skipped; styles and counters inside it do not affect the document. |
| `menu` | Semantic list of commands; treated identically to `<ul>` (default `list-style-type: disc`, `padding-left: 4`). |
| `wbr` | Optional line-break hint. Emits nothing (no terminal equivalent). |
| `table` | See table section below. Defaults to `display: table`; set `display: block` with block `tr`/`td` rules to linearize table markup as ordinary document flow. |
| `caption` | Table caption (default: `display: block; text-align: center`). Rendered above the table, centered over the full table width. |
| `colgroup` | Column group; direct child of `<table>`. A `span` attribute (default 1) applies the group's own `style=` across that many columns when no `<col>` children are present. Style via `style=` or CSS selectors. |
| `col` | Column descriptor inside `<colgroup>`. `span` attribute (default 1) repeats the column's declarations across N consecutive columns. Supports `width`, `min-width`, `max-width`, `text-align`, `color`, `background-color`, `font-weight`, `font-style`, `text-decoration` via `style=` or CSS. A `width` HTML attribute is treated as an absolute char count. Cell-level declarations take priority over `<col>` declarations. |
| `thead`, `tbody`, `tfoot` | Transparent wrappers inside `<table>` |
| `tr` | Table row; first `<tr>` containing `<th>` is the header |
| `th`, `td` | Table cells |
| `div` | Generic block container (default: `display: block`; no other UA styles) |
| `section`, `article`, `aside`, `header`, `footer`, `main`, `nav`, `hgroup`, `search` | HTML5 sectioning elements; all default to `display: block` with no other UA styles. Style freely with CSS. |
| `form` | Generic block container for form controls (default: `display: block`; no other UA styles). |
| `fieldset` | Groups related form controls in a bordered box (default: `display: block; border-style: solid; padding: 1; margin-bottom: 1`). |
| `legend` | Caption for the nearest `<fieldset>` (default: `display: block; font-weight: bold`), rendered as its own line at the top of the fieldset's content — a simplified terminal approximation, not browsers' border-straddling placement. |
| `label` | Inline by default (no UA rule); place its associated control inside it (e.g. `<label>Name: <input type="text"></label>`) to have them flow on one line. |
| `input` | Void element; content is synthesized from attributes, not children (default: `display: inline-block`). `type="checkbox"` → `☐`/`☑` based on the `checked` attribute; `type="radio"` → `○`/`●`; `type="submit"`/`"reset"`/`"button"` → `[ Label ]` using `value` (falling back to "Submit"/"Reset"/"Button"); `type="hidden"` → nothing; every other type (including the default, unset type) → `[value]`, falling back to `[placeholder]` when `value` is absent. `Element.Value()`/`SetValue()`/`Checked()`/`SetChecked()` read and write the exact attributes this renders from. |
| `button` | Renders its children normally, wrapped in brackets via the UA stylesheet's `button::before { content: "[ "; }` / `button::after { content: " ]"; }` (default: `display: inline-block`). |
| `textarea` | Multi-line bordered box (default: `display: block; border-style: solid; padding-left: 1; padding-right: 1`). Shows the `value` attribute if set (matching `Element.Value()`/`SetValue()`); otherwise falls back to its child text, with one leading newline right after the opening tag ignored, per the HTML spec's default-value rule. |
| `select` | Not yet implemented — falls through to generic inline dispatch. |

---

## Global Attributes

The UA stylesheet provides `[hidden] { display: none; }` and
`[aria-hidden=true] { display: none; }`, so the boolean HTML `hidden`
attribute and an explicit `aria-hidden="true"` both hide an element and all
its children — same effect as setting `display: none` directly. Either can be
overridden by a more specific rule (e.g. `[hidden] { display: block !important; }`).

---

## CSS Properties — Inline / Block Elements

These apply to any matched element and control text rendering.
`normal` / `none` values explicitly cancel an inherited value.

#### `display`
`block` | `inline` | `inline-block` | `table` | `none`. Controls layout. `block` emits a newline after content and respects `margin-top`/`margin-bottom`. `inline` renders with no newline. `inline-block` is like `inline` but respects `width`. `table` uses htmlterm's table renderer when set on a `<table>` element. `none` hides the element and all its children. Not inherited. Defaults: `table` defaults to `table`; `p`, `h1`–`h6`, `blockquote`, `pre`, `div`, and common HTML5 sectioning elements (`section`, `article`, `aside`, `header`, `footer`, `main`, `nav`, `hgroup`, `search`) default to `block`; all others default to `inline`.

To treat table markup as a simple linear document flow, opt out of the table renderer:

```css
table, thead, tbody, tfoot, tr, td, th {
  display: block;
}

td, th {
  width: auto;
  white-space: normal;
}

td + td,
tr + tr {
  margin-top: 1;
}
```

#### `color`
Any CSS color value (see [Color Values](#color-values)). Foreground color. Inherited.

#### `background-color`
Any CSS color value (see [Color Values](#color-values)). Background color. Not inherited.

#### `background`
Shorthand recognized only when it contains a CSS color value. The color is
mapped to `background-color`; image, repeat, attachment, position, size, origin,
and clip components are ignored. For example, `background: url(bg.png) #003366
no-repeat` behaves like `background-color: #003366`. Not inherited.

#### `opacity`
`0.0`–`1.0`. Scales the foreground and background color channels. `1` is fully opaque (the default); `0` renders as black. Inherited.

#### `font-weight`
`bold` | `normal`. `normal` cancels inherited bold. Inherited.

#### `font-style`
`italic` | `normal`. `normal` cancels inherited italic. Inherited.

#### `text-decoration`
`underline` | `line-through` | `none` | `normal`. `none`/`normal` cancels both underline and strikethrough. Inherited.

#### `text-align`
`left` | `center` | `right`. Effective on cells when column has a width. Inherited.

#### `margin`
One to four values using CSS shorthand order. Expands to `margin-top`,
`margin-right`, `margin-bottom`, and `margin-left`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

Values use the same formats as the corresponding longhand properties. For
example, `margin: 1 auto` sets top/bottom margins to `1` and left/right margins
to `auto`. Not inherited.

#### `margin-top`
Integer line count (e.g. `1`). Extra blank lines above a block element. Adjacent margins collapse: the larger wins. Not inherited.

#### `margin-bottom`
Integer line count (e.g. `1`). Extra blank lines below a block element. Collapses with the next element's `margin-top`. Not inherited.

#### `margin-left`
Integer (e.g. `4`) or `auto`. Spaces prepended to every line of a block element, outside any `border-left`. Not inherited.

#### `margin-right`
Integer (e.g. `4`) or `auto`. Spaces appended to every line of a block element, outside any `border-right`. Not inherited.

#### `margin-block-start`, `margin-block-end`, `margin-inline-start`, `margin-inline-end`
Logical aliases for the physical margin properties. htmlterm does not model
writing modes or RTL layout, so these always map as follows:

| Property | Alias for |
|----------|-----------|
| `margin-block-start` | `margin-top` |
| `margin-block-end` | `margin-bottom` |
| `margin-inline-start` | `margin-left` |
| `margin-inline-end` | `margin-right` |

Values use the same formats as the corresponding physical longhand properties.
Not inherited.

**`auto` margins:** When an element has an explicit `width` set, `margin-left: auto` and/or `margin-right: auto` distribute the remaining space. Both `auto` centers the element; only `margin-left: auto` right-aligns it; only `margin-right: auto` left-aligns it (fills trailing space). Without an explicit `width` the element already fills the available width and auto margins have no visible effect.

#### `width`
`40` or `50%`. Fixed or percentage width for block and `inline-block` elements. For block elements, `width: 100%` fills the renderer width; margins and border characters are subtracted so the total visual line equals the specified width. Not inherited.

#### `min-width`
`40` or `50%`. Minimum width for block and `inline-block` elements, using
the same box semantics as `width`. On block elements it constrains wrapping,
alignment, borders, and auto-margin placement. Not inherited.

#### `max-width`
`40` or `50%`. Maximum width for block and `inline-block` elements, using
the same box semantics as `width`. On block elements it constrains wrapping,
alignment, borders, and auto-margin placement. Not inherited.

#### `padding`
One to four values using CSS shorthand order. Expands to `padding-top`,
`padding-right`, `padding-bottom`, and `padding-left`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

Values use the same formats as the corresponding longhand properties. Not
inherited.

#### `padding-left`
`4` or `4ch`. Left padding in rune columns; applies to block elements. Not inherited.

#### `padding-right`
`4` or `4ch`. Right padding in rune columns; applies to block elements. Not inherited.

#### `padding-top`
Integer (e.g. `2`). Blank lines inserted above content, inside `border-top`. Each blank row is as wide as the content area so left/right borders and padding align correctly. Not inherited.

#### `padding-bottom`
Integer (e.g. `2`). Blank lines inserted below content, inside `border-bottom`. Same width semantics as `padding-top`. Not inherited.

#### `padding-block-start`, `padding-block-end`, `padding-inline-start`, `padding-inline-end`
Logical aliases for the physical padding properties. htmlterm does not model
writing modes or RTL layout, so these always map as follows:

| Property | Alias for |
|----------|-----------|
| `padding-block-start` | `padding-top` |
| `padding-block-end` | `padding-bottom` |
| `padding-inline-start` | `padding-left` |
| `padding-inline-end` | `padding-right` |

Values use the same formats as the corresponding physical longhand properties.
Not inherited.

#### `height`
Integer line count (e.g. `5`). Content-box height in lines. If the rendered content has fewer lines it is padded with blank lines; if it has more and `overflow: hidden`/`clip` is set it is truncated. Without an overflow setting, extra content is visible. Takes priority over `min-height` and `max-height` when set. Not inherited.

#### `min-height`
Integer line count (e.g. `3`). Minimum content-box height in lines. The element is always padded to at least this many lines regardless of `overflow`. Has no effect when `height` is also set. Not inherited.

#### `max-height`
Integer line count (e.g. `10`). Maximum content-box height in lines. Content beyond this limit is truncated only when `overflow: hidden` or `overflow: clip` is also set; without overflow the content is still visible. Has no effect when `height` is also set. Not inherited.

#### `white-space`
`normal` | `nowrap` | `pre` | `pre-wrap` | `pre-line`. How text-node whitespace is handled. Inherited. Default `normal` for block/inline elements, including `td` and `th`. Block elements with `normal` word-wrap long lines at the available content width, breaking at word boundaries. `nowrap` disables word wrapping; set it on a cell or ancestor to get single-line truncation (see `text-overflow`) instead of multi-line wrapping. `pre` preserves all whitespace and disables wrapping. `pre-wrap` and `pre-line` preserve newlines but still allow wrapping. Content that is already multi-line (lists, `<br>` tags, nested block elements) is not re-wrapped.

#### `overflow`, `overflow-x`, `overflow-y`
`visible` | `hidden` | `clip` | `scroll` | `auto`. Controls whether content that exceeds an explicit `width`/`height` is clipped. `overflow` is shorthand for the two per-axis longhands: one value sets both `overflow-x`/`overflow-y`; two values set `overflow-x` then `overflow-y` respectively. A longhand set directly overrides just its own axis, per the normal cascade (so `overflow: auto; overflow-y: scroll` leaves `overflow-x` at `auto`). `overflow-x` gates horizontal (width) clipping; `overflow-y` gates vertical (height) clipping and scrolling. Default `visible`: content overflows the box. Not inherited.

- **`hidden` / `clip`** — `overflow-x` truncates each line to the content width (**requires an explicit `width`**; without one the element already fills the available width); `overflow-y` truncates excess lines when an explicit `height` is also set. `text-overflow` controls the truncation marker.
- **`auto`** — `overflow-y` (with an explicit `height`; `min-height`/`max-height` alone don't count) makes the element a real scrollable viewport: a live per-element scroll offset (`Document.ScrollTop`/`SetScrollTop`) selects which window of lines is visible, adjustable via mouse wheel (`Document.DispatchWheel`), `PageUp`/`PageDown`/`ArrowUp`/`ArrowDown` on a focused descendant (`Document.DispatchKey`), or focus landing on an off-screen descendant (`Document.Focus` auto-scrolls it into view). No visible scrollbar/indicator is drawn.
- **`scroll`** — same scrolling behavior as `auto`, **plus** an always-reserved one-column gutter with a `│` track and `█` thumb tracking the scroll position — drawn regardless of whether the content actually overflows, matching real CSS's own unconditional-scrollbar semantics for `scroll` vs. only-if-needed for `auto`. Silently omitted (no column reserved, content unaffected) if the box is too narrow to spare one.

See `SCROLLING.md` for the full design (including why `auto` deliberately never gets an indicator).

#### `text-overflow`
`clip` | `ellipsis` | `"‹str›"`. The truncation marker appended to lines clipped by `overflow: hidden`/`clip`. Only effective when `overflow: hidden` or `overflow: clip` and `white-space: nowrap` and an explicit `width` are all set. Default `clip` (no marker). `ellipsis` appends `…`. A quoted string (e.g. `text-overflow: "+"`) uses that string as the marker. Not inherited. **Note:** for table cells, `overflow: hidden` is implicit and the default is `ellipsis` rather than `clip`.

#### `font-variant`
`small-caps` | `normal`. `small-caps` uppercases all text content (terminal rendering cannot distinguish small-cap glyphs from full capitals). `normal` cancels an inherited value. Inherited. When both `font-variant: small-caps` and `text-transform` are set, `text-transform` wins.

#### `text-transform`
`none` | `uppercase` | `lowercase` | `capitalize` | `superscript` | `subscript`. Case/script transformation applied to text content. Inherited. `capitalize` uppercases the first letter of each whitespace-separated word. `superscript` and `subscript` replace each character with its Unicode superscript or subscript equivalent where one exists; characters with no Unicode equivalent are passed through unchanged.

#### `overflow-wrap`
`normal` | `break-word`. Controls whether long words that overflow the container width may be broken mid-word. `normal` (default): words are never broken — a word longer than the column simply overflows the line. `break-word`: a word that cannot fit on any line is hard-broken at the column boundary. Inherited. See also `word-break`.

#### `word-break`
`normal` | `break-all`. Sets the character-level line-break strategy. `normal` (default): word-boundary breaking only (same as `overflow-wrap: normal`). `break-all`: break at any character boundary, ignoring word boundaries — suitable for CJK text or URLs with no natural break points. When both `overflow-wrap` and `word-break` are set, `overflow-wrap` takes priority. Inherited.

#### `text-indent`
`<integer>` or `<N%>`. Indents the first line of a block element's content by the specified number of columns (or percentage of available width). Only applied when the element's own first content is inline text; when the first child is a block-level element, that child applies its own inherited value. Inherited.

#### `tab-size`
`<integer>`. Tab-stop interval for expanding `\t` characters inside `white-space: pre` or `pre-wrap` content. Tab characters advance to the next multiple of `tab-size` columns. Default: `8`. Has no effect when `white-space` is `normal`, `nowrap`, or `pre-line` (tabs are collapsed to a single space like any other whitespace). Inherited.

#### `visibility`
`visible` | `hidden`. `hidden` hides the element's content while preserving its layout space — blank characters of the same dimensions are emitted instead. Unlike `display: none`, a hidden element still occupies lines in the output. `hidden` is inherited, so all descendants are also hidden unless they override with `visibility: visible`. For table cells, `visibility: hidden` renders the cell as blank (preserving the column width from other rows). Meaningful distinction from `display: none` in table and fixed-layout contexts.

#### `content`

Text injected by `::before` or `::after` pseudo-element rules. The value is one or more space-separated **tokens** that are concatenated left-to-right. `none` and `normal` by themselves suppress injection entirely. Not meaningful on regular elements. Not inherited.

| Token | Example | Description |
|-------|---------|-------------|
| Quoted string | `"→ "` | Literal text; CSS escape sequences (`\A` = newline, `\22` = `"`, etc.) are decoded |
| `attr(name)` | `attr(href)` | Value of the named HTML attribute on the element; empty string if absent |
| `counter(name)` | `counter(sec)` | Current value of a CSS counter (see **Counters** below) |
| `counter(name, style)` | `counter(ch, upper-roman)` | Counter value formatted with the given `list-style-type` style |
| `counters(name, sep)` | `counters(item, ".")` | All nested counter values joined by sep (e.g. `1.2.3`) |
| `counters(name, sep, style)` | `counters(item, ".", lower-alpha)` | Nested counter values with a style applied to each |
| `open-quote` | — | Opening quote from the `quotes` property at the current nesting depth; increments depth |
| `close-quote` | — | Closing quote from the `quotes` property; decrements depth |
| `no-open-quote` | — | Increments quote depth without emitting a character |
| `no-close-quote` | — | Decrements quote depth without emitting a character |
| `none` / `normal` | — | Suppress content injection entirely (only valid as the sole token) |

**Concatenation example:**
```css
a::before { content: "["; }
a::after  { content: "](" attr(href) ")"; }
/* renders <a href="/page">link</a> as: [link](/page) */
```

---

### Counters

CSS counters let you auto-number elements. Two companion properties control them:

#### `counter-reset`
`<name> [<integer>] …`. Creates a new counter scope named `<name>`, initialized to `<integer>` (default `0`). Multiple name/value pairs may appear in one declaration. Not inherited.

```css
ol { counter-reset: item; }          /* reset to 0 */
ol { counter-reset: item 9; }        /* reset to 9; first increment → 10 */
```

#### `counter-increment`
`<name> [<integer>] …`. Increments the innermost counter named `<name>` by `<integer>` (default `1`). Multiple name/step pairs may appear. Not inherited.

```css
li { counter-increment: item; }      /* +1 each <li> */
li { counter-increment: item 2; }    /* +2 each <li> */
```

**Complete example — auto-numbered sections:**
```css
body  { counter-reset: section; }
h2    { counter-increment: section; }
h2::before { content: counter(section) ". "; }
```

**Nested numbering with `counters()`:**
```css
ol          { counter-reset: item; list-style-type: none; }
li          { counter-increment: item; }
li::before  { content: counters(item, ".") " "; }
/* produces: 1 · 1.1 · 1.2 · 2 · 2.1 … */
```

Counter styles available in `counter()` / `counters()` match `list-style-type`: `decimal` (default), `lower-alpha`, `upper-alpha`, `lower-roman`, `upper-roman`, `none`.

---

### Quotes

#### `quotes`
`"<open>" "<close>" …`. Pairs of strings used by `open-quote` and `close-quote`. The first pair is used at nesting depth 0, the second at depth 1, and so on; the last pair repeats for any deeper nesting. Inherited. Default: `"“" "”" "‘" "’"` (`"` `"` `'` `'`).

```css
/* English smart quotes (default) */
q { }   /* uses UA-stylesheet q::before/q::after rules */

/* Custom quotes */
blockquote { quotes: "«" "»" "‹" "›"; }
blockquote::before { content: open-quote; }
blockquote::after  { content: close-quote; }
```

The UA stylesheet defines `q::before { content: open-quote; }` and `q::after { content: close-quote; }`, so `<q>` elements are quoted automatically using the inherited `quotes` value.

#### `border`
Shorthand for `border-style` plus `border-color` on the whole box (all four
edges uniformly — there is no per-edge form of this shorthand; use the
individual `border-top`/`border-right`/`border-bottom`/`border-left` and
`border-*-color` longhands for that). Values are matched **positionally**,
not by type, since this engine's `border-style` vocabulary includes `thick`,
which collides with real CSS's `thick` border-width keyword — positional
matching resolves `border: thick solid red` correctly regardless.

| Values | Meaning |
|--------|---------|
| `<style>` | `border-style: <style>` |
| `<style> <color>` | `border-style: <style>; border-color: <color>` |
| `<width> <style> <color>` | `<width>` ignored; `border-style: <style>; border-color: <color>` |

```css
div { border: solid red; }        /* border-style: solid; border-color: red; */
div { border: 1px solid red; }    /* same; "1px" is ignored */
```

**Not supported: the two-value `<width> <style>` form** (e.g. `border: 2px
solid;`, no color) — with no positional color slot to detect its absence,
this is indistinguishable from the two-value `<style> <color>` form and is
silently dropped like any other unrecognized value. Set `border-style`
directly instead. Not inherited.

#### `border-style`
`solid` | `rounded` | `thick` | `double` | `markdown` | `hidden` | `none`. Applies a named border preset as a shorthand for all individual border properties. Individual `border-*` properties set on the same element override the preset for that edge (e.g. `border-top: ═` overrides the fill but keeps preset corners). `hidden`/`none` clears all borders. Not inherited.

#### `border-width`, `border-top-width`, `border-right-width`, `border-bottom-width`, `border-left-width`
Accepted (parsed without error) and always a no-op. Terminal box-drawing
characters have no notion of a line-thickness distinct from the character
itself — draw a thicker border with `border-style: thick` or a custom
`border-top`/`border-left`/etc. character instead. These properties exist
purely so real-world CSS (e.g. copy-pasted `border: 1px solid red`, split
into its longhands) doesn't need to be edited before use. Not inherited.

#### `border-left`, `border-right`, `border-top`, `border-bottom`
Each accepts **two different forms**, dispatched on whether the value is quoted:

| Form | Example | Meaning |
|------|---------|---------|
| Quoted string | `border-left: "▌"` | This engine's literal-glyph form (predates the shorthand below, and remains the primary way to use box-drawing characters that have no CSS style-keyword equivalent). The exact character(s) prepended/appended (for `-left`/`-right`) or repeated as the horizontal-rule fill (for `-top`/`-bottom`). `none` (unquoted) or unset = no border. |
| Bareword, standard CSS shorthand grammar | `border-left: solid red` | `<style>`, `<style> <color>`, or `<width> <style> <color>` (`<width>` ignored) — the same positional grammar as the [`border`](#border) shorthand, just resolved to *this one edge's* glyph from the named preset (e.g. `top.fill` for `border-top`, `left`/`right` for `border-left`/`border-right`) instead of the whole box. `<style>` is one of the [`border-style`](#border-style) preset names. An explicit `border-top: none` clears just that edge, even when `border-style` is also set on the same element (this used to be silently overridden by the preset — no longer). |

```css
div { border-top: "═"; }              /* literal glyph, unchanged from before */
div { border-top: double; }           /* double preset's top glyph, no color change */
div { border-top: double red; }       /* double preset's top glyph, red */
div { border-top: 1px double red; }   /* same; "1px" is ignored */
div { border-style: solid; border-top: none; }  /* solid box with the top edge removed */
```

As with [`border`](#border), the two-value `<width> <style>` form (no color) has
no positional color slot and is silently dropped. Not inherited.

#### `border-left-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the left border character. Not inherited.

#### `border-right-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the right border character. Not inherited.

#### `border-top-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the top border rule. Not inherited.

#### `border-bottom-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the bottom border rule. Not inherited.

#### `border-color`
One to four color values using CSS shorthand order (like `margin`/`padding`).
Expands to `border-top-color`, `border-right-color`, `border-bottom-color`,
and `border-left-color`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

The single-value form (`border-color: #555555`) also doubles as the `<table>`
border-color property described in [CSS Properties — Table Borders](#css-properties--table-borders),
which colors the whole table frame uniformly rather than per edge. Not inherited.

#### `border-top-left-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the left end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-top-right-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the right end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-bottom-left-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the left end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

#### `border-bottom-right-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the right end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

---

## CSS Properties — Lists (`ul`, `ol`)

These properties apply to `<ul>` and `<ol>` elements and control list rendering.
Item content word-wraps at the available content width.

| Property | Values | Notes |
|----------|--------|-------|
| `list-style` | Any supported `list-style-type` and/or `list-style-position`, in either order | Shorthand for the supported list longhands. `list-style-image` values such as `url(...)` are ignored. |
| `list-style-type` | See table below | Prefix string for each `<li>`. A quoted string literal (e.g. `"→ "`) is used verbatim as the bullet. Not inherited. Default: `disc` for `<ul>`, `decimal` for `<ol>`. |
| `list-style-position` | `outside` (default), `inside` | `outside`: prefix hangs to the left; continuation lines align under the first text character. `inside`: prefix flows inline with text; continuation lines align with `padding-left`. Inherited. |
| `padding-left` | `4` | Indents the entire list from the left; combined with `margin-left` for total indentation. Default: `4`. |
| `margin-left` | `4` | Left margin for the list block; added to `padding-left`. Default: `0`. |

### `list-style-type` values

| Value | Prefix | For |
|-------|--------|-----|
| `disc` | `• ` | `<ul>` (default) |
| `circle` | `○ ` | `<ul>` |
| `square` | `■ ` | `<ul>` |
| `none` | *(empty)* | `<ul>`, `<ol>` |
| `decimal` | `1.` `2.` … | `<ol>` (default) |
| `lower-alpha` / `lower-latin` | `a.` `b.` … | `<ol>` |
| `upper-alpha` / `upper-latin` | `A.` `B.` … | `<ol>` |
| `lower-roman` | `i.` `ii.` … | `<ol>` |
| `upper-roman` | `I.` `II.` … | `<ol>` |
| `"<string>"` / `'<string>'` | custom | `<ul>`, `<ol>` |

A quoted string literal sets a custom bullet used verbatim for every item, e.g. `list-style-type: "→ "`. The string is used as-is with no additional spacing — include a trailing space in the string if desired. Works on both `<ul>` and `<ol>`.

The `list-style` shorthand accepts the supported type and position values in
either order, e.g. `list-style: square inside`, `list-style: inside upper-roman`,
or `list-style: "→ " outside`. Image values such as `url(bullet.png)` are
ignored, so `list-style: url(bullet.png) square` behaves like
`list-style-type: square`.

Numeric prefixes (`decimal`, `*-roman`, `*-alpha`) are right-aligned within a
fixed-width column sized to the widest prefix in the list (e.g. `" 1."` aligns
with `"10."` in a ten-item list).

Nested lists are supported: a `<ul>` or `<ol>` anywhere inside an `<li>` is
rendered recursively, indented within the parent item's content width.

**`<ol start="N">`:** The `start` attribute sets the initial counter value.
For example, `<ol start="5">` numbers items 5, 6, 7 … The prefix column width
is sized to the widest number that will appear (e.g. `start="9"` with two items
produces a two-digit-wide column for items 9 and 10).

**Not supported:** `list-style-image`; the `type` HTML attribute on `<ol>` (use
`list-style-type` CSS instead).

---

## Size Values

Wherever one of these sizing declarations is accepted (`width`, `min-width`,
`max-width`, and percentage-capable margins), the following forms are recognized:

| Form | Example | Meaning |
|------|---------|---------|
| Bare integer | `14` | Fixed rune count |
| `ch` unit | `14ch` | Fixed rune count (same as bare integer) |
| Percentage | `50%` | Fraction of the available content width |

Pixel (`px`), `em`, `rem`, and other CSS units are ignored.

For block elements, percentages are resolved against the available block width.
For a `width: 100%` table, the available content width is the terminal width
minus the sum of all separator characters.

---

## CSS Properties — Cell Sizing (`th`, `td`)

The `width` HTML attribute on `<th>`/`<td>` is equivalent to CSS `width`
(always absolute). CSS `width` with a `%` value overrides the HTML attribute.

| Property | Example | Notes |
|----------|---------|-------|
| `width` | `width: 14` or `width: 25%` | Fixed or percentage column width; immune to expand/shrink |
| `min-width` | `min-width: 8` or `min-width: 10%` | Column will not shrink below this value |
| `max-width` | `max-width: 40` or `max-width: 30%` | Column will not expand beyond this value |
| `white-space` | `nowrap` \| `normal` | Controls cell line-wrapping. Default `normal`: content word-wraps across multiple lines. Set to `nowrap` to clip to a single line using `text-overflow` instead. |
| `text-overflow` | `clip` \| `ellipsis` \| `"‹str›"` | How overflowing text is indicated when `white-space: nowrap`. Default `ellipsis` (`…`). `clip` cuts with no marker; a quoted string (e.g. `text-overflow: "+"`) uses that as the marker. Not inherited. Ignored when `white-space: normal`. |
| `vertical-align` | `top` \| `middle` \| `bottom` | Vertical placement of the cell's content within the row height. Default `top`. `bottom` pins content to the last line of the row; `middle` centres it. Not inherited. Only meaningful when rows contain multi-line cells of differing heights. |

### Multi-line cells

When `white-space: normal` is set on a cell (or an ancestor whose value is
inherited), the cell content word-wraps at the column boundary instead of
being truncated. Words that are longer than the column width are hard-broken
at the column edge. All cells in the same row are padded to the same height
(the tallest cell in that row); shorter cells are padded with blank lines.

```css
/* All data cells in this table wrap instead of truncating */
table.wrap td { white-space: normal; }
```

`white-space: normal` can also be set on the `<table>` element itself — since
`white-space` is inherited, every `<td>` and `<th>` in the table will pick it
up unless overridden on a specific cell.

If both absolute and percentage forms of `min-width`/`max-width` were somehow
set (e.g. via cascade), the more restrictive value wins.

Column widths are determined from header constraints plus the maximum natural
content width across all rows. Fixed and percentage columns are immune to the
table-level expand/shrink resizing pass.

---

## CSS Properties — Table Borders (`table`)

These apply only to `<table>` elements and control the visual frame.

### `border-style`

Sets the complete border character set. Individual edges can be overridden
with the properties below.

| Value | Appearance |
|-------|-----------|
| `solid` | `┌─┬─┐ │ │ ├─┼─┤ └─┴─┘` (default) |
| `rounded` | `╭─┬─╮ │ │ ├─┼─┤ ╰─┴─╯` |
| `thick` | `┏━┳━┓ ┃ ┃ ┣━╋━┫ ┗━┻━┛` |
| `double` | `╔═╦═╗ ║ ║ ╠═╬═╣ ╚═╩═╝` |
| `markdown` | `\| - \|` style; no top or bottom border |
| `standard` | No outer frame, no column separators, `─` header underline, space between columns |
| `hidden` / `none` | No borders; space between columns |

### `border-top`, `border-right`, `border-bottom`, `border-left`

Same two-form grammar as the identically-named [block element properties](#border-left-border-right-border-top-border-bottom):
a quoted string is a literal glyph override for that outer edge; a bareword
value is the standard CSS shorthand (`<style>`, `<style> <color>`, or
`<width> <style> <color>`, width ignored), where `<style>` is a
[`border-style`](#border-style) preset name and only that one edge's glyph
is taken from it. `none` (bareword, or a value that resolves to no glyph)
removes that edge's line entirely, corners included.

```css
table { border-top: "═"; }              /* literal glyph */
table { border-top: double; }           /* double preset's top glyph only - rest of the table stays whatever border-style set */
table { border-top: 1px double red; }   /* same glyph, red; "1px" ignored */
table { border-style: markdown; border-top: solid; }  /* adds a top edge markdown doesn't have by default */
```

For `border-left`/`border-right` specifically, resolving to no glyph (`none`)
also clears that side's corner and header/row-separator junction glyphs on
every line — "no left border" means the whole left frame, not just the
plain vertical divider in data rows.

Internal separator lines (the header divider and between-row dividers) always
reuse `border-top`'s resolved fill character rather than having their own —
every built-in preset already keeps these identical, and a table with two
different dash styles isn't a realistic use case.

### `border-top-mid`, `border-bottom-mid`, `border-left-mid`, `border-right-mid`, `border-center`
`"<string>"` | `'<string>'`. Literal-glyph-only (no shorthand grammar — these
have no real-CSS analog to converge toward, the same reasoning as block's
`border-*-corner` properties). Override the junction characters a preset
normally supplies:

| Property | Position |
|----------|----------|
| `border-top-mid` | T-junction where a column separator meets the outer top border (e.g. `┬`) |
| `border-bottom-mid` | T-junction where a column separator meets the outer bottom border (e.g. `┴`) |
| `border-left-mid` | T-junction where the header divider or a row divider meets the left edge (e.g. `├`) — header and row dividers always share this glyph, so one property covers both |
| `border-right-mid` | T-junction where the header divider or a row divider meets the right edge (e.g. `┤`) — same header/row sharing |
| `border-center` | Cross-junction at internal column/row intersections (e.g. `┼`) — same header/row sharing |

Not inherited.

### `border-top-left-corner`, `border-top-right-corner`, `border-bottom-left-corner`, `border-bottom-right-corner`
`"<string>"` | `'<string>'`. Literal-glyph-only override for one outer corner
(e.g. `┌`), identical model to the block element properties of the same
name. Not inherited.

### Edge toggles

Each of these accepts `none` to disable, or any other value to enable.

| Property | Controls |
|----------|---------|
| `border-header: none` | Separator line between header and data rows |
| `border-columns: none` | Vertical separator between columns |
| `border-rows: solid` | Horizontal separator between every data row (off by default) |

### Border color

| Property | Example | Notes |
|----------|---------|-------|
| `border-color` | `border-color: #555566` | ANSI color fallback for any border character without its own edge-specific override below. Applied directly (no fallback available) to internal separator lines, which have no per-edge color property. |
| `border-top-color`, `border-right-color`, `border-bottom-color`, `border-left-color` | `border-top-color: #ff0000` | Per-edge override, same as the identically-named block element properties. Falls back to `border-color` when unset. |

### `caption-side`

`top` (default) | `bottom`. Controls where the `<caption>` element is rendered relative to the table rows. `top` renders the caption above the top border; `bottom` renders it below the bottom border. Set on the `<table>` element (via CSS rule or `style=` attribute). Not inherited.

---

## `<style>` Tags

A `<style>` element anywhere in the HTML is parsed as a CSS stylesheet and
applied for that `Render()` call only — rules do not persist to subsequent
calls. Rules in `<style>` tags override the base stylesheet at equal
specificity (same cascade position as a later `<link>` in a browser).

```html
<style>
  td.highlight { color: #ff9e64; font-weight: bold; }
</style>
<table>...</table>
```

---

## Inline `style` Attribute

Any element can carry a `style=""` attribute with a declaration list. Inline
styles win over all stylesheet rules (same cascade position as in standard CSS).

```html
<table style="width: 100%; border-style: rounded">
<td style="color: #ff9e64; max-width: 40%">
```

The same property names and value forms apply as in the stylesheet, including
`!important`. A normal (non-`!important`) inline declaration wins over any
normal stylesheet rule but loses to an `!important` stylesheet rule; an
`!important` inline declaration wins over everything, including `!important`
stylesheet rules.

---

## Table — Width and Full-Width Expansion

Set `width: 100%` on a `<table>` (via a CSS rule or class) to expand it to
the renderer's terminal width. Flexible columns (no `width` or `width: Nch`)
share remaining space evenly, respecting `max-width` caps.

---

## Color Values

Color strings are parsed using CSS Color Level 4 syntax. The following formats are supported:

| Format | Example | Notes |
|--------|---------|-------|
| 6-digit hex | `#ff6600` | Standard `#rrggbb` |
| 3-digit hex | `#f60` | Expands to `#ff6600` |
| 8-digit hex | `#ff660080` | `#rrggbbaa` with alpha channel |
| 4-digit hex | `#f608` | `#rgba` with alpha channel |
| Named color | `red`, `cornflowerblue` | Full W3C named color list |
| `rgb()` | `rgb(255, 102, 0)` | Space or comma separated |
| `rgba()` | `rgba(255, 102, 0, 0.5)` | Fourth value is alpha 0–1 |
| `hsl()` | `hsl(24, 100%, 50%)` | Hue, saturation, lightness |
| `hsla()` | `hsla(24, 100%, 50%, 0.5)` | With alpha |
| `hwb()` | `hwb(24 0% 0%)` | CSS Color Level 4 |
| `transparent` | `transparent` | Fully transparent (renders as black) |

Color values are downsampled to the terminal's color capability at render time:
- **TrueColor terminals** (`COLORTERM=truecolor`): full 24-bit RGB
- **256-color terminals**: quantized to the nearest xterm-256 palette entry
- **16-color terminals**: quantized to the nearest ANSI basic color
- **No color** (`NO_COLOR=1` or non-TTY): color is stripped

Bare ANSI index numbers (e.g. `"214"`) are not supported; use `#rrggbb` or a named color instead.

---

## What Is Not Implemented

- `px`, `em`, `rem`, `vw`, `vh`, and other CSS units (ignored; use bare integers or `ch`)
- CSS math functions: `calc()`, `min()`, `max()`, `clamp()`
- CSS variables (`--my-var`)
- Media queries (`@media`)
- `@font-face`, `@keyframes`, or any other at-rules
- Pseudo-classes and pseudo-elements
- The two-value `<width> <style>` form (no color) of `border`/`border-top`/`border-right`/`border-bottom`/`border-left` — see those sections
- `display: flex`, `display: grid`, `display: list-item`, or any other display values beyond `block`, `inline`, `inline-block`, `table`, and `none` — not yet implemented, not a permanent boundary; see `SCROLLING.md`'s "Flexbox" section for the planned design direction
- `flex`, `grid`, or positioned layout — same as above
- Multi-line cell content when `white-space: nowrap` is set on a `td`/`th`
- `border-spacing` / cell padding (column separator is always a single character)
