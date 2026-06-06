package tools

import (
	"context"

	"github.com/pushkaranand/finagent/internal/llm"
)

// GetSchema returns the pre-built database schema string to the agent.
// The schema is built once at startup via BuildSchemaString and cached here.
type GetSchema struct {
	schema string
}

// NewGetSchema creates a GetSchema tool with the pre-built schema string.
func NewGetSchema(schema string) *GetSchema {
	return &GetSchema{schema: schema}
}

// Definition returns the tool's name, description, and parameter schema.
func (t *GetSchema) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_schema",
		Description: "Returns the full database schema including table definitions, column types, and enum values. Call this before writing a SQL query with execute_sql so you know the exact table and column names.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

// Execute returns the cached schema string.
func (t *GetSchema) Execute(_ context.Context, _ string, _ string) (string, error) {
	return t.schema, nil
}
