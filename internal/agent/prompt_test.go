package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemPrompt_ContainsUserName(t *testing.T) {
	p := systemPrompt("Alice", testUserID, nil, false)
	assert.Contains(t, p, "Alice")
	assert.Contains(t, p, "INR")
}

func TestSystemPrompt_WithMemories(t *testing.T) {
	p := systemPrompt("Bob", testUserID, []string{"Netflix = subscription", "Zomato = food delivery"}, false)
	assert.Contains(t, p, "Netflix = subscription")
	assert.Contains(t, p, "Zomato = food delivery")
	assert.Contains(t, p, "Relevant facts")
}

func TestSystemPrompt_WithoutMemories(t *testing.T) {
	p := systemPrompt("Bob", testUserID, nil, false)
	assert.NotContains(t, p, "Relevant facts")
}

func TestSystemPrompt_EmptyMemoriesSlice(t *testing.T) {
	p := systemPrompt("Bob", testUserID, []string{}, false)
	assert.NotContains(t, p, "Relevant facts")
}

func TestSystemPrompt_HasZerodha(t *testing.T) {
	p := systemPrompt("Alice", testUserID, nil, true)
	assert.Contains(t, p, "get_investment_holdings")
	assert.NotContains(t, p, "Investment portfolios, stocks")
}

func TestSystemPrompt_NoZerodha(t *testing.T) {
	p := systemPrompt("Alice", testUserID, nil, false)
	assert.Contains(t, p, "Investment portfolios, stocks")
}
