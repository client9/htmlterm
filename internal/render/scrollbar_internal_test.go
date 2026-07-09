package render

import (
	"reflect"
	"testing"
)

func TestAppendScrollbarColumn(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		offset      int
		totalLines  int
		heightLines int
		want        []string
	}{
		{
			name:        "content fits, thumb fills whole track",
			lines:       []string{"a", "b"},
			offset:      0,
			totalLines:  2,
			heightLines: 2,
			want:        []string{"a█", "b█"},
		},
		{
			name:        "scrolled to top, thumb at top of track",
			lines:       []string{"a", "b"},
			offset:      0,
			totalLines:  4,
			heightLines: 2,
			want:        []string{"a█", "b│"},
		},
		{
			name:        "scrolled to bottom, thumb at bottom of track",
			lines:       []string{"c", "d"},
			offset:      2, // maxOffset = 4-2 = 2
			totalLines:  4,
			heightLines: 2,
			want:        []string{"c│", "d█"},
		},
		{
			name:        "large content, small thumb in the middle",
			lines:       []string{"a", "b", "c", "d", "e"},
			offset:      45, // maxOffset = 100-5 = 95; thumbSize = 5*5/100 = 0 -> clamped to 1
			totalLines:  100,
			heightLines: 5,
			// thumbStart = 45 * (5-1) / 95 = 180/95 = 1
			want: []string{"a│", "b█", "c│", "d│", "e│"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appendScrollbarColumn(tc.lines, tc.offset, tc.totalLines, tc.heightLines)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("appendScrollbarColumn(%v, %d, %d, %d) = %v, want %v",
					tc.lines, tc.offset, tc.totalLines, tc.heightLines, got, tc.want)
			}
		})
	}
}
