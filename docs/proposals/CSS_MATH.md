# CSS Math (`calc()`, `min()`, `max()`, `clamp()`) — Design & Implementation Plan

Status: **implemented**, including the table.go caveat below and the
`<number>`/`<integer>`-context follow-up originally scoped out below. This
document remains the design record and rationale — see CSS.md's "Size
Values" and "CSS Math" sections for the user-facing reference. Follows the
same record-then-implement structure as `docs/proposals/VARIABLES.md`.

The `<number>`/`<integer>`-context follow-up (`opacity`, `flex-grow`,
`flex-shrink`, `z-index`, `tab-size`) turned out smaller than the length-
percentage work, as expected: a new `internal/render/number.go` with two
functions. `resolveNumber` mirrors `resolveLength` with no basis parameter
at all — these five properties have no percentage support to defer to one,
so passing `basisOK=false` to `Eval` unconditionally makes a `%` anywhere
in a math function fail closed, the same free consequence `parsePaddingLen`
already gets. `resolveCSSMathInteger` is `z-index`/`tab-size`'s
`<integer>`-context counterpart, rounding a calc() result to the nearest
integer with ties toward positive infinity, matching real CSS's own rule;
a plain, non-calc literal is untouched, still going through each site's
existing `strconv.Atoi`/`parseSizeVal`, so `z-index: 1.5` (not valid CSS
`<integer>` syntax) still fails exactly as before. One small, deliberate
behavior change rode along: `opacity`'s and `flex-grow`'s non-calc fast
paths moved from `strconv.ParseFloat` to `cssengine.ParseNumber` (already
true for `flex-shrink`), which rejects things that aren't real CSS
`<number>` syntax but that `ParseFloat` accepts anyway — `Inf`, `NaN`, Go's
hex-float literals. Tightening, not loosening.

One thing the caveat below didn't anticipate, found only once the table.go
work was actually done: `colConstraints` isn't just read in `table.go`. Every
column's final constraints are built by merging each cell's own
`colConstraints` across all its rows (`mergeColConstraints`,
`table_render.go`), and that merge function had its own hardcoded list of
fields to carry forward — `fixed`, `percent`, and the min/max pairs, but not
the new `calc`/`minCalc`/`maxCalc` fields, since it predates them. The first
end-to-end table test written for this (a `<td style="width: calc(4 + 2)">`)
silently fell back to natural-width sizing, as if no width had been declared
at all, because `mergeColConstraints` dropped the calc string on the floor
before `sizeColumns` ever saw it. A second, smaller instance of the same gap
turned up in `estimateColumnWidths`'s "is this column unconstrained" check.
Both are fixed now, but the lesson is general: adding a field to a struct
that gets merged, copied, or pattern-matched elsewhere means finding every
place that enumerates its fields by name, not just the place that reads it
for its own stated purpose. `grep` for the type name across the whole
package, not just the file being changed, is what caught the second
instance; the first was caught by an end-to-end test succeeding at the
wrong width rather than failing outright, which is worth noting on its own:
a test that only checks the internal resolver functions directly
(`TestCellConstraintsCalc`, `TestSizeColumnsRespectsMaxAndShrinkMin`) passed
throughout, because those functions were individually correct — only a real
`<table>` render exercised the merge step in between and caught it.

---

## Why this is worth doing

