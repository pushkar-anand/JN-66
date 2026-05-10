// Command finagent is the personal financial intelligence agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/agent"
	"github.com/pushkaranand/finagent/internal/api"
	"github.com/pushkaranand/finagent/internal/channel/cli"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/llm/openai"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Flags
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	userFlag := flag.String("user", "", "user email or name to identify as (CLI mode)")
	serveFlag := flag.Bool("serve", false, "start HTTP API server instead of CLI")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	// Logging
	level := slog.LevelInfo
	if *debugFlag {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Config
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	slog.Debug("llm config", "base_url", cfg.LLM.BaseURL, "api_key_set", cfg.LLM.APIKey != "")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database
	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	if cfg.Database.AutoMigrate {
		slog.Info("running migrations")
		if err := db.Migrate(cfg.Database.URL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	// Stores
	userStore := store.NewUserStore(pool)
	accountStore := store.NewAccountStore(pool)
	txnStore := store.NewTransactionStore(pool)
	labelStore := store.NewLabelStore(pool)
	recurringStore := store.NewRecurringStore(pool)
	memoryStore := store.NewMemoryStore(pool)
	convStore := store.NewConversationStore(pool)

	// Resolve CLI user ID
	userID := resolveUser(ctx, userStore, cmp(*userFlag, cfg.Channel.CLI.DefaultUser))

	// LLM provider
	llmProvider := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)

	// Tool registry
	registry := tools.NewRegistry()
	registry.Register(tools.NewQueryTransactions(userID, txnStore))
	registry.Register(tools.NewGetAccountSummary(userID, accountStore))
	registry.Register(tools.NewGetSpendingBreakdown(userID, txnStore))
	registry.Register(tools.NewManageLabels(userID, labelStore))
	registry.Register(tools.NewListRecurring(userID, recurringStore))
	registry.Register(tools.NewRememberFact(userID, memoryStore))
	registry.Register(tools.NewRecallFacts(userID, memoryStore))

	// Agent
	router := agent.NewRouter(cfg.LLM.Routing)
	ag := agent.New(llmProvider, convStore, memoryStore, userStore, registry, router)

	if *serveFlag {
		srv := api.New(cfg.API.Listen, ag.HandleMessage)
		return srv.Start(ctx)
	}

	// CLI mode requires a resolved user.
	if userID == "" {
		return fmt.Errorf("user %q not found in database; seed the users table or pass a valid --user flag", cmp(*userFlag, cfg.Channel.CLI.DefaultUser))
	}

	cliCh := cli.New(userID)
	slog.Info("starting cli", "user", userID)
	return cliCh.Start(ctx, ag.HandleMessage)
}

// userLookup is the subset of store.UserStore used by resolveUser.
type userLookup interface {
	GetByEmail(ctx context.Context, email string) (*sqlcgen.User, error)
	List(ctx context.Context) ([]sqlcgen.User, error)
}

// resolveUser looks up the user by email or name, returning their UUID.
func resolveUser(ctx context.Context, users userLookup, identifier string) string {
	if identifier == "" {
		slog.Warn("no user specified; use --user <email> or set channel.cli.default_user in config")
		return ""
	}
	u, err := users.GetByEmail(ctx, identifier)
	if err == nil {
		return u.ID.String()
	}
	all, err := users.List(ctx)
	if err == nil {
		for _, candidate := range all {
			if candidate.Name == identifier {
				return candidate.ID.String()
			}
		}
	}
	// Accept identifier as-is only if it's already a valid UUID (e.g. passed directly).
	if _, err := uuid.Parse(identifier); err == nil {
		return identifier
	}
	slog.Warn("user not found in database", "identifier", identifier)
	return ""
}

// cmp returns the first non-empty string.
func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
