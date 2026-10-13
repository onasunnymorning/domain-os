package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// The check service reads four repositories; these stubs implement only the
// methods it calls, and panic (through the nil embedded interface) on any
// other.

type checkDomains struct {
	repositories.DomainRepository
	registered map[string]bool
	err        error
	inFlight   atomic.Int32
	maxSeen    atomic.Int32
}

func (r *checkDomains) GetDomainByName(_ context.Context, name string, _ bool) (*entities.Domain, error) {
	n := r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	for {
		m := r.maxSeen.Load()
		if n <= m || r.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(2 * time.Millisecond) // long enough for lookups to overlap
	if r.err != nil {
		return nil, r.err
	}
	if r.registered[name] {
		return &entities.Domain{}, nil
	}
	return nil, entities.ErrDomainNotFound
}

type checkNNDNs struct {
	repositories.NNDNRepository
	reserved map[string]bool
}

func (r *checkNNDNs) GetNNDN(_ context.Context, name string) (*entities.NNDN, error) {
	if r.reserved[name] {
		return &entities.NNDN{}, nil
	}
	return nil, entities.ErrNNDNNotFound
}

type checkTLDs struct {
	repositories.TLDRepository
	known map[string]bool
}

func (r *checkTLDs) GetByName(_ context.Context, name string, _ bool) (*entities.TLD, error) {
	if r.known[name] {
		return &entities.TLD{}, nil
	}
	return nil, entities.ErrTLDNotFound
}

type checkPhases struct {
	repositories.PhaseRepository
	mu     sync.Mutex
	byTLD  map[string][]*entities.Phase
	listed map[string]int
}

// ListPhasesByTLD pages by ID, oldest first, as the Postgres repository does.
func (r *checkPhases) ListPhasesByTLD(_ context.Context, tld string, pageSize int, cursor string) ([]*entities.Phase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cursor == "" {
		r.listed[tld]++
	}
	var after int64
	if cursor != "" {
		var err error
		if after, err = strconv.ParseInt(cursor, 10, 64); err != nil {
			return nil, err
		}
	}
	var page []*entities.Phase
	for _, p := range r.byTLD[tld] {
		if p.ID > after && len(page) < pageSize {
			page = append(page, p)
		}
	}
	return page, nil
}

func gaPhase(t *testing.T, tld string, starts time.Time, minLabel int) *entities.Phase {
	t.Helper()
	p, err := entities.NewPhase("GA-"+tld, string(entities.PhaseTypeGA), starts.UTC())
	require.NoError(t, err)
	p.TLDName = entities.DomainName(tld)
	p.ID = 1
	p.Policy.MinLabelLength = minLabel
	return p
}

func newCheckService(t *testing.T) (*EPPDomainService, *checkDomains, *checkPhases) {
	t.Helper()
	domains := &checkDomains{registered: map[string]bool{"taken.test": true}}
	phases := &checkPhases{listed: map[string]int{}, byTLD: map[string][]*entities.Phase{
		"test":   {gaPhase(t, "test", time.Now().Add(-time.Hour), 3)},
		"future": {gaPhase(t, "future", time.Now().Add(24*time.Hour), 1)},
	}}
	svc := NewEPPDomainService(domains,
		&checkNNDNs{reserved: map[string]bool{"held.test": true}},
		&checkTLDs{known: map[string]bool{"test": true, "future": true, "closed": true}},
		phases, 4)
	return svc, domains, phases
}

func TestEPPDomainCheck(t *testing.T) {
	svc, _, phases := newCheckService(t)
	names := []string{"Free.TEST", "taken.test", "held.test", "ab.test", "x.future", "x.closed", "x.nowhere", "not a name", "single", "taken.test"}
	res, err := svc.Check(context.Background(), names)
	require.NoError(t, err)
	require.Len(t, res, len(names))

	want := []struct {
		avail  bool
		reason string
	}{
		{true, ""},
		{false, CheckReasonInUse},
		{false, CheckReasonReserved},
		{false, CheckReasonPhasePolicy}, // two characters, under the phase's minimum of three
		{false, CheckReasonNotOpen},     // GA starts tomorrow
		{false, CheckReasonNotOpen},     // served, but no phases at all
		{false, CheckReasonNotServed},
		{false, CheckReasonInvalid},
		{false, CheckReasonInvalid},
		{false, CheckReasonInUse}, // a duplicate is answered again
	}
	for i, w := range want {
		assert.Equal(t, names[i], res[i].DomainName, "names are echoed as asked")
		assert.Equal(t, w.avail, res[i].Available, names[i])
		assert.Equal(t, w.reason, res[i].Reason, names[i])
		assert.LessOrEqual(t, len(res[i].Reason), 32, "RFC 5730 bounds <reason> to 32 characters")
	}
	assert.Equal(t, "GA-test", res[0].PhaseName)
	assert.Equal(t, 1, phases.listed["test"], "each TLD's phases are read once per check, however many names")
}

func TestEPPDomainCheckBoundsParallelism(t *testing.T) {
	svc, domains, _ := newCheckService(t)
	names := make([]string, 50)
	for i := range names {
		names[i] = "name" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".test"
	}
	_, err := svc.Check(context.Background(), names)
	require.NoError(t, err)
	assert.LessOrEqual(t, domains.maxSeen.Load(), int32(4), "no more lookups at once than the configured parallelism")
	assert.Greater(t, domains.maxSeen.Load(), int32(1), "lookups do run in parallel")
}

func TestEPPDomainCheckFailsWhole(t *testing.T) {
	svc, domains, _ := newCheckService(t)
	domains.err = errors.New("connection reset")
	res, err := svc.Check(context.Background(), []string{"one1.test", "two2.test", "three3.test"})
	require.ErrorContains(t, err, "connection reset")
	assert.Nil(t, res, "no partial answer")
}

func TestEPPDomainCheckFindsTheGAPhaseOnALaterPage(t *testing.T) {
	svc, _, phases := newCheckService(t)
	var many []*entities.Phase
	for i := range 150 {
		// 149 launch phases that ended long ago, then the GA phase in force.
		p, err := entities.NewPhase(fmt.Sprintf("old-%d", i), string(entities.PhaseTypeLaunch), time.Now().Add(-1000*time.Hour).UTC())
		require.NoError(t, err)
		ended := time.Now().Add(-900 * time.Hour).UTC()
		p.Ends = &ended
		if i == 149 {
			p = gaPhase(t, "paged", time.Now().Add(-time.Hour), 1)
		}
		p.ID = int64(i + 1)
		many = append(many, p)
	}
	phases.byTLD["paged"] = many
	res, err := svc.Check(context.Background(), []string{"open.paged"})
	require.NoError(t, err)
	assert.True(t, res[0].Available, "the current GA phase is phase 150, past the first page of 100")
}
