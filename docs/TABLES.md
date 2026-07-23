# Styling Tables

`<table>` is one of the more complicated corners of htmlterm — real HTML
table CSS is already complicated, and rendering it as character-grid ASCII
art adds its own layer on top (there's no pixel-perfect equivalent of a table
border, only a chosen set of glyphs). This page pulls together everything
about styling a table's frame and cells in one place; CSS.md's own table
entries are one-line pointers here.

## Quick reference

| Property | Applies to | Notes |
|---|---|---|
| `border-style` | `<table>` | Whole-frame preset — see below. Not a real-CSS property name reused; htmlterm's own vocabulary. |
| `border-top`/`-right`/`-bottom`/`-left` | `<table>` | Literal glyph or shorthand grammar, same as block elements |
| `border-top-mid`/`-bottom-mid`/`-left-mid`/`-right-mid`/`border-center` | `<table>` | Junction glyph overrides — no real-CSS equivalent |
| `border-top-left-corner` etc. | `<table>` | Corner glyph overrides |
| `border-header`/`border-columns`/`border-rows` | `<table>` | On/off edge toggles — no real-CSS equivalent |
| `border-color`, `border-*-color` | `<table>` | Whole-frame / per-edge color |
| `margin`, `padding` | `<table>` | Work like any block element — see "Margin and padding" below |
| `width`/`min-width`/`max-width` | `<th>`/`<td>` | Column sizing |
| `white-space`, `text-overflow` | `<th>`/`<td>` | Wrapping vs. truncation |
| `vertical-align` | `<th>`/`<td>` | Content placement within row height |
| `caption-side` | `<table>` | Caption above/below the frame |

## Border-style presets

`border-style` on `<table>` sets the *complete* border character set in one
declaration — a deliberate design choice: writing out a dozen individual
glyph properties for every table would be unusable, and the realistic set of
table "looks" is small enough that a handful of named presets covers it.
Individual `border-*` properties on the same element still override the
preset for that one edge.

```css
table { border-style: solid; }   /* default */
```
```
┌──────┬───┐
│Name  │Qty│
├──────┼───┤
│Apple │3  │
│Banana│5  │
└──────┴───┘
```

```css
table { border-style: rounded; }
```
```
╭──────┬───╮
│Name  │Qty│
├──────┼───┤
│Apple │3  │
│Banana│5  │
╰──────┴───╯
```

```css
table { border-style: heavy; }
```
```
┏━━━━━━┳━━━┓
┃Name  ┃Qty┃
┣━━━━━━╋━━━┫
┃Apple ┃3  ┃
┃Banana┃5  ┃
┗━━━━━━┻━━━┛
```

```css
table { border-style: double; }
```
```
╔══════╦═══╗
║Name  ║Qty║
╠══════╬═══╣
║Apple ║3  ║
║Banana║5  ║
╚══════╩═══╝
```

```css
table { border-style: markdown; }
```
```
|Name  |Qty|
|------|---|
|Apple |3  |
|Banana|5  |
```

```css
table { border-style: standard; }  /* no outer frame, no column rules */
```
```
Name   Qty
────── ───
Apple  3  
Banana 5  
```

```css
table { border-style: hidden; }  /* (or "none") no borders at all */
```
```
Name   Qty
Apple  3  
Banana 5  
```

**Note:** these preset *names* aren't real CSS `border-style` keywords
either — real CSS's `border-style` is a per-edge line-style keyword
(`solid`/`dashed`/`dotted`/etc.), not a whole-table-frame preset. Only the
*concept* of "solid"/"double"/"hidden"/"none" borders carries over in name;
`rounded`/`heavy`/`markdown`/`standard` are htmlterm-specific.

## Edge toggles

```css
table { border-rows: solid; }   /* off by default */
```
```
┌──────┬───┐
│Name  │Qty│
├──────┼───┤
│Apple │3  │
├──────┼───┤
│Banana│5  │
└──────┴───┘
```

```css
table { border-header: none; }
```
```
┌──────┬───┐
│Name  │Qty│
│Apple │3  │
│Banana│5  │
└──────┴───┘
```

```css
table { border-columns: none; }
```
```
┌──────┬───┐
│Name  Qty│
├──────┼───┤
│Apple 3  │
│Banana5  │
└──────┴───┘
```

