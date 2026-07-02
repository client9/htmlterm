package htmlterm

import (
	"strings"
	"unicode"
)

// cappedWriter accumulates rendered output, buffering non-pre newlines so that
// runs exceeding maxBlanks+1 consecutive blank lines are collapsed. When
// maxBlanks is 0 the writer is transparent: newlines pass through uncapped.
//
// pre-formatted regions are signalled with EnterPre/ExitPre; inside those
// regions newlines are written directly and never buffered or capped.
//
// Blank lines are detected via isBlankLine on the completed line (fragment+"\n"),
// which handles space-only content uniformly regardless of how it arrived.
type cappedWriter struct {
	sb        strings.Builder
	maxBlanks int             // 0 = disabled
	preDepth  int             // nesting depth of pre-formatted regions
	nlBuf     int             // buffered pending newlines (structural + blank content lines)
	fragBuf   strings.Builder // partial line awaiting its terminating '\n'
}

// flushNLBuf writes buffered newlines to sb, capped to maxBlanks+1 when
// capping is enabled.
func (w *cappedWriter) flushNLBuf() {
	n := w.nlBuf
	if w.maxBlanks > 0 && n > w.maxBlanks+1 {
		n = w.maxBlanks + 1
	}
	for range n {
		w.sb.WriteByte('\n')
	}
	w.nlBuf = 0
}

// writeNewline writes a structural newline (block terminator). In pre regions
// it goes directly to the underlying builder; otherwise it completes the
// current fragment as a line and buffers the result in nlBuf.
func (w *cappedWriter) writeNewline() {
	if w.preDepth > 0 {
		w.sb.WriteByte('\n')
		return
	}
	frag := w.fragBuf.String()
	if w.maxBlanks > 0 && isBlankLine(frag+"\n") {
		w.nlBuf++
	} else {
		w.flushNLBuf()
		w.sb.WriteString(frag)
		w.nlBuf++
	}
	w.fragBuf.Reset()
}

// WriteString writes s. In pre regions it goes directly to the builder.
// Outside pre regions, each completed line is checked with isBlankLine:
// blank lines accumulate in nlBuf; non-blank lines flush nlBuf (capped) then
// write their content, leaving the line's '\n' in nlBuf.
func (w *cappedWriter) WriteString(s string) {
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			if w.preDepth > 0 {
				w.sb.WriteString(s)
			} else {
				w.fragBuf.WriteString(s)
			}
			return
		}
		before := s[:i]
		s = s[i+1:]
		if w.preDepth > 0 {
			w.sb.WriteString(before)
			w.sb.WriteByte('\n')
			continue
		}
		w.fragBuf.WriteString(before)
		frag := w.fragBuf.String()
		if w.maxBlanks > 0 && isBlankLine(frag+"\n") {
			w.nlBuf++
		} else {
			w.flushNLBuf()
			w.sb.WriteString(frag)
			w.nlBuf++
		}
		w.fragBuf.Reset()
	}
}

// WriteAtLeastNewlines ensures at least n newlines are pending in the output.
// In pre regions n newlines are written directly. Outside pre regions the
// buffer is raised to max(nlBuf, n) — satisfying CSS margin semantics.
func (w *cappedWriter) WriteAtLeastNewlines(n int) {
	if w.preDepth > 0 {
		for range n {
			w.sb.WriteByte('\n')
		}
		return
	}
	// Flush non-blank fragment content; discard blank-only fragments (padding).
	frag := w.fragBuf.String()
	if frag != "" && (w.maxBlanks == 0 || !isBlankLine(frag+"\n")) {
		w.sb.WriteString(frag)
	}
	w.fragBuf.Reset()
	if w.nlBuf < n {
		w.nlBuf = n
	}
}

// Flush commits buffered newlines (capped to maxBlanks+1) and any pending
// fragment to the underlying builder, then resets both. It is a no-op when
// both are empty.
func (w *cappedWriter) Flush() {
	if w.nlBuf == 0 && w.fragBuf.Len() == 0 {
		return
	}
	w.flushNLBuf()
	w.sb.WriteString(w.fragBuf.String())
	w.fragBuf.Reset()
}

// EnterPre signals that subsequent writes are pre-formatted. Flushes the
// newline buffer first so the pre/non-pre boundary is clean.
func (w *cappedWriter) EnterPre() {
	w.Flush()
	w.preDepth++
}

// ExitPre signals the end of a pre-formatted region.
func (w *cappedWriter) ExitPre() {
	if w.preDepth > 0 {
		w.preDepth--
	}
}

// Len returns the total number of bytes that will appear in the output,
// including buffered newlines and any pending fragment.
func (w *cappedWriter) Len() int {
	return w.sb.Len() + w.nlBuf + w.fragBuf.Len()
}

// LastByte returns the effective last byte of the accumulated output.
// Non-blank fragment content takes precedence; then buffered newlines (nlBuf);
// then the last byte of sb. Returns (0, false) if nothing has been written.
func (w *cappedWriter) LastByte() (byte, bool) {
	frag := w.fragBuf.String()
	if frag != "" && (w.maxBlanks == 0 || !isBlankLine(frag+"\n")) {
		return frag[len(frag)-1], true
	}
	if w.nlBuf > 0 {
		return '\n', true
	}
	s := w.sb.String()
	if len(s) > 0 {
		return s[len(s)-1], true
	}
	if frag != "" {
		return frag[len(frag)-1], true
	}
	return 0, false
}

// String flushes all buffers and returns the accumulated string.
func (w *cappedWriter) String() string {
	w.Flush()
	return w.sb.String()
}

func isBlankLine(line string) bool {
	end := len(line)
	if end > 0 && line[end-1] == '\n' {
		end--
	}
	for _, r := range line[:end] {
		if !unicode.IsSpace(r) {
			return false
		}
	}

	return true
}
