package sessionstore

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// The same contract runs against the Redis registry and the in-memory one
// the EPP server's tests use, so the two cannot drift apart.

func TestMemoryRegistry(t *testing.T) {
	runContract(t, func(t *testing.T) repositories.EPPSessionRegistry { return memstore.NewEPPSessions() })
}

// TestRedisRegistry runs against a real Redis on localhost:6379. CI starts
// one and the test fails there without it; elsewhere it skips.
func TestRedisRegistry(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 14})
	if err := client.Ping(context.Background()).Err(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Redis is required in CI for the session registry tests: %v", err)
		}
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	runContract(t, func(t *testing.T) repositories.EPPSessionRegistry {
		require.NoError(t, client.FlushDB(context.Background()).Err())
		return NewRedis(client)
	})
}

var ids atomic.Int64

func newSession(ip string) *entities.EPPSession {
	return &entities.EPPSession{
		ID:          fmt.Sprintf("S%08d", ids.Add(1)),
		Node:        "node-a",
		ClientIP:    netip.MustParseAddr(ip),
		ConnectedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
}

func authed(s *entities.EPPSession, clid, login string, creds ...uuid.UUID) *entities.EPPSession {
	s.Authenticate(&entities.EPPAuthenticatedLogin{ClID: entities.ClIDType(clid), LoginID: entities.ClIDType(login), Role: entities.EPPLoginRoleTransform,
		PasswordID: firstOr(creds, uuid.New()), PinID: uuid.New()}, time.Now())
	return s
}

func firstOr(ids []uuid.UUID, def uuid.UUID) uuid.UUID {
	if len(ids) > 0 {
		return ids[0]
	}
	return def
}

const lease = time.Minute

func runContract(t *testing.T, newReg func(t *testing.T) repositories.EPPSessionRegistry) {
	ctx := context.Background()

	t.Run("source cap", func(t *testing.T) {
		reg := newReg(t)
		for i := 0; i < 2; i++ {
			require.NoError(t, reg.Open(ctx, newSession("192.0.2.1"), lease, 2))
		}
		err := reg.Open(ctx, newSession("192.0.2.1"), lease, 2)
		var limit *entities.EPPSessionLimitError
		require.ErrorAs(t, err, &limit)
		assert.Equal(t, entities.EPPSessionLimitSource, limit.Limit)
		assert.ErrorIs(t, err, entities.ErrEPPSessionLimit)
		require.NoError(t, reg.Open(ctx, newSession("192.0.2.2"), lease, 2), "another source has its own cap")
	})

	t.Run("concurrent opens cannot overshoot a cap", func(t *testing.T) {
		reg := newReg(t)
		var ok atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if reg.Open(ctx, newSession("192.0.2.50"), lease, 5) == nil {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()
		assert.EqualValues(t, 5, ok.Load())
	})

	t.Run("registrar and login caps", func(t *testing.T) {
		reg := newReg(t)
		open := func(clid, login string) error {
			s := newSession("192.0.2.1")
			require.NoError(t, reg.Open(ctx, s, lease, 100))
			return reg.Authenticate(ctx, authed(s, clid, login), lease, 3, 2)
		}
		require.NoError(t, open("rar-a", "rar-a"))
		require.NoError(t, open("rar-a", "rar-a"))
		var limit *entities.EPPSessionLimitError
		require.ErrorAs(t, open("rar-a", "rar-a"), &limit)
		assert.Equal(t, entities.EPPSessionLimitLogin, limit.Limit)

		require.NoError(t, open("rar-a", "rar-a-ro"))
		require.ErrorAs(t, open("rar-a", "rar-a-ro2"), &limit)
		assert.Equal(t, entities.EPPSessionLimitRegistrar, limit.Limit)

		require.NoError(t, open("rar-b", "rar-b"), "another registrar has its own cap")
	})

	t.Run("close frees the slot", func(t *testing.T) {
		reg := newReg(t)
		s := newSession("192.0.2.1")
		require.NoError(t, reg.Open(ctx, s, lease, 1))
		require.NoError(t, reg.Authenticate(ctx, authed(s, "rar-a", "rar-a"), lease, 1, 1))
		require.NoError(t, reg.Close(ctx, s))
		_, err := reg.Get(ctx, s.ID)
		require.ErrorIs(t, err, entities.ErrEPPSessionNotFound)

		next := newSession("192.0.2.1")
		require.NoError(t, reg.Open(ctx, next, lease, 1))
		require.NoError(t, reg.Authenticate(ctx, authed(next, "rar-a", "rar-a"), lease, 1, 1))
	})

	t.Run("an expired lease frees the slot", func(t *testing.T) {
		reg := newReg(t)
		short := 150 * time.Millisecond
		s := newSession("192.0.2.1")
		require.NoError(t, reg.Open(ctx, s, short, 1))
		require.Error(t, reg.Open(ctx, newSession("192.0.2.1"), short, 1))
		time.Sleep(2 * short)
		require.NoError(t, reg.Open(ctx, newSession("192.0.2.1"), short, 1), "a crashed node's session expires")
	})

	t.Run("renew keeps a lease alive and restores a lost one", func(t *testing.T) {
		reg := newReg(t)
		short := 200 * time.Millisecond
		s := authed(newSession("192.0.2.1"), "rar-a", "rar-a")
		require.NoError(t, reg.Open(ctx, s, short, 1))
		for i := 0; i < 3; i++ {
			time.Sleep(short / 2)
			s.Commands++
			require.NoError(t, reg.Renew(ctx, []*entities.EPPSession{s}, short))
		}
		got, err := reg.Get(ctx, s.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 3, got.Commands)
		require.Error(t, reg.Open(ctx, newSession("192.0.2.1"), short, 1), "still holds the slot")

		// After an outage the registry may have lost the session; renewing
		// puts it back.
		require.NoError(t, reg.Close(ctx, s))
		require.NoError(t, reg.Renew(ctx, []*entities.EPPSession{s}, lease))
		_, err = reg.Get(ctx, s.ID)
		require.NoError(t, err)
	})

	t.Run("list filters and pages", func(t *testing.T) {
		reg := newReg(t)
		cred := uuid.New()
		var want []string
		for i := 0; i < 5; i++ {
			s := newSession("192.0.2.1")
			require.NoError(t, reg.Open(ctx, s, lease, 100))
			if i%2 == 0 {
				require.NoError(t, reg.Authenticate(ctx, authed(s, "rar-a", "rar-a", cred), lease, 100, 100))
				want = append(want, s.ID)
			}
		}
		other := authed(newSession("192.0.2.9"), "rar-b", "rar-b")
		require.NoError(t, reg.Open(ctx, other, lease, 100))
		require.NoError(t, reg.Authenticate(ctx, other, lease, 100, 100))

		var got []string
		cursor := ""
		for {
			page, next, err := reg.List(ctx, queries.ListItemsQuery{PageSize: 2, PageCursor: cursor, Filter: queries.ListEPPSessionsFilter{ClID: "rar-a"}})
			require.NoError(t, err)
			require.LessOrEqual(t, len(page), 2)
			for _, s := range page {
				got = append(got, s.ID)
			}
			if next == "" {
				break
			}
			cursor = next
		}
		assert.Equal(t, want, got)

		byCred, _, err := reg.List(ctx, queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{CredentialID: cred}})
		require.NoError(t, err)
		assert.Len(t, byCred, 3)
		bySource, _, err := reg.List(ctx, queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{ClientIP: "192.0.2.9"}})
		require.NoError(t, err)
		require.Len(t, bySource, 1)
		assert.Equal(t, other.ID, bySource[0].ID)
		all, _, err := reg.List(ctx, queries.ListItemsQuery{})
		require.NoError(t, err)
		assert.Len(t, all, 6)
	})

	t.Run("kill requests reach every subscriber", func(t *testing.T) {
		reg := newReg(t)
		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		a, err := reg.KillRequests(subCtx)
		require.NoError(t, err)
		b, err := reg.KillRequests(subCtx)
		require.NoError(t, err)

		require.NoError(t, reg.RequestKill(ctx, "S-1", "S-2"))
		var wg sync.WaitGroup
		for _, ch := range []<-chan string{a, b} {
			wg.Add(1)
			go func(ch <-chan string) {
				defer wg.Done()
				for _, want := range []string{"S-1", "S-2"} {
					select {
					case got := <-ch:
						assert.Equal(t, want, got)
					case <-time.After(2 * time.Second):
						t.Errorf("kill request %s not delivered", want)
					}
				}
			}(ch)
		}
		wg.Wait()
	})
}
