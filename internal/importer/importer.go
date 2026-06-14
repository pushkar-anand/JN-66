// Package importer orchestrates bank statement parsing, deduplication, enrichment, and DB insertion.
package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// transactionStore is the subset of store.TransactionStore used by the importer.
type transactionStore interface {
	IdempotencyKeyExists(ctx context.Context, key string) (bool, error)
	GetIDByIdempotencyKey(ctx context.Context, key string) (uuid.UUID, error)
	Insert(ctx context.Context, p store.InsertTransactionParams) (*sqlcgen.Transaction, error)
	UpdateEnrichment(ctx context.Context, p store.EnrichmentParams) error
}

// importRunStore is the subset of store.ImportRunStore used by the importer.
type importRunStore interface {
	Create(ctx context.Context, p store.CreateImportRunParams) (*sqlcgen.ImportRun, error)
	UpdateCounts(ctx context.Context, id uuid.UUID, parsed, inserted, duplicate, failed int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum) error
	Finish(ctx context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum, errDetail string) error
}

// categoryStore is the subset of store.CategoryStore used by the importer.
type categoryStore interface {
	List(ctx context.Context) ([]sqlcgen.Category, error)
	GetBySlug(ctx context.Context, slug string) (*sqlcgen.Category, error)
}

// Importer runs a full import cycle for a batch of RawTransactions.
type Importer struct {
	txnStore transactionStore
	runStore importRunStore
	catStore categoryStore
	enricher *Enricher // may be nil if --no-enrich
}

// NewImporter creates an Importer wired to the given stores.
func NewImporter(txnStore *store.TransactionStore, runStore *store.ImportRunStore, catStore *store.CategoryStore, enricher *Enricher) *Importer {
	return &Importer{
		txnStore: txnStore,
		runStore: runStore,
		catStore: catStore,
		enricher: enricher,
	}
}

// Result holds final counts for a completed import run.
type Result struct {
	RunID             uuid.UUID
	Parsed            int
	Inserted          int
	Duplicate         int
	Failed            int
	LatestBalance     *int64    // closing balance in paise from the most recent row with balance data
	LatestBalanceDate time.Time // txn date of that row
}

// RunParams holds all inputs for a single import.
type RunParams struct {
	User      *sqlcgen.User
	AccountID uuid.UUID
	SourceRef string
	Rows      []parser.RawTransaction
}

// Run inserts all rows, tracking progress in import_runs.
func (imp *Importer) Run(ctx context.Context, p RunParams) (*Result, error) {
	run, err := imp.runStore.Create(ctx, store.CreateImportRunParams{
		UserID:    p.User.ID,
		AccountID: &p.AccountID,
		Provider:  sqlcgen.ImportProviderEnumCsv,
		SourceRef: p.SourceRef,
	})
	if err != nil {
		return nil, fmt.Errorf("create import run: %w", err)
	}

	res := &Result{RunID: run.ID, Parsed: len(p.Rows)}

	// Scan rows in file order (chronological) to find the latest closing balance.
	// We overwrite on every non-nil balance so the last one in the file wins,
	// which correctly handles same-day transactions by preserving row sequence.
	for _, row := range p.Rows {
		if row.Balance != nil {
			b := *row.Balance
			res.LatestBalance = &b
			res.LatestBalanceDate = row.Date
		}
	}

	total := len(p.Rows)
	for i, row := range p.Rows {
		slog.Info("processing", "n", i+1, "total", total, "date", row.Date.Format("2006-01-02"), "desc", truncateStr(row.Description, 50))
		key := idempotencyKey(p.AccountID, row.Date, row.Amount, row.Description)

		exists, err := imp.txnStore.IdempotencyKeyExists(ctx, key)
		if err != nil {
			slog.Warn("idempotency check failed", "err", err, "key", key)
			res.Failed++
			continue
		}
		if exists {
			res.Duplicate++
			if imp.enricher == nil {
				continue
			}
			// Re-enrich the existing transaction so re-importing overwrites stale tags.
			txnID, err := imp.txnStore.GetIDByIdempotencyKey(ctx, key)
			if err != nil {
				slog.Warn("re-enrich: could not fetch existing txn", "err", err)
				continue
			}
			slog.Info("re-enriching", "n", i+1, "total", total, "desc", truncateStr(row.Description, 50))
			enriched, err := imp.enricher.Enrich(ctx, row)
			if err != nil {
				slog.Warn("re-enrichment failed", "err", err, "txn", txnID)
				continue
			}
			slog.Info("re-enriched", "category", enriched.CategorySlug, "normalized", enriched.DescriptionNormalized)
			ep := store.EnrichmentParams{TransactionID: txnID.String(), DescriptionNormalized: nilIfEmpty(enriched.DescriptionNormalized)}
			auto := sqlcgen.TaggingStatusEnumAuto
			ep.TaggingStatus = &auto
			if enriched.CategorySlug != "" {
				if cat, err := imp.catStore.GetBySlug(ctx, enriched.CategorySlug); err == nil {
					id := cat.ID.String()
					ep.CategoryID = &id
				}
			}
			if err := imp.txnStore.UpdateEnrichment(ctx, ep); err != nil {
				slog.Warn("re-enrichment update failed", "err", err, "txn", txnID)
			}
			continue
		}

		mode := parser.DetectPaymentMode(row.Description)

		txn, err := imp.txnStore.Insert(ctx, store.InsertTransactionParams{
			AccountID:       p.AccountID,
			UserID:          p.User.ID,
			IdempotencyKey:  key,
			ReferenceNumber: nilIfEmpty(row.Reference),
			Amount:          model.Money(row.Amount),
			Currency:        "INR",
			Direction:       row.Direction,
			Description:     row.Description,
			PaymentMode:     mode,
			TxnDate:         row.Date,
		})
		if err != nil {
			slog.Warn("insert transaction failed", "err", err, "desc", row.Description)
			res.Failed++
			continue
		}
		res.Inserted++

		if imp.enricher == nil {
			continue
		}

		slog.Info("enriching", "n", i+1, "total", total, "desc", truncateStr(row.Description, 50))
		enriched, err := imp.enricher.Enrich(ctx, row)
		if err != nil {
			slog.Warn("enrichment failed — leaving pending", "err", err, "txn", txn.ID)
			continue
		}
		slog.Info("enriched", "category", enriched.CategorySlug, "normalized", enriched.DescriptionNormalized)

		ep := store.EnrichmentParams{
			TransactionID:         txn.ID.String(),
			DescriptionNormalized: nilIfEmpty(enriched.DescriptionNormalized),
		}
		auto := sqlcgen.TaggingStatusEnumAuto
		ep.TaggingStatus = &auto

		if enriched.CategorySlug != "" {
			cat, err := imp.catStore.GetBySlug(ctx, enriched.CategorySlug)
			if err == nil {
				id := cat.ID.String()
				ep.CategoryID = &id
			}
		}

		if err := imp.txnStore.UpdateEnrichment(ctx, ep); err != nil {
			slog.Warn("update enrichment failed", "err", err, "txn", txn.ID)
		}
	}

	status := sqlcgen.ImportStatusEnumSuccess
	if res.Failed > 0 && res.Inserted == 0 {
		status = sqlcgen.ImportStatusEnumFailed
	} else if res.Failed > 0 {
		status = sqlcgen.ImportStatusEnumPartial
	}

	_ = imp.runStore.UpdateCounts(ctx, run.ID, res.Parsed, res.Inserted, res.Duplicate, res.Failed)
	_ = imp.runStore.Finish(ctx, run.ID, status, "")

	return res, nil
}

