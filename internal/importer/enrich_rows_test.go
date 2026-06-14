package importer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// --- mock store implementations ---

type mockTxnStore struct {
	getIDFn        func(ctx context.Context, key string) (uuid.UUID, error)
	updateEnrichFn func(ctx context.Context, p store.EnrichmentParams) error
}

func (m *mockTxnStore) IdempotencyKeyExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockTxnStore) GetIDByIdempotencyKey(ctx context.Context, key string) (uuid.UUID, error) {
	if m.getIDFn != nil {
		return m.getIDFn(ctx, key)
	}
	return uuid.Nil, errors.New("not found")
}
func (m *mockTxnStore) Insert(_ context.Context, _ store.InsertTransactionParams) (*sqlcgen.Transaction, error) {
	return nil, nil
}
func (m *mockTxnStore) UpdateEnrichment(ctx context.Context, p store.EnrichmentParams) error {
	if m.updateEnrichFn != nil {
		return m.updateEnrichFn(ctx, p)
	}
	return nil
}

type mockCatStore struct {
	getBySlugFn func(ctx context.Context, slug string) (*sqlcgen.Category, error)
}

func (m *mockCatStore) List(_ context.Context) ([]sqlcgen.Category, error) { return nil, nil }
func (m *mockCatStore) GetBySlug(ctx context.Context, slug string) (*sqlcgen.Category, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, errors.New("not found")
}

type mockImportRunStore struct {
	finishFn func(ctx context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum, errDetail string) error
}

func (m *mockImportRunStore) Create(_ context.Context, _ store.CreateImportRunParams) (*sqlcgen.ImportRun, error) {
	return nil, nil
}
func (m *mockImportRunStore) UpdateCounts(_ context.Context, _ uuid.UUID, _, _, _, _ int) error {
	return nil
}
func (m *mockImportRunStore) UpdateStatus(_ context.Context, _ uuid.UUID, _ sqlcgen.ImportStatusEnum) error {
	return nil
}
func (m *mockImportRunStore) Finish(ctx context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum, errDetail string) error {
	if m.finishFn != nil {
		return m.finishFn(ctx, id, status, errDetail)
	}
	return nil
}

// --- helpers ---

func enrichRow() parser.RawTransaction {
	return parser.RawTransaction{
		Date:        time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "AMAZON PURCHASE",
		Amount:      50000,
		Direction:   sqlcgen.TxnDirectionEnumDebit,
	}
}

func newEnrichImporter(txn transactionStore, run importRunStore, cat categoryStore, enricher *Enricher) *Importer {
	return &Importer{txnStore: txn, runStore: run, catStore: cat, enricher: enricher}
}

// --- tests ---

func TestEnrichRows_NoEnricher(t *testing.T) {
	imp := newEnrichImporter(&mockTxnStore{}, &mockImportRunStore{}, &mockCatStore{}, nil)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})
}

func TestEnrichRows_RowNotFound_Skips(t *testing.T) {
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
		updateEnrichFn: func(_ context.Context, _ store.EnrichmentParams) error {
			t.Error("UpdateEnrichment must not be called when row is not found")
			return nil
		},
	}
	// Build an enricher with a no-op LLM; the key lookup will fail before calling it.
	e := NewEnricher(&mockLLM{}, "model", nil)

	imp := newEnrichImporter(txn, &mockImportRunStore{}, &mockCatStore{}, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})
}

func TestEnrichRows_HappyPath(t *testing.T) {
	txnID := uuid.New()
	runID := uuid.New()
	accountID := uuid.New()
	catID := uuid.New()

	var gotParams store.EnrichmentParams
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return txnID, nil },
		updateEnrichFn: func(_ context.Context, p store.EnrichmentParams) error {
			gotParams = p
			return nil
		},
	}
	cat := &mockCatStore{
		getBySlugFn: func(_ context.Context, slug string) (*sqlcgen.Category, error) {
			if slug == "shopping" {
				return &sqlcgen.Category{ID: catID, Slug: "shopping"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	var finishedRunID uuid.UUID
	var finishedStatus sqlcgen.ImportStatusEnum
	run := &mockImportRunStore{
		finishFn: func(_ context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum, _ string) error {
			finishedRunID = id
			finishedStatus = status
			return nil
		},
	}

	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp(`{"description_normalized":"Amazon","category_slug":"shopping","counterparty_name":"","counterparty_identifier":""}`), nil
		},
	}, "model", []CategoryInfo{{Slug: "shopping", Description: "Retail purchases"}})

	imp := newEnrichImporter(txn, run, cat, e)
	imp.EnrichRows(t.Context(), runID, accountID, []parser.RawTransaction{enrichRow()})

	if gotParams.TransactionID != txnID.String() {
		t.Errorf("TransactionID = %q, want %q", gotParams.TransactionID, txnID.String())
	}
	if gotParams.CategoryID == nil || *gotParams.CategoryID != catID.String() {
		t.Errorf("CategoryID = %v, want %q", gotParams.CategoryID, catID.String())
	}
	if finishedRunID != runID {
		t.Errorf("Finish called with run %q, want %q", finishedRunID, runID)
	}
	if finishedStatus != sqlcgen.ImportStatusEnumSuccess {
		t.Errorf("Finish status = %q, want success", finishedStatus)
	}
}

