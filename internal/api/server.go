// Package api implements the HTTP server for the finagent REST API.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
	"github.com/pushkar-anand/build-with-go/http/middleware"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"

	"github.com/pushkaranand/finagent/internal/channel"
)

// userLookup is the store interface required for API authentication.
type userLookup interface {
	GetByAPIKeyPrefix(context.Context, string) (*sqlcgen.User, error)
}

// Server is the HTTP API server.
type Server struct {
	handler   channel.MessageHandler
	userStore userLookup
	srv       *http.Server
}

// New creates a Server that dispatches chat requests to handler.
// userStore is used for Bearer token authentication; pass nil to disable auth (tests).
func New(listen string, handler channel.MessageHandler, userStore userLookup) *Server {
	s := &Server{handler: handler, userStore: userStore}

	r := mux.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(bwglogger.NewHTTPLogger(slog.Default()))
	r.HandleFunc("/api/health", s.handleHealth).Methods(http.MethodGet)

	protected := r.NewRoute().Subrouter()
	if s.userStore != nil {
		protected.Use(s.authMiddleware)
	}
	protected.HandleFunc("/api/chat", s.handleChat).Methods(http.MethodPost)

	s.srv = &http.Server{
		Addr:         listen,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
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

