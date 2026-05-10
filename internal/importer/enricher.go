package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm"
)

// CategoryInfo carries a category slug and its human-readable description for the LLM prompt.
type CategoryInfo struct {
	Slug        string
	Description string
}

// EnrichmentResult holds the LLM-derived metadata for a single transaction.
type EnrichmentResult struct {
	DescriptionNormalized string `json:"description_normalized"`
	CategorySlug          string `json:"category_slug"`
	CounterpartyName      string `json:"counterparty_name"`
	CounterpartyID        string `json:"counterparty_identifier"`
}

// Enricher calls the LLM to classify and normalise a raw transaction.
type Enricher struct {
	llm     llm.Provider
	model   string
	catList string // pre-rendered "slug: description\n..." list injected into the prompt
}

// NewEnricher creates an Enricher with the given LLM provider, model, and category list.
func NewEnricher(provider llm.Provider, model string, categories []CategoryInfo) *Enricher {
	var b strings.Builder
	for _, c := range categories {
		fmt.Fprintf(&b, "%s: %s\n", c.Slug, c.Description)
	}
	return &Enricher{
		llm:     provider,
		model:   model,
		catList: b.String(),
	}
}

const enrichSystemPrompt = `You are classifying Indian bank transactions for a personal finance app.
Given a raw transaction, respond ONLY with valid JSON — no markdown, no explanation.

## Categories (slug: when to use)
%s
## Normalization
description_normalized: Clean merchant name, max 40 chars. Strip ref numbers, txn IDs, account numbers, dates.
Examples:
- "NEFT/IDFBH25093/Mr John Doe/IDFC FIRST" → "John Doe"
- "DCARDFEE2615DEC25-NOV26+GST" → "Debit Card Annual Fee"
- "046301004351:Int.Pd:29-03-2025 to 29-06-2025" → "Interest Paid"
- "SMSChgsJan25-Mar25+GST" → "SMS Charges"

## Output
{
  "description_normalized": "...",
  "category_slug": "one valid slug from the list above",
  "counterparty_name": "human-readable payee name or empty string",
  "counterparty_identifier": "VPA like merchant@bank, or empty string"
}`

// Enrich calls the LLM to classify a single transaction. Returns an error if the LLM fails
// or returns invalid JSON; the caller should insert the transaction without enrichment in that case.
func (e *Enricher) Enrich(ctx context.Context, tx parser.RawTransaction) (*EnrichmentResult, error) {
	dir := "credit"
	if tx.Direction == "debit" {
		dir = "debit"
	}

	userMsg := fmt.Sprintf("Date: %s\nDirection: %s\nAmount: ₹%.2f\nDescription: %s",
		tx.Date.Format("2006-01-02"),
		dir,
		float64(tx.Amount)/100,
		tx.Description,
	)

	slog.Debug("enrich request", "model", e.model, "msg", userMsg)

	resp, err := e.llm.Chat(ctx, llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: fmt.Sprintf(enrichSystemPrompt, e.catList)},
			{Role: llm.RoleUser, Content: userMsg},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("enrich llm: %w", err)
	}

	raw := strings.TrimSpace(resp.Message.Content)
	slog.Debug("enrich response", "raw", raw)

	// Strip markdown code fences if the model added them.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result EnrichmentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		slog.Warn("enrich json parse failed", "raw", raw, "err", err)
		return nil, fmt.Errorf("enrich parse json: %w", err)
	}

	slog.Info("enrich result", "normalized", result.DescriptionNormalized, "category", result.CategorySlug, "counterparty", result.CounterpartyName)
	return &result, nil
}
