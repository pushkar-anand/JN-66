// Command finagent is the personal financial intelligence agent.
package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	matrixchannel "github.com/pushkar-anand/agentrig/channel/matrix"
	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/agent"
	"github.com/pushkaranand/finagent/internal/api"
	"github.com/pushkaranand/finagent/internal/apikey"
	"github.com/pushkaranand/finagent/internal/app"
	"github.com/pushkaranand/finagent/internal/channel/cli"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/llm/openai"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
	"github.com/pushkaranand/finagent/internal/zerodha"
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
		case "zerodha":
			if err := runZerodha(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "zerodha: %v\n", err)
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
	configPath := flag.String("config", "", "path to config file (default: ./config.yaml)")
	userFlag := flag.String("user", "", "user username, email, or name to identify as (CLI mode)")
	serveFlag := flag.Bool("serve", false, "start HTTP API server instead of CLI")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *configPath == "" {
		*configPath = "config.yaml"
	}

	// Config
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.SetupLogger(cfg.Log, *debugFlag)
	slog.Debug("llm config", "base_url", cfg.LLM.BaseURL, "api_key_set", cfg.LLM.APIKey != "")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database
	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	var roPool *pgxpool.Pool
	if cfg.Database.ReadOnlyURL != "" {
		roPool, err = db.Open(ctx, cfg.Database.ReadOnlyURL, 5)
		if err != nil {
			return fmt.Errorf("open readonly db: %w", err)
		}
		defer roPool.Close()
	}

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
	runStore := store.NewImportRunStore(pool)
	catStore := store.NewCategoryStore(pool)
	convStore := store.NewConversationStore(pool)
	zStore := store.NewZerodhaStore(pool)

	// Bootstrap users from config.
	if err := bootstrapUsers(ctx, userStore, cfg); err != nil {
		return fmt.Errorf("bootstrap users: %w", err)
	}

	// Resolve CLI user.
	u, resolveErr := resolveUser(ctx, userStore, *userFlag, cfg.Channel.CLI.DefaultUser)
	userID := ""
	if resolveErr == nil {
		userID = u.ID.String()
	}

	// LLM provider
	llmProvider := openai.New(cfg.LLM.BaseURL, cfg.LLM.APIKey)

	// Tool registry — shared base tools.
	var schemaStr string
	if roPool != nil {
		schemaStr, err = tools.BuildSchemaString(ctx, roPool)
		if err != nil {
			slog.Warn("could not build schema string — SQL agent tools disabled", "err", err)
			roPool = nil
		}
	}
	registry, memoryStore := app.BuildToolRegistry(pool, roPool, schemaStr, userID)

	// Conditionally register Zerodha investment tools if credentials are configured for this user.
	if u != nil {
		loc, err := time.LoadLocation(cmp.Or(u.Timezone, "Asia/Kolkata"))
		if err != nil {
			return fmt.Errorf("user timezone %q: %w", u.Timezone, err)
		}
		if zerCreds, ok := cfg.Zerodha.Users[u.Username]; ok {
			zSvc := store.NewZerodhaService(zStore, zerodha.NewClient(zerCreds.APIKey))

			// loginURLFunc generates a fresh Kite login URL for in-chat re-auth.
			// Only wired when --serve is active: the OAuth callback endpoint
			// /api/zerodha/callback only exists when the HTTP server is running.
			var loginURLFunc func() string
			if *serveFlag && cfg.Zerodha.ServerSecret != "" {
				client := zerodha.NewClient(zerCreds.APIKey)
				uid, secret := userID, cfg.Zerodha.ServerSecret
				loginURLFunc = func() string {
					return client.LoginURL("", zerodha.NewNonce(uid, secret))
				}
			}

			registry.Register(tools.NewGetInvestmentHoldings(userID, zSvc, loc))
			registry.Register(tools.NewGetMFHoldings(userID, zSvc, loc))
			registry.Register(tools.NewGetInvestmentSummary(userID, zSvc, loc))
			registry.Register(tools.NewSyncPortfolio(userID, zSvc, loginURLFunc))
		}
	}

	// Agent
	router := agent.NewRouter(cfg.LLM.Routing)
	ag := agent.New(llmProvider, convStore, memoryStore, userStore, registry, router, registry.Has("get_investment_holdings"), sqlcgen.ChannelEnumCli)

	if *serveFlag {
		var zerCbCfg *api.ZerodhaCallbackConfig
		if len(cfg.Zerodha.Users) > 0 && cfg.Zerodha.ServerSecret != "" {
			userCreds := make(map[string]api.ZerodhaCallbackCreds, len(cfg.Zerodha.Users))
			for uname, creds := range cfg.Zerodha.Users {
				userCreds[uname] = api.ZerodhaCallbackCreds{APIKey: creds.APIKey, APISecret: creds.APISecret}
			}
			zerCbCfg = &api.ZerodhaCallbackConfig{
				ServerSecret: cfg.Zerodha.ServerSecret,
				UserCreds:    userCreds,
				Store:        zStore,
				UserByID:     userStore.GetByID,
			}
		}
		accountsCfg := &api.AccountsConfig{Store: accountStore}
		importCfg := &api.ImportConfig{
			UserGetter:   userStore,
			AccountStore: accountStore,
			TxnStore:     txnStore,
			RunStore:     runStore,
			CatStore:     catStore,
			LLMProvider:  llmProvider,
			TaggingModel: cfg.LLM.Routing.TaggingModel,
		}
		fdCfg := &api.FDConfig{Store: store.NewFDStore(pool)}
		srv := api.New(cfg.API.Listen, ag.HandleMessage, userStore, pool, zerCbCfg, accountsCfg, importCfg, fdCfg)

		// Start Matrix channel alongside the HTTP API when configured.
		if cfg.Channel.Matrix.HomeserverURL != "" {
			matrixErrCh := make(chan error, 1)
			go func() {
				matrixErrCh <- startMatrix(ctx, cfg, pool, roPool, schemaStr, llmProvider, convStore, userStore, zStore, router)
			}()
			// Run HTTP API in a separate goroutine so both channels run concurrently.
			apiErrCh := make(chan error, 1)
			go func() { apiErrCh <- srv.Start(ctx) }()
			select {
			case err := <-matrixErrCh:
				return fmt.Errorf("matrix channel: %w", err)
			case err := <-apiErrCh:
				return err
			}
		}

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

// bootstrapUsers upserts all users declared in config. Every user must have a
// corresponding api_keys entry — startup fails loudly if one is missing.
func bootstrapUsers(ctx context.Context, us *store.UserStore, cfg *config.Config) error {
	for _, u := range cfg.Users {
		key := cfg.APIKeys[u.Username]
		if key == "" {
			return fmt.Errorf(
				"user %q has no api_key — add api_keys.%s to config or set FINAGENT_API_KEYS__%s",
				u.Username, u.Username, strings.ToUpper(u.Username),
			)
		}
		hash, err := apikey.Hash(key)
		if err != nil {
			return fmt.Errorf("hash api key for %s: %w", u.Username, err)
		}
		if _, err := us.Upsert(ctx, store.UpsertUserParams{
			Username:     u.Username,
			Name:         u.Name,
			Email:        u.Email,
			Timezone:     u.Timezone,
			APIKeyPrefix: apikey.Prefix(key),
			APIKeyHash:   hash,
		}); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.Username, err)
		}
	}
	return nil
}

