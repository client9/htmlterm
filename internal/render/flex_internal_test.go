package render

import (
	"reflect"
	"testing"
)

// allIndices is the "every item is on this flex line" group argument
// distributeFlexGrow/distributeFlexShrink take, matching layoutFlexColumn's
// own allIdx.
func allIndices(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}

func TestProportionalSharesSumsExactlyAndStaysNonNegative(t *testing.T) {
	tests := []struct {
		name    string
		weights []float64
		total   int
		want    []int
	}{
		{name: "equal weights split evenly when divisible", weights: []float64{1, 1}, total: 18, want: []int{9, 9}},
		{name: "weighted split is proportional", weights: []float64{1, 2}, total: 19, want: []int{6, 13}},
		{name: "flooring remainder goes to the earliest largest fractions", weights: []float64{1, 1, 1}, total: 2, want: []int{1, 1, 0}},
		{name: "zero-weight members get nothing", weights: []float64{0, 1, 0}, total: 5, want: []int{0, 5, 0}},
		{name: "no positive weight distributes nothing", weights: []float64{0, 0}, total: 5, want: []int{0, 0}},
		{name: "non-positive total distributes nothing", weights: []float64{1, 1}, total: 0, want: []int{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proportionalShares(tt.weights, allIndices(len(tt.weights)), tt.total)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("proportionalShares(%v, %d) = %v, want %v", tt.weights, tt.total, got, tt.want)
			}
		})
	}
}

// TestProportionalSharesManyMembersNeverGoesNegative pins the regression that
// motivated the largest-remainder method: rounding each member's share up
// independently and dumping the (negative) remainder on the last member drove
// that member's size below its own basis - here to -7 - so the line overflowed
// its container. See proportionalShares' doc comment.
func TestProportionalSharesManyMembersNeverGoesNegative(t *testing.T) {
	const n, total = 20, 10
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = 1
	}
	shares := proportionalShares(weights, allIndices(n), total)
	sum := 0
	for i, s := range shares {
		if s < 0 {
			t.Errorf("share[%d] = %d, want >= 0", i, s)
		}
		sum += s
	}
	if sum != total {
		t.Errorf("shares sum = %d, want %d (shares=%v)", sum, total, shares)
	}
}

func TestParseFlexDirection(t *testing.T) {
	tests := []struct {
		value      string
		wantColumn bool
		wantRev    bool
	}{
		{value: "row"},
		{value: "row-reverse", wantRev: true},
		{value: "column", wantColumn: true},
		{value: "column-reverse", wantColumn: true, wantRev: true},
		// Unset and invalid both mean the initial value, row. A prefix/suffix
		// match would read the last two as column and as row-reverse.
		{value: ""},
		{value: "columns"},
		{value: "sideways-reverse"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			gotColumn, gotRev := parseFlexDirection(map[string]string{"flex-direction": tt.value})
			if gotColumn != tt.wantColumn || gotRev != tt.wantRev {
				t.Errorf("parseFlexDirection(%q) = (column=%v, reverse=%v), want (column=%v, reverse=%v)",
					tt.value, gotColumn, gotRev, tt.wantColumn, tt.wantRev)
			}
		})
	}
}