func TestEnrichRows_UnknownCategorySlug(t *testing.T) {
	txnID := uuid.New()
	var gotParams store.EnrichmentParams

	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return txnID, nil },
		updateEnrichFn: func(_ context.Context, p store.EnrichmentParams) error {
			gotParams = p
			return nil
		},
	}
	cat := &mockCatStore{
		getBySlugFn: func(_ context.Context, _ string) (*sqlcgen.Category, error) {
			return nil, errors.New("slug not found")
		},
	}

	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp(`{"description_normalized":"Amazon","category_slug":"hallucinated","counterparty_name":"","counterparty_identifier":""}`), nil
		},
	}, "model", nil)

	imp := newEnrichImporter(txn, &mockImportRunStore{}, cat, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})

	// UpdateEnrichment is still called but without CategoryID.
	if gotParams.TransactionID != txnID.String() {
		t.Errorf("TransactionID = %q, want %q", gotParams.TransactionID, txnID.String())
	}
	if gotParams.CategoryID != nil {
		t.Errorf("CategoryID should be nil for unknown slug, got %q", *gotParams.CategoryID)
	}
}

func TestEnrichRows_EnricherError_Skips(t *testing.T) {
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return uuid.New(), nil },
		updateEnrichFn: func(_ context.Context, _ store.EnrichmentParams) error {
			t.Error("UpdateEnrichment must not be called when enricher errors")
			return nil
		},
	}

	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp("not valid json at all"), nil
		},
	}, "model", nil)

	imp := newEnrichImporter(txn, &mockImportRunStore{}, &mockCatStore{}, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})
}

func TestEnrichRows_AllFailed_StatusFailed(t *testing.T) {
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return uuid.New(), nil },
	}
	var gotStatus sqlcgen.ImportStatusEnum
	run := &mockImportRunStore{
		finishFn: func(_ context.Context, _ uuid.UUID, status sqlcgen.ImportStatusEnum, _ string) error {
			gotStatus = status
			return nil
		},
	}

	// Invalid JSON → all rows fail enrichment.
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp("not valid json"), nil
		},
	}, "model", nil)

	imp := newEnrichImporter(txn, run, &mockCatStore{}, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})

	if gotStatus != sqlcgen.ImportStatusEnumFailed {
		t.Errorf("status = %q, want failed", gotStatus)
	}
}

func TestEnrichRows_SomeFailed_StatusPartial(t *testing.T) {
	calls := 0
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return uuid.New(), nil },
	}
	var gotStatus sqlcgen.ImportStatusEnum
	run := &mockImportRunStore{
		finishFn: func(_ context.Context, _ uuid.UUID, status sqlcgen.ImportStatusEnum, _ string) error {
			gotStatus = status
			return nil
		},
	}

	// First call succeeds, second fails.
	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			calls++
			if calls == 1 {
				return chatResp(`{"description_normalized":"Ok","category_slug":"","counterparty_name":"","counterparty_identifier":""}`), nil
			}
			return chatResp("invalid json"), nil
		},
	}, "model", nil)

	rows := []parser.RawTransaction{enrichRow(), {
		Date: enrichRow().Date, Description: "OTHER", Amount: 1000, Direction: sqlcgen.TxnDirectionEnumDebit,
	}}
	imp := newEnrichImporter(txn, run, &mockCatStore{}, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), rows)

	if gotStatus != sqlcgen.ImportStatusEnumPartial {
		t.Errorf("status = %q, want partial", gotStatus)
	}
}

func TestEnrichRows_DBWriteFailure_CountsAsFailed(t *testing.T) {
	txn := &mockTxnStore{
		getIDFn: func(_ context.Context, _ string) (uuid.UUID, error) { return uuid.New(), nil },
		updateEnrichFn: func(_ context.Context, _ store.EnrichmentParams) error {
			return errors.New("connection reset")
		},
	}
	var gotStatus sqlcgen.ImportStatusEnum
	run := &mockImportRunStore{
		finishFn: func(_ context.Context, _ uuid.UUID, status sqlcgen.ImportStatusEnum, _ string) error {
			gotStatus = status
			return nil
		},
	}

	e := NewEnricher(&mockLLM{
		chatFn: func(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
			return chatResp(`{"description_normalized":"Ok","category_slug":"","counterparty_name":"","counterparty_identifier":""}`), nil
		},
	}, "model", nil)

	imp := newEnrichImporter(txn, run, &mockCatStore{}, e)
	imp.EnrichRows(t.Context(), uuid.New(), uuid.New(), []parser.RawTransaction{enrichRow()})

	if gotStatus != sqlcgen.ImportStatusEnumFailed {
		t.Errorf("status = %q, want failed when DB write fails for all rows", gotStatus)
	}
}