Every sizing property in this engine (`width`, `height`, `margin`, `padding`,
`flex-basis`, `gap`, `top`/`right`/`bottom`/`left`, `text-indent`) accepts a
narrow vocabulary: a bare integer, its `ch` spelling, a percentage, or `vw`/
`vh` (see CSS.md's "Size Values"). Real stylesheets that use this engine's own
custom property system (`var()`, already implemented) hit the ceiling of that
vocabulary immediately: `width: calc(100% - var(--sidebar))` is the first
thing anyone reaches for once variables exist, and there is no way to write
it today short of computing the value in Go before it reaches CSS.

The good news is this codebase already has almost every primitive `calc()`
needs, built for other features:

- `findMatchingParen` / `consumeCSSQuotedToken` (`internal/cssengine/customprops.go`)
  already do paren-and-quote-aware scanning for `var()`. The same primitives
  parse a math function's argument list.
- `splitCSSComponentValues` (`internal/cssengine/css.go`) already tokenizes a
  shorthand value by whitespace while treating a parenthesized run as one
  token — exactly what `margin: calc(1 + 2) auto` needs so its shorthand
  expander doesn't split `calc(1 + 2)` on the space around `+`.
- `foldKeywordValue` already lowercases function names it doesn't
  special-case, so `CALC(1 + 2)` folds to `calc(1 + 2)` for free, the same as
  every other keyword.
- var() substitution already runs before keyword-folding and before
  `internal/render` ever sees a value, so `calc(var(--gap) + 4)` needs no new
  code at all: by the time a calc parser runs, `var(--gap)` is already gone,
  replaced with `--gap`'s literal text.
- Every existing length resolver (`resolveCSSSize`, `resolveCSSHeight`,
  `resolveFlexCSSSize`, `resolveMarginSide`, `parseGapLen`, table's own
  column-width resolution) already funnels through **one** parsing function,
  `parseSizeVal` (`internal/render/table.go`), with `parseFlexSizeVal`
  (`internal/render/flex.go`) as its only variant. That single choke point is
  exactly where `calc()` support has to slot in.

## Where the real cost is

`parseSizeVal` returns `(abs int, pct float64, ok bool)` and its doc comment
states the invariant every caller relies on: "Exactly one of abs>0 or pct>0
is set on success." `calc(50% + 4)` breaks that invariant on purpose — it
*is* both an absolute part and a percentage part at once, added together
once the percentage's basis (the containing block's width, height, or
whatever axis the property resolves against) is known. `min()`/`max()`/
`clamp()` compound that: which branch wins can depend on the basis too
(`min(50%, 20)` picks a different one depending on the container).

So this is not "teach the existing parser one more unit." It's a real
signature change: the resolver has to move from "parse a string into a
(number, kind) pair" to "parse a string into an **expression**, then
**evaluate** that expression against a basis supplied by the call site."
Most of the call sites listed above change from a two-branch `if pct > 0 {
… } else { … }` to a single call into the new evaluator, which is a net
simplification once done. `table.go` is the exception: its column-width
resolution defers percentage resolution rather than doing it immediately,
in a data structure (`colConstraints`) built around "exactly one of a fixed
width or a percent width," which is the same invariant calc() breaks
everywhere else — see the dedicated caveat under Design below. Everywhere
but there, this is mechanical; there, it's a small sub-design of its own.

The second real cost is shorthand splitting. `expandShorthand` and
`expandFlexShorthand` (`internal/cssengine/css.go`) split `margin`,
`padding`, `overflow`, `gap`, `border-spacing`, and `flex` values with plain
`strings.Fields`, which breaks on the mandatory whitespace CSS requires
around `calc()`'s `+`/`-` operators (`calc(1 + 2)` tokenizes to four
fields, not one). This has to be fixed as a prerequisite, not a follow-up,
or `margin: calc(1 + 2) auto` silently misparses.

## Scope decision: length-percentage contexts only

Real CSS also allows `calc()` in `<number>` contexts (`opacity: calc(1 -
0.5)`, `flex-grow: calc(1 + 1)`) and `<integer>` contexts (`z-index`,
`tab-size`). This plan scopes to the **length-percentage** properties CSS.md's
"Size Values" section already enumerates: `width`/`min-width`/`max-width`,
`height`/`min-height`/`max-height`, `margin-*`, `padding-*`, `flex-basis`,
`gap`/`row-gap`/`column-gap`, `border-spacing`, `text-indent`, and
`top`/`right`/`bottom`/`left`. Number and integer contexts are a separate,
smaller follow-up (different resolver, no basis/percentage concerns at all)
and are called out as a non-goal below so this pass stays reviewable.

## Design

### New file: `internal/cssengine/calc.go`

Owns parsing *and* arithmetic evaluation — everything that is pure CSS value
grammar and needs no layout state. `internal/render` supplies only the one
thing `cssengine` can't know: what basis a percentage resolves against.

