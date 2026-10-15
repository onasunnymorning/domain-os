package memstore

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Registry is an in-memory registry of TLDs and their phases, registrars'
// accreditations, domains, contacts, hosts and reserved names: enough to
// run services.EPPDomainService and services.EPPHostService without a
// database. Its views implement
// only the repository methods that service calls; any other method panics,
// through the nil interface each view embeds.
//
// Register stands in for DomainService.RegisterDomain: it builds the domain
// with entities.RegisterDomain in the TLD's current GA phase and stores it.
// Pricing, events and persistence details belong to DomainService and are
// tested with it.
type Registry struct {
	mu           sync.Mutex
	err          error
	now          func() time.Time
	nextID       int64
	repository   entities.RepositoryID
	phases       map[string][]*entities.Phase // by TLD
	accredited   map[string]map[string]bool   // registrar → TLD
	domains      map[string]*entities.Domain
	contacts     map[string]*entities.Contact
	hosts        map[hostKey]*entities.Host
	reserved     map[string]bool
	knownTLDs    map[string]bool
	Registered   []string // names Register stored, in order
	registerHook func(*commands.RegisterDomainCommand) error
}

// hostKey identifies a host object: each registrar has its own host object
// for a name.
type hostKey struct{ name, clid string }

// NewRegistry returns an empty registry that mints ROIDs with repository,
// as DomainService does with the configured one.
func NewRegistry(repository entities.RepositoryID) *Registry {
	return &Registry{
		now: time.Now, repository: repository, phases: map[string][]*entities.Phase{}, accredited: map[string]map[string]bool{},
		domains: map[string]*entities.Domain{}, contacts: map[string]*entities.Contact{},
		hosts: map[hostKey]*entities.Host{}, reserved: map[string]bool{}, knownTLDs: map[string]bool{},
	}
}

// GenerateID returns the next of the registry's IDs, so that a
// services.RoidService over the registry mints ROIDs that follow its own:
// the registry is a repositories.IDGenerator.
func (r *Registry) GenerateID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	return r.nextID
}

// ListNode is the generator's node: the registry is the only one.
func (r *Registry) ListNode() int64 { return 0 }

// Fail makes every read and write return err; nil ends it.
func (r *Registry) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// AddTLD adds a TLD, with a GA phase in force since an hour ago when open.
func (r *Registry) AddTLD(name string, open bool) *entities.Phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.knownTLDs[name] = true
	if !open {
		return nil
	}
	p, err := entities.NewPhase("GA-"+name, string(entities.PhaseTypeGA), r.now().Add(-time.Hour).UTC())
	if err != nil {
		panic(err)
	}
	r.nextID++
	p.ID = r.nextID
	p.TLDName = entities.DomainName(name)
	r.phases[name] = append(r.phases[name], p)
	return p
}

// Accredit accredits a registrar for a TLD.
func (r *Registry) Accredit(clid, tld string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accredited[clid] == nil {
		r.accredited[clid] = map[string]bool{}
	}
	r.accredited[clid][tld] = true
}

// AddContact adds a contact sponsored by clid.
func (r *Registry) AddContact(id, clid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contacts[id] = &entities.Contact{ID: entities.ClIDType(id), ClID: entities.ClIDType(clid)}
}

// AddHost adds a host object sponsored by clid, with addrs.
func (r *Registry) AddHost(name, clid string, addrs ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	roid, err := entities.NewRoidType(r.nextID, entities.RoidTypeHost, r.repository)
	if err != nil {
		panic(err) // a fixture's own mistake
	}
	h, err := entities.NewHost(name, roid.String(), clid)
	if err != nil {
		panic(err) // a fixture's own mistake
	}
	for _, a := range addrs {
		if _, err := h.AddAddress(a); err != nil {
			panic(err) // a fixture's own mistake
		}
	}
	h.CreatedAt = r.now().UTC()
	r.hosts[hostKey{name, clid}] = h
}

// Host returns a copy of clid's host object named name, or nil.
func (r *Registry) Host(name, clid string) *entities.Host {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.hosts[hostKey{name, clid}]; ok {
		return h.DeepCopy()
	}
	return nil
}

// SetHostStatus changes the statuses of clid's host object named name, as
// the registry's operator would.
func (r *Registry) SetHostStatus(name, clid string, fn func(*entities.HostStatus)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(&r.hosts[hostKey{name, clid}].Status)
}

// Reserve puts a name on the reserved list.
func (r *Registry) Reserve(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reserved[name] = true
}

// AddDomain stores a domain as it is.
func (r *Registry) AddDomain(d *entities.Domain) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.domains[d.Name.String()] = d
}

// Domain returns a stored domain, or nil.
func (r *Registry) Domain(name string) *entities.Domain {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.domains[name]
}

// Domains is the registry as a repositories.DomainRepository.
func (r *Registry) Domains() repositories.DomainRepository { return registryDomains{r: r} }

// NNDNs is the registry as a repositories.NNDNRepository.
func (r *Registry) NNDNs() repositories.NNDNRepository { return registryNNDNs{r: r} }

