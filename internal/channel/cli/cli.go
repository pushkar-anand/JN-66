// Package cli implements a readline-based CLI channel.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/chzyer/readline"
	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/internal/channel"
)

// CLI is a readline-backed interactive channel. UserID is resolved from the
// --user flag at startup and fixed for the lifetime of the session.
type CLI struct {
	userID    string
	sessionID string
}

// New creates a CLI channel for the given user.
func New(userID string) *CLI {
	return &CLI{
		userID:    userID,
		sessionID: uuid.New().String(),
	}
}

// Name returns "cli".
func (c *CLI) Name() string { return "cli" }

// Start begins the interactive REPL. It blocks until ctx is cancelled or the
// user sends EOF (Ctrl-D).
func (c *CLI) Start(ctx context.Context, handler channel.MessageHandler) error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "you> ",
		HistoryFile: "/tmp/finagent_history",
	})
	if err != nil {
		return fmt.Errorf("init readline: %w", err)
	}
	defer rl.Close()

	fmt.Println("finagent ready. Type your question or Ctrl-D to exit.")
	fmt.Println()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := rl.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			continue
		}
		if errors.Is(err, io.EOF) {
			fmt.Println("\nGoodbye.")
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline: %w", err)
		}
		if line == "" {
			continue
		}

		msg := channel.Message{
			ID:        uuid.New().String(),
			SessionID: c.sessionID,
			UserID:    c.userID,
			Text:      line,
			Timestamp: time.Now(),
		}

		resp, err := handler(ctx, msg)
		if err != nil {
			slog.Error("agent error", "err", err)
			fmt.Printf("error: %v\n\n", err)
			continue
		}

		fmt.Printf("\nagent> %s\n\n", resp.Text)
	}
}
