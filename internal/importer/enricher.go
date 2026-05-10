package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm"
)

// EnrichmentResult holds the LLM-derived metadata for a single transaction.
type EnrichmentResult struct {
	DescriptionNormalized string `json:"description_normalized"`
	CategorySlug          string `json:"category_slug"`
	CounterpartyName      string `json:"counterparty_name"`
	CounterpartyID        string `json:"counterparty_identifier"`
}

// Enricher calls the LLM to classify and normalise a raw transaction.
type Enricher struct {
	llm      llm.Provider
	model    string
	catSlugs string // comma-separated list of valid category slugs for the prompt
}

// NewEnricher creates an Enricher with the given LLM provider, model, and category slug list.
func NewEnricher(provider llm.Provider, model string, categorySlugs []string) *Enricher {
	return &Enricher{
		llm:      provider,
		model:    model,
		catSlugs: strings.Join(categorySlugs, "|"),
	}
}

const enrichSystemPrompt = `You are classifying Indian bank transactions for a personal finance app.
Given a raw transaction, respond ONLY with a JSON object — no markdown, no explanation.

Valid category slugs: %s

JSON format:
{
  "description_normalized": "clean merchant or payee name, max 60 chars",
  "category_slug": "one of the valid slugs above",
  "counterparty_name": "human-readable payee name or empty string",
  "counterparty_identifier": "VPA (e.g. merchant@bank) or account+IFSC if detectable, else empty string"
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

	resp, err := e.llm.Chat(ctx, llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: fmt.Sprintf(enrichSystemPrompt, e.catSlugs)},
			{Role: llm.RoleUser, Content: userMsg},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("enrich llm: %w", err)
	}

	raw := strings.TrimSpace(resp.Message.Content)
	// Strip markdown code fences if the model added them.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result EnrichmentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("enrich parse json: %w", err)
	}
	return &result, nil
}
