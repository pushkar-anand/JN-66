package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/pushkar-anand/agentrig/channel"
)

// Pool manages per-user Agent instances for channels where the sender is
// determined per-message (e.g. Matrix DMs with multiple household members).
// Each userID gets its own Agent (and tool registry) built lazily on first use.
type Pool struct {
	mu       sync.Mutex
	agents   map[string]*Agent
	newAgent func(ctx context.Context, userID string) (*Agent, error)
}

// NewPool creates a Pool. newAgent is called at most once per unique userID
// to build the agent; subsequent messages from the same user reuse the cached agent.
func NewPool(newAgent func(ctx context.Context, userID string) (*Agent, error)) *Pool {
	return &Pool{
		agents:   make(map[string]*Agent),
		newAgent: newAgent,
	}
}

// HandleMessage satisfies channel.MessageHandler. It routes msg to the
// per-user Agent, creating it on first use via the factory passed to NewPool.
func (p *Pool) HandleMessage(ctx context.Context, msg channel.Message) (channel.Response, error) {
	ag, err := p.get(ctx, msg.UserID)
	if err != nil {
		return channel.Response{}, fmt.Errorf("build agent for user %s: %w", msg.UserID, err)
	}
	return ag.HandleMessage(ctx, msg)
}

func (p *Pool) get(ctx context.Context, userID string) (*Agent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ag, ok := p.agents[userID]; ok {
		return ag, nil
	}
	ag, err := p.newAgent(ctx, userID)
	if err != nil {
		return nil, err
	}
	p.agents[userID] = ag
	return ag, nil
}
