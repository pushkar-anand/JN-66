package agent

import "strings"

// sanitizePromptField strips newlines and ASCII control characters from a
// DB-sourced string before it is interpolated into the system prompt.
// Newlines in the system prompt allow attacker-controlled data (e.g. a crafted
// user display name or a poisoned memory entry) to inject new "lines" that the
// LLM may interpret as additional instructions.
func sanitizePromptField(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || (r < 0x20 && r != '\t') {
			return ' '
		}
		return r
	}, s)
}
