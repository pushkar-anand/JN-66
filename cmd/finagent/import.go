package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/importer"
	"github.com/pushkaranand/finagent/internal/importer/parser"
	"github.com/pushkaranand/finagent/internal/llm/openai"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	configPath := fs.String("config", "config/config.yaml", "path to config file")
	fileFlag := fs.String("file", "", "path to bank statement file (required)")
	userFlag := fs.String("user", "", "user email (required)")
	accountFlag := fs.String("account", "", "account UUID (optional — auto-detected from statement)")
	dryRun := fs.Bool("dry-run", false, "parse and display rows without inserting")
	noEnrich := fs.Bool("no-enrich", false, "skip LLM enrichment (insert with tagging_status=pending)")
	_ = fs.Parse(args)

	if *fileFlag == "" {
		return fmt.Errorf("--file is required")
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

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

	userStore := store.NewUserStore(pool)
	u, err := resolveUser(ctx, userStore, *userFlag, cfg.Channel.CLI.DefaultUser)
	if err != nil {
		return err
	}

	reg := parser.NewRegistry()

	// Auto-detect bank from file.
	bank, err := reg.DetectFile(*fileFlag)
	if err != nil {
		return fmt.Errorf("detect bank: %w", err)
	}

	result, err := parseFile(ctx, reg, *fileFlag, bank, u)
	if err != nil {
		return err
	}

	if *dryRun {
		printDryRun(bank, result)
		return nil
	}

	// Resolve account: use explicit UUID if provided, otherwise find-or-create.
	accountStore := store.NewAccountStore(pool)
	var accountID uuid.UUID

	if *accountFlag != "" {
		accountID, err = uuid.Parse(*accountFlag)
		if err != nil {
			return fmt.Errorf("invalid --account UUID: %w", err)
		}
	} else {
		acc, created, err := accountStore.FindOrCreate(ctx, u.ID.String(), bank, store.AccountMetaParams{
			AccountNumber: result.Meta.AccountNumber,
			IFSC:          result.Meta.IFSC,
		})
		if err != nil {
			return fmt.Errorf("resolve account: %w", err)
		}
		accountID = acc.ID
		if created {
			fmt.Printf("\nAuto-created account: %s (%s)\n", acc.Name, acc.ID)
		} else {
			fmt.Printf("\nUsing account: %s (%s)\n", acc.Name, acc.ID)
		}
	}

	txnStore := store.NewTransactionStore(pool)
	runStore := store.NewImportRunStore(pool)
	catStore := store.NewCategoryStore(pool)

	var enricher *importer.Enricher
	if !*noEnrich {
		llmProvider := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
		cats, _ := catStore.List(ctx)
		slugs := make([]string, len(cats))
		for i, c := range cats {
			slugs[i] = c.Slug
		}
		enricher = importer.NewEnricher(llmProvider, cfg.LLM.Routing.TaggingModel, slugs)
	}

	imp := importer.NewImporter(txnStore, runStore, catStore, enricher)
	res, err := imp.Run(ctx, importer.RunParams{
		User:      u,
		AccountID: accountID,
		SourceRef: filepath.Base(*fileFlag),
		Rows:      result.Transactions,
	})
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	enrichMsg := ""
	if *noEnrich {
		enrichMsg = " (enrichment skipped)"
	}

	fmt.Printf("\n=== import: %s — %s ===\n\n", strings.ToUpper(bank), *userFlag)
	fmt.Printf("Parsed:    %d rows\n", res.Parsed)
	fmt.Printf("Inserted:  %d%s\n", res.Inserted, enrichMsg)
	fmt.Printf("Duplicate: %d\n", res.Duplicate)
	fmt.Printf("Failed:    %d\n", res.Failed)
	fmt.Printf("\nimport run: %s\n\n", res.RunID)

	return nil
}

func parseFile(_ context.Context, reg *parser.Registry, filePath, bank string, u *sqlcgen.User) (parser.ParseResult, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// ICICI: binary XLS — must use ParsePath.
	if bank == "icici" || ext == ".xls" {
		p, err := reg.ByBank("icici")
		if err != nil {
			return parser.ParseResult{}, err
		}
		return p.(*parser.ICICIV1).ParsePath(filePath)
	}

	// SBI: password-encrypted XLSX — derive password from user profile.
	if bank == "sbi" || ext == ".xlsx" {
		p, err := reg.ByBank("sbi")
		if err != nil {
			return parser.ParseResult{}, err
		}
		password := ""
		if u.DateOfBirth.Valid {
			password = parser.SBIPassword(u.Name, u.DateOfBirth.Time)
		}
		return p.(*parser.SBIV1).ParseXLSX(filePath, password)
	}

	// CSV parsers (axis, idfc).
	f, err := os.Open(filePath)
	if err != nil {
		return parser.ParseResult{}, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	p, err := reg.ByBank(bank)
	if err != nil {
		return parser.ParseResult{}, err
	}
	return p.Parse(f)
}

func printDryRun(bank string, result parser.ParseResult) {
	meta := result.Meta
	fmt.Printf("\n=== dry run: %s — %d rows ===\n", strings.ToUpper(bank), len(result.Transactions))
	if meta.AccountNumber != "" {
		fmt.Printf("Account:  %s", meta.AccountNumber)
		if meta.AccountHolder != "" {
			fmt.Printf(" — %s", meta.AccountHolder)
		}
		fmt.Println()
	}
	if meta.IFSC != "" {
		fmt.Printf("IFSC:     %s\n", meta.IFSC)
	}
	fmt.Println()
	for i, r := range result.Transactions {
		dir := "CR"
		if r.Direction == "debit" {
			dir = "DR"
		}
		fmt.Printf("  %3d  %s  %s  ₹%.2f  %s\n",
			i+1,
			r.Date.Format("2006-01-02"),
			dir,
			float64(r.Amount)/100,
			truncate(r.Description, 60),
		)
	}
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
