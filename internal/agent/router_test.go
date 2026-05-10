package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pushkaranand/finagent/config"
)

func newTestRouter() *Router {
	return NewRouter(config.RoutingConfig{
		ChatModel:      "chat-model",
		AnalysisModel:  "analysis-model",
		SummarizeModel: "summarize-model",
	})
}

func TestRouter_Summarize(t *testing.T) {
	r := newTestRouter()
	assert.Equal(t, "summarize-model", r.Select("anything", RouterHintSummarize))
}

func TestRouter_AnalysisKeywords(t *testing.T) {
	r := newTestRouter()
	cases := []string{
		"what is my total spending",
		"calculate tax on this",
		"how much did I spend",
		"compare last month",
		"LTCG on zerodha",
		"profit and loss",
		"average monthly spend",
		"breakdown by category",
	}
	for _, input := range cases {
		assert.Equal(t, "analysis-model", r.Select(input, RouterHintChat), "input: %q", input)
	}
}

func TestRouter_Chat(t *testing.T) {
	r := newTestRouter()
	assert.Equal(t, "chat-model", r.Select("what accounts do I have", RouterHintChat))
	assert.Equal(t, "chat-model", r.Select("show me recent transactions", RouterHintChat))
	assert.Equal(t, "chat-model", r.Select("", RouterHintChat))
}
