package htmlterm

import "testing"

// helpers

func newCapped(maxBlanks int) *cappedWriter {
	return &cappedWriter{maxBlanks: maxBlanks}
}

func (w *cappedWriter) ws(s string) *cappedWriter { w.WriteString(s); return w }
func (w *cappedWriter) wn() *cappedWriter         { w.writeNewline(); return w }
func (w *cappedWriter) waln(n int) *cappedWriter  { w.WriteAtLeastNewlines(n); return w }
func (w *cappedWriter) ep() *cappedWriter         { w.EnterPre(); return w }
func (w *cappedWriter) xp() *cappedWriter         { w.ExitPre(); return w }

// --- Transparent mode (maxBlanks=0) ---

func TestCappedWriterTransparentPassthrough(t *testing.T) {
	// All content passes through unchanged when capping is disabled.
	w := newCapped(0)
	w.ws("hello\n\n\nworld")
	if got := w.String(); got != "hello\n\n\nworld" {
		t.Errorf("got %q, want %q", got, "hello\n\n\nworld")
	}
}

func TestCappedWriterTransparentStructuralNLs(t *testing.T) {
	// Multiple structural newlines accumulate and flush without capping.
	w := newCapped(0)
	w.ws("A").wn().wn().wn().ws("B")
	if got := w.String(); got != "A\n\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\n\nB")
	}
}

func TestCappedWriterTransparentSpaceLinesPreserved(t *testing.T) {
	// Space-only lines between blocks are preserved when maxBlanks=0.
	// This covers the visibility:hidden case: WriteString("      ") + writeNewline().
	w := newCapped(0)
	w.ws("before").wn().waln(2).ws("      ").wn().waln(2).ws("after")
	if got := w.String(); got != "before\n\n      \n\nafter" {
		t.Errorf("got %q, want %q", got, "before\n\n      \n\nafter")
	}
}

func TestCappedWriterTransparentSpaceNewlineInString(t *testing.T) {
	// " \n" within a WriteString call also passes through unchanged when maxBlanks=0.
	w := newCapped(0)
	w.ws("A").wn().wn().ws(" \n").ws("B")
	if got := w.String(); got != "A\n\n \nB" {
		t.Errorf("got %q, want %q", got, "A\n\n \nB")
	}
}

// --- Capping mode ---

func TestCappedWriterCapsStructuralNLs(t *testing.T) {
	// Many structural newlines collapse to maxBlanks+1.
	w := newCapped(1)
	w.ws("A").wn().wn().wn().wn().wn().ws("B")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterCapsContentNLs(t *testing.T) {
	// Newlines embedded in a WriteString call are also capped.
	w := newCapped(1)
	w.ws("A\n\n\n\nB")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterCapsMaxBlanks2(t *testing.T) {
	// maxBlanks=2 allows up to 2 blank lines (3 newlines total).
	w := newCapped(2)
	w.ws("A\n\n\n\n\nB")
	if got := w.String(); got != "A\n\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\n\nB")
	}
}
func TestCappedWriterCapsMaxBlanks3(t *testing.T) {
	// maxBlanks=2 allows up to 2 blank lines (3 newlines total).
	w := newCapped(2)
	w.ws("A\n\n \n\n\nB")
	if got := w.String(); got != "A\n\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\n\nB")
	}
}

