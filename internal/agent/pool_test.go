package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

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
	pool := NewPool(func(_ context.Context, _ string) (*Agent, error) {
		calls.Add(1)
		return minimalAgent(), nil
	})
	assert.Equal(t, int64(0), calls.Load(), "factory must not be called before any message")
	_ = pool
}

func TestPool_SeparateAgentsPerUser(t *testing.T) {
	var callsMu sync.Mutex
	created := map[string]int{}

	pool := NewPool(func(_ context.Context, userID string) (*Agent, error) {
		callsMu.Lock()
		created[userID]++
		callsMu.Unlock()
		return minimalAgent(), nil
	})

	ctx := t.Context()
	ag1, err := pool.get(ctx, "user-a")
	require.NoError(t, err)
	ag2, err := pool.get(ctx, "user-b")
	require.NoError(t, err)
	ag3, err := pool.get(ctx, "user-a")
	require.NoError(t, err)

	assert.NotSame(t, ag1, ag2, "different users must get separate agents")
	assert.Same(t, ag1, ag3, "same user must reuse the cached agent")

	callsMu.Lock()
	defer callsMu.Unlock()
	assert.Equal(t, 1, created["user-a"], "factory called once for user-a")
	assert.Equal(t, 1, created["user-b"], "factory called once for user-b")
}

func TestPool_FactoryErrorPropagates(t *testing.T) {
	wantErr := errors.New("db connection lost")
	pool := NewPool(func(_ context.Context, _ string) (*Agent, error) {
		return nil, wantErr
	})

	_, err := pool.HandleMessage(t.Context(), channel.Message{UserID: "any-user", Text: "hello"})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPool_ConcurrentAccess(t *testing.T) {
	const n = 20
	var callCount atomic.Int64

	pool := NewPool(func(_ context.Context, _ string) (*Agent, error) {
		callCount.Add(1)
		return minimalAgent(), nil
	})

	var wg sync.WaitGroup
	for i := range n {
		userID := fmt.Sprintf("user-%d", i)
		wg.Go(func() {
			_, _ = pool.get(t.Context(), userID)
		})
	}
	wg.Wait()

	assert.Equal(t, int64(n), callCount.Load(), "factory must be called exactly once per unique user")
}
