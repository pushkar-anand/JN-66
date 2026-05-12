package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/zerodha"
)

var zerodhaIST = time.FixedZone("IST", 5*60*60+30*60)

// ZerodhaCallbackConfig holds dependencies for the Zerodha OAuth callback endpoint.
type ZerodhaCallbackConfig struct {
	ServerSecret string
	UserCreds    map[string]ZerodhaCallbackCreds // username → Kite credentials
	Store        zerodhaTokenStore
	UserByID     func(ctx context.Context, id string) (*sqlcgen.User, error)
}

// ZerodhaCallbackCreds is the per-user Kite Connect API key pair.
type ZerodhaCallbackCreds struct {
	APIKey    string
	APISecret string
}

// zerodhaTokenStore is the minimal store interface used by the callback handler.
type zerodhaTokenStore interface {
	UpsertToken(ctx context.Context, userID uuid.UUID, accessToken string, expiresAt time.Time) error
}

func (s *Server) handleZerodhaCallback(w http.ResponseWriter, r *http.Request) {
	cfg := s.zerodha
	requestToken := r.URL.Query().Get("request_token")
	state := r.URL.Query().Get("state")

	if requestToken == "" || state == "" {
		http.Error(w, "missing request_token or state", http.StatusBadRequest)
		return
	}

	userID, err := zerodha.VerifyNonce(state, cfg.ServerSecret)
	if err != nil {
		slog.WarnContext(r.Context(), "zerodha callback: invalid nonce", slog.String("err", err.Error()))
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	user, err := cfg.UserByID(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "zerodha callback: user lookup failed",
			slog.String("user_id", userID), slog.String("err", err.Error()))
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	creds, ok := cfg.UserCreds[user.Username]
	if !ok {
		http.Error(w, fmt.Sprintf("no Zerodha credentials configured for user %q", user.Username), http.StatusInternalServerError)
		return
	}

	client := zerodha.NewClient(creds.APIKey)
	resp, err := client.ExchangeToken(r.Context(), requestToken, creds.APISecret)
	if err != nil {
		slog.ErrorContext(r.Context(), "zerodha callback: token exchange failed",
			slog.String("user", user.Username), slog.String("err", err.Error()))
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	uid, _ := uuid.Parse(userID)
	now := time.Now().In(zerodhaIST)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, zerodhaIST)
	if err := cfg.Store.UpsertToken(r.Context(), uid, resp.AccessToken, midnight); err != nil {
		slog.ErrorContext(r.Context(), "zerodha callback: upsert token failed",
			slog.String("user", user.Username), slog.String("err", err.Error()))
		http.Error(w, "failed to save token", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "zerodha authenticated", slog.String("user", user.Username))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, zerodhaSuccessHTML)
}

var zerodhaSuccessHTML = `<!doctype html>
<html><head><title>Zerodha Auth</title>
<meta charset="utf-8">
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5}
.card{background:#fff;border-radius:8px;padding:2rem 3rem;box-shadow:0 2px 8px rgba(0,0,0,.1);text-align:center}
h1{color:#2e7d32}p{color:#555}</style>
</head><body>
<div class="card"><h1>&#10003; Authentication Successful</h1>
<p>Your Zerodha account has been connected. You can close this tab.</p></div>
</body></html>`
