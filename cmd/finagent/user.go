package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/apikey"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/store"
)

func runUser(args []string) error {
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	_ = fs.Parse(args)

	sub := ""
	if fs.NArg() > 0 {
		sub = fs.Arg(0)
	}

	switch sub {
	case "add", "":
		return runUserAdd(*configPath)
	case "list":
		return runUserList(*configPath)
	default:
		return fmt.Errorf("unknown user subcommand %q — use: add, list", sub)
	}
}

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func runUserAdd(configPath string) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n=== Add User ===")
	fmt.Println()

	name := prompt(scanner, "Full name", "")
	if name == "" {
		return fmt.Errorf("name is required")
	}

	suggested := slugify(name)
	username := prompt(scanner, "Username", suggested)
	if username == "" {
		return fmt.Errorf("username is required")
	}

	email := prompt(scanner, "Email", "")
	phone := prompt(scanner, "Phone (e.g. +919876543210, leave blank to skip)", "")
	timezone := prompt(scanner, "Timezone", "Asia/Kolkata")

	// Generate API key.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate api key: %w", err)
	}
	apiKey := hex.EncodeToString(raw)
	keyHash, err := apikey.Hash(apiKey)
	if err != nil {
		return fmt.Errorf("hash api key: %w", err)
	}

	u, err := userStore.Create(ctx, store.CreateUserParams{
		Username:     username,
		Name:         name,
		Email:        email,
		Phone:        phone,
		Timezone:     timezone,
		APIKeyPrefix: apikey.Prefix(apiKey),
		APIKeyHash:   keyHash,
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Printf("\nCreated user: %s\n", u.ID)
	fmt.Printf("Username:     %s\n", u.Username)
	fmt.Printf("Name:         %s\n", u.Name)
	if u.Email != nil {
		fmt.Printf("Email:        %s\n", *u.Email)
	}

	fmt.Println()
	fmt.Println("API Key (save this — it will not be shown again):")
	fmt.Printf("  %s\n", apiKey)
	fmt.Println()
	fmt.Printf("Use as:  Authorization: Bearer %s\n\n", apiKey)

	// Optionally set DOB.
	dobStr := prompt(scanner, "Date of birth (DD/MM/YYYY, for SBI password derivation — leave blank to skip)", "")
	if dobStr != "" {
		dob, err := time.Parse("02/01/2006", dobStr)
		if err != nil {
			return fmt.Errorf("invalid date %q — use DD/MM/YYYY: %w", dobStr, err)
		}
		u, err = userStore.UpdateDOB(ctx, u.ID.String(), dob)
		if err != nil {
			return fmt.Errorf("update dob: %w", err)
		}
		fmt.Printf("DOB saved: %s\n\n", u.DateOfBirth.Time.Format("02 Jan 2006"))
	}

	return nil
}

func runUserList(configPath string) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	users, err := userStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	fmt.Println("\nUsers:")
	fmt.Println()
	fmt.Printf("  %-20s  %-30s  %-30s  %-10s  %s\n", "USERNAME", "NAME", "EMAIL", "API KEY", "ID")
	for _, u := range users {
		email := "—"
		if u.Email != nil {
			email = *u.Email
		}
		apiKeySet := "set"
		if len(u.ApiKeyHash) == 0 {
			apiKeySet = "—"
		}
		fmt.Printf("  %-20s  %-30s  %-30s  %-10s  %s\n", u.Username, u.Name, email, apiKeySet, u.ID)
	}
	fmt.Println()
	return nil
}
