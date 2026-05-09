// Package tools contains the agent tool registry and all individual tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/pushkaranand/finagent/internal/llm"
)

// Tool is a callable function the agent can invoke.
type Tool interface {
	// Definition returns the JSON Schema descriptor sent to the LLM.
	Definition() llm.ToolDefinition
	// Execute runs the tool with the given args JSON and returns a result string.
	Execute(ctx context.Context, callID string, argsJSON string) (string, error)
}

// Registry holds all registered tools and dispatches calls by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate names (programming error).
func (r *Registry) Register(t Tool) {
	name := t.Definition().Name
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("duplicate tool name: %s", name))
	}
	r.tools[name] = t
}

// Definitions returns the tool descriptors for the LLM chat request.
func (r *Registry) Definitions() []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute dispatches a tool call by name.
func (r *Registry) Execute(ctx context.Context, name, callID, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, callID, argsJSON)
}
