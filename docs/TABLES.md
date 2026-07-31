# Styling Tables

`<table>` is one of the more complicated corners of htmlterm — real HTML
table CSS is already complicated, and rendering it as character-grid ASCII
art adds its own layer on top (there's no pixel-perfect equivalent of a table
border, only a chosen set of glyphs). This page pulls together everything
about styling a table's frame and cells in one place; CSS.md's own table
entries are one-line pointers here.

**A bare `<table>` with no CSS at all renders completely borderless**,
matching a real browser exactly (`border-style`'s initial value is `none`;
`border-collapse`'s initial value is `separate`). There is no hidden
fallback "default look" here — every visible border comes from CSS you
actually wrote.

## Quick reference

| Property | Applies to | Notes |
|---|---|---|
| `border-style` | `<table>`, `<th>`/`<td>` | Whole-box preset — see below. Not a real-CSS property value set; htmlterm's own vocabulary |
| `border-top`/`-right`/`-bottom`/`-left` | `<table>`, `<th>`/`<td>` | Literal glyph or shorthand grammar, same as block elements |
| `border-top-left-corner` etc. | `<table>`, `<th>`/`<td>` | Corner glyph overrides |
| `border-top-junction`/`-right`/`-bottom`/`-left`/`-center` | `<table>`, `<th>`/`<td>` | Literal junction-glyph overrides (T-shapes/cross) under `border-collapse: collapse` — see "Junction and corner overrides" below |
| `border-top-left-junction` etc. | `<table>` only | Literal junction-glyph overrides (corner-shapes anywhere in the grid) under `border-collapse: collapse` — no cell-level equivalent (redundant with cell-level `border-*-corner`) |
| `border-color`, `border-*-color` | `<table>`, `<th>`/`<td>` | Whole-box / per-edge color |
| `margin`, `padding` | `<table>` | Work like any block element — see "Margin and padding" below |
| `width`/`min-width`/`max-width` | `<th>`/`<td>` | Column sizing |
| `white-space`, `text-overflow` | `<th>`/`<td>` | Wrapping vs. truncation |
| `vertical-align` | `<th>`/`<td>` | Content placement within row height |
| `caption-side` | `<table>` | Caption above/below the table |
| `border-collapse: separate` | `<table>` | The real-CSS default (also what unset falls back to): every `<th>`/`<td>` gets its own independent border box — see "`border-collapse: separate`" below |
| `border-collapse: collapse` | `<table>` | Adjacent cell/table borders merge into shared lines via real conflict resolution — see "`border-collapse: collapse`" below |
| `border-spacing` | `<table>` | Gap between cell boxes under `border-collapse: separate`. Default `0` |

## Border-style presets

**Not a real CSS property value set.** Real CSS's `border-style` is a
per-edge line-style keyword (`solid`/`dashed`/`dotted`/`groove`/`ridge`/
`inset`/`outset`/`double`/`none`/`hidden`) — htmlterm's `border-style`
instead names a *complete glyph-set preset*
(`solid`/`rounded`/`heavy`/`double`/`markdown`/`standard`/`hidden`/`none`).
Only the *concept* of "solid"/"double"/"hidden"/"none" carries the name
over from real CSS; `rounded`/`heavy`/`markdown`/`standard` are
htmlterm-specific, and even the overlapping names pick a specific
box-drawing character set rather than a line style. This is a deliberate
design choice, not an accident: writing out a dozen individual glyph
properties for every border would be unusable, and the realistic set of
"looks" is small enough that a handful of named presets covers it.
Individual `border-*` properties on the same element still override the
preset for that one edge. This property is not table-specific — it's the
same one any block element uses; a `<table>`/`<th>`/`<td>` box just uses it
like any other bordered box would.

The examples below all use `border-collapse: collapse` with the same
`border-style` set on the table and its cells, since that's how you get a
full shared grid out of these presets (see "`border-collapse: collapse`"
below for why a table-level preset alone doesn't reach into its cells):

```css
table, td, th { border-style: solid; }
table { border-collapse: collapse; }
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
table, td, th { border-style: rounded; }
table { border-collapse: collapse; }
```
```
╭──────┬───╮
│Name  │Qty│
├──────┼───┤
│Apple │3  │
├──────┼───┤
│Banana│5  │
╰──────┴───╯
```

```css
table, td, th { border-style: heavy; }
table { border-collapse: collapse; }
```
```
┏━━━━━━┳━━━┓
┃Name  ┃Qty┃
┣━━━━━━╋━━━┫
┃Apple ┃3  ┃
┣━━━━━━╋━━━┫
┃Banana┃5  ┃
┗━━━━━━┻━━━┛
```

```css
table, td, th { border-style: double; }
table { border-collapse: collapse; }
```
```
╔══════╦═══╗
║Name  ║Qty║
╠══════╬═══╣
║Apple ║3  ║
╠══════╬═══╣
║Banana║5  ║
╚══════╩═══╝
```

```css
table, td, th { border-style: markdown; }
table { border-collapse: collapse; }
```
```
|Name  |Qty|
|Apple |3  |
|Banana|5  |
```

`markdown` only supplies left/right glyphs (no top/bottom), so it never
draws any row divider on its own — including the header-separator row real
Markdown tables have — add an explicit `border-bottom` on the cells that
need one if you want a divider.

```css
table, td, th { border-style: standard; }
table { border-collapse: collapse; }
```
```
Name  Qty
Apple 3  
Banana5  
```

`standard` supplies no glyphs on any edge at all — it's a documented
no-op preset, useful mainly as an explicit "no border" marker distinct
from simply not setting `border-style`. With no border glyphs anywhere,
there's also no `border-spacing` gap to space columns apart under
`collapse` (that's a `separate`-mode-only property) — hence `Banana`/`5`
touching directly above.

## Corner glyph overrides

`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner` override one outer
corner character each — a literal-glyph-only property (no preset keywords),
the same model on `<table>`/`<th>`/`<td>` as on any other block element:

```css
table {
  border-style: solid; border-spacing: 0;
  border-top-left-corner: '1'; border-top-right-corner: '2';
  border-bottom-left-corner: '3'; border-bottom-right-corner: '4';
}
```
```
1───2
│A  │
3───4
```

**`border-left: none`/`border-right: none` keep that side's corners** even
though the vertical rule itself disappears:

```css
table { border-style: solid; border-left: none; border-spacing: 0; }
```
```
┌─────┐
A  B  │
└─────┘
```

This matches how any other bordered block element handles `border-left:
none` in this renderer — a corner glyph is generated independently per
corner from whichever of its two adjacent edges resolve to a real border
(here, top and bottom still do), not tied to the side edge's own presence.

## `border-collapse: separate`

**The real-CSS default** (what unset `border-collapse` falls back to, since
`separate` *is* CSS's own initial value): every `<th>`/`<td>` gets its own
real, independent border box — ordinary `border`/`border-style`/
`border-color` CSS on the cell itself, resolved exactly the same way as on
any other element — with `border-spacing` as the gap between adjacent cell
boxes (and between the table's own border and its outermost cells,
including a table with zero columns of visible border at all, which is
exactly what makes a bare `<table>` render borderless: no CSS anywhere
means no cell ever gets a border box, full stop).

```css
table { border-collapse: separate; border-spacing: 1; }
td, th { border: solid; padding-left: 1; padding-right: 1; }
```
```

 ┌────────┐ ┌─────┐ 
 │ Name   │ │ Qty │ 
 └────────┘ └─────┘ 

 ┌────────┐ ┌─────┐ 
 │ Apple  │ │ 3   │ 
 └────────┘ └─────┘ 

 ┌────────┐ ┌─────┐ 
 │ Banana │ │ 5   │ 
 └────────┘ └─────┘ 

```

`border-spacing` takes one or two values — one applies to both axes; two
are horizontal then vertical (the opposite order from `gap`'s row-then-
column convention). **Default is `0`.** Real CSS defaults to `2px`, which is
imperceptible against a typical line-height — but a terminal "row" of
spacing is a full line (100% of a row's height, not the ~10% a pixel gap
would be against real text), so a literal `1`-character default would read
as a much more prominent gap than the real default it's meant to
approximate, indistinguishable from leftover border structure even with no
border set anywhere:

```css
border-spacing: 0;      /* cell borders touch directly, no gap (the default) */
border-spacing: 1;      /* 1 column and 1 row of spacing */
border-spacing: 2 1;    /* 2 columns horizontal, 1 row vertical */
```

Cells with no border of their own render with no box at all — row height
still equalizes across the row (a plain cell just gets blank padding to
match its bordered neighbor's height), the same as a mix of bordered and
unbordered block elements would:

```css
table { border-collapse: separate; border-spacing: 1; }
```
```html
<tr><td style="border:solid">bordered</td><td>plain</td></tr>
```
```

 ┌────────┐ plain 
 │bordered│       
 └────────┘       

```

`colspan`/`rowspan` are supported: a spanning cell's own border box covers
every column/row it spans, including the `border-spacing` gap(s) between
them (its border/background runs continuously through what would otherwise
be a gap — there's no separate divider drawn through the middle of a single
cell's own box, matching real CSS). `vertical-align`/`text-align` apply per
cell exactly as they do outside a table. The table's own
`border`/`padding`/`margin` still wrap the whole assembled grid, exactly
like any other block element (including the fact that a table's own
`padding` genuinely works — see "Margin and padding" below).

**Known simplifications** (documented, not accidental):

- **Column-width allocation for border thickness uses one representative
  cell per column** (that column's header cell if the table has one, else
  the first row's cell there) — if cells within the same column set
  inconsistent border widths, their total rendered widths may not perfectly
  align. Matches how a real browser looks when a table's cell borders are
  genuinely inconsistent (an unusual authoring pattern).
- **A `colspan` cell's available content width doesn't reclaim the interior
  per-column border width** it swallows (only the `border-spacing` gap
  between the columns it spans) — its own box may end up very slightly
  narrower than the exact combined width of the columns below/above it,
  padded with blank space to stay rectangular. Only noticeable with
  unusually thick borders combined with colspan.

## `border-collapse: collapse`

The other real CSS border model: adjacent cell/table borders merge into
**shared grid lines** instead of each cell keeping its own independent box.
Real CSS's conflict-resolution algorithm (CSS 2.1 §17.6.2.1) picks a single
winner for each shared line segment from whichever elements border it —
htmlterm implements the same idea, adapted for its own style vocabulary and
lack of real border thickness:

1. **`hidden` always wins** — a `hidden` edge on either side suppresses that
   shared segment outright, regardless of what the other side sets.
2. **An edge with no border set (or explicit `none`) simply loses** — if
   only one side has a real border, it wins by default; if neither does,
   the segment is absent entirely (consuming no space at all — a
   collapsed-mode table with no borders anywhere is exactly as borderless
   as a `separate`-mode one with none).
3. **Otherwise, style precedence decides**: `double` > `heavy` > `solid` >
   `rounded` > `markdown`/`standard`/any literal-glyph style (real CSS's own
   `border-width` "widest wins" tier is skipped entirely — border thickness
   is a no-op deviation here, documented in COMPATIBILITY.md).
4. **A same-style tie goes to the cell over the table** (matching real
   CSS's own "cell beats table" rule at the tail of its hierarchy). For two
   adjacent cells' own borders — a case real CSS doesn't precisely specify
   — the tie goes to the *earlier* element in reading order: the row above
   wins a horizontal tie, the column to the left wins a vertical one.
   Verified directly against Firefox, Safari, and Chrome (not just an
   arbitrary choice this engine made up) — all three converge on the same
   behavior.

**Worked example of that tie-break:**

```css
th { border-bottom: "═"; }
td { border-top: "╌"; }
```

Both are literal glyphs, which always share the same lowest precedence tier
regardless of which glyph they are (rule 3) — so at the boundary between a
header row and the body row below it, this is a genuine tie, and rule 4
picks the *earlier* cell: `th`'s `═` wins, matching how a real browser
renders this. If you actually want the *later* cell to win instead (the
opposite of the default), only declare the edge on that side and leave the
earlier side's edge unset — with `th` no longer setting `border-bottom` at
all, rule 2 applies instead of rule 4 (only one side has a real border, so
it wins outright, no tie to break):

```css
/* th leaves border-bottom unset */
td { border-top: "╌"; }
```

Each grid vertex (where up to 4 segments meet) gets a junction glyph
synthesized from which of its arms are present and which style won each —
`solid`/`heavy`/`double` each have a complete box-drawing character set
covering every combination; `rounded` reuses `solid`'s interior junctions
with its own glyphs only at the four true outer corners. `markdown`/
`standard`/an unnamed literal-glyph style have no junction glyphs of their
own at all, so a vertex where one of them won renders as a **blank space**
instead — deliberately, not a gap to be filled in later: borrowing a
box-drawing character from an unrelated style (say, solid's `┼` in the
middle of an otherwise-ASCII `|`/`-` table) would look like an accidental
mismatch, worse than no glyph at all. See "Junction and corner overrides"
below for how to supply a real character there instead. When a vertex's
arms won with different styles, the highest-precedence style among them
picks which style's junction table to consult for that one glyph — an
approximation, not an attempt at an exact mixed-weight Unicode character
for every combination (e.g. a real single-weight vertical divider meeting a
double-weight horizontal one would, in principle, want a mixed glyph like
`╞`; this renders the whole vertex in double's own glyph set instead).

```css
table { border-collapse: collapse; }
td, th { border: solid; }
```
```html
<table><tr><th colspan="2">Header</th></tr><tr><td>A</td><td>B</td></tr></table>
```
```
┌───────┐
│Header │
├───┬───┤
│A  │B  │
└───┴───┘
```

Note the header cell's own interior: since it spans both columns, no
divider is drawn through the middle of its box even though the column
boundary is active below it — matching real CSS (`colspan`/`rowspan`
suppress the interior grid-line segments they cover; only a spanning cell's
own outer edges participate in conflict resolution).

### How much width the grid lines take

A collapsed table's columns are sized against the space its **grid lines**
will occupy: one column per *active* vertical line, where a line is active if
any row draws a border there at all (rule 2 above — a boundary nobody borders
consumes no space). There are `columns + 1` boundaries, so an N-column table of
fully bordered cells spends N+1 columns on grid lines, not 2N: the border
between two cells is a single shared column that both sides resolved, and the
outermost two come from whichever of the table's or the edge cells' borders won
there.

That count is exact, not estimated — conflict resolution compares border styles
and widths, never sizes, so the whole grid can be resolved before any column is
measured. It matters in both directions. Sizing a collapsed table as if each
cell kept its own two borders (`separate`'s model) leaves a `width: 100%` table
one column short per interior boundary; sizing it from its cells' borders alone
misses the two outer lines a table-level `border` contributes, and the table
then overruns its width — at full width, past the terminal.

## Junction and corner overrides

`border-*-corner` (`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner`) works under
`border-collapse: collapse` on **both** `<table>` and `<th>`/`<td>`, the
same as every other border property — on the table, it's the table's own
true 4 outer corners:

```css
table {
  border-collapse: collapse; border-style: solid;
  border-top-left-corner: '1'; border-top-right-corner: '2';
  border-bottom-left-corner: '3'; border-bottom-right-corner: '4';
}
```
```
1───2
│A  │
3───4
```

On a cell, it means the same thing at cell scope: that cell's own corner,
wherever it lands. A single cell only has 4 corners total — there's no
"interior, non-corner position" belonging to just one cell the way the
whole table has — so no separate cell-level property is needed for this;
it's the exact same `border-top-left-corner` you'd set on any other
bordered box.

For a glyph anywhere else in the grid — a T-junction, a cross, or a
corner-shape that isn't the table's own outer corner (jagged row lengths
can produce one; see below) — 9 properties supply a literal override, one
per arm-combination shape, named after `border-top-left-corner`'s own word
order (location first): `border-top-junction`/`border-right-junction`/
`border-bottom-junction`/`border-left-junction` (the 4 T-shapes — named
for which side of the table the divider meets, not which arm is missing)
and `border-center-junction` (the interior 4-arm cross), plus
`border-top-left-junction`/`border-top-right-junction`/
`border-bottom-left-junction`/`border-bottom-right-junction`
(corner-shapes). **The 5 T-shape/cross properties work on both `<table>`
and `<th>`/`<td>`**; a cell's own version applies only at its own edges,
wherever that shape actually occurs there. **The 4 corner-shape junction
properties are table-only** — once cell-level `border-*-corner` covers "my
own corner, whichever shape lands there," a cell-level
`border-top-left-junction` would mean the exact same thing, so there's no
separate property for it; at table scope the two remain genuinely
different (one literal position vs. every occurrence of that shape), which
is why both exist there.

```css
table { border-collapse: collapse; border-top: "="; border-top-junction: '+'; }
td { border-left: "|"; border-right: "|"; }
```
```html
<table><tr><td>A</td><td>B</td></tr></table>
```
```
 =+= 
|A|B|
```

This is exactly the escape hatch for the "blank space" case above: a
`markdown`/`standard`/literal-glyph border gets a real junction character
by naming it directly, rather than the engine trying to guess one from an
unrelated style's glyph set. Each is a plain CSS property (quoted-string
value only, no named-preset keyword support, matching `border-*-corner`'s
own literal-only grammar) — `unset`/`inherit`/`initial` all work on them
with no special handling, the same as any other property. There is
currently no shorthand to set several at once (a possible future
addition); write out the ones you need.

**Precedence, most specific first:**

1. A cell's own override, at its own corner/edge. Up to 4 cells can share
   one interior vertex; if more than one sets a conflicting override for
   the same shape there, the earlier cell in reading order wins (the same
   rule — verified against real browsers — that resolves same-type edge
   ties; see "`border-collapse: collapse`" above).
2. `border-*-corner` set on the table — only at the table's own true 4
   outer corners.
3. `border-*-junction` set on the table — wherever that shape occurs
   anywhere in the grid.
4. The computed glyph from whichever style actually won the surrounding
   edges (the pre-override behavior, unchanged).

That order matters for correctness, not just priority: an earlier version
of the table-level corner check read a table's own `border-style` preset's
corner glyph as a fallback even when a *cell's* higher-precedence style
had actually won the surrounding edges — producing a visibly mismatched
corner (e.g. a `rounded` corner around a `heavy` box, if the table set
`rounded` but a cell's own `heavy` correctly won the edges). The corner/
junction override tiers now only ever consult an explicit literal value;
the computed fallback (tier 4) is the only place a *style's own* glyph
comes from, and it already correctly reflects whichever style won.

**Real-world motivating example**: comfy-table-style ASCII tables (see
https://github.com/Nukesor/comfy-table) give their header/body divider a
distinct look — a seamless `=` run with `+` ends — while ordinary row
dividers use `|`/`-`/`+`. Both dividers share the exact same arm-shapes
(only which divider they are differs), so this is only reachable with
per-cell overrides:

```css
table {
  border-collapse: collapse; border: none;
  border-top: "-"; border-bottom: "-";
  border-center-junction: "+"; border-top-junction: "+"; border-bottom-junction: "+";
  border-left-junction: "|"; border-right-junction: "|";
  border-top-left-corner: "+"; border-top-right-corner: "+";
  border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
}
th, td { border-left: "|"; border-right: "|"; }
th {
  border-bottom: "=";
  border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
  border-center-junction: "="; border-bottom-junction: "=";
  border-left-junction: "+"; border-right-junction: "+";
}
td { border-bottom: "-"; }
```
```
+--+--+
|H1|H2|
+=====+
|a |b |
|--+--|
|c |d |
+--+--+
```

**Scope of what's compared, as currently implemented**: **cell-level,
table-level, and `<col>`/`<colgroup>`** `border` declarations are
considered — a `<col>`'s own border is resolved against the cells in its
column via the same style-precedence conflict rules as any other pair of
candidates (a `<col>`'s higher-precedence style, e.g. `double`, wins over a
cell's own lower-precedence style, e.g. `solid`, even if the cell also
declares that exact edge — style precedence outranks element-type
precedence, matching real CSS's own tier ordering). `<tr>`/`<thead>`/
`<tbody>`/`<tfoot>` borders are not consulted, even though real CSS's own
collapse algorithm does include them in its full hierarchy — those elements
are real DOM ancestors of their cells (unlike `<col>`/`<colgroup>`, which sit
outside the row/cell tree entirely and need this resolution to reach their
column's cells at all), and border on `tr`/`thead`/`tbody`/`tfoot` also has
no effect under real CSS's `border-collapse: separate` — most real-world
markup styles cell borders directly instead for exactly that
cross-browser-inconsistency reason, so it wasn't judged worth the added
conflict-resolution complexity. The legacy HTML
`border`/`cellpadding`/`cellspacing` presentational attributes aren't read
either — use the CSS properties
above.

**A real-CSS deviation worth calling out**: under `border-collapse:
collapse`, the table's own `padding` has no effect (matching real CSS — "the
table does not have padding, though its cells do") — only the table's own
`margin` and its own `border` (merged into the grid like any other cell's,
not a separate wrapping frame) apply at the table level.

## Margin and padding

`margin`/`padding` on `<table>` itself work exactly like any other block
element under `border-collapse: separate` (the default) — including the
case that's easy to assume doesn't apply to tables at all:

```css
table { padding: 1; border-style: solid; }
```

Padding adds blank space *inside* the border, between the frame and the
outermost cells — this is genuinely how real CSS's `border-collapse:
separate` model behaves too; it's just rarely used because most real-world
tables reach for `border-spacing`/cell padding instead of table-level
`padding`. `margin-left`/`margin-right`, including `margin: auto`
centering, work the same way they do for other block elements (see
CSS.md's `margin` entries) — and `auto`-centering a `<table>` specifically
is one of the few auto-margin cases real browsers support without
requiring extra tricks, which htmlterm matches. Under `border-collapse:
collapse`, `padding` on the table itself has no effect (see above) — only
`margin` does.

`margin-top`/`margin-bottom` also work like any other block element:
collapsing with adjacent siblings' margins the same way (larger value wins),
whether the `<table>` is at the document root or nested inside another
block, under either border model.

## Not supported

- **`border-collapse: collapse`'s conflict resolution doesn't consult
  `<tr>`/`<thead>`/`<tbody>`/`<tfoot>` `border`** (`<col>`/`<colgroup>` are
  consulted, via real conflict resolution against their column's cells); see
  above.
- **The legacy HTML `border`/`cellpadding`/`cellspacing` presentational
  attributes** aren't read — use `border-collapse`/`border-spacing`/cell
  `padding` instead.
- **Multi-line cell content combined with `white-space: nowrap`** — a
  `nowrap` cell is always clipped to one line (see `text-overflow`), never
  both non-wrapping and multi-line. Applies under both border models.
- **The 4 corner-shape `border-*-junction` properties have no cell-level
  equivalent** — table-only; see "Junction and corner overrides" above for
  why (a cell-level version would be redundant with cell-level
  `border-*-corner`).
- **No `border-junctions` shorthand** for setting several junction
  properties at once — a possible future addition, not built yet.

## See Also

- CSS.md's `border`, `margin`, `padding`, `width` entries — the same
  properties and value forms apply to `<table>`/`<th>`/`<td>` as to any
  other element, documented once there rather than duplicated here.
- `docs/RENDERING.md` for how table layout fits into the wider box-model
  pipeline.
