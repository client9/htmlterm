package htmlterm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/client9/htmlterm"
)

// Flex layout is the one part of this renderer that measures by *rendering*:
// resolving an item's flex base size (flex.go's measureNaturalWidth) and its
// automatic minimum size (measureMinContentWidth) each lay the item out for
// real and throw the result away. Inside a nested flex container those trial
// renders recurse, so the cost of a flex tree is a product down its depth
// rather than a sum across its nodes, and a change that adds one measurement
// per item multiplies rather than adds. These benchmarks exist to make that
// visible: nesting depth is the axis that bites, so it gets its own benchmark
// separate from document size.

// buildNestedFlexDocument returns `depth` nested bordered flex containers
// wrapping two bordered leaf items. Every wrapper carries `flex: 1`, which is
// what makes this the worst case rather than a merely deep one: a declared
// flex-basis sends resolveMainBasis through the automatic-minimum probe, so
// each level measures min-content for its child, which is itself a flex
// container that measures min-content for *its* child.
func buildNestedFlexDocument(depth int) string {
	var sb strings.Builder
	sb.WriteString(`<div style="display:flex;width:100%">`)
	for range depth {
		sb.WriteString(`<div style="display:flex;flex:1;border-style:solid">`)
	}
	sb.WriteString(`<div style="border-style:solid">aa aa aa aa</div><div style="border-style:solid">bb bb</div>`)
	for range depth {
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// buildWideFlexDocument returns `rows` independent single-level flex rows, the
// shape real documents actually have: many items, no nesting. Each row mixes
// the three basis paths — a declared flex-basis (`flex: N`, which probes
// min-content), an explicit width (which doesn't), and a plain auto item (which
// measures fit-content) — so the per-item measurement cost is what's on show
// rather than any one path.
func buildWideFlexDocument(rows int) string {
	var sb strings.Builder
	for i := range rows {
		fmt.Fprintf(&sb, `<div style="display:flex;gap:1;width:100%%">`+
			`<div style="flex:1;border-style:solid">cell %d alpha</div>`+
			`<div style="flex:2;border-style:solid">cell %d beta gamma</div>`+
			`<div style="width:10">fixed</div>`+
			`<div>auto sized tail</div>`+
			`</div>`, i, i)
	}
	return sb.String()
}

// buildOverflowingFlexDocument returns `rows` flex rows whose items cannot fit,
// forcing every row down layoutFlexLine's shrink path. That path resolves an
// automatic minimum size per item — the measurement a non-overflowing line
// skips entirely — so this is the row shape that pays for it.
func buildOverflowingFlexDocument(rows int) string {
	var sb strings.Builder
	for i := range rows {
		fmt.Fprintf(&sb, `<div style="display:flex;width:100%%">`+
			`<div style="border-style:solid">row %d has a great deal of text in it</div>`+
			`<div style="border-style:solid">and this second column is also quite long</div>`+
			`</div>`, i)
	}
	return sb.String()
}

func benchmarkRender(b *testing.B, html string, width int) {
	b.Helper()
	r, err := htmlterm.New(htmlterm.Options{Width: width})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	// Render once outside the timer: the first call resolves the document's
	// counters and cascade, which isn't what any of these measure.
	if _, err := r.Render(html); err != nil {
		b.Fatalf("Render: %v", err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := r.Render(html); err != nil {
			b.Fatalf("Render: %v", err)
		}
	}
}

// BenchmarkFlexNestedContainers is the regression guard for measurement
// recursion, and the reason this file exists. Engine.minContentCache memoizes
// measureMinContentWidth per node, so every node is measured once per render
// even though the measurement itself renders that node's whole subtree.
//
// Read it as a *curve*, not as a number: what this guards is the shape, and the
// two shapes are far apart. Measured on an M4, per depth-doubling:
//
//	depth      2        4        8       16
//	cached   2.7ms    7.8ms   28.8ms   158ms     (~3-5x per doubling)
//	no cache 3.3ms   16.4ms    651ms   (>120s)   (~5x, then ~40x)
//
// Cached is polynomial and dominated by the box model itself — plain nested
// bordered <div>s with no flex at all run 2.1/4.9/12.8/38ms at the same depths,
// since every level re-wraps the lines of the level below. Uncached is
// exponential: each level re-measures the subtree that every level beneath it
// already measured. A regression that drops or defeats the cache will not look
// like a slow benchmark, it will look like depth=16 never finishing.
//
// The remaining gap between the cached row and the plain-block control is
// inherent to measuring by rendering: a node's measurement renders its subtree,
// so a chain of n containers costs O(n^2) even when each is measured only once.
// Closing that would need a real intrinsic-size pass rather than a trial render.
func BenchmarkFlexNestedContainers(b *testing.B) {
	for _, depth := range []int{2, 4, 8, 16} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			benchmarkRender(b, buildNestedFlexDocument(depth), 100)
		})
	}
}

// BenchmarkFlexWideDocument covers the ordinary case: a document that is mostly
// flex rows, none of them nested. Cost here should be linear in the row count
// and dominated by the per-item trial renders, so it's the benchmark that moves
// if flex base size resolution gets more expensive per item.
func BenchmarkFlexWideDocument(b *testing.B) {
	benchmarkRender(b, buildWideFlexDocument(300), 80)
}

// BenchmarkFlexShrinkOverflowingRows isolates layoutFlexLine's shrink path,
// where each item's automatic minimum size has to be resolved. Comparing it
// against BenchmarkFlexWideDocument is what shows whether that extra
// measurement is being taken lazily (only on lines that actually overflow) or
// eagerly for every line.
func BenchmarkFlexShrinkOverflowingRows(b *testing.B) {
	benchmarkRender(b, buildOverflowingFlexDocument(300), 40)
}
