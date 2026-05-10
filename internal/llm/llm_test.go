package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolResultMessage(t *testing.T) {
	m := ToolResultMessage("call-1", "result text", "my_tool")
	assert.Equal(t, RoleTool, m.Role)
	assert.Equal(t, "result text", m.Content)
	assert.Equal(t, "call-1", m.ToolCallID)
	assert.Equal(t, "my_tool", m.Name)
}

func TestSystemMessage(t *testing.T) {
	m := SystemMessage("you are an assistant")
	assert.Equal(t, RoleSystem, m.Role)
	assert.Equal(t, "you are an assistant", m.Content)
	assert.Empty(t, m.ToolCalls)
}

func TestUserMessage(t *testing.T) {
	m := UserMessage("hello")
	assert.Equal(t, RoleUser, m.Role)
	assert.Equal(t, "hello", m.Content)
}
