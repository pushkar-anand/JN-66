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
	entries  map[string]*poolEntry
	sf       singleflight.Group
	newAgent func(ctx context.Context, userID string) (*Agent, string, error)
}

// poolEntry pairs an Agent with the canonical DB UUID for the user.
// The key used to look up entries is the channel-level user identifier
// (e.g. a Matrix user ID or username), which may differ from the DB UUID.
type poolEntry struct {
	ag  *Agent
	uid string // DB UUID — used to rewrite channel.Message.UserID
}

// NewPool creates a Pool. newAgent is called at most once per unique userID
// to build the agent; it must return the agent, the canonical DB UUID for the
// user, and any error. Subsequent messages from the same user reuse the cache.
func NewPool(newAgent func(ctx context.Context, userID string) (*Agent, string, error)) *Pool {
	return &Pool{
		entries:  make(map[string]*poolEntry),
		newAgent: newAgent,
	}
}

// HandleMessage satisfies channel.MessageHandler. It routes msg to the
// per-user Agent, creating it on first use via the factory passed to NewPool.
// msg.UserID is rewritten to the canonical DB UUID before forwarding.
func (p *Pool) HandleMessage(ctx context.Context, msg channel.Message) (channel.Response, error) {
	entry, err := p.get(ctx, msg.UserID)
	if err != nil {
		return channel.Response{}, fmt.Errorf("build agent for user %s: %w", msg.UserID, err)
	}
	msg.UserID = entry.uid
	return entry.ag.HandleMessage(ctx, msg)
}

func (p *Pool) get(ctx context.Context, userID string) (*poolEntry, error) {
	// Fast path: already built.
	p.mu.RLock()
	e, ok := p.entries[userID]
	p.mu.RUnlock()
	if ok {
		return e, nil
	}

	// Slow path: build once per userID; concurrent callers for the same user
	// share the result via singleflight without blocking unrelated users.
	v, err, _ := p.sf.Do(userID, func() (any, error) {
		// Re-check inside singleflight — a racing goroutine may have just built it.
		p.mu.RLock()
		e, ok := p.entries[userID]
		p.mu.RUnlock()
		if ok {
			return e, nil
		}

		// Use context.WithoutCancel so that a single caller's timeout or
		// cancellation does not abort construction for all concurrent waiters
		// sharing this singleflight call.
		ag, uid, err := p.newAgent(context.WithoutCancel(ctx), userID)
		if err != nil {
			return nil, err
		}

		e = &poolEntry{ag: ag, uid: uid}
		p.mu.Lock()
		p.entries[userID] = e
		p.mu.Unlock()
		return e, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*poolEntry), nil //nolint:forcetypeassert
}
