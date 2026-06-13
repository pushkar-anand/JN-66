package app

import (
	"context"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/store"
)

// llmEmbedder adapts llm.Provider to the store.Embedder interface.
type llmEmbedder struct {
	provider llm.Provider
	model    string
}

// NewLLMEmbedder wraps provider as a store.Embedder using the given model name.
// Returns nil when model is empty, which signals MemoryStore to skip embedding.
func NewLLMEmbedder(provider llm.Provider, model string) store.Embedder {
	if model == "" {
		return nil
	}
	return &llmEmbedder{provider: provider, model: model}
}

func (a *llmEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	resp, err := a.provider.Embed(ctx, llm.EmbedRequest{Model: a.model, Input: text})
	if err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}