```go
// Expr is a parsed calc()/min()/max()/clamp() expression tree.
type Expr struct {
    // leaf: a bare number/ch (abs, in cells) or a percentage (pct, 0.0–1.0).
    // exactly one of the two constructors below produces a leaf; op is unset.
    abs, pct float64
    leaf     bool

    op   mathOp   // opAdd, opSub, opMul, opDiv, opMin, opMax, opClamp
    args []Expr   // 2 for +,-,*,/; 1+ for min/max; exactly 3 for clamp
}

// IsMathFunctionToken reports whether s (trimmed) opens a calc(), min(),
// max(), or clamp() call — a shape check only, no argument validation. Used
// by isCSSFlexBasisToken (css.go) so `flex: 1 1 calc(50% - 4)` classifies
// its third token as a basis, and by internal/render as a cheap guard to
// skip the parser entirely for the overwhelming common case of a plain
// length with no math function in it.
func IsMathFunctionToken(s string) bool

// ParseMathExpr parses s as a top-level calc(), min(), max(), or clamp()
// call. ok is false for anything else, including a plain "50%" — callers
// keep using their own fast path for that; this is only the math-function
// path.
func ParseMathExpr(s string) (Expr, bool)

// Eval resolves e to a single number of cells, given basis (the containing
// block's size on the relevant axis) and basisOK (whether that basis is
// itself definite). A percentage anywhere in e when basisOK is false is not
// a length at all, the same rule resolveCSSHeight already applies to a bare
// percentage against an indefinite containing-block height — so Eval
// reports ok=false rather than silently multiplying against a sentinel.
// Division by a percentage, or by zero, is invalid per spec and also
// reports ok=false. The result is not floored, clamped, or rounded — every
// caller in internal/render decides its own property's sign and zero
// policy on top of the raw number, the same way they already decide it for
// a plain percentage today.
func Eval(e Expr, basis float64, basisOK bool) (value float64, ok bool)
```

**Grammar accepted inside a math function's arguments:** a bare number, a
`ch`-suffixed number (identical, per this engine's existing "a bare integer
and its `ch` spelling are the same value" rule), a `%`-suffixed number,
`+`/`-`/`*`/`/` with the real CSS precedence (`*`/`/` bind tighter, and CSS
requires surrounding whitespace on `+`/`-` specifically, not `*`/`/`),
parenthesized sub-expressions, and nested `calc()`/`min()`/`max()`/
`clamp()` calls. `min()`/`max()` take one or more comma-separated arguments;
`clamp()` takes exactly three (`clamp(MIN, VAL, MAX)`, evaluating to
`max(MIN, min(VAL, MAX))`). Intrinsic keywords (`auto`, `min-content`,
`fit-content`, …) are not valid calc operands, matching spec — they're
already handled as separate tokens by `isCSSFlexBasisToken` outside of
calc entirely.

**`vw`/`vh` are deliberately not a leaf kind here.** They're resolved by
`convertViewportLengths` (`internal/render/cascade.go`) before any value
reaches `parseSizeVal`-family code today; extending that pass to also look
inside a math function's parens (see below) means `calc()`'s own grammar
only ever needs to understand `ch` and `%`, the same two units
`parseSizeVal` already knows, rather than a second, redundant
viewport-resolution path living inside `cssengine`.

**Signed numeric literals.** `min(-4, 10)`, `calc(4 + -2)`, and `calc(-50%)`
all need a leaf number to accept an optional leading `-` as part of the
literal token itself — a different thing from the mandatory whitespace CSS
requires around a *binary* `+`/`-`. `calc(4 + -2)` is `4` plus the literal
`-2`; `calc(4-2)` is invalid (no surrounding whitespace on the binary `-`).
The tokenizer has to tell these apart: a `-` is part of the next number
token when it's in an operand position (start of input, right after `(`,
`,`, or another operator), and a binary operator everywhere else.

**Decimal literals, inside calc() only.** Outside a math function, a bare or
`ch`-suffixed length is integer-only (`parseSizeVal` uses `strconv.Atoi`).
Inside one, intermediate arithmetic needs to be fractional —
`calc(100% / 3)` and `calc(1.5 * 4ch)` both require it — with truncation to
an integer cell count happening exactly once, at `Eval`'s final result, not
at each leaf or each operation. `Expr`'s leaf fields are `float64` for this
reason; the math-function leaf parser accepts a decimal point where the
plain fast-path parser in `resolveLength` does not, and that's a deliberate
difference between the two, not an inconsistency to reconcile.

