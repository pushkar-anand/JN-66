package importer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// mockLLM is a test double for llm.Provider.
type mockLLM struct {
	chatFn func(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error)
}

func (m *mockLLM) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return m.chatFn(ctx, req)
}

func (m *mockLLM) Embed(_ context.Context, _ llm.EmbedRequest) (llm.EmbedResponse, error) {
	return llm.EmbedResponse{}, nil
}

func (m *mockLLM) Name() string { return "mock" }

func chatResp(content string) llm.ChatResponse {
	return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: content}}
}

func testTx() parser.RawTransaction {
	return parser.RawTransaction{
		Date:        time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "TEST TRANSACTION",
		Amount:      100000,
		Direction:   sqlcgen.TxnDirectionEnumDebit,
	}
}

func TestNewEnricher_CatList(t *testing.T) {
	cats := []CategoryInfo{
		{Slug: "salary", Description: "Regular employment income"},
		{Slug: "shopping", Description: "Retail purchases"},
	}
	e := NewEnricher(&mockLLM{}, "model", cats)
	want := "salary: Regular employment income\nshopping: Retail purchases\n"
	if e.catList != want {
		t.Errorf("catList =\n%q\nwant\n%q", e.catList, want)
	}
}

func TestNewEnricher_EmptyCats(t *testing.T) {
	e := NewEnricher(&mockLLM{}, "model", nil)
	if e.catList != "" {
		t.Errorf("catList = %q, want empty", e.catList)
	}
}

func TestEnrich_ParsesValidJSON(t *testing.T) {
	const body = `{"description_normalized":"Salary","category_slug":"salary","counterparty_name":"XYZ Corp","counterparty_identifier":""}`
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp(body), nil
		},
	}, "model", nil)

	res, err := e.Enrich(context.Background(), testTx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DescriptionNormalized != "Salary" {
		t.Errorf("DescriptionNormalized = %q, want %q", res.DescriptionNormalized, "Salary")
	}
	if res.CategorySlug != "salary" {
		t.Errorf("CategorySlug = %q, want %q", res.CategorySlug, "salary")
	}
	if res.CounterpartyName != "XYZ Corp" {
		t.Errorf("CounterpartyName = %q, want %q", res.CounterpartyName, "XYZ Corp")
	}
}

func TestEnrich_StripsMarkdownFences(t *testing.T) {
	const body = "```json\n{\"description_normalized\":\"SMS Charges\",\"category_slug\":\"bank_charges\",\"counterparty_name\":\"\",\"counterparty_identifier\":\"\"}\n```"
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp(body), nil
		},
	}, "model", nil)

	res, err := e.Enrich(context.Background(), testTx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CategorySlug != "bank_charges" {
		t.Errorf("CategorySlug = %q, want %q", res.CategorySlug, "bank_charges")
	}
}

func TestEnrich_InvalidJSONError(t *testing.T) {
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp("not valid json at all"), nil
		},
	}, "model", nil)

	_, err := e.Enrich(context.Background(), testTx())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "enrich parse json") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "enrich parse json")
	}
}

func TestEnrich_LLMError(t *testing.T) {
	llmErr := errors.New("upstream timeout")
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return llm.ChatResponse{}, llmErr
		},
	}, "model", nil)

	_, err := e.Enrich(context.Background(), testTx())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "enrich llm") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "enrich llm")
	}
	if !errors.Is(err, llmErr) {
		t.Error("errors.Is(err, llmErr) = false, want true")
	}
}
