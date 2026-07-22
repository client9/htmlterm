package render

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestAppendScrollbarColumn(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		offset      int
		totalLines  int
		heightLines int
		innerW      int
		want        []string
	}{
		{
			name:        "content fits, thumb fills whole track",
			lines:       []string{"a", "b"},
			offset:      0,
			totalLines:  2,
			heightLines: 2,
			innerW:      1,
			want:        []string{"a█", "b█"},
		},
		{
			name:        "scrolled to top, thumb at top of track",
			lines:       []string{"a", "b"},
			offset:      0,
			totalLines:  4,
			heightLines: 2,
			innerW:      1,
			want:        []string{"a█", "b│"},
		},
		{
			name:        "scrolled to bottom, thumb at bottom of track",
			lines:       []string{"c", "d"},
			offset:      2, // maxOffset = 4-2 = 2
			totalLines:  4,
			heightLines: 2,
			innerW:      1,
			want:        []string{"c│", "d█"},
		},
		{
			name:        "large content, small thumb in the middle",
			lines:       []string{"a", "b", "c", "d", "e"},
			offset:      45, // maxOffset = 100-5 = 95; thumbSize = 5*5/100 = 0 -> clamped to 1
			totalLines:  100,
			heightLines: 5,
			innerW:      1,
			// thumbStart = 45 * (5-1) / 95 = 180/95 = 1
			want: []string{"a│", "b█", "c│", "d│", "e│"},
		},
		{
			name:        "ragged line widths are padded to a straight gutter column",
			lines:       []string{"short", "a much longer line here", ""},
			offset:      0,
			totalLines:  3,
			heightLines: 3,
			innerW:      23,
			want: []string{
				"short                  █",
				"a much longer line here█",
				"                       █",
			},
		},
		{
			name:        "overlong line is truncated to the gutter column, not left ragged",
			lines:       []string{"this line is way too long for the box"},
			offset:      0,
			totalLines:  1,
			heightLines: 1,
			innerW:      10,
			want:        []string{"this line █"},
		},
	}
	defaultTrack := scrollbarStyle{char: scrollbarTrackChar, style: newInlineStyle()}
	defaultThumb := scrollbarStyle{char: scrollbarThumbChar, style: newInlineStyle()}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appendScrollbarColumn(tc.lines, tc.offset, tc.totalLines, tc.heightLines, tc.innerW, 1, defaultTrack, defaultThumb, colorprofile.NoTTY)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("appendScrollbarColumn(%v, %d, %d, %d, %d) = %v, want %v",
					tc.lines, tc.offset, tc.totalLines, tc.heightLines, tc.innerW, got, tc.want)
			}
		})
	}
}

func TestAppendScrollbarColumnCustomGlyphsAndWidth(t *testing.T) {
	track := scrollbarStyle{char: "-", style: newInlineStyle()}
	thumb := scrollbarStyle{char: "=", style: newInlineStyle()}
	got := appendScrollbarColumn([]string{"a", "b"}, 0, 2, 2, 1, 3, track, thumb, colorprofile.NoTTY)
	want := []string{"a===", "b==="}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendScrollbarColumn with width 3 = %v, want %v", got, want)
	}
}

func TestAppendScrollbarColumnStyledGlyphs(t *testing.T) {
	track := scrollbarStyle{char: "|", style: extractInlineStyle(map[string]string{"color": "#ff0000"})}
	thumb := scrollbarStyle{char: "#", style: extractInlineStyle(map[string]string{"background-color": "#0000ff", "font-weight": "bold"})}
	got := appendScrollbarColumn([]string{"x", "y"}, 0, 4, 2, 1, 1, track, thumb, colorprofile.TrueColor)
	wantTrack := extractInlineStyle(map[string]string{"color": "#ff0000"}).render("|", colorprofile.TrueColor)
	wantThumb := extractInlineStyle(map[string]string{"background-color": "#0000ff", "font-weight": "bold"}).render("#", colorprofile.TrueColor)
	want := []string{"x" + wantThumb, "y" + wantTrack}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendScrollbarColumn styled glyphs = %q, want %q", got, want)
	}
}
