# termrender CSS Reference

termrender renders a restricted subset of HTML+CSS to terminal strings via
[lipgloss](https://github.com/charmbracelet/lipgloss). This document lists
every selector form, HTML element, and CSS property that is recognized.
Anything not listed here is silently ignored.

---

## Selectors

| Form | Example |
|------|---------|
| Element | `th { }` |
| Class | `.num { }` |
| Multiple classes | `.warn.big { }` |
| Element + class(es) | `tr.unseen { }`, `p.a.b { }` |
| ID | `#intro { }` |
| Element + ID | `h1#title { }` |
| Attribute presence | `a[href] { }` |
| Attribute value | `td[data-role=header] { }` |
| Descendant (space) | `tr.unseen td { }` |
| Child (`>`) | `div > p { }` |
| Pseudo-class | `li:first-child { }`, `tr:nth-child(odd) { }` |
| Pseudo-element | `p::before { content: "→ "; }`, `p::after { content: " ←"; }` |
| Comma-separated (any of the above) | `h1, h2, h3 { }` |

**Specificity** follows CSS rules: ID = 100, class / pseudo-class / attribute = 10,
element = 1. Higher specificity wins; equal specificity last-write wins.

**Supported pseudo-classes:** `:first-child`, `:last-child`, `:nth-child(odd)`,
`:nth-child(even)`. Full `An+B` expressions are not supported.

**Supported pseudo-elements:** `::before` and `::after` (also accepted with a single
colon: `:before`, `:after`). Inject inline text at the start or end of an element's
content. Requires the `content` property; without it the rule has no effect.
All combinator and element-matching forms work: `div p::before`, `.warn::after`, etc.

**Supported attribute operators:** `[attr]` (presence) and `[attr=val]` (exact
match). Compound operators (`~=`, `^=`, `$=`, `*=`) are not supported; selectors
containing them never match.

**Not supported:** `:not()`, `:hover`, `:focus`, and other pseudo-classes beyond
the four listed above; adjacent (`+`) and general sibling (`~`) combinators.

---

## Inheritance

The following properties inherit from parent to child when no direct rule
applies to the child element:

`color` · `font-weight` · `font-style` · `font-variant` · `text-decoration` · `text-align` · `white-space` · `text-transform`

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
| `blockquote` | Inline content followed by a newline; default `border-left: │; border-left-color: #555555; padding-left: 1; padding-right: 2` |
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
| `abbr` | Abbreviation. When a `title` attribute is present the expansion is appended inline as ` (expansion)` — e.g. `<abbr title="HyperText Markup Language">HTML</abbr>` renders as `HTML (HyperText Markup Language)`. |
| `small` | Fine print / secondary text (default: `color: #888888`). No font-size reduction is possible in terminals. |
| `q` | Inline quotation; content is wrapped in Unicode curly quotes `"…"`. |
| `code` | Inline styled span |
| `pre` | Raw text block; `white-space: pre` by default; child elements are styled |
| `br` | Line break (inline or block) |
| `hr` | Full-width `─` line |
| `ul` | Unordered list; `• ` prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `ol` | Ordered list; decimal prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `li` | List item; content word-wraps with a hanging indent aligned to the prefix |
| `dl` | Definition list block (default: `display: block; margin-bottom: 1`) |
| `dt` | Definition term; rendered as a bold block (default: `display: block; font-weight: bold`) |
| `dd` | Definition description; rendered as an indented block (default: `display: block; padding-left: 4`) |
| `figure` | Block container for self-contained content such as illustrations or code (default: `display: block`). Style with `margin-left`/`margin-right` to indent. |
| `figcaption` | Caption for the nearest `<figure>` ancestor (default: `display: block; font-style: italic`) |
| `table` | See table section below |
| `colgroup` | Column group; direct child of `<table>`. A `span` attribute (default 1) applies the group's own `style=` across that many columns when no `<col>` children are present. Style via `style=` or CSS selectors. |
| `col` | Column descriptor inside `<colgroup>`. `span` attribute (default 1) repeats the column's declarations across N consecutive columns. Supports `width`, `min-width`, `max-width`, `text-align`, `color`, `background-color`, `font-weight`, `font-style`, `text-decoration` via `style=` or CSS. A `width` HTML attribute is treated as an absolute char count. Cell-level declarations take priority over `<col>` declarations. |
| `thead`, `tbody`, `tfoot` | Transparent wrappers inside `<table>` |
| `tr` | Table row; first `<tr>` containing `<th>` is the header |
| `th`, `td` | Table cells |
| `div` | Generic block container (default: `display: block`; no other UA styles) |
| `section`, `article`, `aside`, `header`, `footer`, `main`, `nav` | HTML5 sectioning elements; all default to `display: block` with no other UA styles. Style freely with CSS. |

---

## CSS Properties — Inline / Block Elements

These apply to any matched element and control text rendering.
`normal` / `none` values explicitly cancel an inherited value.

#### `display`
`block` | `inline` | `inline-block` | `none`. Controls layout. `block` emits a newline after content and respects `margin-top`/`margin-bottom`. `inline` renders with no newline. `inline-block` is like `inline` but respects `width`. `none` hides the element and all its children. Not inherited. Defaults: `p`, `h1`–`h6`, `blockquote`, `pre`, `div`, and common HTML5 sectioning elements default to `block`; all others default to `inline`.

#### `color`
`#rrggbb`, named color. Foreground color. Inherited.

#### `background-color`
`#rrggbb`, named color. Background color. Not inherited.

#### `font-weight`
`bold` | `normal`. `normal` cancels inherited bold. Inherited.

#### `font-style`
`italic` | `normal`. `normal` cancels inherited italic. Inherited.

#### `text-decoration`
`underline` | `line-through` | `none` | `normal`. `none`/`normal` cancels both underline and strikethrough. Inherited.

#### `text-align`
`left` | `center` | `right`. Effective on cells when column has a width. Inherited.

#### `margin-top`
Integer line count (e.g. `1`). Extra blank lines above a block element. Adjacent margins collapse: the larger wins. Not inherited.

#### `margin-bottom`
Integer line count (e.g. `1`). Extra blank lines below a block element. Collapses with the next element's `margin-top`. Not inherited.

#### `margin-left`
Integer (e.g. `4`) or `auto`. Spaces prepended to every line of a block element, outside any `border-left`. Not inherited.

#### `margin-right`
Integer (e.g. `4`) or `auto`. Spaces appended to every line of a block element, outside any `border-right`. Not inherited.

**`auto` margins:** When an element has an explicit `width` set, `margin-left: auto` and/or `margin-right: auto` distribute the remaining space. Both `auto` centers the element; only `margin-left: auto` right-aligns it; only `margin-right: auto` left-aligns it (fills trailing space). Without an explicit `width` the element already fills the available width and auto margins have no visible effect.

#### `width`
`40` or `50%`. Fixed or percentage width for block and `inline-block` elements. For block elements, `width: 100%` fills the renderer width; margins and border characters are subtracted so the total visual line equals the specified width. Not inherited.

#### `padding-left`
`4` or `4ch`. Left padding in rune columns; applies to block elements. Not inherited.

#### `padding-right`
`4` or `4ch`. Right padding in rune columns; applies to block elements. Not inherited.

#### `padding-top`
Integer (e.g. `2`). Blank lines inserted above content, inside `border-top`. Each blank row is as wide as the content area so left/right borders and padding align correctly. Not inherited.

#### `padding-bottom`
Integer (e.g. `2`). Blank lines inserted below content, inside `border-bottom`. Same width semantics as `padding-top`. Not inherited.

#### `height`
Integer line count (e.g. `5`). Content-box height in lines. If the rendered content has fewer lines it is padded with blank lines; if it has more and `overflow: hidden`/`clip` is set it is truncated. Without an overflow setting, extra content is visible. Not inherited.

#### `white-space`
`normal` | `nowrap` | `pre` | `pre-wrap` | `pre-line`. How text-node whitespace is handled. Inherited. Default `normal` for block/inline elements. Block elements with `normal` word-wrap long lines at the available content width, breaking at word boundaries. `nowrap` disables word wrapping. `pre` preserves all whitespace and disables wrapping. `pre-wrap` and `pre-line` preserve newlines but still allow wrapping. **`td` and `th` default to `nowrap`** (single-line truncation); set `white-space: normal` on a cell or ancestor to enable multi-line wrapping instead. Content that is already multi-line (lists, `<br>` tags, nested block elements) is not re-wrapped.

#### `overflow`
`visible` | `hidden` | `clip`. Controls whether content that exceeds an explicit `width` is clipped. Default `visible`: text overflows the box. `hidden` and `clip` both truncate each line to the content width. **Requires an explicit `width`**; without one the element already fills the available width. `text-overflow` controls the truncation marker. Not inherited.

#### `text-overflow`
`clip` | `ellipsis` | `"‹str›"`. The truncation marker appended to lines clipped by `overflow: hidden`/`clip`. Only effective when `overflow: hidden` or `overflow: clip` and `white-space: nowrap` and an explicit `width` are all set. Default `clip` (no marker). `ellipsis` appends `…`. A quoted string (e.g. `text-overflow: "+"`) uses that string as the marker. Not inherited. **Note:** for table cells, `overflow: hidden` is implicit and the default is `ellipsis` rather than `clip`.

#### `font-variant`
`small-caps` | `normal`. `small-caps` uppercases all text content (terminal rendering cannot distinguish small-cap glyphs from full capitals). `normal` cancels an inherited value. Inherited. When both `font-variant: small-caps` and `text-transform` are set, `text-transform` wins.

#### `text-transform`
`none` | `uppercase` | `lowercase` | `capitalize` | `superscript` | `subscript`. Case/script transformation applied to text content. Inherited. `capitalize` uppercases the first letter of each whitespace-separated word. `superscript` and `subscript` replace each character with its Unicode superscript or subscript equivalent where one exists; characters with no Unicode equivalent are passed through unchanged.

#### `content`
`"<string>"` | `'<string>'` | `none` | `normal`. Text injected by `::before` or `::after` pseudo-element rules. A quoted string literal is the injected text; `none` and `normal` suppress injection. Other CSS content values (`attr()`, `counter()`, etc.) are not supported. Not meaningful on regular elements. Not inherited.

#### `border-style`
`normal` | `rounded` | `thick` | `double` | `markdown` | `hidden` | `none`. Applies a named border preset as a shorthand for all individual border properties. Individual `border-*` properties set on the same element override the preset for that edge (e.g. `border-top: ═` overrides the fill but keeps preset corners). `hidden`/`none` clears all borders. Not inherited.

#### `border-left`
`<string>` | `none`. Character(s) prepended to every rendered line of a block element. `none` or unset = no border. Not inherited.

#### `border-right`
`<string>` | `none`. Character(s) appended to every rendered line of a block element. Not inherited.

#### `border-top`
`<string>` | `none`. Fill character repeated across the full block width (minus margins) to draw a horizontal rule above the content. Not inherited.

#### `border-bottom`
`<string>` | `none`. Fill character repeated across the full block width (minus margins) to draw a horizontal rule below the content. Not inherited.

#### `border-left-color`
`#rrggbb`, named color. ANSI color applied to the left border character. Not inherited.

#### `border-right-color`
`#rrggbb`, named color. ANSI color applied to the right border character. Not inherited.

#### `border-top-color`
`#rrggbb`, named color. ANSI color applied to the top border rule. Not inherited.

#### `border-bottom-color`
`#rrggbb`, named color. ANSI color applied to the bottom border rule. Not inherited.

#### `border-top-left-corner`
`<string>`. Character placed at the left end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-top-right-corner`
`<string>`. Character placed at the right end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-bottom-left-corner`
`<string>`. Character placed at the left end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

#### `border-bottom-right-corner`
`<string>`. Character placed at the right end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

---

## CSS Properties — Lists (`ul`, `ol`)

These properties apply to `<ul>` and `<ol>` elements and control list rendering.
Item content word-wraps at the available content width, with continuation lines
hanging-indented to align under the first line of text (not the prefix).

| Property | Values | Notes |
|----------|--------|-------|
| `list-style-type` | See table below | Prefix character for each `<li>`. Not inherited. Default: `disc` for `<ul>`, `decimal` for `<ol>`. |
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

Numeric prefixes (`decimal`, `*-roman`, `*-alpha`) are right-aligned within a
fixed-width column sized to the widest prefix in the list (e.g. `" 1."` aligns
with `"10."` in a ten-item list).

Nested lists are supported: a `<ul>` or `<ol>` anywhere inside an `<li>` is
rendered recursively, indented within the parent item's content width.

**Not supported:** `<ol start="N">` (counter always begins at 1);
`list-style-image`; `list-style-position`; `list-style` shorthand; the
`type` HTML attribute on `<ol>` (use `list-style-type` CSS instead).

---

## Size Values

Wherever a size is accepted (cell `width`, `min-width`, `max-width`), the
following forms are recognized:

| Form | Example | Meaning |
|------|---------|---------|
| Bare integer | `14` | Fixed rune count |
| `ch` unit | `14ch` | Fixed rune count (same as bare integer) |
| Percentage | `50%` | Fraction of the available content width |

Pixel (`px`), `em`, `rem`, and other CSS units are ignored.

The **available content width** is the terminal width minus border/separator
overhead. For a `width: 100%` table it equals the full terminal width minus
the sum of all separator characters.

---

## CSS Properties — Cell Sizing (`th`, `td`)

The `width` HTML attribute on `<th>`/`<td>` is equivalent to CSS `width`
(always absolute). CSS `width` with a `%` value overrides the HTML attribute.

| Property | Example | Notes |
|----------|---------|-------|
| `width` | `width: 14` or `width: 25%` | Fixed or percentage column width; immune to expand/shrink |
| `min-width` | `min-width: 8` or `min-width: 10%` | Column will not shrink below this value |
| `max-width` | `max-width: 40` or `max-width: 30%` | Column will not expand beyond this value |
| `white-space` | `nowrap` \| `normal` | Controls cell line-wrapping. Default `nowrap`: content is clipped to one line using `text-overflow`. Set to `normal` (or inherit it from a parent) to enable multi-line word-wrapping; `text-overflow` is then ignored. |
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
| `normal` | `┌─┬─┐ │ │ ├─┼─┤ └─┴─┘` (default) |
| `rounded` | `╭─┬─╮ │ │ ├─┼─┤ ╰─┴─╯` |
| `thick` | `┏━┳━┓ ┃ ┃ ┣━╋━┫ ┗━┻━┛` |
| `double` | `╔═╦═╗ ║ ║ ╠═╬═╣ ╚═╩═╝` |
| `markdown` | `\| - \|` style; no top or bottom border |
| `standard` | No outer frame, no column separators, `─` header underline, space between columns |
| `hidden` / `none` | No borders; space between columns |

### Edge toggles

Each property accepts `none` to disable that edge, or any other value to
enable it. The four outer edges also suppress the corresponding corner
characters on horizontal separator lines.

| Property | Controls |
|----------|---------|
| `border-top: none` | Outer top border line |
| `border-bottom: none` | Outer bottom border line |
| `border-left: none` | Outer left edge on all rows and separator lines |
| `border-right: none` | Outer right edge on all rows and separator lines |
| `border-header: none` | Separator line between header and data rows |
| `border-columns: none` | Vertical separator between columns |
| `border-rows: solid` | Horizontal separator between every data row (off by default) |

### Border color

| Property | Example | Notes |
|----------|---------|-------|
| `border-color` | `border-color: #555566` | ANSI color applied to all border characters |

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

The same property names and value forms apply as in the stylesheet.

---

## Table — Width and Full-Width Expansion

Set `width: 100%` on a `<table>` (via a CSS rule or class) to expand it to
the renderer's terminal width. Flexible columns (no `width` or `width: Nch`)
share remaining space evenly, respecting `max-width` caps.

---

## Color Values

Any value accepted by lipgloss is accepted here: `#rrggbb` hex, ANSI color
names (`red`, `green`, …), and ANSI 256-color numbers (`"214"`).

---

## What Is Not Supported

- `px`, `em`, `rem`, `vw`, `vh`, and other CSS units (ignored; use bare integers or `ch`)
- CSS variables (`--my-var`)
- `!important`
- Media queries (`@media`)
- `@font-face`, `@keyframes`, or any other at-rules
- Pseudo-classes and pseudo-elements
- Multi-value `border-top`/`border-bottom` shorthand (e.g. `border-top: 1px solid red`) —
  use a single fill character (e.g. `border-top: ─`) and `border-top-color` for color
- `display: flex`, `display: grid`, `display: table`, `display: list-item`, or any other display values beyond `block`, `inline`, `inline-block`, and `none`
- `margin`, `padding` shorthand (use `margin-top`, `margin-bottom`, `padding-top`, `padding-bottom`, `padding-left`, `padding-right`)
- `flex`, `grid`, or positioned layout
- Multi-line cell content when `white-space: nowrap` (the default for `td`/`th`); set `white-space: normal` to opt in to word wrapping
- `border-spacing` / cell padding (column separator is always a single character)
