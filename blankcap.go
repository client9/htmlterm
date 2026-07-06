package htmlterm

import "unicode"

// isBlankLine reports whether line (with or without a trailing "\n") consists
// entirely of whitespace. Ported verbatim from the retired cappedWriter.
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

// capBlankRuns collapses runs of consecutive blank lines down to at most
// maxBlanks, replacing every retained blank line's content with "" — this
// mirrors cappedWriter's flushNLBuf, which only ever tracked a pending
// newline count, never the original blank line's bytes, so a blank line's
// real content (e.g. stray spaces, an NBSP) was always discarded once
// capping was enabled, regardless of whether that specific run actually
// exceeded the cap. maxBlanks <= 0 disables capping (returns lines
// unchanged). pre marks lines that must never be touched or counted as part
// of a run — a pre line always ends whatever run preceded it and starts a
// fresh one after; nil means no lines are pre.
//
// This is a single pass over the fully-composed root box, replacing
// cappedWriter's per-instance buffering entirely — maxBlankLines is a
// defensive feature for messy/untrusted input (e.g. pasted email HTML),
// not a layout-correctness requirement, so a single final pass is
// sufficient; nothing needs it applied at intermediate composition points.
func capBlankRuns(lines []string, pre []bool, maxBlanks int) []string {
	if maxBlanks <= 0 {
		return lines
	}
	isPre := func(i int) bool { return pre != nil && i < len(pre) && pre[i] }
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if isPre(i) || !isBlankLine(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i
		for j < len(lines) && !isPre(j) && isBlankLine(lines[j]) {
			j++
		}
		runLen := min(j-i, maxBlanks)
		for range runLen {
			out = append(out, "")
		}
		i = j
	}
	return out
}
