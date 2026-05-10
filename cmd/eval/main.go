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
	userEmail := flag.String("user", "alice@example.com", "seed user email to run evals as")
	filter := flag.String("run", "", "run only scenarios whose name contains this substring")
	verbose := flag.Bool("verbose", false, "print full message trace for failed (or all) scenarios")
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
	userStore := store.NewUserStore(pool)
	u, err := userStore.GetByEmail(ctx, *userEmail)
	if err != nil {
		return fmt.Errorf("user %q not found in database (run make seed first): %w", *userEmail, err)
	}
	userID := u.ID.String()

	// Build real stores.
	accountStore := store.NewAccountStore(pool)
	txnStore := store.NewTransactionStore(pool)
	labelStore := store.NewLabelStore(pool)
	recurringStore := store.NewRecurringStore(pool)
	memoryStore := store.NewMemoryStore(pool)
	convStore := store.NewConversationStore(pool)

	// Build real LLM provider + recording wrapper.
	realLLM := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	llmRec := eval.NewRecordingLLM(realLLM)

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
	ag := agent.New(llmRec, convStore, memoryStore, userStore, regRec, router)

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

		if *verbose || !res.Passed {
			printTrace(res)
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

// printTrace prints the full LLM turn and tool call log for a scenario result.
func printTrace(res eval.EvalResult) {
	fmt.Printf("\n      ┌─ trace: %s ─────────────────────────────\n", res.Case.Name)

	toolIdx := 0
	for _, turn := range res.LLMTurns {
		fmt.Printf("      │ round %d  (%d msgs in context)\n", turn.Round, len(turn.Messages))

		// Show the last new message(s) sent to the LLM this round.
		// The last message before the assistant reply is the most recently appended input.
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

		// Show tool calls made this round.
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

		// Show stop reason and final content.
		if turn.Response.StopReason == "stop" || len(turn.Response.Message.ToolCalls) == 0 {
			content := turn.Response.Message.Content
			if len(content) > 200 {
				content = content[:200] + "…"
			}
			fmt.Printf("      │   ← %s\n", strings.ReplaceAll(content, "\n", " "))
		}
	}

	// Print the system prompt role and user message for context.
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