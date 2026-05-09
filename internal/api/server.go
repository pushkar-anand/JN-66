// Package api implements the HTTP server for the finagent REST API.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/pushkaranand/finagent/internal/channel"
)

// Server is the HTTP API server.
type Server struct {
	handler channel.MessageHandler
	srv     *http.Server
}

// New creates a Server that dispatches chat requests to handler.
func New(listen string, handler channel.MessageHandler) *Server {
	s := &Server{handler: handler}

	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.HandleFunc("/api/health", s.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/api/chat", s.handleChat).Methods(http.MethodPost)

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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
