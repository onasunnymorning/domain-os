package epp

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// errCreateQueueFull is a create that waited its longest for a slot.
var errCreateQueueFull = errors.New("no create slot within the queue wait")

// createGate admits <domain:create>s on this node (anti-dropcatch measure 4,
// docs/epp/ARCHITECTURE.md §5): at most capacity run at once, so creates
// cannot take the database pool from everything else. When every slot is
// taken, creates wait in a queue per key (a registrar, or its family) and
// a freed slot goes to the next key in turn, not the next create in line.
// So a registrar, or a family of accreditations, sending many creates at
// once waits behind itself, and one sending a few is let in at the next
// turn.
type createGate struct {
	capacity int
	mu       sync.Mutex
	inUse    int
	// queues are each key's waiters, oldest first; turns lists the keys
	// with waiters, the next to be let in first.
	queues map[string]*list.List
	turns  *list.List
	where  map[string]*list.Element
}

// createWaiter is one create waiting for a slot. granted is closed when it
// has one.
type createWaiter struct{ granted chan struct{} }

func newCreateGate(capacity int) *createGate {
	return &createGate{capacity: capacity, queues: map[string]*list.List{}, turns: list.New(), where: map[string]*list.Element{}}
}

// acquire waits for a slot for key, at most wait or until ctx is done. It
// returns how long it waited, and release, which the caller must call
// when the create is done.
func (g *createGate) acquire(ctx context.Context, key string, wait time.Duration) (release func(), waited time.Duration, err error) {
	g.mu.Lock()
	if g.inUse < g.capacity {
		g.inUse++
		g.mu.Unlock()
		return g.release, 0, nil
	}
	w := &createWaiter{granted: make(chan struct{})}
	q := g.queues[key]
	if q == nil {
		q = list.New()
		g.queues[key] = q
		g.where[key] = g.turns.PushBack(key)
	}
	el := q.PushBack(w)
	g.mu.Unlock()

	start := time.Now()
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-w.granted:
		return g.release, time.Since(start), nil
	case <-t.C:
		err = errCreateQueueFull
	case <-ctx.Done():
		err = ctx.Err()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-w.granted:
		// Granted as it gave up: the slot is its own to hand on.
		g.handOn()
	default:
		q.Remove(el)
		if q.Len() == 0 {
			g.drop(key)
		}
	}
	return nil, time.Since(start), err
}

// release frees a slot, handing it to the next key's oldest waiter if any.
func (g *createGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.handOn()
}

// handOn passes a slot in use to the next waiter, or frees it. g.mu is
// held.
func (g *createGate) handOn() {
	front := g.turns.Front()
	if front == nil {
		g.inUse--
		return
	}
	key := front.Value.(string)
	q := g.queues[key]
	w := q.Remove(q.Front()).(*createWaiter)
	if q.Len() == 0 {
		g.drop(key)
	} else {
		g.turns.MoveToBack(front) // its next waiter has the last turn
	}
	close(w.granted)
}

// drop forgets a key with no waiters. g.mu is held.
func (g *createGate) drop(key string) {
	g.turns.Remove(g.where[key])
	delete(g.where, key)
	delete(g.queues, key)
}

// inFlight is how many slots are taken, and how many creates wait.
func (g *createGate) inFlight() (running, waiting int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, q := range g.queues {
		waiting += q.Len()
	}
	return g.inUse, waiting
}

// admitCreate runs handler for a <domain:create> once the create gate lets
// it in, the registrar's family's turn if it is in one, its own if not. A
// create that gets no slot within EPP_CREATE_QUEUE_WAIT is 2400, with an
// <extValue> reason, and the session continues.
func (s *Server) admitCreate(handler func(context.Context, *command)) func(context.Context, *command) {
	return func(ctx context.Context, cmd *command) {
		key := "reg:" + cmd.session.registrar()
		if f := cmd.session.familyName(); f != "" {
			key = "fam:" + f
		}
		release, waited, err := s.creates.acquire(ctx, key, s.cfg.Commands.CreateQueueWait)
		if waited > 0 {
			s.metrics.createQueueWait.Observe(waited.Seconds())
		}
		if err != nil {
			if errors.Is(err, errCreateQueueFull) {
				s.metrics.createQueueTimeouts.Inc()
			}
			cmd.respond(2400, "Command failed; the registry is busy with creates, try again", nil)
			addReason(cmd, fmt.Sprintf("create-queue; waited-ms=%d", waited.Milliseconds()))
			return
		}
		defer release()
		handler(ctx, cmd)
	}
}
