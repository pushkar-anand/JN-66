// Command eval runs the finagent behavioural eval suite against the real agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/agent"
	"github.com/pushkaranand/finagent/internal/app"
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
	compare := flag.String("compare", "", "compare two models sequentially: 'model_a,model_b'")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.SetupLogger(cfg.Log, false)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	if *compare != "" {
		return runCompare(ctx, cfg, pool, *userEmail, *compare, *filter)
	}

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

		convStore := store.NewConversationStore(pool)

		llmRec := eval.NewRecordingLLM(realLLM)

		registry, memoryStore := app.BuildToolRegistry(pool, userID)
		zStore := store.NewZerodhaStore(pool)
		registry.Register(tools.NewGetInvestmentSummary(userID, zStore, time.UTC))
		registry.Register(tools.NewGetInvestmentHoldings(userID, zStore, time.UTC))
		registry.Register(tools.NewGetMFHoldings(userID, zStore, time.UTC))
		regRec := eval.NewRecordingRegistry(registry)

		router := agent.NewRouter(cfg.LLM.Routing)
		ag := agent.New(llmRec, convStore, memoryStore, userStore, regRec, router, true)

		scenarios := slices.Clone(eval.Scenarios)
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
			res := sc.Run(ctx, pool, ag.HandleMessage, llmRec, regRec)
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

// routingAllModel returns a RoutingConfig with all LLM slots set to model,
// preserving the embed model which is not used for generation.
func routingAllModel(base config.RoutingConfig, model string) config.RoutingConfig {
	return config.RoutingConfig{
		ChatModel:      model,
		AnalysisModel:  model,
		TaggingModel:   model,
		EmbedModel:     base.EmbedModel,
		SummarizeModel: model,
	}
}

// modelRun holds the results of running all agent scenarios against one model.
type modelRun struct {
	model   string
	results []eval.EvalResult
}

// enrichRun holds the results of running all enrichment scenarios against one model.
type enrichRun struct {
	model   string
	results []eval.EnrichEvalResult
}

