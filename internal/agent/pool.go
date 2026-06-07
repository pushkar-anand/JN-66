package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/pushkar-anand/agentrig/channel"
	"golang.org/x/sync/singleflight"
)

// Pool manages per-user Agent instances for channels where the sender is
// determined per-message (e.g. Matrix DMs with multiple household members).
// Each userID gets its own Agent (and tool registry) built lazily on first use.
//
// The mutex is held only for cache reads and writes — never during construction.
// singleflight deduplicates concurrent build calls for the same userID so the
// factory runs at most once per user, while different users build in parallel.
type Pool struct {
	mu       sync.RWMutex
	agents   map[string]*Agent
	sf       singleflight.Group
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
	// Fast path: already built.
	p.mu.RLock()
	ag, ok := p.agents[userID]
	p.mu.RUnlock()
	if ok {
		return ag, nil
	}

	// Slow path: build once per userID; concurrent callers for the same user
	// share the result via singleflight without blocking unrelated users.
	v, err, _ := p.sf.Do(userID, func() (any, error) {
		// Re-check inside singleflight — a racing goroutine may have just built it.
		p.mu.RLock()
		ag, ok := p.agents[userID]
		p.mu.RUnlock()
		if ok {
			return ag, nil
		}

		ag, err := p.newAgent(ctx, userID)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.agents[userID] = ag
		p.mu.Unlock()
		return ag, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Agent), nil //nolint:forcetypeassert
}
