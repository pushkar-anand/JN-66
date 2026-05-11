package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pushkaranand/finagent/internal/apikey"
)

type contextKey int

const userIDKey contextKey = 0

// WithUserID returns a context with the given user ID stored for retrieval by UserIDFromContext.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFromContext returns the authenticated user ID injected by authMiddleware, or "".
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || token == r.Header.Get("Authorization") {
			slog.WarnContext(r.Context(), "auth: missing bearer token", slog.String("path", r.URL.Path))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		prefix := apikey.Prefix(token)
		user, err := s.userStore.GetByAPIKeyPrefix(r.Context(), prefix)
		if err != nil {
			slog.WarnContext(r.Context(), "auth: unknown api key prefix", slog.String("prefix", prefix))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !apikey.Verify(token, user.ApiKeyHash) {
			slog.WarnContext(r.Context(), "auth: invalid api key hash", slog.String("user_id", user.ID.String()))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), user.ID.String())))
	})
}
