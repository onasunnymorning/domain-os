package epp

import (
	"context"
	"strings"
	"sync"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// fakeDomains stands in for the registry's domain service in adapter
// tests: names under .test are served, registered names are in use, and
// reserved names are reserved. services.EPPDomainService is tested against
// the repositories it reads.
type fakeDomains struct {
	mu         sync.Mutex
	registered map[string]bool
	reserved   map[string]bool
	err        error
	asked      [][]string
}

func newFakeDomains() *fakeDomains {
	return &fakeDomains{
		registered: map[string]bool{"registered.test": true},
		reserved:   map[string]bool{"reserved.test": true},
	}
}

func (f *fakeDomains) Check(_ context.Context, names []string) ([]*queries.DomainCheckResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, names)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]*queries.DomainCheckResult, 0, len(names))
	for _, n := range names {
		r := &queries.DomainCheckResult{DomainName: n}
		dn, err := entities.NewDomainName(n)
		switch {
		case err != nil || !strings.Contains(dn.String(), "."):
			r.Reason = services.CheckReasonInvalid
		case dn.ParentDomain() != "test":
			r.Reason = services.CheckReasonNotServed
		case f.registered[dn.String()]:
			r.Reason = services.CheckReasonInUse
		case f.reserved[dn.String()]:
			r.Reason = services.CheckReasonReserved
		default:
			r.Available = true
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeDomains) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeDomains) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func (f *fakeDomains) call(i int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[i]
}