Note in that last example: the top/bottom border still draws the `┬`/`┴`
junction glyphs even though the column divider itself is gone (a minor,
harmless visual inconsistency — `border-columns: none` only removes the
vertical rule in the body, not the outer frame's own junction characters).

None of `border-header`/`border-columns`/`border-rows` are real CSS
properties — they have no equivalent at all in the spec, which has no
concept of an on/off toggle for a table's internal rule lines.

## Junction and corner glyph overrides

`border-top-mid`/`border-bottom-mid`/`border-left-mid`/`border-right-mid`/
`border-center` override the individual T-junction and cross-junction
characters a preset supplies — again, no real-CSS analog (CSS has no notion
of styling where a divider "joins" a border):

```css
table {
  border-top-mid: "╥"; border-bottom-mid: "╨";
  border-left-mid: "╠"; border-right-mid: "╣";
  border-center: "╫";
}
```
```
┌──────╥───┐
│Name  │Qty│
╠──────╫───╣
│Apple │3  │
│Banana│5  │
└──────╨───┘
```

`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner` override one outer
corner character each, the same literal-only model as the identically-named
block-element properties.

**A deviation from literal CSS semantics worth calling out explicitly:**
setting `border-left: none` (or `border-right: none`) doesn't just remove
that one vertical rule — it also clears that side's corner and every
internal junction glyph (header divider, row dividers) on every horizontal
line, not just the outer frame:

```css
table { border-left: none; }
```
```
──────┬───┐
Name  │Qty│
──────┼───┤
Apple │3  │
Banana│5  │
──────┴───┘
```

Real CSS's `border-left: none` never touches the top/bottom border at all —
in pixel rendering there's no "corner glyph" concept to begin with, since a
border is just where two independent lines happen to meet in space. In
ASCII art, though, leaving the `┌`/`├`/`└` corner and junction glyphs in
place with no vertical rule for them to connect to would look like a
dangling, disconnected character — so removing them is the deliberate,
intentional interpretation of "no left frame at all" for this renderer, not
an oversight. If you want a literal one-sided-rule look without touching the
rest of the frame, use the corner-override properties above to substitute a
plain fill character instead of `none`.

## Margin and padding

`margin`/`padding` on `<table>` itself work exactly like any other block
element — including the case that's easy to assume doesn't apply to tables
at all:

```css
table { padding: 1; border-style: solid; }
```

Padding adds blank space *inside* the border, between the frame and the
outermost cells — this is genuinely how real CSS's `border-collapse:
separate` (the actual default, contrary to the common assumption that
tables have no meaningful padding) behaves too; it's just rarely used
because most real-world tables reach for `border-spacing`/cell padding
instead of table-level `padding`. `margin-left`/`margin-right`, including
`margin: auto` centering, work the same way they do for other block
elements (see CSS.md's `margin` entries) — and `auto`-centering a `<table>`
specifically is one of the few auto-margin cases real browsers support
without requiring extra tricks, which htmlterm matches.

`margin-top`/`margin-bottom` also work like any other block element:
collapsing with adjacent siblings' margins the same way (larger value wins),
whether the `<table>` is at the document root or nested inside another
block.

## Not supported

- **`border-collapse`/`border-spacing`.** There is no concept of collapsed
  vs. separated table borders — htmlterm's table border model is a single
  fixed frame-drawing system, closer in spirit to CSS's `separate` mode
  (borders are drawn as their own distinct lines/columns, not merged with
  cell borders) but with no spacing control at all. The gap between
  adjacent columns is always exactly the frame's own separator character —
  0 columns wide (`border-columns: none`) or 1 (any other value) — never a
  configurable width, and there is no equivalent gap control between rows
  either (`border-rows` only toggles a divider *line* on or off, never
  blank spacing between rows). Since table-level `padding` genuinely does
  work (see above), a configurable `border-spacing` isn't architecturally
  out of reach if ever needed — the column-gap width is already computed
  from `runeLen()` of the separator string rather than hardcoded to 1
  character, so widening it is a smaller change than it might look like;
  it's just not exposed as a CSS property today, and inter-row spacing
  would need new logic in the row-composition pass.
- **`border-spacing`'s cell padding half** — real CSS also lets
  `border-spacing`/cell `padding` add space *around* each cell individually
  (visible as gaps between cell content and its own border under
  `separate`); htmlterm has no per-cell padding-via-spacing concept, only
  the whole-table `padding` described above.
- **Multi-line cell content combined with `white-space: nowrap`** — a
  `nowrap` cell is always clipped to one line (see `text-overflow`), never
  both non-wrapping and multi-line.

## See Also

- CSS.md's `border`, `margin`, `padding`, `width` entries — the same
  properties and value forms apply to `<table>`/`<th>`/`<td>` as to any
  other element, documented once there rather than duplicated here.
- `docs/RENDERING.md` for how table layout fits into the wider box-model
  pipeline.
