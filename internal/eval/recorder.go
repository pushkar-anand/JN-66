// Package eval provides an agent eval framework for behavioural regression testing.
package eval

import (
	"context"
	"sync"

	"github.com/pushkaranand/finagent/internal/llm"
)

// llmProvider mirrors the agent package's chatProvider interface.
type llmProvider interface {
	Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error)
}

// toolRegistry mirrors the agent package's toolRegistry interface.
type toolRegistry interface {
	Definitions() []llm.ToolDefinition
	Execute(ctx context.Context, name, callID, argsJSON string) (string, error)
}

// RecordingLLM wraps an LLM provider and counts Chat() calls (= agent loop rounds).
type RecordingLLM struct {
	inner  llmProvider
	mu     sync.Mutex
	rounds int
}

// NewRecordingLLM creates a recorder wrapping inner.
func NewRecordingLLM(inner llmProvider) *RecordingLLM {
	return &RecordingLLM{inner: inner}
}

func (r *RecordingLLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	r.mu.Lock()
	r.rounds++
	r.mu.Unlock()
	return r.inner.Chat(ctx, req)
}

// Rounds returns the number of Chat() calls made since the last Reset.
func (r *RecordingLLM) Rounds() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rounds
}

// Reset clears the round counter.
func (r *RecordingLLM) Reset() {
	r.mu.Lock()
	r.rounds = 0
	r.mu.Unlock()
}

// RecordingRegistry wraps a tool registry and records tool names in call order.
type RecordingRegistry struct {
	inner toolRegistry
	mu    sync.Mutex
	calls []string
}

// NewRecordingRegistry creates a recorder wrapping inner.
func NewRecordingRegistry(inner toolRegistry) *RecordingRegistry {
	return &RecordingRegistry{inner: inner}
}

func (r *RecordingRegistry) Definitions() []llm.ToolDefinition {
	return r.inner.Definitions()
}

func (r *RecordingRegistry) Execute(ctx context.Context, name, callID, argsJSON string) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, name)
	r.mu.Unlock()
	return r.inner.Execute(ctx, name, callID, argsJSON)
}

// Calls returns a snapshot of tool names in the order they were called.
func (r *RecordingRegistry) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// Reset clears the recorded calls.
func (r *RecordingRegistry) Reset() {
	r.mu.Lock()
	r.calls = r.calls[:0]
	r.mu.Unlock()
}
