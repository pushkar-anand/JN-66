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
	"github.com/pushkaranand/finagent/internal/importer"
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
	userEmail := flag.String("user", "alice@example.com", "seed user email to run agent evals as")
	filter := flag.String("run", "", "run only scenarios whose name contains this substring")
	verbose := flag.Bool("verbose", false, "print full message trace for failed (or all) agent scenarios")
	onlyEnrich := flag.Bool("only-enrich", false, "run only enrichment evals, skip agent evals")
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

	realLLM := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)

	var totalFailed int

	// Agent evals — require a seeded user in the DB.
	if !*onlyEnrich {
		userStore := store.NewUserStore(pool)
		u, err := userStore.GetByEmail(ctx, *userEmail)
		if err != nil {
			return fmt.Errorf("user %q not found in database (run make seed first): %w", *userEmail, err)
		}
		userID := u.ID.String()

		accountStore := store.NewAccountStore(pool)
		txnStore := store.NewTransactionStore(pool)
		labelStore := store.NewLabelStore(pool)
		recurringStore := store.NewRecurringStore(pool)
		memoryStore := store.NewMemoryStore(pool)
		convStore := store.NewConversationStore(pool)

		llmRec := eval.NewRecordingLLM(realLLM)

		registry := tools.NewRegistry()
		registry.Register(tools.NewQueryTransactions(userID, txnStore))
		registry.Register(tools.NewGetAccountSummary(userID, accountStore))
		registry.Register(tools.NewGetSpendingBreakdown(userID, txnStore))
		registry.Register(tools.NewManageLabels(userID, labelStore))
		registry.Register(tools.NewListRecurring(userID, recurringStore))
		registry.Register(tools.NewRememberFact(userID, memoryStore))
		registry.Register(tools.NewRecallFacts(userID, memoryStore))
		regRec := eval.NewRecordingRegistry(registry)

		router := agent.NewRouter(cfg.LLM.Routing)
		ag := agent.New(llmRec, convStore, memoryStore, userStore, regRec, router)

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

		fmt.Printf("\n=== finagent agent eval — %s ===\n\n", *userEmail)

		var passed, failed int
		agentStart := time.Now()
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
			if *verbose || !res.Passed {
				printTrace(res)
			}
		}
		fmt.Printf("\n%d agent scenarios: %d passed, %d failed   total: %.0fs\n\n",
			len(scenarios), passed, failed, time.Since(agentStart).Seconds())
		totalFailed += failed
	}

	// Enrichment evals — classify raw transactions with the LLM.
	{
		catStore := store.NewCategoryStore(pool)
		cats, err := catStore.List(ctx)
		if err != nil {
			return fmt.Errorf("load categories: %w", err)
		}
		catInfos := make([]importer.CategoryInfo, len(cats))
		for i, c := range cats {
			catInfos[i] = importer.CategoryInfo{Slug: c.Slug, Description: c.Description}
		}
		enricher := importer.NewEnricher(realLLM, cfg.LLM.Routing.TaggingModel, catInfos)

		enrichScenarios := eval.EnrichScenarios
		if *filter != "" {
			var filtered []eval.EnrichEvalCase
			for _, s := range enrichScenarios {
				if strings.Contains(s.Name, *filter) {
					filtered = append(filtered, s)
				}
			}
			enrichScenarios = filtered
		}

		fmt.Printf("=== finagent enrichment eval ===\n\n")

		var passed, failed int
		enrichStart := time.Now()
		for _, sc := range enrichScenarios {
			fmt.Printf("  %-32s", sc.Name)
			res := eval.RunEnrichEval(ctx, enricher, sc)
			if res.Passed {
				passed++
				fmt.Printf("✓  %-28s  %.1fs\n", res.GotCategory, res.Duration.Seconds())
			} else {
				failed++
				fmt.Printf("✗\n")
				for _, f := range res.Failures {
					fmt.Printf("      %s\n", f)
				}
			}
		}
		pct := 0
		if len(enrichScenarios) > 0 {
			pct = passed * 100 / len(enrichScenarios)
		}
		fmt.Printf("\n%d enrichment cases: %d passed, %d failed (%d%%)   total: %.0fs\n\n",
			len(enrichScenarios), passed, failed, pct, time.Since(enrichStart).Seconds())
		totalFailed += failed
	}

	if totalFailed > 0 {
		return fmt.Errorf("%d eval(s) failed", totalFailed)
	}
	return nil
}

// printTrace prints the full LLM turn and tool call log for an agent scenario result.
func printTrace(res eval.EvalResult) {
	fmt.Printf("\n      ┌─ trace: %s ─────────────────────────────\n", res.Case.Name)

	toolIdx := 0
	for _, turn := range res.LLMTurns {
		fmt.Printf("      │ round %d  (%d msgs in context)\n", turn.Round, len(turn.Messages))

		if len(turn.Messages) > 0 {
			last := turn.Messages[len(turn.Messages)-1]
			role := string(last.Role)
			content := last.Content
			if len(content) > 120 {
				content = content[:120] + "…"
			}
			fmt.Printf("      │   → [%s] %s\n", role, content)
		}

		if turn.Err != nil {
			fmt.Printf("      │   ✗ llm error: %v\n", turn.Err)
			continue
		}

		for range turn.Response.Message.ToolCalls {
			if toolIdx < len(res.Invocations) {
				inv := res.Invocations[toolIdx]
				args := inv.ArgsJSON
				if len(args) > 80 {
					args = args[:80] + "…"
				}
				result := inv.Result
				if len(result) > 120 {
					result = result[:120] + "…"
				}
				if inv.Err != nil {
					fmt.Printf("      │   ⚙ %s(%s)  ✗ %v\n", inv.Name, args, inv.Err)
				} else {
					fmt.Printf("      │   ⚙ %s(%s)\n", inv.Name, args)
					fmt.Printf("      │     ↳ %s\n", strings.ReplaceAll(result, "\n", " "))
				}
				toolIdx++
			}
		}

		if turn.Response.StopReason == "stop" || len(turn.Response.Message.ToolCalls) == 0 {
			content := turn.Response.Message.Content
			if len(content) > 200 {
				content = content[:200] + "…"
			}
			fmt.Printf("      │   ← %s\n", strings.ReplaceAll(content, "\n", " "))
		}
	}

	fmt.Printf("      │\n      │ final output (%d chars):\n", len(res.Output))
	out := res.Output
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	for _, line := range strings.Split(out, "\n") {
		fmt.Printf("      │   %s\n", line)
	}
	fmt.Printf("      └───────────────────────────────────────────\n\n")
}