func TestDistributeFlexGrow(t *testing.T) {
	t.Run("grown sizes never drop below their basis", func(t *testing.T) {
		sizes := make([]int, 20)
		grows := make([]float64, 20)
		for i := range sizes {
			sizes[i], grows[i] = 2, 1
		}
		left := distributeFlexGrow(sizes, grows, allIndices(20), 10, nil)
		total := 0
		for i, s := range sizes {
			if s < 2 {
				t.Errorf("sizes[%d] = %d, want >= its basis 2", i, s)
			}
			total += s
		}
		if total != 50 || left != 0 {
			t.Errorf("total = %d, unabsorbed = %d; want 50 and 0 (sizes=%v)", total, left, sizes)
		}
	})

	t.Run("a ceilinged item's unusable share is redistributed to the rest", func(t *testing.T) {
		sizes := []int{10, 10}
		left := distributeFlexGrow(sizes, []float64{1, 1}, allIndices(2), 6, []int{11, 0})
		if want := []int{11, 15}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (item 0 frozen at its ceiling, its 2 leftover units re-split onto item 1)", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("space no member can absorb is reported, not silently dropped", func(t *testing.T) {
		sizes := []int{10, 10}
		left := distributeFlexGrow(sizes, []float64{1, 1}, allIndices(2), 6, []int{11, 12})
		if want := []int{11, 12}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (both at their ceilings)", sizes, want)
		}
		if left != 3 {
			t.Errorf("unabsorbed = %d, want 3", left)
		}
	})

	t.Run("no growable member absorbs nothing", func(t *testing.T) {
		sizes := []int{4, 4}
		if left := distributeFlexGrow(sizes, []float64{0, 0}, allIndices(2), 6, nil); left != 6 {
			t.Errorf("unabsorbed = %d, want 6", left)
		}
		if want := []int{4, 4}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
	})

	// CSS Flexbox 1 section 9.7 step 4b: flex factors summing to less than one
	// distribute only that fraction of the initial free space. Without the
	// clamp, proportionalShares' normalize-by-total-weight handed a lone
	// flex-grow:0.5 item all of the free space.
	t.Run("factors summing below one distribute only that fraction", func(t *testing.T) {
		sizes := []int{10}
		left := distributeFlexGrow(sizes, []float64{0.5}, allIndices(1), 20, nil)
		if want := []int{20}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (0.5 x 20 free = 10 absorbed)", sizes, want)
		}
		if left != 10 {
			t.Errorf("unabsorbed = %d, want 10", left)
		}
	})

	t.Run("the sub-one clamp sums across every growable member", func(t *testing.T) {
		sizes := []int{5, 5}
		left := distributeFlexGrow(sizes, []float64{0.25, 0.25}, allIndices(2), 20, nil)
		if want := []int{10, 10}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (0.5 x 20 free = 10, split evenly)", sizes, want)
		}
		if left != 10 {
			t.Errorf("unabsorbed = %d, want 10", left)
		}
	})

	t.Run("factors summing to one or more still absorb everything", func(t *testing.T) {
		sizes := []int{5, 5}
		left := distributeFlexGrow(sizes, []float64{0.5, 0.5}, allIndices(2), 20, nil)
		if want := []int{15, 15}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	// A member frozen at its ceiling drops out of the factor sum, tightening
	// the clamp on the next round; the survivor's entitlement is a cumulative
	// bound on what it may hold in total, not a fresh per-round allowance, so
	// the units the frozen member couldn't use stay unplaced here rather than
	// moving to the survivor the way an unclamped redistribution would.
	t.Run("a frozen member's factor leaves the sub-one sum", func(t *testing.T) {
		sizes := []int{10, 10}
		left := distributeFlexGrow(sizes, []float64{0.3, 0.3}, allIndices(2), 20, []int{11, 0})
		if want := []int{11, 16}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (0.6 x 20 = 12: 6 each, item 0 capped at 11; item 1's own 0.3 x 20 = 6 entitlement is already spent)", sizes, want)
		}
		if left != 13 {
			t.Errorf("unabsorbed = %d, want 13", left)
		}
	})
}

// TestGrowFlexLine covers the distinction CSS Flexbox §9.7 draws between an
// item's flex base size (what growth is measured from) and its hypothetical
// main size (the base already clamped by the item's used minimum, applied here
// only as a step-4c violation). Collapsing the two — growing straight from the
// hypothetical — is what used to make `flex: 1` items come out sized by their
// content instead of by their flex factors.
func TestGrowFlexLine(t *testing.T) {
	t.Run("equal factors split the container, not the leftover above min-content", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{0, 0}, []int{3, 8}, nil, []float64{1, 1}, allIndices(2), 40)
		if want := []int{20, 20}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (growing from the hypotheticals instead gives 17/23)", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("an item below its hypothetical freezes there and the rest is redivided", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{0, 0}, []int{3, 18}, nil, []float64{1, 1}, allIndices(2), 30)
		if want := []int{12, 18}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (item 1 violates its minimum at the equal 15, freezes at 18, and item 0 takes all 12 that remain — not the 15 the violated round offered it)", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	// §9.7 step 3, "size inflexible items": an item that can't grow takes no
	// part in the distribution and sits at its hypothetical main size.
	t.Run("a zero flex-grow item is frozen at its hypothetical", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{0, 0}, []int{6, 4}, nil, []float64{0, 1}, allIndices(2), 20)
		if want := []int{6, 14}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	// Step 3's other freeze case: the item's own maximum already clamped its
	// base size *down* to the hypothetical, so there is nothing to grow into.
	t.Run("a base above its hypothetical is frozen there", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{12, 0}, []int{5, 4}, []int{5, 0}, []float64{1, 1}, allIndices(2), 20)
		if want := []int{5, 15}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (item 0 clamped by max-width, item 1 takes the rest)", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("space no member can absorb is reported, not silently dropped", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{2, 2}, []int{2, 2}, []int{6, 6}, []float64{1, 1}, allIndices(2), 20)
		if want := []int{6, 6}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (both at their ceilings)", sizes, want)
		}
		if left != 8 {
			t.Errorf("unabsorbed = %d, want 8", left)
		}
	})

	t.Run("every item frozen leaves the whole free space for justify-content", func(t *testing.T) {
		sizes := make([]int, 2)
		left := growFlexLine(sizes, []int{4, 4}, []int{4, 4}, nil, []float64{0, 0}, allIndices(2), 20)
		if want := []int{4, 4}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
		if left != 12 {
			t.Errorf("unabsorbed = %d, want 12", left)
		}
	})
}

