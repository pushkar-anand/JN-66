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

// Importer runs a full import cycle for a batch of RawTransactions.
type Importer struct {
	txnStore *store.TransactionStore
	runStore *store.ImportRunStore
	catStore *store.CategoryStore
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
	RunID     uuid.UUID
	Parsed    int
	Inserted  int
	Duplicate int
	Failed    int
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

	for _, row := range p.Rows {
		key := idempotencyKey(p.AccountID, row.Date, row.Amount, row.Description)

		exists, err := imp.txnStore.IdempotencyKeyExists(ctx, key)
		if err != nil {
			slog.Warn("idempotency check failed", "err", err, "key", key)
			res.Failed++
			continue
		}
		if exists {
			res.Duplicate++
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

		enriched, err := imp.enricher.Enrich(ctx, row)
		if err != nil {
			slog.Warn("enrichment failed — leaving pending", "err", err, "txn", txn.ID)
			continue
		}

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
