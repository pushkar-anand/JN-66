package agent

import "github.com/pushkaranand/finagent/internal/model"

// sanitizePromptField removes newlines and control characters from a
// DB-sourced string before it is interpolated into the system prompt.
// Delegates to model.SanitizeText so both the agent and tools layers share
// a single implementation.
func sanitizePromptField(s string) string {
	return model.SanitizeText(s)
}