func TestCappedWriterWriteAtLeastNewlinesTakesMax(t *testing.T) {
	// WriteAtLeastNewlines raises the buffer but never lowers it.
	w := newCapped(1)
	w.ws("A").waln(3).waln(1).ws("B")
	// nlBuf raised to 3 by first call; second call (1) is a no-op.
	// Capped to 2 on flush.
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterSpaceLineCountsAsBlankViaWriteNewline(t *testing.T) {
	// A space-only segment followed by writeNewline() is treated as a blank
	// line: the spaces are discarded and the NL count accumulates.
	w := newCapped(1)
	w.ws("A").wn().waln(2) // nlBuf=2
	w.ws(" ").wn()         // space buffered, writeNewline discards it → nlBuf=3
	w.ws("B")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterSpaceLineCountsAsBlankViaContentNL(t *testing.T) {
	// A space-only segment followed by a '\n' within WriteString is also
	// treated as a blank line.
	w := newCapped(1)
	w.ws("A").wn().waln(2) // nlBuf=2
	w.ws(" \n")            // '\n' discards spBuf → nlBuf=3
	w.ws("B")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterNonSpaceSegmentFlushesSpBuf(t *testing.T) {
	// If non-space content arrives after buffered spaces, the spaces are
	// flushed as real content (not a blank line).
	w := newCapped(1)
	w.ws("A").wn().waln(2) // nlBuf=2, structuralNL=true
	w.ws("   ")            // spaces → spBuf=3
	w.ws("B")              // non-space → Flush (2 NLs + 3 spaces), then "B"
	if got := w.String(); got != "A\n\n   B" {
		t.Errorf("got %q, want %q", got, "A\n\n   B")
	}
}

// --- Pre mode ---

func TestCappedWriterPreNLsPassThrough(t *testing.T) {
	// Inside a pre region, all newlines write directly and are never capped.
	w := newCapped(1)
	w.ws("A").ep().ws("line1\n\n\n\nline2").xp().wn()
	if got := w.String(); got != "Aline1\n\n\n\nline2\n" {
		t.Errorf("got %q, want %q", got, "Aline1\n\n\n\nline2\n")
	}
}

func TestCappedWriterEnterPreFlushesBuffer(t *testing.T) {
	// EnterPre flushes pending NLs (capped) before switching to pre mode.
	w := newCapped(1)
	w.ws("A").wn().wn().wn() // nlBuf=3
	w.ep().ws("pre\ncontent").xp().wn()
	if got := w.String(); got != "A\n\npre\ncontent\n" {
		t.Errorf("got %q, want %q", got, "A\n\npre\ncontent\n")
	}
}

func TestCappedWriterPreNestingDepth(t *testing.T) {
	// EnterPre/ExitPre nest: inner ExitPre doesn't leave pre mode.
	w := newCapped(1)
	w.ep().ep().ws("in\n\n\npre").xp().ws("\nstill pre").xp()
	w.ws("out").wn().wn().wn().ws("B")
	if got := w.String(); got != "in\n\n\npre\nstill preout\n\nB" {
		t.Errorf("got %q, want %q", got, "in\n\n\npre\nstill preout\n\nB")
	}
}

// --- Len and LastByte ---

func TestCappedWriterLen(t *testing.T) {
	w := newCapped(0)
	if w.Len() != 0 {
		t.Errorf("empty Len = %d, want 0", w.Len())
	}
	w.ws("abc") // sb.Len()=3, nlBuf=0
	if w.Len() != 3 {
		t.Errorf("after ws Len = %d, want 3", w.Len())
	}
	w.wn() // nlBuf=1
	if w.Len() != 4 {
		t.Errorf("after wn Len = %d, want 4", w.Len())
	}
}

func TestCappedWriterLastByte(t *testing.T) {
	w := newCapped(0)
	if _, ok := w.LastByte(); ok {
		t.Error("empty: LastByte ok=true, want false")
	}
	w.ws("abc")
	if b, ok := w.LastByte(); !ok || b != 'c' {
		t.Errorf("after 'abc': LastByte = %q %v, want 'c' true", b, ok)
	}
	w.wn()
	if b, ok := w.LastByte(); !ok || b != '\n' {
		t.Errorf("after writeNewline: LastByte = %q %v, want '\\n' true", b, ok)
	}
	// Flush changes what's in sb.
	w.ws("d") // triggers Flush, then writes 'd'
	if b, ok := w.LastByte(); !ok || b != 'd' {
		t.Errorf("after 'd': LastByte = %q %v, want 'd' true", b, ok)
	}
}

// --- WriteAtLeastNewlines semantics ---

func TestCappedWriterWriteAtLeastNewlinesFromZero(t *testing.T) {
	// WriteAtLeastNewlines(n) from zero sets nlBuf=n.
	w := newCapped(0)
	w.ws("A").waln(3).ws("B")
	if got := w.String(); got != "A\n\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\n\nB")
	}
}

func TestCappedWriterWriteAtLeastNewlinesPreMode(t *testing.T) {
	// Inside pre, WriteAtLeastNewlines writes n newlines directly.
	w := newCapped(1)
	w.ep().waln(3).xp()
	if got := w.String(); got != "\n\n\n" {
		t.Errorf("got %q, want %q", got, "\n\n\n")
	}
}

// --- Unicode whitespace ---

func TestCappedWriterNBSPCountsAsBlank(t *testing.T) {
	// U+00A0 (no-break space / HTML &nbsp;) on its own line counts as blank.
	w := newCapped(1)
	w.ws("A\n\u00a0\n\u00a0\n\u00a0\nB")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterNBSPViaWriteNewline(t *testing.T) {
	// U+00A0 in fragment when writeNewline() is called also counts as blank.
	w := newCapped(1)
	w.ws("A").wn().ws("\u00a0").wn().ws("B")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}

func TestCappedWriterNBSPTransparentPreserved(t *testing.T) {
	// In transparent mode (maxBlanks=0), U+00A0 lines are not suppressed.
	w := newCapped(0)
	w.ws("A\n\u00a0\nB")
	if got := w.String(); got != "A\n\u00a0\nB" {
		t.Errorf("got %q, want %q", got, "A\n\u00a0\nB")
	}
}

func TestCappedWriterMixedUnicodeSpacesBlank(t *testing.T) {
	// A line containing only Unicode space characters (nbsp, thin, em) is blank.
	w := newCapped(1)
	w.ws("A\n\u00a0\u2009\u2003\nB")
	if got := w.String(); got != "A\n\nB" {
		t.Errorf("got %q, want %q", got, "A\n\nB")
	}
}
