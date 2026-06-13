package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pushkar-anand/agentrig/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// stubAgent returns a minimal *Agent whose HandleMessage can be called
// without real dependencies — all fields nil except channelType.
func minimalAgent() *Agent {
	return &Agent{channelType: sqlcgen.ChannelEnumCli}
}


func TestPool_LazyCreation(t *testing.T) {
	var calls atomic.Int64
	pool := NewPool(func(_ context.Context, _ string) (*Agent, string, error) {
		calls.Add(1)
		return minimalAgent(), "uid", nil
	})
	assert.Equal(t, int64(0), calls.Load(), "factory must not be called before any message")
	_ = pool
}

func TestPool_SeparateAgentsPerUser(t *testing.T) {
	var callsMu sync.Mutex
	created := map[string]int{}

	pool := NewPool(func(_ context.Context, userID string) (*Agent, string, error) {
		callsMu.Lock()
		created[userID]++
		callsMu.Unlock()
		return minimalAgent(), userID + "-uid", nil
	})

	ctx := t.Context()
	e1, err := pool.get(ctx, "user-a")
	require.NoError(t, err)
	e2, err := pool.get(ctx, "user-b")
	require.NoError(t, err)
	e3, err := pool.get(ctx, "user-a")
	require.NoError(t, err)

	assert.NotSame(t, e1.ag, e2.ag, "different users must get separate agents")
	assert.Same(t, e1.ag, e3.ag, "same user must reuse the cached agent")
	assert.Equal(t, "user-a-uid", e1.uid)
	assert.Equal(t, "user-b-uid", e2.uid)

	callsMu.Lock()
	defer callsMu.Unlock()
	assert.Equal(t, 1, created["user-a"], "factory called once for user-a")
	assert.Equal(t, 1, created["user-b"], "factory called once for user-b")
}

func TestPool_FactoryErrorPropagates(t *testing.T) {
	wantErr := errors.New("db connection lost")
	pool := NewPool(func(_ context.Context, _ string) (*Agent, string, error) {
		return nil, "", wantErr
	})

	_, err := pool.HandleMessage(t.Context(), channel.Message{UserID: "any-user", Text: "hello"})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPool_UIDRewrite(t *testing.T) {
	// HandleMessage must rewrite msg.UserID to the canonical DB UUID before
	// forwarding to the agent, so the agent never sees the raw channel identifier.
	pool := NewPool(func(_ context.Context, _ string) (*Agent, string, error) {
		return minimalAgent(), "real-uuid-123", nil
	})

	entry, err := pool.get(t.Context(), "alice")
	require.NoError(t, err)
	assert.Equal(t, "real-uuid-123", entry.uid, "pool must store the resolved UUID")
}

func TestPool_ConcurrentAccess(t *testing.T) {
	// N goroutines for N distinct users: factory called exactly once per user,
	// and different users' constructions run in parallel (total wall time < n×delay).
	const n = 5
	const delay = 20 * time.Millisecond
	var callCount atomic.Int64

	pool := NewPool(func(_ context.Context, userID string) (*Agent, string, error) {
		callCount.Add(1)
		time.Sleep(delay) // simulate DB lookup + registry build
		return minimalAgent(), userID + "-uid", nil
	})

	start := time.Now()
	var wg sync.WaitGroup
	for i := range n {
		userID := fmt.Sprintf("user-%d", i)
		wg.Go(func() {
			_, _ = pool.get(t.Context(), userID)
		})
	}
	wg.Wait()
	elapsed := time.Since(start)

	assert.Equal(t, int64(n), callCount.Load(), "factory must be called exactly once per unique user")
	// If constructions were serialised the total would be ≥ n×delay.
	// With singleflight they run in parallel so elapsed ≪ n×delay.
	assert.Less(t, elapsed, time.Duration(n)*delay, "different users' agent construction must run in parallel")
}

func TestPool_CancelledCallerContextDoesNotAbortConstruction(t *testing.T) {
	// If the caller's context is cancelled, the factory must still succeed because
	// the pool passes context.WithoutCancel(ctx). Without that guard, a cancelled
	// context would propagate into DB calls inside newAgent and fail all waiters.
	pool := NewPool(func(ctx context.Context, _ string) (*Agent, string, error) {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return minimalAgent(), "uid", nil
	})

	cancelCtx, cancel := context.WithCancel(t.Context())
	cancel() // cancel before calling get

	e, err := pool.get(cancelCtx, "alice")
	require.NoError(t, err, "cancelled caller context must not abort agent construction")
	require.NotNil(t, e.ag)
}

func TestPool_ConcurrentSameUser(t *testing.T) {
	// N goroutines all request the agent for the same user simultaneously.
	// The factory must be called exactly once regardless of the race.
	const n = 20
	var callCount atomic.Int64

	pool := NewPool(func(_ context.Context, _ string) (*Agent, string, error) {
		callCount.Add(1)
		return minimalAgent(), "uid", nil
	})

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			_, _ = pool.get(t.Context(), "alice")
		})
	}
	wg.Wait()

	assert.Equal(t, int64(1), callCount.Load(), "factory must be called exactly once for the same user")
}
