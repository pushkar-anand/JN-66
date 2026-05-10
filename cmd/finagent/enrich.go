package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/importer"
	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm/openai"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

func runEnrich(args []string) error {
	fs := flag.NewFlagSet("enrich", flag.ExitOnError)
	configPath := fs.String("config", "config/config.yaml", "path to config file")
	desc := fs.String("desc", "", "raw transaction description (required)")
	amountFlag := fs.Float64("amount", 0, "amount in rupees, e.g. 1500.00 (required)")
	directionFlag := fs.String("direction", "debit", "debit or credit")
	dateFlag := fs.String("date", "", "transaction date YYYY-MM-DD (default: today)")
	rawFlag := fs.Bool("raw", false, "also print the raw LLM JSON response")
	_ = fs.Parse(args)

	if *desc == "" {
		return fmt.Errorf("--desc is required")
	}
	if *amountFlag <= 0 {
		return fmt.Errorf("--amount must be > 0")
	}

	var dir sqlcgen.TxnDirectionEnum
	switch *directionFlag {
	case "debit":
		dir = sqlcgen.TxnDirectionEnumDebit
	case "credit":
		dir = sqlcgen.TxnDirectionEnumCredit
	default:
		return fmt.Errorf("--direction must be debit or credit")
	}

	txnDate := time.Now()
	if *dateFlag != "" {
		d, err := time.Parse("2006-01-02", *dateFlag)
		if err != nil {
			return fmt.Errorf("--date must be YYYY-MM-DD: %w", err)
		}
		txnDate = d
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	catStore := store.NewCategoryStore(pool)
	cats, err := catStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	slugs := make([]string, len(cats))
	for i, c := range cats {
		slugs[i] = c.Slug
	}

	llmProvider := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	enricher := importer.NewEnricher(llmProvider, cfg.LLM.Routing.TaggingModel, slugs)

	tx := parser.RawTransaction{
		Date:        txnDate,
		Description: *desc,
		Amount:      int64(*amountFlag * 100),
		Direction:   dir,
	}

	fmt.Printf("\n── Input ─────────────────────────────────────────\n")
	fmt.Printf("  Description : %s\n", tx.Description)
	fmt.Printf("  Direction   : %s\n", tx.Direction)
	fmt.Printf("  Amount      : ₹%.2f\n", *amountFlag)
	fmt.Printf("  Date        : %s\n", txnDate.Format("2006-01-02"))
	fmt.Printf("  Model       : %s\n", cfg.LLM.Routing.TaggingModel)
	fmt.Println()

	result, err := enricher.Enrich(ctx, tx)
	if err != nil {
		return fmt.Errorf("enrich: %w", err)
	}

	fmt.Printf("── Result ────────────────────────────────────────\n")
	fmt.Printf("  Normalized  : %s\n", result.DescriptionNormalized)
	fmt.Printf("  Category    : %s\n", result.CategorySlug)
	fmt.Printf("  Counterparty: %s\n", result.CounterpartyName)
	if result.CounterpartyID != "" {
		fmt.Printf("  Identifier  : %s\n", result.CounterpartyID)
	}
	fmt.Println()

	if *rawFlag {
		raw, _ := json.MarshalIndent(result, "  ", "  ")
		fmt.Printf("── Raw JSON ──────────────────────────────────────\n  %s\n\n", raw)
	}

	// Warn if the assigned category isn't in the valid list.
	valid := false
	for _, s := range slugs {
		if s == result.CategorySlug {
			valid = true
			break
		}
	}
	if !valid && result.CategorySlug != "" {
		fmt.Fprintf(os.Stderr, "warning: category %q is not in the valid slug list\n", result.CategorySlug)
	}

	return nil
}
