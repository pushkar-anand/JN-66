package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/db"
	"github.com/pushkaranand/finagent/internal/store"
)

func runUser(args []string) error {
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	configPath := fs.String("config", "config/config.yaml", "path to config file")
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

	email := prompt(scanner, "Email", "")
	if email == "" {
		return fmt.Errorf("email is required")
	}

	phone := prompt(scanner, "Phone (e.g. +919876543210, leave blank to skip)", "")
	timezone := prompt(scanner, "Timezone", "Asia/Kolkata")

	u, err := userStore.Create(ctx, store.CreateUserParams{
		Name:     name,
		Email:    email,
		Phone:    phone,
		Timezone: timezone,
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Printf("\nCreated user: %s\n", u.ID)
	fmt.Printf("Name:         %s\n", u.Name)
	fmt.Printf("Email:        %s\n", u.Email)
	fmt.Printf("Timezone:     %s\n\n", u.Timezone)

	// Optionally set DOB (needed for SBI password derivation).
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
	for _, u := range users {
		dob := "—"
		if u.DateOfBirth.Valid {
			dob = u.DateOfBirth.Time.Format("02 Jan 2006")
		}
		fmt.Printf("  %-30s  %-30s  %s  %s\n", u.Name, u.Email, dob, u.ID)
	}
	fmt.Println()
	return nil
}