// TLDs is the registry as a repositories.TLDRepository.
func (r *Registry) TLDs() repositories.TLDRepository { return registryTLDs{r: r} }

// Phases is the registry as a repositories.PhaseRepository.
func (r *Registry) Phases() repositories.PhaseRepository { return registryPhases{r: r} }

// Contacts is the registry as a repositories.ContactRepository.
func (r *Registry) Contacts() repositories.ContactRepository { return registryContacts{r: r} }

// Hosts is the registry as a repositories.HostRepository.
func (r *Registry) Hosts() repositories.HostRepository { return registryHosts{r: r} }

// Registrars is the registry as a repositories.RegistrarRepository.
func (r *Registry) Registrars() repositories.RegistrarRepository { return registryRegistrars{r: r} }

// RegisterDomain builds the domain in the TLD's current GA phase and
// stores it, failing if the name is taken.
func (r *Registry) RegisterDomain(_ context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error) {
	r.mu.Lock()
	hook := r.registerHook
	r.registerHook = nil
	r.mu.Unlock()
	if hook != nil {
		// Outside the lock: the hook may change the registry.
		if err := hook(cmd); err != nil {
			return nil, err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	if _, ok := r.domains[cmd.Name]; ok {
		return nil, entities.ErrDomainAlreadyExists
	}
	dn := entities.DomainName(cmd.Name)
	t := entities.TLD{}
	for _, p := range r.phases[dn.ParentDomain()] {
		t.Phases = append(t.Phases, *p)
	}
	phase, err := t.GetCurrentGAPhase()
	if err != nil {
		return nil, err
	}
	r.nextID++
	roid, err := entities.NewRoidType(r.nextID, entities.RoidTypeDomain, r.repository)
	if err != nil {
		return nil, err
	}
	dom, err := entities.RegisterDomain(roid.String(), cmd.Name, cmd.ClID, cmd.AuthInfo,
		cmd.RegistrantID, cmd.AdminID, cmd.TechID, cmd.BillingID, phase, cmd.Years)
	if err != nil {
		return nil, err
	}
	if err := dom.SetDSRecords(cmd.DSRecords); err != nil {
		return nil, err
	}
	for _, h := range cmd.HostNames {
		host, ok := r.hosts[hostKey{strings.ToLower(h), cmd.ClID}]
		if !ok {
			return nil, entities.ErrHostNotFound
		}
		if _, err := dom.AddHost(host, false); err != nil {
			return nil, err
		}
	}
	dom.CreatedAt = r.now().UTC()
	dom.ExpiryDate = dom.CreatedAt.AddDate(cmd.Years, 0, 0)
	r.domains[cmd.Name] = dom
	r.Registered = append(r.Registered, cmd.Name)
	return dom, nil
}

// OnRegister runs hook once, at the start of the next RegisterDomain and
// outside the registry's lock; an error from it fails that registration.
// It simulates a race.
func (r *Registry) OnRegister(hook func(*commands.RegisterDomainCommand) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerHook = hook
}

type registryDomains struct {
	repositories.DomainRepository
	r *Registry
}

func (v registryDomains) GetDomainByName(_ context.Context, name string, _ bool) (*entities.Domain, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if d, ok := v.r.domains[name]; ok {
		c := *d
		return &c, nil
	}
	return nil, entities.ErrDomainNotFound
}

type registryNNDNs struct {
	repositories.NNDNRepository
	r *Registry
}

func (v registryNNDNs) GetNNDN(_ context.Context, name string) (*entities.NNDN, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if v.r.reserved[name] {
		return &entities.NNDN{}, nil
	}
	return nil, entities.ErrNNDNNotFound
}

type registryTLDs struct {
	repositories.TLDRepository
	r *Registry
}

func (v registryTLDs) GetByName(_ context.Context, name string, _ bool) (*entities.TLD, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if v.r.knownTLDs[name] {
		return &entities.TLD{Name: entities.DomainName(name)}, nil
	}
	return nil, entities.ErrTLDNotFound
}

type registryPhases struct {
	repositories.PhaseRepository
	r *Registry
}

// ListPhasesByTLD pages by ID, oldest first, as the Postgres repository does.
func (v registryPhases) ListPhasesByTLD(_ context.Context, tld string, pageSize int, cursor string) ([]*entities.Phase, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	var after int64
	if cursor != "" {
		var err error
		if after, err = strconv.ParseInt(cursor, 10, 64); err != nil {
			return nil, err
		}
	}
	var page []*entities.Phase
	for _, p := range v.r.phases[tld] {
		if p.ID > after && len(page) < pageSize {
			c := *p
			page = append(page, &c)
		}
	}
	return page, nil
}

type registryContacts struct {
	repositories.ContactRepository
	r *Registry
}

func (v registryContacts) GetContactByID(_ context.Context, id string) (*entities.Contact, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if c, ok := v.r.contacts[id]; ok {
		cc := *c
		return &cc, nil
	}
	return nil, entities.ErrContactNotFound
}

type registryHosts struct {
	repositories.HostRepository
	r *Registry
}

func (v registryHosts) GetHostByNameAndClID(_ context.Context, name, clid string) (*entities.Host, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if h, ok := v.r.hosts[hostKey{name, clid}]; ok {
		return h.DeepCopy(), nil
	}
	return nil, entities.ErrHostNotFound
}

// hostByRoid returns the stored host with roid, or nil. The caller holds
// the lock.
func (r *Registry) hostByRoid(roid int64) *entities.Host {
	for _, h := range r.hosts {
		if id, _ := h.RoID.Int64(); id == roid {
			return h
		}
	}
	return nil
}

func (v registryHosts) GetHostByRoid(_ context.Context, roid int64) (*entities.Host, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if h := v.r.hostByRoid(roid); h != nil {
		return h.DeepCopy(), nil
	}
	return nil, entities.ErrHostNotFound
}

// BulkCreate stores the hosts, failing on the first a registrar already
// has by that name.
func (v registryHosts) BulkCreate(_ context.Context, hosts []*entities.Host) error {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return v.r.err
	}
	for _, h := range hosts {
		if _, ok := v.r.hosts[hostKey{h.Name.String(), h.ClID.String()}]; ok {
			return entities.ErrHostAlreadyExists
		}
	}
	for _, h := range hosts {
		c := h.DeepCopy()
		c.CreatedAt = v.r.now().UTC()
		v.r.hosts[hostKey{h.Name.String(), h.ClID.String()}] = c
	}
	return nil
}

// UpdateHostAndAddresses replaces the stored host in place, so the domains
// that have it as a nameserver see the change, as they do through the
// database's link table. Like the Postgres repository, it refuses a host
// changed since h was read (ErrHostChanged).
func (v registryHosts) UpdateHostAndAddresses(_ context.Context, h *entities.Host) (*entities.Host, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	id, _ := h.RoID.Int64()
	stored := v.r.hostByRoid(id)
	if stored == nil {
		return nil, entities.ErrHostNotFound
	}
	if !stored.UpdatedAt.Equal(h.UpdatedAt) {
		return nil, entities.ErrHostChanged
	}
	key := hostKey{h.Name.String(), h.ClID.String()}
	if other, ok := v.r.hosts[key]; ok && other != stored {
		return nil, entities.ErrHostAlreadyExists
	}
	delete(v.r.hosts, hostKey{stored.Name.String(), stored.ClID.String()})
	*stored = *h.DeepCopy()
	stored.UpdatedAt = v.r.now().UTC()
	v.r.hosts[key] = stored
	return stored.DeepCopy(), nil
}

func (v registryHosts) DeleteHostByRoid(_ context.Context, roid int64) error {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return v.r.err
	}
	if h := v.r.hostByRoid(roid); h != nil {
		delete(v.r.hosts, hostKey{h.Name.String(), h.ClID.String()})
	}
	return nil
}