func TestDistributeFlexShrink(t *testing.T) {
	t.Run("shrinks proportionally to the scaled shrink factor", func(t *testing.T) {
		sizes := []int{4, 4, 4}
		left := distributeFlexShrink(sizes, []float64{1, 1, 1}, append([]int(nil), sizes...), []int{1, 1, 1}, allIndices(3), 2)
		if want := []int{3, 3, 4}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("a floored item's unusable share is redistributed to the rest", func(t *testing.T) {
		sizes := []int{8, 8}
		left := distributeFlexShrink(sizes, []float64{1, 1}, append([]int(nil), sizes...), []int{7, 1}, allIndices(2), 4)
		if want := []int{7, 5}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (item 0 frozen at its floor, its leftover unit re-split onto item 1)", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	// The last member being the floored one is the case a single pass got
	// wrong: it had nowhere to hand the remainder, so a unit of the deficit
	// went unabsorbed even though the first two items still had room.
	t.Run("a floored last item does not strand deficit the others could absorb", func(t *testing.T) {
		sizes := []int{10, 10, 10}
		left := distributeFlexShrink(sizes, []float64{1, 1, 1}, append([]int(nil), sizes...), []int{1, 1, 9}, allIndices(3), 6)
		if got, want := sizes[0]+sizes[1]+sizes[2], 24; got != want {
			t.Errorf("shrunk total = %d, want %d (sizes=%v)", got, want, sizes)
		}
		if sizes[2] < 9 {
			t.Errorf("sizes[2] = %d, want >= its floor 9", sizes[2])
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("deficit no member can absorb is reported, not silently dropped", func(t *testing.T) {
		sizes := []int{10, 10}
		left := distributeFlexShrink(sizes, []float64{1, 1}, append([]int(nil), sizes...), []int{9, 9}, allIndices(2), 6)
		if want := []int{9, 9}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (both at their floors)", sizes, want)
		}
		if left != 4 {
			t.Errorf("unabsorbed = %d, want 4", left)
		}
	})

	t.Run("no shrinkable member absorbs nothing", func(t *testing.T) {
		sizes := []int{4, 4}
		if left := distributeFlexShrink(sizes, []float64{0, 0}, append([]int(nil), sizes...), []int{1, 1}, allIndices(2), 3); left != 3 {
			t.Errorf("unabsorbed = %d, want 3", left)
		}
	})
}
