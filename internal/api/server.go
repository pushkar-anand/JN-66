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

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"

	"github.com/pushkaranand/finagent/internal/channel"
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
	handler   channel.MessageHandler
	userStore userLookup
	db        dbPinger
	zerodha   *ZerodhaCallbackConfig
	srv       *http.Server
}

// New creates a Server that dispatches chat requests to handler.
// userStore is used for Bearer token authentication; pass nil to disable auth (tests).
// db is used for the readiness probe; pass nil to skip the DB check.
// zerodha is optional; when non-nil it registers GET /api/zerodha/callback.
func New(listen string, handler channel.MessageHandler, userStore userLookup, db dbPinger, zerodha *ZerodhaCallbackConfig) *Server {
	s := &Server{handler: handler, userStore: userStore, db: db, zerodha: zerodha}

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