// startMatrix starts the Matrix channel. It blocks until ctx is cancelled.
// Agents are built lazily per user so each household member gets a properly
// scoped tool registry.
func startMatrix(
	ctx context.Context,
	cfg *config.Config,
	pool, roPool *pgxpool.Pool,
	schemaStr string,
	llmProvider *openai.Client,
	convStore *store.ConversationStore,
	userStore *store.UserStore,
	zStore *store.ZerodhaStore,
	router *agent.Router,
) error {
	agPool := agent.NewPool(func(ctx context.Context, userID string) (*agent.Agent, error) {
		u, err := userStore.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load user %s: %w", userID, err)
		}

		loc, err := time.LoadLocation(cmp.Or(u.Timezone, "Asia/Kolkata"))
		if err != nil {
			return nil, fmt.Errorf("user timezone %q: %w", u.Timezone, err)
		}

		registry, memoryStore := app.BuildToolRegistry(pool, roPool, schemaStr, userID)

		if zerCreds, ok := cfg.Zerodha.Users[u.Username]; ok {
			zSvc := store.NewZerodhaService(zStore, zerodha.NewClient(zerCreds.APIKey))
			registry.Register(tools.NewGetInvestmentHoldings(userID, zSvc, loc))
			registry.Register(tools.NewGetMFHoldings(userID, zSvc, loc))
			registry.Register(tools.NewGetInvestmentSummary(userID, zSvc, loc))
			registry.Register(tools.NewSyncPortfolio(userID, zSvc, nil))
		}

		return agent.New(
			llmProvider, convStore, memoryStore, userStore, registry, router,
			registry.Has("get_investment_holdings"), sqlcgen.ChannelEnumMatrix,
		), nil
	})

	ch, err := matrixchannel.New(toMatrixChannelConfig(cfg.Channel.Matrix))
	if err != nil {
		return fmt.Errorf("create matrix channel: %w", err)
	}

	slog.Info("starting matrix channel", "homeserver", cfg.Channel.Matrix.HomeserverURL)
	return ch.Start(ctx, agPool.HandleMessage)
}

// toMatrixChannelConfig converts finagent's MatrixConfig to agentrig's matrix.Config.
// Kept as a named function so it can be unit-tested independently.
func toMatrixChannelConfig(cfg config.MatrixConfig) matrixchannel.Config {
	return matrixchannel.Config{
		HomeserverURL:     cfg.HomeserverURL,
		UserID:            cfg.UserID,
		AccessToken:       cfg.AccessToken,
		EncryptionEnabled: cfg.EncryptionEnabled,
		CryptoStorePath:   cfg.CryptoStorePath,
		PickleKey:         cfg.PickleKey,
		RecoveryKey:       cfg.RecoveryKey,
		AllowedUsers:      cfg.AllowedUsers,
		Users:             cfg.Users,
	}
}

// userLookup is the subset of store.UserStore used by resolveUser.
type userLookup interface {
	GetByUsername(ctx context.Context, username string) (*sqlcgen.User, error)
	GetByEmail(ctx context.Context, email string) (*sqlcgen.User, error)
	List(ctx context.Context) ([]sqlcgen.User, error)
}

// resolveUser returns the user matching identifier or defaultIdentifier.
// Tries username → email → name. Falls back to the sole DB user when
// neither locates a match.
func resolveUser(ctx context.Context, users userLookup, identifier, defaultIdentifier string) (*sqlcgen.User, error) {
	if identifier != "" {
		if u, err := users.GetByUsername(ctx, identifier); err == nil {
			return u, nil
		}
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
		if u, err := users.GetByUsername(ctx, defaultIdentifier); err == nil {
			return u, nil
		}
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
	return nil, fmt.Errorf("multiple users in database — pass --user <username>")
}
