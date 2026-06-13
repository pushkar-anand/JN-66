package store

import "context"

// Embedder generates a float32 embedding vector for a text string.
// Defined here so store does not import internal/llm; the adapter lives in internal/app.
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
}
