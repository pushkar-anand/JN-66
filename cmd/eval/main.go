// Command eval runs the finagent behavioural eval suite against the real agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/agent"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/eval"
	"github.com/pushkaranand/finagent/internal/llm/openai"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	userEmail  := flag.String("user", "alice@example.com", "seed user email to run evals as")
	filter     := flag.String("run", "", "run only scenarios whose name contains this substring")
	flag.Parse()

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

	// Resolve the eval user.
	userStore     := store.NewUserStore(pool)
	u, err := userStore.GetByEmail(ctx, *userEmail)
	if err != nil {
		return fmt.Errorf("user %q not found in database (run make seed first): %w", *userEmail, err)
	}
	userID := u.ID.String()

	// Build real stores.
	accountStore  := store.NewAccountStore(pool)
	txnStore      := store.NewTransactionStore(pool)
	labelStore    := store.NewLabelStore(pool)
	recurringStore := store.NewRecurringStore(pool)
	memoryStore   := store.NewMemoryStore(pool)
	convStore     := store.NewConversationStore(pool)

	// Build real LLM provider + recording wrapper.
	realLLM   := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	llmRec    := eval.NewRecordingLLM(realLLM)

	// Build real tool registry + recording wrapper.
	registry := tools.NewRegistry()
	registry.Register(tools.NewQueryTransactions(userID, txnStore))
	registry.Register(tools.NewGetAccountSummary(userID, accountStore))
	registry.Register(tools.NewGetSpendingBreakdown(userID, txnStore))
	registry.Register(tools.NewManageLabels(userID, labelStore))
	registry.Register(tools.NewListRecurring(userID, recurringStore))
	registry.Register(tools.NewRememberFact(userID, memoryStore))
	registry.Register(tools.NewRecallFacts(userID, memoryStore))
	regRec := eval.NewRecordingRegistry(registry)

	// Wire agent with recorders instead of raw LLM + registry.
	router := agent.NewRouter(cfg.LLM.Routing)
	ag     := agent.New(llmRec, convStore, memoryStore, userStore, regRec, router)

	// Select scenarios.
	scenarios := eval.Scenarios
	for i := range scenarios {
		scenarios[i].UserID = userID
	}
	if *filter != "" {
		var filtered []eval.EvalCase
		for _, s := range scenarios {
			if strings.Contains(s.Name, *filter) {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
	}

	fmt.Printf("\n=== finagent eval — %s ===\n\n", *userEmail)

	var passed, failed int
	totalStart := time.Now()

	for i := range scenarios {
		sc := &scenarios[i]
		fmt.Printf("  %-32s", sc.Name)
		res := sc.Run(ctx, ag.HandleMessage, llmRec, regRec)
		if res.Passed {
			passed++
			fmt.Printf("✓  %d rounds  %.1fs\n", res.LLMRounds, res.Duration.Seconds())
		} else {
			failed++
			fmt.Printf("✗\n")
			for _, f := range res.Failures {
				fmt.Printf("      %s\n", f)
			}
		}
	}

	total := time.Since(totalStart)
	fmt.Printf("\n%d scenarios: %d passed, %d failed   total: %.0fs\n\n",
		len(scenarios), passed, failed, total.Seconds())

	if failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", failed)
	}
	return nil
}
