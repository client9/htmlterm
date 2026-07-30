# `internal/textcell`

Measures and manipulates already-rendered terminal text in **cells** — the
unit a terminal actually charges for — rather than in bytes or runes.

## Why it's its own package

Three callers have to agree, exactly, on how wide a string is:

- `internal/render` lays content out and decides where a box's edges fall.
- `tui` paints that layout onto a real screen and positions the hardware
  cursor on top of it.
- `document` turns a click's column back into an offset into a value.

Any disagreement between them is visible: a cursor sitting inside a glyph, a
border landing a column past its box, a repaint that smears. Before this
package existed, `tui` reached into `internal/render` for two measurement
functions and kept a byte-for-byte copy of a third, and the width rules lived
in a 979-line file next to roman numerals and CSS `text-transform`. Putting
the measurement in one leaf package that all three depend on makes their
agreement structural instead of a convention someone has to remember.

It also fixes the dependency shape: `tui` depends on a leaf, not on the
layout engine.

## Three units, and which one to use

Mixing these up is the single most common bug this package exists to prevent:

| Unit | What it is | Where it belongs |
|---|---|---|
| bytes | Go's native string index | never a screen position |
| runes | Unicode code points | caret/selection offsets, `[]rune` indexing |
| cells (columns) | what a terminal charges | every width, every screen position |

Runes and cells coincide for ASCII and diverge everywhere else: a CJK
ideograph is one rune and two cells; a combining accent is a second rune and
zero extra cells; an emoji ZWJ sequence is five runes, one grapheme cluster,
and two cells. **Arithmetic that adds a rune count to a column is wrong even
when its tests pass**, because an all-ASCII corpus cannot tell the two apart.
That's what `widecolumn_test.go` in the repository root is for — it renders
awkward-width payloads through every box-producing construct and checks the
result is the width the layout claimed.

## What must NOT be folded in here

**A text entry's caret and selection are rune offsets, and they stay that
way.** This is not an inconsistency waiting to be tidied up:

- The DOM's `selectionStart`/`selectionEnd`/`setSelectionRange` are offsets
  into the value's character sequence. `document` mirrors them so a caller
  who knows the web platform gets the answer they expect (see
  `COMPATIBILITY.md`'s text-selection deviation — rune offsets, not UTF-16
  code units, and not columns either).
- Columns are a property of how a value is *rendered*. They change with
  East Asian width settings, and they don't exist at all for a value that
  was never rendered — a `Document` that has never had `Render` called still
  has a perfectly well-defined selection.

So the conversion is a **boundary, not a representation**. `ColumnForRuneIndex`
and `RuneIndexForColumn` are the only two places the units meet, and they're
called at the edges: placing a terminal cursor (`tui`'s `focusCursorPos`), or
turning a click into an offset (`document`'s `caretIndexFromClick`).
Everything on the document side stays in runes; everything on the painting
side stays in cells.

"These are all just character counts, unify them" is the tempting wrong move.
It would silently break every non-ASCII selection.

## API shape

- **Measure** — `Width`, `VisibleLen` (ANSI-aware), `NextGrapheme`
- **Convert** — `ColumnForRuneIndex`, `RuneIndexForColumn` (the boundary above)
- **Slice by column** — `SplitAtWidth`, `VisibleWindow`, `SpliceColumns`,
  `VisiblePrefix`, `TruncateToWidth`, `TrimTrailingSpace`
- **ANSI plumbing** — `Carry` (style/hyperlink state carried across an
  inserted line break), `ConsumeANSI`, `Strip`, `SplitTokens`,
  `CollapseDeadSpans`

Everything is a pure function of its arguments. There is deliberately no
HTML, no CSS, and no layout here: callers that need to know what a string
*means* keep that knowledge. This package only answers how much room it takes
and where its boundaries fall.
