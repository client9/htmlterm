package document_test

import "regexp"

var ansiRe = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