**Confirmed against spec: a math function resolves as a whole, not
per-branch.** `Eval` fails the *whole* expression when any percentage
inside it can't resolve — `min(50%, 20)` against `basisOK=false` returns
not-ok, not `20`. This isn't just the simpler option; it's what CSS2.1
§10.5 specifies outright, with a worked example: `height: calc(100% - 100%
+ 1em)` computes to `calc(0% + 1em)`, not `calc(1em)`, even though the two
percentages cancel algebraically — and because it still contains a
percentage relative to an indefinite containing-block height, the whole
declaration resolves as `auto`, not `1em`. CSS deliberately does not
pre-simplify the percentage away before deciding whether the value depends
on the basis; it's a syntactic check ("does a `%` appear anywhere"), not a
semantic one ("does the net percentage coefficient happen to be nonzero"),
which is exactly what a plain scan of the parse tree for a `%` leaf (rather
than simplify-then-check-the-coefficient) gives for free. `min()`/`max()`/
`clamp()` get no separate rule: CSS Values 4 §10 defines them as sugar over
the same calc()-style resolution algorithm, sharing its type system, so
`max-height: min(50%, 20)` on an auto-height container falls back to
"unconstrained," not `20` — the absolute-length branch does not rescue the
value. (This is a well-documented real-world gotcha, not a corner nobody
hits: people repeatedly expect the absolute branch to win and file bugs
when it doesn't.) No open question remains here.

**One accepted, documented deviation from spec:** real CSS distinguishes
`<number>` (`3`) from `<length>` (`3ch`) well enough to make `4ch * 3ch` a
type error (you can't multiply two lengths). This engine's own design
already treats a bare number and its `ch` spelling as identical everywhere
outside calc(), so `Expr`'s leaf representation can't tell them apart either
— both are just "N cells" (`abs: N, pct: 0`). The practical effect: this
implementation accepts `calc(4ch * 3ch)` (evaluating it as `4 * 3` = 12
cells) where spec would reject it. Nobody writes that expecting a type
error; the constructs people actually write (`calc(2 * 4ch)`,
`calc(100% / 3)`) both work correctly. Division and multiplication by a
value with a nonzero percentage component is still rejected (`calc(100% /
50%)` fails), which is the one restriction that has an actual footgun to
protect against (silently discarding a basis-dependent divisor's own
basis-dependence).

### `css.go` changes

1. Swap `strings.Fields(val)` for `splitCSSComponentValues(val)` at the five
   shorthand-splitting sites that accept lengths: `margin`/`padding` (the
   1–4-value expansion), `overflow`, `gap`, `border-spacing`, and `flex`.
   `splitCSSComponentValues` already exists and is already paren-aware
   (used today by `border-color`, `list-style`, `background`); this is a
   one-line change per site, not new logic. Independently fixes a latent gap
   for `var()` fallbacks with embedded spaces, e.g. `margin: var(--x, 1 2)
   3`, which mis-tokenizes today for the same reason.
2. `isCSSFlexBasisToken`: add `cssengine.IsMathFunctionToken(s)` as an
   accepted case, so a `calc()`/`min()`/`max()`/`clamp()` token in `flex:
   1 1 calc(50% - 4)` or `flex-basis: min(50%, 40)` classifies as a basis
   rather than falling through to "junk" and leaving the shorthand inert.
3. Border shorthand's `<width>`/`<style>`/`<color>` classifier
   (`parseBorderComponents`) needs no change: `border-width` and its
   per-edge longhands are already permanently a no-op in this engine (CSS.md
   — terminal box-drawing has no line-thickness concept), so a `calc()`
   token there hits the same best-effort fallback classification a `var()`
   token already does, with no functional consequence either way.

### `cascade.go` changes

`convertViewportLengths` currently tokenizes with `strings.Fields` and
matches a `vh`/`vw` suffix per token — which fails inside a math function,
since `calc(2vw + 4)` tokenizes to `["calc(2vw", "+", "4)"]` and `"calc(2vw"`
doesn't parse as a bare number even though it ends in `vw`. Switch its
tokenizer to the same paren-aware split `css.go` uses (exported from
`cssengine` for this one reuse, e.g. `cssengine.SplitComponentValues`, so
there's exactly one implementation of "paren-aware whitespace split" rather
than a second copy living in `internal/render`), then within each
paren-aware token, find and convert every `Nvw`/`Nvh` occurrence, not just
whole-token matches. After this pass, `calc()`'s own parser in `cssengine`
never sees a `vw`/`vh` unit at all — by the time it runs, `calc(2vw + 4)`
has already become `calc(2 + 4)` (or whatever cell count `2vw` resolved to).

### New file: `internal/render/length.go`

The one place every sizing property's value gets turned into cells, with
calc-awareness built in and a fast path that keeps today's non-calc cost
unchanged:

```go
// resolveLength resolves a CSS <length-percentage> value against basis, in
// cells: a bare integer, its ch spelling, a percentage of basis (only when
// basisOK), or a calc()/min()/max()/clamp() expression combining any of
// those. ok is false when s isn't a recognized length at all, or contains a
// percentage while basisOK is false — the same "not a length" rule
// resolveCSSHeight already applies to a bare percentage against an
// indefinite containing block, now shared by every caller instead of each
// reimplementing it.
//
// Does not floor, clamp to zero, or otherwise apply a property's own sign
// policy — callers do that themselves, the same way they already do for a
// plain percentage today (see resolveCSSSize vs. resolveMarginSide for the
// two existing policies this preserves).
func resolveLength(s string, basis int, basisOK bool) (int, bool) {
    s = strings.TrimSpace(s)
    if !cssengine.IsMathFunctionToken(s) {
        // Unchanged fast path: today's exact bare-integer/ch/percent parse,
        // same cost as before this change for the overwhelming common case.
        abs, pct, ok := parseSizeValRaw(s)
        if !ok {
            return 0, false
        }
        if pct > 0 {
            if !basisOK {
                return 0, false
            }
            return int(pct * float64(basis)), true
        }
        return abs, true
    }
    expr, ok := cssengine.ParseMathExpr(s)
    if !ok {
        return 0, false
    }
    v, ok := cssengine.Eval(expr, float64(basis), basisOK)
    if !ok {
        return 0, false
    }
    return int(v), true
}
```

Every existing call site becomes a thin wrapper supplying its own basis and
its own sign/zero policy on top of `resolveLength`'s raw result, exactly
mirroring the split that already exists between `resolveCSSSize` (width
family: a resolved `0` means "not set", matching `parseSizeVal`'s current
`abs>0`/`pct>0` gate) and `resolveFlexCSSSize` (flex-basis: a resolved `0`
is a real, meaningful length, matching `parseFlexSizeVal`'s existing zero
carve-out). That split is preserved, not changed, by this refactor — it
just moves from "two different parse functions" to "two different policies
wrapping the one parse function."

| Caller | Basis | Zero policy | Sign policy |
|---|---|---|---|
| `resolveCSSSize` (width family) | `availWidth` | 0 = unset (unchanged) | clamp to 0 |
| `resolveCSSHeight` (height family) | `cbHeight`, `basisOK = cbHeight > 0` | 0 = unset (unchanged) | clamp to 0 |
| `resolveFlexCSSSize` (flex-basis, main axis) | `axisSize`, `basisOK = axisSize >= 0` | 0 = real length, preserved exactly (unchanged) | clamp to 0 |
| `resolveFlexAxisSize` (cross-axis width/height) | `axisSize` | 0 → floored to 1 (unchanged — this engine renders no zero-width/height box) | clamp to 0 (before the floor) |
| `resolveMarginSide` | `availWidth` | 0 = real length | **no clamp** (negative margins valid) |
| `resolveAbsoluteAxis` (top/right/bottom/left) | `cbSize` | *(calls `resolveMarginSide` directly — same row as above, not a separate policy)* | — |
| `parseGapLen` | container's own content size, `basisOK = basis > 0` | no `ok` in its signature today (plain `int`, defaults to 0 on failure) — that stays true; a resolved 0 and a failed parse remain indistinguishable, matching current behavior | clamp to 0 |
| `parsePaddingLen` (also covers `border-spacing-x`/`-y` via `parseSpacingLen`) | none (`basisOK = false`, always) | same no-`ok` shape as `parseGapLen`, unchanged | clamp to 0 |
| table column width resolution | see caveat below — not a simple wrapper swap | — | clamp to 0 |
| `text-indent` (block.go) | `innerW` | 0 = real length | existing `indent > 0` filter unchanged (a calc-produced negative indent is silently dropped exactly like a literal negative one is today — not a new gap) |

`parsePaddingLen`'s `basisOK = false` is a small, free consequence of the
shared "percentage against no basis is not a length" rule: padding accepts
no percentage today (CSS.md's `padding-left`/`padding-right` entries), and
routing it through `resolveLength` with no basis means a `%` literal
anywhere inside a padding `calc()` correctly fails closed to "not a length"
with zero special-casing, while pure-arithmetic padding calc's with no `%`
in them (`padding: calc(2 * 2)`) still resolve normally. No new percent
support for padding is introduced; this is exactly today's "padding ignores
percentages" behavior, now falling out of the shared resolver instead of
being a separate omission.

`table.go`'s own `parseSizeVal` (the base implementation `resolveCSSSize`
and friends currently build on) becomes a thin call into `resolveLength`
too, removing the last independent reimplementation of length parsing.
`parseFlexSizeVal`'s zero-length carve-out (documented at length in
`flex.go` — `flex: 1 1 0` needs `0` to mean a real, definite flex-basis, not
"unset") moves to `resolveFlexCSSSize`'s wrapper policy, unchanged in
behavior.

### Caveat: table column widths need their own sub-design, not a call-site swap

**Resolved**, via the first option below: `colConstraints` gained
`calc`/`calcDelta` and the `min`/`max` equivalents, resolved through
`resolveLength` at the same point `sizeColumns`/`effectiveMinMax` already
resolve `percent`. Kept here as the design record for why, plus the
`mergeColConstraints` gap this approach turned out to have (see the status
note at the top of this document) that a resolved-`Expr`-in-the-struct
design wouldn't have made any more obvious up front.

Every other caller above resolves its length immediately, against a basis
it already has in hand. Table column sizing doesn't: `cellConstraints`
(`table.go`) parses each cell's `width`/`min-width`/`max-width` once, up
front, into `colConstraints` — but a percentage is *not* resolved there. It
is stored as a bare `percent float64` field, deliberately kept separate
from `fixed int`, and the two are treated as mutually exclusive alternatives
(`sizeColumns`: `case c.percent > 0: … case c.fixed > 0: …`). The actual
multiplication against `contentWidth` happens later, once every column's
natural width is known, with a separate `percentDelta int` box-sizing
adjustment folded in after the multiply (`effectiveMinMax` does the same
for `minPercent`/`maxPercent`). This deferral exists because a column's
percentage can't be resolved until the whole table's `contentWidth` is
settled — it is not an oversight this plan can route around.

A calc() result like `calc(50% + 4)` cannot be decomposed into that
existing `fixed`-**or**-`percent` pair: it is both at once, and the pair's
whole design assumes it never is. Slotting calc() in here means one of:

- adding a third, calc-specific field to `colConstraints` (a parsed `Expr`,
  resolved against `contentWidth` at the same point `sizeColumns` currently
  does the `percent` multiply, with `percentDelta`'s box-sizing adjustment
  applied the same way afterward), or
- collapsing `fixed`/`percent`/`percentDelta` into one deferred
  `resolveLength(rawDecl, contentWidth, true)` call at resolution time,
  which would be a larger, more invasive rewrite of `cellConstraints` and
  `effectiveMinMax` than this plan otherwise requires anywhere else, in
  exchange for removing a field.

Either is real design work specific to `table.go`, done once `Expr`/`Eval`
exist elsewhere, not a peer change to the single-line swaps the rest of this
section describes. It's called out here so it isn't discovered mid-implementation
as a surprise: **table column/cell width calc() support is the single
largest piece of implementation risk in this plan**, not because the math is
different, but because the surrounding data structure was built around
"exactly one of fixed or percent," which calc() breaks by design the same
way it broke `parseSizeVal`'s own invariant.

### Performance

The fast-path guard (`cssengine.IsMathFunctionToken`, a cheap prefix check
paralleling `substituteViewportUnits`'s existing `!strings.ContainsAny(val,
"vV")` early exit) means every declaration that doesn't contain `calc(`,
`min(`, `max(`, or `clamp(` — the overwhelming majority — takes the exact
same code path at the exact same cost as today. No caching of parsed `Expr`
trees across renders is proposed for this pass; like `Cascade.Resolve`
itself (see VARIABLES.md's own performance section), this re-parses on every
call, which is consistent with the rest of this layer and can be revisited
later if profiling shows it matters.

## Testing plan

`internal/cssengine` (`calc_internal_test.go`):
- Arithmetic and precedence: `calc(1 + 2)`, `calc(2 + 3 * 4)` = 14 not 20,
  parenthesized grouping, `calc(100% / 3)`.
- Mixed percent+absolute: `calc(50% + 4)`, `calc(100% - 4)`.
- Nesting: `calc(min(50%, 40) + 2)`, bare `min()`/`max()`/`clamp()` with no
  outer `calc()` wrapper.
- Rejections: division by zero, division/multiplication by a value with a
  percentage component, unbalanced parens, missing mandatory whitespace
  around `+`/`-`, wrong argument count for `clamp()`, an intrinsic keyword
  as an operand — all `ok=false`, none panic.
- Case-insensitivity (`CALC(1 + 2)`), and that a `var()`-containing calc
  value is left alone until after substitution (existing `foldKeywordValue`
  behavior, verified rather than changed).
- Signed literals: `min(-4, 10)`, `calc(4 + -2)`, `calc(-50%)`, and that
  `calc(4-2)` (no whitespace around a binary minus) is rejected rather than
  silently read as `2`.
- Fractional intermediate arithmetic: `calc(100% / 3)`, `calc(1.5 * 4ch)`,
  truncated once at the end, not per-operation.
- `isCSSFlexBasisToken`/shorthand-splitting regressions: `margin: calc(1 +
  2) auto` expands to exactly two longhand-pairs' worth of tokens, not four;
  `flex: 1 1 calc(50% - 4)` classifies its basis correctly.

`internal/render` (`length_internal_test.go` or extending
`helpers_internal_test.go`):
- `resolveLength` directly: percentage against `basisOK=false` returns not
  ok; the per-property table above, one case per row.
- End-to-end render tests: `width: calc(100% - 4)` in a fixed-width
  container; `min(50%, 20)` / `max(10, 20%)` / `clamp(10, 50%, 30)` on
  `width`; `flex-basis: calc(50% - 2)`; `gap: calc(1 + 1)`;
  `margin-left: calc(-2)` (negative, rendered) versus `padding-left:
  calc(-2)` (clamped to 0); a `<td>`/`<th>` with `width: calc(50% - 2)` in a
  multi-column table, specifically exercising the `colConstraints` sub-design
  above rather than assuming the general-case tests cover it.

## Non-goals for this pass

- **`clamp()`'s `none` keyword** for an open-ended bound (CSS Values 4,
  `clamp(none, 1rem + 2vw, none)`). This pass requires exactly three
  comma-separated arguments.
- **`calc-size()`** and the trigonometric/exponential functions (`sin()`,
  `sqrt()`, `pow()`, …) — no terminal use case, real complexity for none.
- **Caching parsed `Expr` trees** across renders.
- **Telling `<number>` apart from `<length>` inside calc()** well enough to
  reject `4ch * 3ch` — see the documented deviation above.
- **Adding percentage support to padding** beyond what routing through
  `resolveLength` with `basisOK=false` already gives for free (arithmetic
  with no `%` in it still works; any `%` still correctly fails closed).

## Docs to update

- **CSS.md**: extend the "Size Values" table with `calc()`/`min()`/`max()`/
  `clamp()` rows, describe the accepted grammar (operators, nesting,
  argument counts, the zero/negative policy table above, the documented
  `4ch * 3ch` deviation), and remove the "CSS math functions" line from the
  closing "not yet supported" list.
- **COMPATIBILITY.md**: remove the `calc()`/`min()`/`max()`/`clamp()` bullet
  from "Not Implemented", and add a terse deviations entry for the
  `<number>`-context non-goal and the `4ch * 3ch` over-acceptance, per
  CLAUDE.md's rule that a new deviation updates both docs.

## Summary

The parsing and evaluation machinery is genuinely new (`internal/cssengine/calc.go`),
but everything it needs to integrate with — paren-aware tokenizing, a single
length-resolution choke point, var()-then-fold ordering, the
basis/indefinite-basis convention `resolveCSSHeight` already established —
already exists in this codebase for other reasons. Most of the effort is
mechanical: rewire the existing call sites to go through one shared
`resolveLength`, each keeping the sign/zero policy it already has (see the
per-caller table above — they're not all the same policy, and two of them
looked more alike than they are), and fix five shorthand-splitting sites
that were only ever exposed by punctuation `calc()` introduces (spaces
inside a single value). Table column/cell widths are the one place that
isn't mechanical: `colConstraints`' fixed-or-percent design predates calc()
and has to change shape, not just gain a new call. No changes are needed in
`document` or `tui`, matching the layering `ARCHITECTURE.md` describes.
