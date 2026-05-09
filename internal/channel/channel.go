// Package channel defines the transport-agnostic messaging interfaces.
// Each concrete channel (CLI, Slack, API) implements Channel and calls the
// MessageHandler provided by the agent layer.
package channel

import (
	"context"
	"time"
)

// Message is an inbound message from a user.
type Message struct {
	// ID is a channel-assigned unique identifier for this message.
	ID string
	// SessionID identifies the ongoing conversation (may be empty on first turn).
	SessionID string
	// UserID is the resolved user who sent the message.
	UserID string
	// Text is the raw message content.
	Text      string
	Timestamp time.Time
}

// Response is the agent's reply to a Message.
type Response struct {
	Text     string
	Markdown bool // whether Text contains Markdown formatting
}

// MessageHandler is the function the agent layer exposes to channels.
type MessageHandler func(ctx context.Context, msg Message) (Response, error)

// Channel is the interface every transport must implement.
type Channel interface {
	// Start begins listening for messages and dispatches them to handler.
	// It blocks until ctx is cancelled.
	Start(ctx context.Context, handler MessageHandler) error
	// Name returns a short identifier for logging (e.g. "cli", "slack").
	Name() string
}
