package tools

import (
	"context"
	"testing"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTool struct {
	name   string
	result string
}

func (s *stubTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{Name: s.name, Description: "stub"}
}

func (s *stubTool) Execute(_ context.Context, _ string, _ string) (string, error) {
	return s.result, nil
}

func TestRegistry_ExecuteRegisteredTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "ping", result: "pong"})

	got, err := r.Execute(t.Context(), "ping", "call-1", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "pong", got)
}

func TestRegistry_ExecuteUnknownTool(t *testing.T) {
	r := NewRegistry()

	_, err := r.Execute(t.Context(), "nonexistent", "call-1", `{}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestRegistry_DefinitionsReturnsAllTools(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "tool-a"})
	r.Register(&stubTool{name: "tool-b"})

	defs := r.Definitions()
	assert.Len(t, defs, 2)

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	assert.True(t, names["tool-a"])
	assert.True(t, names["tool-b"])
}

func TestRegistry_DuplicateRegistrationPanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "dup"})

	assert.Panics(t, func() {
		r.Register(&stubTool{name: "dup"})
	})
}
