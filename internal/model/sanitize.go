package model

import "strings"

// SanitizeText replaces newlines and ASCII control characters (except tab) in s
// with a space. Call this on any DB-sourced free-text string before it is
// embedded in LLM tool output or system prompt text, to prevent indirect prompt
// injection via crafted merchant names, account names, or memory content.
func SanitizeText(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return ' '
		}
		return r
	}, s)
}
