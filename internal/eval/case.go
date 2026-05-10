package eval

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pushkaranand/finagent/internal/channel"
)

// EvalCase defines a single behavioural eval scenario.
type EvalCase struct {
	Name   string
	UserID string // filled by runner from resolved seed user

	// PreambleInputs are sent first in the same session before Input.
	// They warm up memory/context; assertions apply only to the final Input turn.
	PreambleInputs []string

	// Input is the message being evaluated.
	Input string

	// MustCallTools: all must appear in the tool call trace (any order).
	MustCallTools []string
	// MustNotCallTools: none may appear in the tool call trace.
	MustNotCallTools []string
	// FirstToolMustBe: if set, the very first tool call must match.
	FirstToolMustBe string
	// MaxLLMRounds: fail if LLM is called more times than this (default 4).
	MaxLLMRounds int

	// OutputMustContain: all substrings must appear in the final response (case-insensitive).
	OutputMustContain []string
	// OutputMustContainOneOf: at least one substring must appear (OR logic, case-insensitive).
	OutputMustContainOneOf []string
	// OutputMustNotContain: none may appear in the final response (case-insensitive).
	OutputMustNotContain []string
}

// EvalResult holds the outcome of a single EvalCase run.
type EvalResult struct {
	Case      *EvalCase
	Passed    bool
	Failures  []string
	ToolCalls []string
	LLMRounds int
	Output    string
	Duration  time.Duration
	Err       error
}

// Run executes the scenario against the given agent HandleMessage function.
// The llmRec and regRec recorders must already be wired into the agent.
func (c *EvalCase) Run(ctx context.Context, handle channel.MessageHandler, llmRec *RecordingLLM, regRec *RecordingRegistry) EvalResult {
	maxRounds := c.MaxLLMRounds
	if maxRounds <= 0 {
		maxRounds = 4
	}

	sessionID := uuid.NewString()

	// Send preamble turns (same session, no assertions).
	for _, pre := range c.PreambleInputs {
		llmRec.Reset()
		regRec.Reset()
		_, _ = handle(ctx, channel.Message{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			UserID:    c.UserID,
			Text:      pre,
			Timestamp: time.Now(),
		})
	}

	// Reset recorders for the actual evaluated turn.
	llmRec.Reset()
	regRec.Reset()

	t0 := time.Now()
	resp, err := handle(ctx, channel.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		UserID:    c.UserID,
		Text:      c.Input,
		Timestamp: time.Now(),
	})
	dur := time.Since(t0)

	res := EvalResult{
		Case:      c,
		ToolCalls: regRec.Calls(),
		LLMRounds: llmRec.Rounds(),
		Output:    resp.Text,
		Duration:  dur,
		Err:       err,
	}

	if err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("handler error: %v", err))
		res.Passed = false
		return res
	}

	// Tool call assertions.
	for _, must := range c.MustCallTools {
		if !slices.Contains(res.ToolCalls, must) {
			res.Failures = append(res.Failures, fmt.Sprintf("tool not called: %s", must))
		}
	}
	for _, mustNot := range c.MustNotCallTools {
		if slices.Contains(res.ToolCalls, mustNot) {
			res.Failures = append(res.Failures, fmt.Sprintf("unexpected tool called: %s", mustNot))
		}
	}
	if c.FirstToolMustBe != "" {
		if len(res.ToolCalls) == 0 {
			res.Failures = append(res.Failures, fmt.Sprintf("no tool called, want first=%s", c.FirstToolMustBe))
		} else if res.ToolCalls[0] != c.FirstToolMustBe {
			res.Failures = append(res.Failures, fmt.Sprintf("first_tool=%s  want=%s", res.ToolCalls[0], c.FirstToolMustBe))
		}
	}
	if res.LLMRounds > maxRounds {
		res.Failures = append(res.Failures, fmt.Sprintf("llm_rounds=%d  max=%d", res.LLMRounds, maxRounds))
	}

	// Output assertions (case-insensitive).
	lower := strings.ToLower(res.Output)
	for _, sub := range c.OutputMustContain {
		if !strings.Contains(lower, strings.ToLower(sub)) {
			res.Failures = append(res.Failures, fmt.Sprintf("output missing %q", sub))
		}
	}
	if len(c.OutputMustContainOneOf) > 0 {
		found := false
		for _, sub := range c.OutputMustContainOneOf {
			if strings.Contains(lower, strings.ToLower(sub)) {
				found = true
				break
			}
		}
		if !found {
			res.Failures = append(res.Failures, fmt.Sprintf("output missing all of %v", c.OutputMustContainOneOf))
		}
	}
	for _, sub := range c.OutputMustNotContain {
		if strings.Contains(lower, strings.ToLower(sub)) {
			res.Failures = append(res.Failures, fmt.Sprintf("output contains forbidden %q", sub))
		}
	}

	res.Passed = len(res.Failures) == 0
	return res
}
