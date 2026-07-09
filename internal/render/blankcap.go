package render

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
// unchanged, and rowRemap is the identity mapping). pre marks lines that
// must never be touched or counted as part of a run — a pre line always
// ends whatever run preceded it and starts a fresh one after; nil means no
// lines are pre.
//
// rowRemap[i] gives the row in the returned lines that old row i landed on,
// for every i in [0, len(lines)) — needed because capping can remove rows
// entirely, which would otherwise silently invalidate any row index recorded
// against the pre-capping lines (e.g. Document.Rect's position map, built
// during composition before this final pass runs). A capped-away row's
// entry points at the nearest surviving row before it (or 0 if it was
// capped away entirely at the very start) — a tracked element's own row is
// never itself blank, so it's never actually removed; only rows strictly
// between two tracked elements can be.
//
// This is a single pass over the fully-composed root box, replacing
// cappedWriter's per-instance buffering entirely — maxBlankLines is a
// defensive feature for messy/untrusted input (e.g. pasted email HTML),
// not a layout-correctness requirement, so a single final pass is
// sufficient; nothing needs it applied at intermediate composition points.
func capBlankRuns(lines []string, pre []bool, maxBlanks int) (out []string, rowRemap []int) {
	rowRemap = make([]int, len(lines))
	if maxBlanks <= 0 {
		for i := range rowRemap {
			rowRemap[i] = i
		}
		return lines, rowRemap
	}
	isPre := func(i int) bool { return pre != nil && i < len(pre) && pre[i] }
	out = make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if isPre(i) || !isBlankLine(lines[i]) {
			rowRemap[i] = len(out)
			out = append(out, lines[i])
			i++
			continue
		}
		j := i
		for j < len(lines) && !isPre(j) && isBlankLine(lines[j]) {
			j++
		}
		runLen := min(j-i, maxBlanks)
		for k := i; k < j; k++ {
			rowRemap[k] = len(out) - 1 + min(k-i+1, runLen)
		}
		for range runLen {
			out = append(out, "")
		}
		i = j
	}
	return out, rowRemap
}
