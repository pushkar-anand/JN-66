package tools

import "strings"

// sanitizeField replaces newlines and ASCII control characters in a string
// with a space. Applied to all free-text values read from the DB before they
// are embedded in tool-result text that the LLM will read.
//
// Without this, a crafted merchant description or account name containing
// newlines can break the line-based structure of tool output and make
// injected text look like a new instruction to the model (indirect prompt
// injection via data).
func sanitizeField(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || (r < 0x20 && r != '\t') {
			return ' '
		}
		return r
	}, s)
}