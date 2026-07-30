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
}

func TestDistributeFlexShrink(t *testing.T) {
	t.Run("shrinks proportionally to the scaled shrink factor", func(t *testing.T) {
		sizes := []int{4, 4, 4}
		left := distributeFlexShrink(sizes, []float64{1, 1, 1}, []int{1, 1, 1}, allIndices(3), 2)
		if want := []int{3, 3, 4}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v", sizes, want)
		}
		if left != 0 {
			t.Errorf("unabsorbed = %d, want 0", left)
		}
	})

	t.Run("a floored item's unusable share is redistributed to the rest", func(t *testing.T) {
		sizes := []int{8, 8}
		left := distributeFlexShrink(sizes, []float64{1, 1}, []int{7, 1}, allIndices(2), 4)
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
		left := distributeFlexShrink(sizes, []float64{1, 1, 1}, []int{1, 1, 9}, allIndices(3), 6)
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
		left := distributeFlexShrink(sizes, []float64{1, 1}, []int{9, 9}, allIndices(2), 6)
		if want := []int{9, 9}; !reflect.DeepEqual(sizes, want) {
			t.Errorf("sizes = %v, want %v (both at their floors)", sizes, want)
		}
		if left != 4 {
			t.Errorf("unabsorbed = %d, want 4", left)
		}
	})

	t.Run("no shrinkable member absorbs nothing", func(t *testing.T) {
		sizes := []int{4, 4}
		if left := distributeFlexShrink(sizes, []float64{0, 0}, []int{1, 1}, allIndices(2), 3); left != 3 {
			t.Errorf("unabsorbed = %d, want 3", left)
		}
	})
}