// EnrichRows enriches already-inserted transactions identified by rows.
// It resolves each row back to its DB transaction via idempotency key, calls the
// enricher, and writes the result. Designed to be called from a background goroutine
// after Run() has returned — the context must outlive the HTTP request.
// Silently skips rows whose transaction cannot be found (e.g. rows that were
// rejected by Insert). Marks the run partial/failed when LLM errors prevent enrichment.
func (imp *Importer) EnrichRows(ctx context.Context, runID, accountID uuid.UUID, rows []parser.RawTransaction) {
	if imp.enricher == nil {
		return
	}
	total := len(rows)
	enrichFailed := 0
	for i, row := range rows {
		key := idempotencyKey(accountID, row.Date, row.Amount, row.Description)
		txnID, err := imp.txnStore.GetIDByIdempotencyKey(ctx, key)
		if err != nil {
			// Row was likely a failed insert; skip silently.
			continue
		}
		slog.DebugContext(ctx, "enriching", "n", i+1, "total", total, "desc", truncateStr(row.Description, 50))
		enriched, err := imp.enricher.Enrich(ctx, row)
		if err != nil {
			slog.WarnContext(ctx, "enrichment failed — leaving pending", "err", err, "txn", txnID)
			enrichFailed++
			continue
		}
		ep := store.EnrichmentParams{
			TransactionID:         txnID.String(),
			DescriptionNormalized: nilIfEmpty(enriched.DescriptionNormalized),
		}
		auto := sqlcgen.TaggingStatusEnumAuto
		ep.TaggingStatus = &auto
		if enriched.CategorySlug != "" {
			if cat, err := imp.catStore.GetBySlug(ctx, enriched.CategorySlug); err == nil {
				id := cat.ID.String()
				ep.CategoryID = &id
			} else {
				slog.WarnContext(ctx, "unknown category slug from LLM", "slug", enriched.CategorySlug, "err", err)
			}
		}
		if err := imp.txnStore.UpdateEnrichment(ctx, ep); err != nil {
			slog.WarnContext(ctx, "update enrichment failed", "err", err, "txn", txnID)
		}
	}
	slog.InfoContext(ctx, "background enrichment complete", "total", total, "failed", enrichFailed)
	status := sqlcgen.ImportStatusEnumSuccess
	if enrichFailed > 0 && enrichFailed == total {
		status = sqlcgen.ImportStatusEnumFailed
	} else if enrichFailed > 0 {
		status = sqlcgen.ImportStatusEnumPartial
	}
	_ = imp.runStore.Finish(ctx, runID, status, "")
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func idempotencyKey(accountID uuid.UUID, date time.Time, amount int64, desc string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s",
		accountID.String(),
		date.Format("2006-01-02"),
		strconv.FormatInt(amount, 10),
		desc,
	)
	return hex.EncodeToString(h.Sum(nil))
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
