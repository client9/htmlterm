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
	defaultTrack := scrollbarStyle{char: "│", style: newInlineStyle()}
	defaultThumb := scrollbarStyle{char: "█", style: newInlineStyle()}
	noCap := scrollbarStyle{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, capStart, capEnd := appendScrollbarColumn(tc.lines, tc.offset, tc.totalLines, tc.heightLines, tc.innerW, 1, defaultTrack, defaultThumb, noCap, noCap, false, false, colorprofile.NoTTY)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("appendScrollbarColumn(%v, %d, %d, %d, %d) = %v, want %v",
					tc.lines, tc.offset, tc.totalLines, tc.heightLines, tc.innerW, got, tc.want)
			}
			if capStart || capEnd {
				t.Errorf("appendScrollbarColumn with hasCapStart/hasCapEnd both false returned capStart=%v capEnd=%v, want both false", capStart, capEnd)
			}
		})
	}
}

func TestAppendScrollbarColumnCustomGlyphsAndWidth(t *testing.T) {
	track := scrollbarStyle{char: "-", style: newInlineStyle()}
	thumb := scrollbarStyle{char: "=", style: newInlineStyle()}
	noCap := scrollbarStyle{}
	got, _, _ := appendScrollbarColumn([]string{"a", "b"}, 0, 2, 2, 1, 3, track, thumb, noCap, noCap, false, false, colorprofile.NoTTY)
	want := []string{"a===", "b==="}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendScrollbarColumn with width 3 = %v, want %v", got, want)
	}
}

func TestAppendScrollbarColumnStyledGlyphs(t *testing.T) {
	track := scrollbarStyle{char: "|", style: extractInlineStyle(map[string]string{"color": "#ff0000"})}
	thumb := scrollbarStyle{char: "#", style: extractInlineStyle(map[string]string{"background-color": "#0000ff", "font-weight": "bold"})}
	noCap := scrollbarStyle{}
	got, _, _ := appendScrollbarColumn([]string{"x", "y"}, 0, 4, 2, 1, 1, track, thumb, noCap, noCap, false, false, colorprofile.TrueColor)
	wantTrack := extractInlineStyle(map[string]string{"color": "#ff0000"}).render("|", colorprofile.TrueColor)
	wantThumb := extractInlineStyle(map[string]string{"background-color": "#0000ff", "font-weight": "bold"}).render("#", colorprofile.TrueColor)
	want := []string{"x" + wantThumb, "y" + wantTrack}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendScrollbarColumn styled glyphs = %q, want %q", got, want)
	}
}

func TestAppendScrollbarColumnCaps(t *testing.T) {
	track := scrollbarStyle{char: "│", style: newInlineStyle()}
	thumb := scrollbarStyle{char: "█", style: newInlineStyle()}
	capStart := scrollbarStyle{char: "▲", style: newInlineStyle()}
	capEnd := scrollbarStyle{char: "▼", style: newInlineStyle()}
	noCap := scrollbarStyle{}

	t.Run("both caps active, thumb confined to interior track", func(t *testing.T) {
		// heightLines=4, both caps active -> interior=2 rows (indices 1,2).
		// totalLines==heightLines (4) so thumbSize==interior==2: thumb fills
		// the entire interior track, matching the no-scroll-needed
		// convention appendScrollbarColumn already has for track/thumb.
		lines := []string{"a", "b", "c", "d"}
		got, gotCapStart, gotCapEnd := appendScrollbarColumn(lines, 0, 4, 4, 1, 1, track, thumb, capStart, capEnd, true, true, colorprofile.NoTTY)
		want := []string{"a▲", "b█", "c█", "d▼"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("appendScrollbarColumn with both caps = %v, want %v", got, want)
		}
		if !gotCapStart || !gotCapEnd {
			t.Errorf("appendScrollbarColumn with both caps active returned capStart=%v capEnd=%v, want both true", gotCapStart, gotCapEnd)
		}
	})

	t.Run("only cap-start active", func(t *testing.T) {
		lines := []string{"a", "b", "c"}
		got, gotCapStart, gotCapEnd := appendScrollbarColumn(lines, 0, 3, 3, 1, 1, track, thumb, capStart, noCap, true, false, colorprofile.NoTTY)
		want := []string{"a▲", "b█", "c█"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("appendScrollbarColumn with only cap-start = %v, want %v", got, want)
		}
		if !gotCapStart || gotCapEnd {
			t.Errorf("appendScrollbarColumn with only cap-start returned capStart=%v capEnd=%v, want true/false", gotCapStart, gotCapEnd)
		}
	})

	t.Run("not enough room drops both caps", func(t *testing.T) {
		// heightLines=2, both caps requested -> heightLines-activeCaps == 0,
		// less than the required 1 interior row, so both caps are dropped
		// and ordinary track/thumb rendering applies instead.
		lines := []string{"a", "b"}
		got, gotCapStart, gotCapEnd := appendScrollbarColumn(lines, 0, 2, 2, 1, 1, track, thumb, capStart, capEnd, true, true, colorprofile.NoTTY)
		want := []string{"a█", "b█"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("appendScrollbarColumn with no room for caps = %v, want %v", got, want)
		}
		if gotCapStart || gotCapEnd {
			t.Errorf("appendScrollbarColumn with no room for caps returned capStart=%v capEnd=%v, want both false", gotCapStart, gotCapEnd)
		}
	})
}
