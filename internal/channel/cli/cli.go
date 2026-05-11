// Package cli implements a readline-based CLI channel.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/chzyer/readline"
	"github.com/google/uuid"
	"golang.org/x/term"

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

		stopSpinner := startSpinner()
		resp, err := handler(ctx, msg)
		stopSpinner()
		if err != nil {
			slog.Error("agent error", "err", err)
			fmt.Printf("error: %v\n\n", err)
			continue
		}

		fmt.Printf("\nagent> %s\n\n", resp.Text)
	}
}

// startSpinner prints an animated "Thinking..." indicator while the agent is
// working. It returns a stop function that clears the line. When stdout is not
// a TTY (e.g. piped output) it is a no-op.
func startSpinner() func() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return func() {}
	}

	done := make(chan struct{})
	frames := []string{"|", "/", "-", "\\"}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-done:
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				fmt.Printf("\r%s Thinking...", frames[i%len(frames)])
				i++
			}
		}
	}()

	return func() { close(done) }
}
