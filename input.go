package htmlterm

import (
	"bufio"
	"strconv"
	"strings"
)

// eventKind distinguishes the two shapes an inputEvent can take.
type eventKind int

const (
	keyEvent eventKind = iota
	clickEvent
)

// inputEvent is decodeEvent's result: either a keyEvent (key holds a
// DispatchKey-shaped string) or a clickEvent (row/col are 0-indexed,
// terminal-viewport-absolute — Loop.Run translates them into a Document's
// own coordinate space before calling DispatchClick).
type inputEvent struct {
	kind     eventKind
	key      string
	row, col int
}

// decodeEvent reads one logical input event off r: a control key, a
// printable rune, or an SGR mouse report. It intentionally covers only the
// vocabulary Document.DispatchKey defines and left-button mouse presses —
// see terminal.go's callers and INTERACTIVE.md for the rest of the input
// story; this is a restricted decoder, not a general VT100 parser (the same
// "restricted subset" stance this package already takes for HTML/CSS).
func decodeEvent(r *bufio.Reader) (inputEvent, error) {
	b, err := r.ReadByte()
	if err != nil {
		return inputEvent{}, err
	}

	switch b {
	case '\r', '\n':
		return inputEvent{kind: keyEvent, key: "Enter"}, nil
	case '\x7f', '\x08':
		return inputEvent{kind: keyEvent, key: "Backspace"}, nil
	case '\t':
		return inputEvent{kind: keyEvent, key: "Tab"}, nil
	case 0x1b:
		return decodeEscape(r)
	}

	// Reassemble a full UTF-8 rune: b is its first byte: read any
	// continuation bytes buffered right behind it.
	n := utf8ByteCount(b)
	buf := make([]byte, 0, n)
	buf = append(buf, b)
	for len(buf) < n {
		peek, err := r.Peek(1)
		if err != nil || peek[0]&0xC0 != 0x80 {
			break
		}
		cb, _ := r.ReadByte()
		buf = append(buf, cb)
	}
	return inputEvent{kind: keyEvent, key: string(buf)}, nil
}

// utf8ByteCount returns how many bytes a UTF-8 sequence starting with lead
// should occupy, based on its high bits.
func utf8ByteCount(lead byte) int {
	switch {
	case lead&0x80 == 0x00:
		return 1
	case lead&0xE0 == 0xC0:
		return 2
	case lead&0xF0 == 0xE0:
		return 3
	case lead&0xF8 == 0xF0:
		return 4
	default:
		return 1
	}
}

// decodeEscape handles the byte(s) following a lone 0x1b: either a bare
// Escape key (nothing more buffered), an arrow-key CSI sequence, or an SGR
// mouse report (CSI < ... M/m).
func decodeEscape(r *bufio.Reader) (inputEvent, error) {
	peek, err := r.Peek(1)
	if err != nil || peek[0] != '[' {
		return inputEvent{kind: keyEvent, key: "Escape"}, nil
	}
	if _, err := r.ReadByte(); err != nil { // consume '['
		return inputEvent{kind: keyEvent, key: "Escape"}, nil
	}

	next, err := r.Peek(1)
	if err != nil {
		return inputEvent{kind: keyEvent, key: "Escape"}, nil
	}
	switch next[0] {
	case 'A':
		_, _ = r.ReadByte()
		return inputEvent{kind: keyEvent, key: "ArrowUp"}, nil
	case 'B':
		_, _ = r.ReadByte()
		return inputEvent{kind: keyEvent, key: "ArrowDown"}, nil
	case 'C':
		_, _ = r.ReadByte()
		return inputEvent{kind: keyEvent, key: "ArrowRight"}, nil
	case 'D':
		_, _ = r.ReadByte()
		return inputEvent{kind: keyEvent, key: "ArrowLeft"}, nil
	case '<':
		_, _ = r.ReadByte()
		return decodeSGRMouse(r)
	}
	return inputEvent{kind: keyEvent, key: "Escape"}, nil
}

// decodeSGRMouse parses the body of an SGR mouse report (everything after
// "\x1b[<") of the form "Cb;Cx;Cy" followed by a terminating 'M' (press) or
// 'm' (release): "\x1b[<Cb;Cx;Cyf" — https://vt100.net/docs/vt510-rm/SGR.
// Only a left-button press (Cb's low 2 bits == 0, terminator 'M') produces a
// clickEvent; releases, drags, other buttons, and wheel events are consumed
// but reported back as a (non-click) keyEvent with an empty key, which the
// caller (Loop.Run) simply ignores — a deliberate v1 simplification.
func decodeSGRMouse(r *bufio.Reader) (inputEvent, error) {
	var body strings.Builder
	var terminator byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return inputEvent{}, err
		}
		if b == 'M' || b == 'm' {
			terminator = b
			break
		}
		body.WriteByte(b)
	}

	parts := strings.SplitN(body.String(), ";", 3)
	if len(parts) != 3 {
		return inputEvent{kind: keyEvent}, nil
	}
	cb, err1 := strconv.Atoi(parts[0])
	cx, err2 := strconv.Atoi(parts[1])
	cy, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return inputEvent{kind: keyEvent}, nil
	}

	isLeftButton := cb&0x3 == 0 && cb&0x20 == 0 // low 2 bits 0 (left), motion bit (0x20) clear
	if terminator != 'M' || !isLeftButton {
		return inputEvent{kind: keyEvent}, nil
	}
	return inputEvent{kind: clickEvent, row: cy - 1, col: cx - 1}, nil
}
