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

// LLMTurn captures one Chat() call and its response.
type LLMTurn struct {
	Round    int
	Messages []llm.Message
	Response llm.ChatResponse
	Err      error
}

// ToolInvocation captures one tool Execute() call and its result.
type ToolInvocation struct {
	Name     string
	ArgsJSON string
	Result   string
	Err      error
}

// RecordingLLM wraps an LLM provider, counting Chat() calls and logging each turn.
type RecordingLLM struct {
	inner  llmProvider
	mu     sync.Mutex
	rounds int
	turns  []LLMTurn
}

// NewRecordingLLM creates a recorder wrapping inner.
func NewRecordingLLM(inner llmProvider) *RecordingLLM {
	return &RecordingLLM{inner: inner}
}

func (r *RecordingLLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	r.mu.Lock()
	r.rounds++
	round := r.rounds
	r.mu.Unlock()

	resp, err := r.inner.Chat(ctx, req)

	r.mu.Lock()
	r.turns = append(r.turns, LLMTurn{
		Round:    round,
		Messages: req.Messages,
		Response: resp,
		Err:      err,
	})
	r.mu.Unlock()

	return resp, err
}

// Rounds returns the number of Chat() calls made since the last Reset.
func (r *RecordingLLM) Rounds() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rounds
}

// Turns returns a snapshot of all captured LLM turns since the last Reset.
func (r *RecordingLLM) Turns() []LLMTurn {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]LLMTurn, len(r.turns))
	copy(out, r.turns)
	return out
}

// Reset clears the round counter and turn log.
func (r *RecordingLLM) Reset() {
	r.mu.Lock()
	r.rounds = 0
	r.turns = r.turns[:0]
	r.mu.Unlock()
}

// RecordingRegistry wraps a tool registry, recording tool names, args, and results.
type RecordingRegistry struct {
	inner   toolRegistry
	mu      sync.Mutex
	calls   []string
	invokes []ToolInvocation
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

	result, err := r.inner.Execute(ctx, name, callID, argsJSON)

	r.mu.Lock()
	r.invokes = append(r.invokes, ToolInvocation{
		Name:     name,
		ArgsJSON: argsJSON,
		Result:   result,
		Err:      err,
	})
	r.mu.Unlock()

	return result, err
}

// Calls returns a snapshot of tool names in the order they were called.
func (r *RecordingRegistry) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// Invocations returns a snapshot of all tool calls with args and results.
func (r *RecordingRegistry) Invocations() []ToolInvocation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ToolInvocation, len(r.invokes))
	copy(out, r.invokes)
	return out
}

// Reset clears the recorded calls and invocations.
func (r *RecordingRegistry) Reset() {
	r.mu.Lock()
	r.calls = r.calls[:0]
	r.invokes = r.invokes[:0]
	r.mu.Unlock()
}
