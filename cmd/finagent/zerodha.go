package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/zerodha"
)

func runZerodha(args []string) error {
	fs := flag.NewFlagSet("zerodha", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	userFlag := fs.String("user", "", "username (optional if only one user in DB)")
	_ = fs.Parse(args)

	sub := ""
	if fs.NArg() > 0 {
		sub = fs.Arg(0)
	}

	switch sub {
	case "auth", "":
		return runZerodhaAuth(*configPath, *userFlag)
	case "sync":
		return runZerodhaSync(*configPath, *userFlag)
	default:
		return fmt.Errorf("unknown zerodha subcommand %q — use: auth, sync", sub)
	}
}

// runZerodhaAuth spins up a temporary local HTTP server, prints the Kite login
// URL, and waits for the OAuth redirect. On success it stores the access token.
func runZerodhaAuth(configPath, userIdentifier string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.SetupLogger(cfg.Log, false)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	u, err := resolveUser(ctx, userStore, userIdentifier, cfg.Channel.CLI.DefaultUser)
	if err != nil {
		return err
	}

	creds, ok := cfg.Zerodha.Users[u.Username]
	if !ok {
		return fmt.Errorf("no Zerodha credentials configured for user %q — add zerodha.users.%s to config", u.Username, u.Username)
	}
	if cfg.Zerodha.ServerSecret == "" {
		return fmt.Errorf("zerodha.server_secret is not set in config")
	}

	// Start temporary local HTTP server on a random available port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	nonce := zerodha.NewNonce(u.ID.String(), cfg.Zerodha.ServerSecret)
	client := zerodha.NewClient(creds.APIKey)
	loginURL := client.LoginURL(redirectURL, nonce)

	fmt.Printf("\nOpen this URL in your browser to authenticate with Zerodha:\n\n  %s\n\nWaiting for redirect...\n", loginURL)

	tokenCh := make(chan *zerodha.TokenResponse, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		requestToken := r.URL.Query().Get("request_token")
		state := r.URL.Query().Get("state")

		if requestToken == "" || state == "" {
			http.Error(w, "missing params", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing request_token or state")
			return
		}

		verifiedUserID, err := zerodha.VerifyNonce(state, cfg.Zerodha.ServerSecret)
		if err != nil {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("nonce verification failed: %w", err)
			return
		}
		if verifiedUserID != u.ID.String() {
			http.Error(w, "user mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("nonce user %s != expected %s", verifiedUserID, u.ID)
			return
		}

		resp, err := client.ExchangeToken(r.Context(), requestToken, creds.APISecret)
		if err != nil {
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			errCh <- fmt.Errorf("exchange token: %w", err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body><h2>Authentication successful. You can close this tab.</h2></body></html>")
		tokenCh <- resp
	})

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var tokenResp *zerodha.TokenResponse
	select {
	case tokenResp = <-tokenCh:
	case err = <-errCh:
		_ = srv.Shutdown(context.Background())
		return err
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return fmt.Errorf("interrupted")
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)

	zStore := store.NewZerodhaStore(pool)
	uid, _ := uuid.Parse(u.ID.String())
	ist := time.FixedZone("IST", 5*60*60+30*60)
	now := time.Now().In(ist)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, ist)
	if err := zStore.UpsertToken(ctx, uid, tokenResp.AccessToken, midnight); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Printf("\nAuthenticated as Zerodha user %s (%s)\n", tokenResp.UserName, tokenResp.UserID)
	fmt.Println("Token saved. Run: finagent zerodha sync")

	// Optionally do an immediate sync.
	fmt.Println("\nSyncing holdings...")
	zSvc := store.NewZerodhaService(zStore, zerodha.NewClient(creds.APIKey))
	eq, mf, err := zSvc.ForceSync(ctx, uid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: sync failed: %v\n", err)
	} else {
		fmt.Printf("Synced %d equity holdings, %d mutual fund holdings.\n", eq, mf)
	}
	return nil
}

// runZerodhaSync fetches fresh holdings from Zerodha and updates the local cache.
func runZerodhaSync(configPath, userIdentifier string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.SetupLogger(cfg.Log, false)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	u, err := resolveUser(ctx, userStore, userIdentifier, cfg.Channel.CLI.DefaultUser)
	if err != nil {
		return err
	}

	creds, ok := cfg.Zerodha.Users[u.Username]
	if !ok {
		return fmt.Errorf("no Zerodha credentials configured for user %q", u.Username)
	}

	zStore := store.NewZerodhaStore(pool)
	zSvc := store.NewZerodhaService(zStore, zerodha.NewClient(creds.APIKey))

	uid, _ := uuid.Parse(u.ID.String())
	eq, mf, err := zSvc.ForceSync(ctx, uid)
	if err != nil {
		if errors.Is(err, store.ErrZerodhaTokenExpired) {
			return fmt.Errorf("Zerodha token expired or not set — run: finagent zerodha auth")
		}
		return fmt.Errorf("sync: %w", err)
	}

	fmt.Printf("Synced %d equity holdings, %d mutual fund holdings.\n", eq, mf)
	return nil
}
