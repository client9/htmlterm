package htmlterm

import (
	"bufio"
	"strings"
	"testing"
)

func decodeOne(t *testing.T, raw string) inputEvent {
	t.Helper()
	ev, err := decodeEvent(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("decodeEvent(%q): %v", raw, err)
	}
	return ev
}

func TestDecodeEventControlKeys(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"\r", "Enter"},
		{"\n", "Enter"},
		{"\x7f", "Backspace"},
		{"\x08", "Backspace"},
		{"\t", "Tab"},
		{"\x1b", "Escape"},
		{"\x1b[A", "ArrowUp"},
		{"\x1b[B", "ArrowDown"},
		{"\x1b[C", "ArrowRight"},
		{"\x1b[D", "ArrowLeft"},
	}
	for _, c := range cases {
		ev := decodeOne(t, c.raw)
		if ev.kind != keyEvent || ev.key != c.want {
			t.Errorf("decodeEvent(%q) = {kind:%v key:%q}, want key %q", c.raw, ev.kind, ev.key, c.want)
		}
	}
}

func TestDecodeEventPrintableRune(t *testing.T) {
	ev := decodeOne(t, "a")
	if ev.kind != keyEvent || ev.key != "a" {
		t.Errorf("decodeEvent(%q) = %+v, want key %q", "a", ev, "a")
	}
}

func TestDecodeEventMultiByteRune(t *testing.T) {
	ev := decodeOne(t, "é")
	if ev.kind != keyEvent || ev.key != "é" {
		t.Errorf("decodeEvent(%q) = %+v, want key %q", "é", ev, "é")
	}
}

func TestDecodeEventSGRMouseLeftClick(t *testing.T) {
	ev := decodeOne(t, "\x1b[<0;5;3M")
	if ev.kind != clickEvent {
		t.Fatalf("decodeEvent(SGR left press) kind = %v, want clickEvent", ev.kind)
	}
	if ev.row != 2 || ev.col != 4 {
		t.Errorf("decodeEvent(SGR left press) = {row:%d col:%d}, want {row:2 col:4}", ev.row, ev.col)
	}
}

func TestDecodeEventSGRMouseReleaseIgnored(t *testing.T) {
	ev := decodeOne(t, "\x1b[<0;5;3m")
	if ev.kind == clickEvent {
		t.Errorf("decodeEvent(SGR release) = %+v, want a non-click event", ev)
	}
}

func TestDecodeEventSGROtherButtonIgnored(t *testing.T) {
	ev := decodeOne(t, "\x1b[<2;5;3M") // right button press
	if ev.kind == clickEvent {
		t.Errorf("decodeEvent(SGR right-button press) = %+v, want a non-click event", ev)
	}
}
