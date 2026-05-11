package api

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		hash := sha256.Sum256([]byte(token))
		user, err := s.userStore.GetByAPIKeyHash(r.Context(), hash[:])
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), user.ID.String())))
	})
}
