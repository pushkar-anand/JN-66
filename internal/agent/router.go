// Package agent implements the ReAct agent loop, prompt construction, and model routing.
package agent

import (
	"strings"

	"github.com/pushkaranand/finagent/config"
)

// RouterHint tells the router what kind of task is being performed.
type RouterHint int

const (
	RouterHintChat      RouterHint = iota // default chat
	RouterHintAnalysis                    // calculations, totals, tax
	RouterHintSummarize                   // session title generation
)

// Router maps RouterHints to configured model IDs.
type Router struct {
	cfg config.RoutingConfig
}

// NewRouter creates a Router from routing config.
func NewRouter(cfg config.RoutingConfig) *Router {
	return &Router{cfg: cfg}
}

// Select returns the model ID to use for the given input and hint.
// It upgrades to the analysis model when the input contains financial calculation keywords.
func (r *Router) Select(input string, hint RouterHint) string {
	if hint == RouterHintSummarize {
		return r.cfg.SummarizeModel
	}

	lower := strings.ToLower(input)
	analysisKeywords := []string{
		"total", "calculate", "how much", "compare", "tax", "ltcg", "stcg",
		"returns", "profit", "loss", "percentage", "average", "breakdown",
	}
	for _, kw := range analysisKeywords {
		if strings.Contains(lower, kw) {
			return r.cfg.AnalysisModel
		}
	}
	return r.cfg.ChatModel
}
