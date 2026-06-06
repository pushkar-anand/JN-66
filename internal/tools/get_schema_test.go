package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSchema_ReturnsSchema(t *testing.T) {
	schema := "TABLE foo\n  id uuid NOT NULL\n"
	got, err := NewGetSchema(schema).Execute(t.Context(), "", "")
	require.NoError(t, err)
	assert.Equal(t, schema, got)
}

func TestGetSchema_Definition(t *testing.T) {
	def := NewGetSchema("").Definition()
	assert.Equal(t, "get_schema", def.Name)
}
