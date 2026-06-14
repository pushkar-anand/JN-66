// Package api implements the HTTP server for the finagent REST API.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pushkar-anand/build-with-go/http/middleware"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
	bwgvalidator "github.com/pushkar-anand/build-with-go/validator"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"

	"github.com/pushkar-anand/agentrig/channel"
)

// userLookup is the store interface required for API authentication.
type userLookup interface {
	GetByAPIKeyPrefix(context.Context, string) (*sqlcgen.User, error)
}

// dbPinger is satisfied by *pgxpool.Pool and used for the readiness probe.
type dbPinger interface {
	Ping(context.Context) error
}

// Server is the HTTP API server.
type Server struct {
	handler     channel.MessageHandler
	userStore   userLookup
	db          dbPinger
	zerodha     *ZerodhaCallbackConfig
	accountsCfg *AccountsConfig
	importCfg   *ImportConfig
	fdCfg       *FDConfig
	txnCfg      *TransactionConfig
	uiCfg       *UIConfig
	v           *bwgvalidator.Validator
	srv         *http.Server
}

// New creates a Server that dispatches chat requests to handler.
// userStore is used for Bearer token authentication; pass nil to disable auth (tests).
// db is used for the readiness probe; pass nil to skip the DB check.
// zerodha, accountsCfg, importCfg, fdCfg, txnCfg, and uiCfg are optional; non-nil values enable their routes.
func New(listen string, handler channel.MessageHandler, userStore userLookup, db dbPinger, zerodha *ZerodhaCallbackConfig, accountsCfg *AccountsConfig, importCfg *ImportConfig, fdCfg *FDConfig, txnCfg *TransactionConfig, uiCfg *UIConfig) *Server {
	// bwgvalidator.New uses sync.Once internally — this must be the only call site in the binary.
	v, err := bwgvalidator.New(
		bwgvalidator.WithCustomTags(map[string]bwgvalidator.ValidationFunc{
			"txndate": validateTxnDate,
		}),
	)
	if err != nil {
		panic("api: failed to initialise validator: " + err.Error())
	}

	s := &Server{
		handler:     handler,
		userStore:   userStore,
		db:          db,
		zerodha:     zerodha,
		accountsCfg: accountsCfg,
		importCfg:   importCfg,
		fdCfg:       fdCfg,
		txnCfg:      txnCfg,
		uiCfg:       uiCfg,
		v:           v,
	}

	r := mux.NewRouter()
	r.Use(recoveryMiddleware)
	r.Use(middleware.RequestID)
	r.Use(bwglogger.NewHTTPLogger(slog.Default()))
	r.HandleFunc("/healthz/live", s.handleLive).Methods(http.MethodGet)
	r.HandleFunc("/healthz/ready", s.handleReady).Methods(http.MethodGet)

	if s.zerodha != nil {
		r.HandleFunc("/api/zerodha/callback", s.handleZerodhaCallback).Methods(http.MethodGet)
	}

	protected := r.NewRoute().Subrouter()
	if s.userStore != nil {
		protected.Use(s.authMiddleware)
	}
	protected.HandleFunc("/api/chat", s.handleChat).Methods(http.MethodPost)

	if s.accountsCfg != nil {
		protected.HandleFunc("/api/accounts", s.handleListAccounts).Methods(http.MethodGet)
		protected.HandleFunc("/api/accounts", s.handleCreateAccount).Methods(http.MethodPost)
		protected.HandleFunc("/api/accounts/{id}/balance", s.handleUpdateAccountBalance).Methods(http.MethodPatch)
	}
	if s.importCfg != nil {
		protected.HandleFunc("/api/import", s.handleImport).Methods(http.MethodPost)
		protected.HandleFunc("/api/import/{id}", s.handleGetImportRun).Methods(http.MethodGet)
	}
	if s.fdCfg != nil {
		protected.HandleFunc("/api/fds", s.handleCreateFD).Methods(http.MethodPost)
	}
	if s.txnCfg != nil {
		protected.HandleFunc("/api/transactions", s.handleListTransactions).Methods(http.MethodGet)
		protected.HandleFunc("/api/transactions/{id}", s.handleGetTransaction).Methods(http.MethodGet)
		protected.HandleFunc("/api/transactions/{id}/enrichment", s.handlePatchEnrichment).Methods(http.MethodPatch)
		protected.HandleFunc("/api/transactions/{id}/labels", s.handleAddLabel).Methods(http.MethodPost)
		protected.HandleFunc("/api/transactions/{id}/labels/{labelId}", s.handleRemoveLabel).Methods(http.MethodDelete)
		protected.HandleFunc("/api/categories", s.handleListCategories).Methods(http.MethodGet)
		protected.HandleFunc("/api/labels", s.handleListLabels).Methods(http.MethodGet)
	}
	if s.uiCfg != nil {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/transactions", http.StatusSeeOther)
		}).Methods(http.MethodGet)
		r.HandleFunc("/ui/login", s.handleUILogin).Methods(http.MethodGet)
		r.HandleFunc("/ui/login", s.handleUILoginPost).Methods(http.MethodPost)

		ui := r.NewRoute().Subrouter()
		ui.Use(s.uiSessionMiddleware)
		ui.HandleFunc("/ui/logout", s.handleUILogout).Methods(http.MethodGet)
		ui.HandleFunc("/ui/transactions", s.handleUITransactions).Methods(http.MethodGet)
		ui.HandleFunc("/ui/transactions/{id}/enrich", s.handleUIEnrich).Methods(http.MethodPost)
		ui.HandleFunc("/ui/transactions/{id}/label", s.handleUIAddLabel).Methods(http.MethodPost)
		ui.HandleFunc("/ui/transactions/{id}/label/{labelId}", s.handleUIRemoveLabel).Methods(http.MethodDelete)
		ui.HandleFunc("/ui/accounts", s.handleUIAccounts).Methods(http.MethodGet)
		ui.HandleFunc("/ui/investments", s.handleUIInvestments).Methods(http.MethodGet)
	}

	s.srv = &http.Server{
		Addr:         listen,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "handler panic", slog.Any("panic", rec))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
	}()

	slog.Info("api server listening", "addr", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
