package epp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitFor polls until n creates are waiting at the gate.
func waitFor(t *testing.T, g *createGate, n int) {
	t.Helper()
	require.Eventually(t, func() bool { _, w := g.inFlight(); return w == n }, time.Second, time.Millisecond)
}

// A freed slot goes to the next key in turn: a key with many creates
// waiting waits behind itself.
func TestCreateGateTakesTurns(t *testing.T) {
	g := newCreateGate(1)
	ctx := context.Background()
	hold, _, err := g.acquire(ctx, "busy", time.Second)
	require.NoError(t, err)

	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	queue := func(key, name string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, _, err := g.acquire(ctx, key, 5*time.Second)
			if !assert.NoError(t, err) {
				return
			}
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			release()
		}()
	}
	queue("busy", "busy-1")
	waitFor(t, g, 1)
	queue("busy", "busy-2")
	waitFor(t, g, 2)
	queue("busy", "busy-3")
	waitFor(t, g, 3)
	queue("quiet", "quiet-1")
	waitFor(t, g, 4)

	hold()
	wg.Wait()
	assert.Equal(t, []string{"busy-1", "quiet-1", "busy-2", "busy-3"}, order, "quiet is let in at the next turn, not after every busy create")
	running, waiting := g.inFlight()
	assert.Zero(t, running)
	assert.Zero(t, waiting)
}

func TestCreateGateCapacity(t *testing.T) {
	g := newCreateGate(2)
	ctx := context.Background()
	r1, _, err := g.acquire(ctx, "a", time.Second)
	require.NoError(t, err)
	r2, _, err := g.acquire(ctx, "a", time.Second)
	require.NoError(t, err)
	running, _ := g.inFlight()
	assert.Equal(t, 2, running)

	start := time.Now()
	_, _, err = g.acquire(ctx, "b", 30*time.Millisecond)
	assert.ErrorIs(t, err, errCreateQueueFull)
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond)
	_, waiting := g.inFlight()
	assert.Zero(t, waiting, "a create that gave up leaves the queue")

	cctx, cancel := context.WithCancel(ctx)
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	_, _, err = g.acquire(cctx, "b", time.Second)
	assert.True(t, errors.Is(err, context.Canceled))

	r1()
	r3, waited, err := g.acquire(ctx, "b", time.Second)
	require.NoError(t, err)
	assert.Zero(t, waited)
	r2()
	r3()
	running, _ = g.inFlight()
	assert.Zero(t, running)
}
