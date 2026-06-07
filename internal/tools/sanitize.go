package tools

import "github.com/pushkaranand/finagent/internal/model"

// sanitizeField removes newlines and control characters from a DB-sourced
// string before it is embedded in tool-result text that the LLM will read.
// Delegates to model.SanitizeText so both the tools and agent layers share
// a single implementation.
func sanitizeField(s string) string {
	return model.SanitizeText(s)
}
