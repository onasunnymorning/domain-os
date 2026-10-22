package epp

import (
	"context"
	"sync"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// testDomains is the registry behind adapter tests: the real
// services.EPPDomainService over an in-memory registry, recording what
// each <check> asked. It is seeded in RST's shape (ADR 0015):
//   - .test is served and open, and both test registrars are accredited
//     for it; .closed is served but has no GA phase; .example is not served.
//   - registered.test stands in for epp.registeredNames; reserved.test is
//     reserved.
//   - each registrar has contacts (<clid>-c1, <clid>-c2) and host objects
//     (ns1.<clid>.net, ns2.<clid>.net).
type testDomains struct {
	*services.EPPDomainService
	registry *memstore.Registry

	mu    sync.Mutex
	asked [][]string
	err   error
}

// testRepositoryID is the repository ID the test registry mints ROIDs with,
// as a deployment's ROID_REPOSITORY_ID.
const testRepositoryID = entities.RepositoryID("EPPTEST")

func newTestDomains() *testDomains {
	reg := memstore.NewRegistry(testRepositoryID)
	reg.AddTLD("test", true)
	reg.AddTLD("closed", false)
	reg.AddTLD("unaccredited", true) // open, but nobody is accredited for it
	for _, clid := range []string{testRegistrar, "registrar2"} {
		reg.Accredit(clid, "test")
		reg.Accredit(clid, "closed")
		reg.AddContact(clid+"-c1", clid)
		reg.AddContact(clid+"-c2", clid)
		reg.AddHost("ns1."+clid+".net", clid)
		reg.AddHost("ns2."+clid+".net", clid)
	}
	reg.AddDomain(&entities.Domain{Name: "registered.test", ClID: "registrar2", RoID: entities.RoidType("1_DOM-" + testRepositoryID)})
	reg.Reserve("reserved.test")
	return &testDomains{
		registry: reg,
		EPPDomainService: services.NewEPPDomainService(services.EPPDomainServiceDeps{
			Domains: reg.Domains(), NNDNs: reg.NNDNs(), TLDs: reg.TLDs(), Phases: reg.Phases(),
			Contacts: reg.Contacts(), Hosts: reg.Hosts(), Registrars: reg.Registrars(), Register: reg,
			CheckParallelism: 4,
		}),
	}
}

func (f *testDomains) Check(ctx context.Context, names []string) ([]*queries.DomainCheckResult, error) {
	f.mu.Lock()
	f.asked = append(f.asked, names)
	err := f.err
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.EPPDomainService.Check(ctx, names)
}

func (f *testDomains) Info(ctx context.Context, scope entities.RegistrarClID, name string, authInfo *string) (*queries.EPPDomainInfo, error) {
	if err := f.failure(); err != nil {
		return nil, err
	}
	return f.EPPDomainService.Info(ctx, scope, name, authInfo)
}

func (f *testDomains) Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*entities.Domain, error) {
	if err := f.failure(); err != nil {
		return nil, err
	}
	return f.EPPDomainService.Create(ctx, scope, cmd)
}

func (f *testDomains) failure() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// fail makes every command fail with err, as a registry that cannot answer
// does; nil ends it.
func (f *testDomains) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *testDomains) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func (f *testDomains) call(i int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[i]
}
