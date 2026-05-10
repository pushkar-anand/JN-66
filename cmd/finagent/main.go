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
	// Subcommand dispatch — must happen before flag.Parse().
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			if err := runImport(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "import: %v\n", err)
				os.Exit(1)
			}
			return
		case "account":
			if err := runAccount(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "account: %v\n", err)
				os.Exit(1)
			}
			return
		case "user":
			if err := runUser(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "user: %v\n", err)
				os.Exit(1)
			}
			return
		case "enrich":
			if err := runEnrich(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "enrich: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
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

	// Resolve CLI user.
	u, resolveErr := resolveUser(ctx, userStore, *userFlag, cfg.Channel.CLI.DefaultUser)
	userID := ""
	if resolveErr == nil {
		userID = u.ID.String()
	}

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
		return resolveErr
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

// resolveUser returns the user matching identifier or defaultIdentifier.
// Falls back to the sole DB user when neither locates a match.
// Returns an error if identifier is given but not found, or if the
// single-user fallback finds zero or multiple users.
func resolveUser(ctx context.Context, users userLookup, identifier, defaultIdentifier string) (*sqlcgen.User, error) {
	if identifier != "" {
		if u, err := users.GetByEmail(ctx, identifier); err == nil {
			return u, nil
		}
		all, err := users.List(ctx)
		if err == nil {
			for i := range all {
				if all[i].Name == identifier {
					return &all[i], nil
				}
			}
		}
		return nil, fmt.Errorf("user %q not found in database", identifier)
	}
	if defaultIdentifier != "" {
		if u, err := users.GetByEmail(ctx, defaultIdentifier); err == nil {
			return u, nil
		}
	}
	all, err := users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if len(all) == 1 {
		return &all[0], nil
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no users in database — run: finagent user add")
	}
	return nil, fmt.Errorf("multiple users in database — pass --user <email>")
}

// cmp returns the first non-empty string.
func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
