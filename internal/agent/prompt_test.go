package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemPrompt_ContainsUserName(t *testing.T) {
	p := systemPrompt("Alice", testUserID, nil)
	assert.Contains(t, p, "Alice")
	assert.Contains(t, p, "INR")
}

func TestSystemPrompt_WithMemories(t *testing.T) {
	p := systemPrompt("Bob", testUserID, []string{"Netflix = subscription", "Zomato = food delivery"})
	assert.Contains(t, p, "Netflix = subscription")
	assert.Contains(t, p, "Zomato = food delivery")
	assert.Contains(t, p, "Relevant facts")
}

func TestSystemPrompt_WithoutMemories(t *testing.T) {
	p := systemPrompt("Bob", testUserID, nil)
	assert.NotContains(t, p, "Relevant facts")
}

func TestSystemPrompt_EmptyMemoriesSlice(t *testing.T) {
	p := systemPrompt("Bob", testUserID, []string{})
	assert.NotContains(t, p, "Relevant facts")
}