// collectAgentResults builds an agent wired to the given routing config and runs
// all (optionally filtered) scenarios against it, printing progress as it goes.
func collectAgentResults(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, userID, filter string, routing config.RoutingConfig) (*modelRun, error) {
	realLLM := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	llmRec := eval.NewRecordingLLM(realLLM)

	registry, memoryStore := app.BuildToolRegistry(pool, userID)
	zStore := store.NewZerodhaStore(pool)
	registry.Register(tools.NewGetInvestmentSummary(userID, zStore, time.UTC))
	registry.Register(tools.NewGetInvestmentHoldings(userID, zStore, time.UTC))
	registry.Register(tools.NewGetMFHoldings(userID, zStore, time.UTC))
	regRec := eval.NewRecordingRegistry(registry)

	userStore := store.NewUserStore(pool)
	convStore := store.NewConversationStore(pool)
	router := agent.NewRouter(routing)
	ag := agent.New(llmRec, convStore, memoryStore, userStore, regRec, router, true)

	scenarios := slices.Clone(eval.Scenarios)
	for i := range scenarios {
		scenarios[i].UserID = userID
	}
	if filter != "" {
		var filtered []eval.EvalCase
		for _, s := range scenarios {
			if strings.Contains(s.Name, filter) {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
	}

	run := &modelRun{model: routing.ChatModel}
	for i := range scenarios {
		sc := &scenarios[i]
		fmt.Printf("  %-32s", sc.Name)
		res := sc.Run(ctx, pool, ag.HandleMessage, llmRec, regRec)
		run.results = append(run.results, res)
		if res.Passed {
			fmt.Printf("✓  %dr  %.1fs\n", res.LLMRounds, res.Duration.Seconds())
		} else {
			fmt.Printf("✗  %v\n", res.Failures[0])
		}
	}
	return run, nil
}

// collectEnrichResults runs all enrichment scenarios against the given model and prints progress.
func collectEnrichResults(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, model, filter string) (*enrichRun, error) {
	catStore := store.NewCategoryStore(pool)
	cats, err := catStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	catInfos := make([]importer.CategoryInfo, len(cats))
	for i, c := range cats {
		catInfos[i] = importer.CategoryInfo{Slug: c.Slug, Description: c.Description}
	}

	realLLM := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)
	enricher := importer.NewEnricher(realLLM, model, catInfos)

	scenarios := eval.EnrichScenarios
	if filter != "" {
		var filtered []eval.EnrichEvalCase
		for _, s := range scenarios {
			if strings.Contains(s.Name, filter) {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
	}

	run := &enrichRun{model: model}
	for _, sc := range scenarios {
		fmt.Printf("  %-32s", sc.Name)
		res := eval.RunEnrichEval(ctx, enricher, sc)
		run.results = append(run.results, res)
		if res.Passed {
			fmt.Printf("✓  %-28s  %.1fs\n", res.GotCategory, res.Duration.Seconds())
		} else {
			fmt.Printf("✗  %v\n", res.Failures[0])
		}
	}
	return run, nil
}

// runCompare runs the agent eval suite sequentially against two models and prints
// a side-by-side comparison table. Only one model is active at a time.
func runCompare(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, userEmail, compareFlag, filter string) error {
	parts := strings.SplitN(compareFlag, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("--compare requires exactly two comma-separated model names, got %q", compareFlag)
	}
	modelA := strings.TrimSpace(parts[0])
	modelB := strings.TrimSpace(parts[1])
	if modelA == "" || modelB == "" {
		return fmt.Errorf("--compare: model names must not be empty")
	}

	userStore := store.NewUserStore(pool)
	u, err := userStore.GetByEmail(ctx, userEmail)
	if err != nil {
		return fmt.Errorf("user %q not found in database (run make seed first): %w", userEmail, err)
	}
	userID := u.ID.String()

	fmt.Printf("\n=== model compare: %s  vs  %s ===\n\n", modelA, modelB)

	fmt.Printf("--- %s (agent) ---\n", modelA)
	agentA, err := collectAgentResults(ctx, cfg, pool, userID, filter, routingAllModel(cfg.LLM.Routing, modelA))
	if err != nil {
		return fmt.Errorf("run %s: %w", modelA, err)
	}
	fmt.Printf("\n--- %s (enrich) ---\n", modelA)
	enrichA, err := collectEnrichResults(ctx, cfg, pool, modelA, filter)
	if err != nil {
		return fmt.Errorf("enrich %s: %w", modelA, err)
	}

	fmt.Printf("\n--- %s (agent) ---\n", modelB)
	agentB, err := collectAgentResults(ctx, cfg, pool, userID, filter, routingAllModel(cfg.LLM.Routing, modelB))
	if err != nil {
		return fmt.Errorf("run %s: %w", modelB, err)
	}
	fmt.Printf("\n--- %s (enrich) ---\n", modelB)
	enrichB, err := collectEnrichResults(ctx, cfg, pool, modelB, filter)
	if err != nil {
		return fmt.Errorf("enrich %s: %w", modelB, err)
	}

	printComparison(agentA, agentB)
	printEnrichComparison(enrichA, enrichB)
	return nil
}

// printComparison prints a side-by-side table comparing two model runs.
func printComparison(a, b *modelRun) {
	const nameW = 32

	// Truncate model names for column headers if needed.
	ha := truncate(a.model, 22)
	hb := truncate(b.model, 22)

	fmt.Printf("\n%-*s  %-22s  %-22s\n", nameW, "Scenario", ha, hb)
	fmt.Printf("%s  %s  %s\n", strings.Repeat("─", nameW), strings.Repeat("─", 22), strings.Repeat("─", 22))

	// Index B results by scenario name for alignment (order may differ if filter is used).
	bByName := make(map[string]eval.EvalResult, len(b.results))
	for _, r := range b.results {
		bByName[r.Case.Name] = r
	}

	var (
		aPass, bPass       int
		aTotalMs, bTotalMs int64
		aFaster, bFaster   int
	)

	for _, ra := range a.results {
		rb, ok := bByName[ra.Case.Name]

		aCell := formatCell(ra)
		bCell := ""
		if ok {
			bCell = formatCell(rb)
		} else {
			bCell = "—"
		}

		// Highlight winner in time (only when both passed).
		marker := ""
		if ok && ra.Passed && rb.Passed {
			diff := rb.Duration - ra.Duration
			switch {
			case diff > 500*time.Millisecond:
				marker = fmt.Sprintf("  A faster +%.1fs", diff.Seconds())
				aFaster++
			case diff < -500*time.Millisecond:
				marker = fmt.Sprintf("  B faster +%.1fs", (-diff).Seconds())
				bFaster++
			}
		}

		fmt.Printf("%-*s  %-22s  %-22s%s\n", nameW, ra.Case.Name, aCell, bCell, marker)

		if ra.Passed {
			aPass++
		}
		aTotalMs += ra.Duration.Milliseconds()
		if ok {
			if rb.Passed {
				bPass++
			}
			bTotalMs += rb.Duration.Milliseconds()
		}
	}

	fmt.Printf("\n%-*s  %-22s  %-22s\n", nameW, "─────", strings.Repeat("─", 22), strings.Repeat("─", 22))

	aTotal := time.Duration(aTotalMs) * time.Millisecond
	bTotal := time.Duration(bTotalMs) * time.Millisecond
	aSummary := fmt.Sprintf("%d/%d passed  %.0fs", aPass, len(a.results), aTotal.Seconds())
	bSummary := fmt.Sprintf("%d/%d passed  %.0fs", bPass, len(b.results), bTotal.Seconds())
	fmt.Printf("%-*s  %-22s  %-22s\n", nameW, "TOTAL", aSummary, bSummary)

	if aFaster+bFaster > 0 {
		fmt.Printf("\nSpeed: %s faster on %d scenario(s), %s faster on %d scenario(s)\n",
			a.model, aFaster, b.model, bFaster)
	}
}

// printEnrichComparison prints a side-by-side enrichment eval comparison table.
func printEnrichComparison(a, b *enrichRun) {
	const nameW = 32

	ha := truncate(a.model, 22)
	hb := truncate(b.model, 22)

	fmt.Printf("\n=== enrichment comparison ===\n\n")
	fmt.Printf("%-*s  %-22s  %-22s\n", nameW, "Scenario", ha, hb)
	fmt.Printf("%s  %s  %s\n", strings.Repeat("─", nameW), strings.Repeat("─", 22), strings.Repeat("─", 22))

	bByName := make(map[string]eval.EnrichEvalResult, len(b.results))
	for _, r := range b.results {
		bByName[r.Case.Name] = r
	}

	var aPass, bPass int

	for _, ra := range a.results {
		rb, ok := bByName[ra.Case.Name]

		aCell := formatEnrichCell(ra)
		bCell := "—"
		if ok {
			bCell = formatEnrichCell(rb)
		}

		fmt.Printf("%-*s  %-22s  %-22s\n", nameW, ra.Case.Name, aCell, bCell)

		if ra.Passed {
			aPass++
		}
		if ok && rb.Passed {
			bPass++
		}
	}

	fmt.Printf("\n%-*s  %-22s  %-22s\n", nameW, "─────", strings.Repeat("─", 22), strings.Repeat("─", 22))
	aSummary := fmt.Sprintf("%d/%d passed", aPass, len(a.results))
	bSummary := fmt.Sprintf("%d/%d passed", bPass, len(b.results))
	fmt.Printf("%-*s  %-22s  %-22s\n", nameW, "TOTAL", aSummary, bSummary)
}

// formatEnrichCell formats a single EnrichEvalResult for the comparison table.
func formatEnrichCell(r eval.EnrichEvalResult) string {
	if !r.Passed {
		return "✗ " + truncate(r.Failures[0], 18)
	}
	return fmt.Sprintf("✓ %-16s %.1fs", truncate(r.GotCategory, 16), r.Duration.Seconds())
}

// formatCell formats a single EvalResult into a fixed-width column string.
func formatCell(r eval.EvalResult) string {
	if !r.Passed {
		return "✗ " + truncate(r.Failures[0], 18)
	}
	return fmt.Sprintf("✓ %dr %.1fs", r.LLMRounds, r.Duration.Seconds())
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
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
	for line := range strings.SplitSeq(out, "\n") {
		fmt.Printf("      │   %s\n", line)
	}
	fmt.Printf("      └───────────────────────────────────────────\n\n")
}
