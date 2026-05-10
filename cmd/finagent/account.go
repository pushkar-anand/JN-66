package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/db"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

func runAccount(args []string) error {
	fs := flag.NewFlagSet("account", flag.ExitOnError)
	configPath := fs.String("config", "config/config.yaml", "path to config file")
	userFlag := fs.String("user", "", "user email (optional if only one user in DB)")
	_ = fs.Parse(args)

	sub := ""
	if fs.NArg() > 0 {
		sub = fs.Arg(0)
	}

	switch sub {
	case "add", "":
		return runAccountAdd(*configPath, *userFlag)
	case "list":
		return runAccountList(*configPath, *userFlag)
	default:
		return fmt.Errorf("unknown account subcommand %q — use: add, list", sub)
	}
}

func runAccountAdd(configPath, userEmail string) error {
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
	accountStore := store.NewAccountStore(pool)

	u, err := resolveImportUser(ctx, userStore, userEmail, cfg.Channel.CLI.DefaultUser)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n=== Add Account ===")
	fmt.Printf("User: %s (%s)\n\n", u.Name, u.Email)

	// Optionally set DOB if not already set.
	if !u.DateOfBirth.Valid {
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
			fmt.Printf("  DOB saved for %s\n\n", u.Name)
		}
	}

	institution := prompt(scanner, "Bank/institution (hdfc/icici/sbi/axis/idfc/other)", "")
	if institution == "" {
		return fmt.Errorf("institution is required")
	}

	accountTypeName := prompt(scanner, "Account type (savings/current/salary/credit_card/loan/wallet/fd/ppf)", "savings")
	accountType, err := parseAccountType(accountTypeName)
	if err != nil {
		return err
	}

	nickname := prompt(scanner, fmt.Sprintf(`Account nickname (e.g. "%s Savings ****1234")`, strings.ToUpper(institution)), "")
	if nickname == "" {
		return fmt.Errorf("nickname is required")
	}

	p := store.CreateAccountParams{
		Institution: institution,
		Name:        nickname,
		AccountType: accountType,
		Currency:    "INR",
		IsActive:    true,
	}

	acc, err := accountStore.Create(ctx, p, u.ID.String())
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}

	fmt.Printf("\nCreated account: %s\n", acc.ID)
	fmt.Printf("Name:            %s\n", acc.Name)
	fmt.Printf("Type:            %s\n", acc.AccountType)
	fmt.Printf("Class:           %s\n", acc.AccountClass)
	fmt.Printf("Owner:           %s\n\n", u.Email)

	return nil
}

func runAccountList(configPath, userEmail string) error {
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
	accountStore := store.NewAccountStore(pool)

	u, err := resolveImportUser(ctx, userStore, userEmail, cfg.Channel.CLI.DefaultUser)
	if err != nil {
		return err
	}

	accounts, err := accountStore.ListByUser(ctx, u.ID.String())
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}

	fmt.Printf("\nAccounts for %s:\n\n", u.Name)
	for _, a := range accounts {
		fmt.Printf("  %-40s  %-12s  %s  %s\n", a.Name, a.AccountType, a.AccountClass, a.ID)
	}
	fmt.Println()
	return nil
}

func prompt(scanner *bufio.Scanner, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	if !scanner.Scan() {
		return defaultVal
	}
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return defaultVal
	}
	return v
}

func parseAccountType(s string) (sqlcgen.AccountTypeEnum, error) {
	m := map[string]sqlcgen.AccountTypeEnum{
		"savings":     sqlcgen.AccountTypeEnumBankSavings,
		"current":     sqlcgen.AccountTypeEnumBankCurrent,
		"salary":      sqlcgen.AccountTypeEnumBankSalary,
		"credit_card": sqlcgen.AccountTypeEnumCreditCard,
		"loan":        sqlcgen.AccountTypeEnumLoan,
		"wallet":      sqlcgen.AccountTypeEnumWallet,
		"fd":          sqlcgen.AccountTypeEnumFd,
		"ppf":         sqlcgen.AccountTypeEnumPpf,
	}
	t, ok := m[strings.ToLower(s)]
	if !ok {
		return "", fmt.Errorf("unknown account type %q — valid: savings, current, salary, credit_card, loan, wallet, fd, ppf", s)
	}
	return t, nil
}