func (v registryHosts) CountHostsByName(_ context.Context, name string) (int64, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return 0, v.r.err
	}
	var n int64
	for k := range v.r.hosts {
		if k.name == name {
			n++
		}
	}
	return n, nil
}

func (v registryHosts) GetHostAssociationCount(_ context.Context, roid int64) (int64, error) {
	return v.links(roid, func(*entities.Domain) bool { return true })
}

func (v registryHosts) CountHostLinksFromOtherSponsors(_ context.Context, roid int64, clid string) (int64, error) {
	return v.links(roid, func(d *entities.Domain) bool { return d.ClID.String() != clid })
}

// links counts the domains that match and have the host with roid as a
// nameserver.
func (v registryHosts) links(roid int64, match func(*entities.Domain) bool) (int64, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return 0, v.r.err
	}
	var n int64
	for _, d := range v.r.domains {
		if !match(d) {
			continue
		}
		for _, h := range d.Hosts {
			if id, _ := h.RoID.Int64(); id == roid {
				n++
				break
			}
		}
	}
	return n, nil
}

// ListHosts filters by ClidEquals and NameLike (a substring, as ILIKE
// %x% is), ordered by name, cursor by name.
func (v registryHosts) ListHosts(_ context.Context, q queries.ListItemsQuery) ([]*entities.Host, string, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, "", v.r.err
	}
	f, _ := q.Filter.(queries.ListHostsFilter)
	var all []*entities.Host
	for _, h := range v.r.hosts {
		if (f.ClidEquals == "" || h.ClID.String() == f.ClidEquals) &&
			strings.Contains(strings.ToLower(h.Name.String()), strings.ToLower(f.NameLike)) &&
			h.Name.String() > q.PageCursor {
			all = append(all, h.DeepCopy())
		}
	}
	slices.SortFunc(all, func(a, b *entities.Host) int { return strings.Compare(a.Name.String(), b.Name.String()) })
	if q.PageSize > 0 && len(all) > q.PageSize {
		return all[:q.PageSize], all[q.PageSize-1].Name.String(), nil
	}
	return all, "", nil
}

type registryRegistrars struct {
	repositories.RegistrarRepository
	r *Registry
}

func (v registryRegistrars) IsRegistrarAccreditedForTLD(_ context.Context, tld, clid string) (bool, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return false, v.r.err
	}
	return v.r.accredited[clid][tld], nil
}
