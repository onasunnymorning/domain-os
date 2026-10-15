package services

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Why a <host:check> reports a name unavailable. They are the <host:reason>
// text, which RFC 5730 bounds to 32 characters.
const (
	HostCheckReasonInvalid = "Invalid host name"
	HostCheckReasonInUse   = "In use"
	// HostCheckReasonSuperordinate: the name is under a domain another
	// registrar sponsors, and only that registrar can create it.
	HostCheckReasonSuperordinate = "Superordinate domain not yours"
)

// clientHostStatuses are the host statuses a client adds and removes
// (RFC 5732 §2.3). The others are the server's.
var clientHostStatuses = []string{entities.HostStatusClientDeleteProhibited, entities.HostStatusClientUpdateProhibited}

// EPPHostService answers registrars' host commands over EPP (RFC 5732,
// docs/epp/ARCHITECTURE.md §4).
//
// Host objects belong to a registrar. Each registrar has its own host object
// for a name, unique per registrar, and a domain's nameservers are host
// objects of the domain's sponsor (entities.Domain.AddHost). So every
// command is scoped to the session's registrar, and a host by that name of
// another registrar is answered 2201 (ADR 0006), not shown.
//
// A host under a TLD this registry serves is internal (RFC 5732 §1.1). Its
// superordinate domain, the TLD plus one label, must be registered, and only
// that domain's sponsor can create or rename a host into it. So no
// registrar can create a host, with or without glue, under another's
// domain (RST epp-25 to epp-27). Only an internal host has addresses: an
// external host is created without them, and a host renamed out of this
// registry's TLDs loses them.
type EPPHostService struct {
	hosts   repositories.HostRepository
	domains repositories.DomainRepository
	tlds    repositories.TLDRepository
	roids   interfaces.RoidService
	events  repositories.EventPublisher
}

// EPPHostServiceDeps are what EPPHostService reads and writes through.
type EPPHostServiceDeps struct {
	Hosts   repositories.HostRepository
	Domains repositories.DomainRepository
	TLDs    repositories.TLDRepository
	// Roids mints the ROIDs of created hosts (ADR 0017).
	Roids interfaces.RoidService
	// Events publishes the hosts' domain events; nil publishes none.
	Events repositories.EventPublisher
}

// NewEPPHostService returns the service.
func NewEPPHostService(d EPPHostServiceDeps) *EPPHostService {
	return &EPPHostService{hosts: d.Hosts, domains: d.Domains, tlds: d.TLDs, roids: d.Roids, events: d.Events}
}

// Check reports, for each name in order, whether the registrar can create
// a host by that name. A name is unavailable if it is not a valid host
// name, if the registrar already has a host by that name, or if it is under
// a domain another registrar sponsors. A name under a domain that is not
// registered is available here, as RST epp-05 requires of an unregistered
// name; creating it fails until the domain is registered.
func (s *EPPHostService) Check(ctx context.Context, scope entities.RegistrarClID, names []string) ([]*queries.HostCheckResult, error) {
	tlds := tldCache{}
	results := make([]*queries.HostCheckResult, len(names))
	for i, raw := range names {
		r := &queries.HostCheckResult{Name: raw}
		results[i] = r
		name, err := hostName(raw)
		if err != nil {
			r.Reason = HostCheckReasonInvalid
			continue
		}
		reason, err := s.unavailable(ctx, scope, name, tlds)
		if err != nil {
			return nil, err
		}
		r.Reason, r.Available = reason, reason == ""
	}
	return results, nil
}

// unavailable returns why the registrar cannot create name, or "".
func (s *EPPHostService) unavailable(ctx context.Context, scope entities.RegistrarClID, name string, tlds tldCache) (string, error) {
	if _, err := s.hosts.GetHostByNameAndClID(ctx, name, scope.String()); err == nil {
		return HostCheckReasonInUse, nil
	} else if !errors.Is(err, entities.ErrHostNotFound) {
		return "", fmt.Errorf("look up host %s: %w", name, err)
	}
	p, err := s.place(ctx, name, tlds)
	if err != nil {
		return "", err
	}
	if p.domain != nil && p.domain.ClID.String() != scope.String() {
		return HostCheckReasonSuperordinate, nil
	}
	return "", nil
}

// Info returns the registrar's host object named name. Its linked status
// says whether any domain has it as a nameserver now.
func (s *EPPHostService) Info(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Host, error) {
	host, err := s.sponsored(ctx, scope, name)
	if err != nil {
		return nil, err
	}
	links, err := s.links(ctx, host)
	if err != nil {
		return nil, err
	}
	host.Status.Linked = links > 0
	return host, nil
}

// Create creates a host object for the registrar, checking in order: the
// name and addresses are valid, with no address twice and at most
// entities.MAX_ADDRESSES_PER_HOST; the registrar has no host by that name;
// an internal host's superordinate domain is registered and the registrar
// sponsors it; and an external host has no addresses.
func (s *EPPHostService) Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateHostCommand) (*entities.Host, error) {
	name, err := hostName(cmd.Name)
	if err != nil {
		return nil, err
	}
	addrs, err := hostAddresses(cmd.Addresses, true)
	if err != nil {
		return nil, err
	}
	if err := applyAddresses(&entities.Host{}, nil, addrs); err != nil {
		return nil, err
	}
	if _, err := s.hosts.GetHostByNameAndClID(ctx, name, scope.String()); err == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrHostAlreadyExists, name)
	} else if !errors.Is(err, entities.ErrHostNotFound) {
		return nil, fmt.Errorf("look up host %s: %w", name, err)
	}
	p, err := s.place(ctx, name, tldCache{})
	if err != nil {
		return nil, err
	}
	if err := p.admits(scope, name); err != nil {
		return nil, err
	}
	if !p.internal && len(addrs) > 0 {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPExternalHostAddress, name)
	}

	roid, err := s.roids.GenerateRoid("host")
	if err != nil {
		return nil, fmt.Errorf("mint host ROID: %w", err)
	}
	host, err := entities.NewHost(name, roid.String(), scope.String())
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		if _, err := host.AddAddress(a.String()); err != nil {
			return nil, fmt.Errorf("%w: %s", err, a)
		}
	}
	if err := host.Validate(); err != nil {
		return nil, err
	}
	// One insert writes the host and its addresses together.
	if err := s.hosts.BulkCreate(ctx, []*entities.Host{host}); err != nil {
		return nil, fmt.Errorf("create host %s: %w", name, err)
	}
	id, err := roid.Int64()
	if err != nil {
		return nil, err
	}
	created, err := s.hosts.GetHostByRoid(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("read created host %s: %w", name, err)
	}
	publishHostEvent(ctx, s.events, "host.created", created.RoID.String(), fmt.Sprintf("Host %s created", name), cmd, created, nil)
	return created, nil
}

// Update changes the registrar's host object: it removes, then adds,
// statuses and addresses, and renames the host. It checks, in order: the
// values are valid; the host has no pending action and no update
// prohibition (a clientUpdateProhibited the command removes excepted); each
// status removed or added is a client status, and set or not set as the
// command assumes; each address removed is the host's and each address
// added is not; a new name is not one of the registrar's hosts, the host
// is no other registrar's nameserver, and an internal new name is under a
// registered domain the registrar sponsors; an external host gets no
// addresses; and an internal host that is a nameserver keeps one.
//
// The checks and the write are made on one reading of the host. If another
// command changes the host in between, the write is refused and the update
// is read and checked again, up to updateAttempts times: no command's
// change is overwritten by one that never saw it.
func (s *EPPHostService) Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateHostCommand) (*entities.Host, error) {
	for attempt := 1; ; attempt++ {
		host, err := s.update(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrHostChanged) || attempt == updateAttempts {
			return host, err
		}
	}
}

// updateAttempts bounds how often Update reads a host again after another
// command changed it.
const updateAttempts = 3

func (s *EPPHostService) update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateHostCommand) (*entities.Host, error) {
	host, err := s.sponsored(ctx, scope, cmd.Name)
	if err != nil {
		return nil, err
	}
	add, err := hostAddresses(cmd.AddAddresses, true)
	if err != nil {
		return nil, err
	}
	rem, err := hostAddresses(cmd.RemoveAddresses, false)
	if err != nil {
		return nil, err
	}
	newName := ""
	if cmd.NewName != "" {
		if newName, err = hostName(cmd.NewName); err != nil {
			return nil, err
		}
	}
	for _, st := range slices.Concat(cmd.RemoveStatuses, cmd.AddStatuses) {
		if !slices.Contains(clientHostStatuses, st) {
			return nil, fmt.Errorf("%w: %s", entities.ErrEPPHostStatusNotClient, st)
		}
	}
	switch {
	case host.Status.IsPending():
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPHostPending, host.Name)
	case host.Status.ServerUpdateProhibited:
		return nil, fmt.Errorf("%w: serverUpdateProhibited", entities.ErrHostUpdateProhibited)
	case host.Status.ClientUpdateProhibited && !slices.Contains(cmd.RemoveStatuses, entities.HostStatusClientUpdateProhibited):
		return nil, fmt.Errorf("%w: clientUpdateProhibited; remove it first", entities.ErrHostUpdateProhibited)
	}

	before := host.DeepCopy()
	if err := applyClientStatuses(host, cmd.RemoveStatuses, cmd.AddStatuses); err != nil {
		return nil, err
	}
	if err := applyAddresses(host, rem, add); err != nil {
		return nil, err
	}
	links, err := s.links(ctx, host)
	if err != nil {
		return nil, err
	}
	renamed := newName != "" && newName != host.Name.String()
	if renamed {
		if err := s.checkRename(ctx, scope, host, newName); err != nil {
			return nil, err
		}
		host.Name = entities.DomainName(newName)
		// Whether the host's glue is published (the glue query reads
		// InBailiwick) follows its name: Domain.AddHost set it for the old
		// one.
		if host.InBailiwick, err = s.inBailiwick(ctx, host); err != nil {
			return nil, err
		}
	}
	p, err := s.place(ctx, host.Name.String(), tldCache{})
	if err != nil {
		return nil, err
	}
	if renamed {
		if err := p.admits(scope, newName); err != nil {
			return nil, err
		}
	}
	switch {
	case !p.internal && len(add) > 0:
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPExternalHostAddress, host.Name)
	case !p.internal:
		// Glue belongs only to a host within this registry's TLDs: one
		// renamed out of them loses its addresses.
		host.Addresses = nil
	case links > 0 && len(host.Addresses) == 0:
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPHostNeedsAddress, host.Name)
	}

	host.UpRr = entities.ClIDType(scope.String())
	if err := host.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.hosts.UpdateHostAndAddresses(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("update host %s: %w", before.Name, err)
	}
	publishHostEvent(ctx, s.events, "host.updated", updated.RoID.String(), fmt.Sprintf("Host %s updated", updated.Name), cmd, updated, before)
	return updated, nil
}

// checkRename refuses a rename onto one of the registrar's hosts, and a
// rename of a host that is a nameserver of another registrar's domain
// (RST epp-23: 2305).
func (s *EPPHostService) checkRename(ctx context.Context, scope entities.RegistrarClID, host *entities.Host, newName string) error {
	if _, err := s.hosts.GetHostByNameAndClID(ctx, newName, scope.String()); err == nil {
		return fmt.Errorf("%w: %s", entities.ErrHostAlreadyExists, newName)
	} else if !errors.Is(err, entities.ErrHostNotFound) {
		return fmt.Errorf("look up host %s: %w", newName, err)
	}
	id, err := host.RoID.Int64()
	if err != nil {
		return err
	}
	others, err := s.hosts.CountHostLinksFromOtherSponsors(ctx, id, scope.String())
	if err != nil {
		return fmt.Errorf("count other sponsors' domains using host %s: %w", host.Name, err)
	}
	if others > 0 {
		return fmt.Errorf("%w: %s", entities.ErrEPPHostLinkedByOthers, host.Name)
	}
	return nil
}

// Delete deletes the registrar's host object, unless it has a pending
// action or a delete prohibition, or is a nameserver of a domain.
func (s *EPPHostService) Delete(ctx context.Context, scope entities.RegistrarClID, name string) error {
	host, err := s.sponsored(ctx, scope, name)
	if err != nil {
		return err
	}
	switch {
	case host.Status.IsPending():
		return fmt.Errorf("%w: %s", entities.ErrEPPHostPending, host.Name)
	case !host.CanBeDeleted():
		return fmt.Errorf("%w: %s", entities.ErrEPPHostDeleteProhibited, host.Name)
	}
	links, err := s.links(ctx, host)
	if err != nil {
		return err
	}
	if links > 0 {
		return fmt.Errorf("%w: %s", entities.ErrEPPHostLinked, host.Name)
	}
	id, err := host.RoID.Int64()
	if err != nil {
		return err
	}
	if err := s.hosts.DeleteHostByRoid(ctx, id); err != nil {
		return fmt.Errorf("delete host %s: %w", host.Name, err)
	}
	publishHostEvent(ctx, s.events, "host.deleted", host.RoID.String(), fmt.Sprintf("Host %s deleted", host.Name), nil, nil, host)
	return nil
}

// sponsored returns the registrar's host object named raw. If the
// registrar has none but another registrar has one, that is
// ErrEPPNotSponsor (ADR 0006); if no registrar has one, ErrHostNotFound.
func (s *EPPHostService) sponsored(ctx context.Context, scope entities.RegistrarClID, raw string) (*entities.Host, error) {
	name, err := hostName(raw)
	if err != nil {
		return nil, err
	}
	host, err := s.hosts.GetHostByNameAndClID(ctx, name, scope.String())
	if err == nil {
		return host, nil
	}
	if !errors.Is(err, entities.ErrHostNotFound) {
		return nil, fmt.Errorf("look up host %s: %w", name, err)
	}
	n, err := s.hosts.CountHostsByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("look up host %s: %w", name, err)
	}
	if n > 0 {
		return nil, fmt.Errorf("%w: host %s", entities.ErrEPPNotSponsor, name)
	}
	return nil, fmt.Errorf("%w: %s", entities.ErrHostNotFound, name)
}

// inBailiwick reports whether host is a nameserver of its parent domain,
// as entities.Domain.AddHost decides it when a domain takes the host.
func (s *EPPHostService) inBailiwick(ctx context.Context, host *entities.Host) (bool, error) {
	parent := host.Name.ParentDomain()
	dom, err := s.domains.GetDomainByName(ctx, parent, true)
	switch {
	case errors.Is(err, entities.ErrDomainNotFound):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("look up domain %s: %w", parent, err)
	}
	for _, h := range dom.Hosts {
		if h.RoID == host.RoID {
			return true, nil
		}
	}
	return false, nil
}

// links counts the domains that have host as a nameserver.
func (s *EPPHostService) links(ctx context.Context, host *entities.Host) (int64, error) {
	id, err := host.RoID.Int64()
	if err != nil {
		return 0, err
	}
	n, err := s.hosts.GetHostAssociationCount(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("count domains using host %s: %w", host.Name, err)
	}
	return n, nil
}

// hostName validates and normalises a host name: a hostname of at least
// two labels, in lower case.
func hostName(raw string) (string, error) {
	dn, err := entities.NewDomainName(raw)
	if err != nil || !strings.Contains(dn.String(), ".") {
		return "", fmt.Errorf("%w: %s", entities.ErrEPPInvalidHostName, raw)
	}
	return dn.String(), nil
}

// hostAddresses parses the addresses of a command. Each must be an address
// of the IP version its ip attribute names; one being added must also be
// public unicast, so that it can be glue.
func hostAddresses(given []commands.EPPHostAddress, adding bool) ([]netip.Addr, error) {
	addrs := make([]netip.Addr, 0, len(given))
	for _, g := range given {
		a, err := netip.ParseAddr(g.Address)
		if err != nil || a.Zone() != "" {
			return nil, fmt.Errorf("%w: %s", entities.ErrInvalidIP, g.Address)
		}
		if v6 := g.Version == "v6"; v6 == a.Is4() {
			return nil, fmt.Errorf("%w: %s is not %s", entities.ErrEPPHostAddressVersion, g.Address, ipVersionName(v6))
		}
		if adding && (a.Is4In6() || !a.IsGlobalUnicast() || a.IsPrivate()) {
			return nil, fmt.Errorf("%w: %s", entities.ErrEPPHostAddressNotPublic, g.Address)
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func ipVersionName(v6 bool) string {
	if v6 {
		return "IPv6"
	}
	return "IPv4"
}

// applyClientStatuses removes, then adds, client statuses: each removed
// must be set, and each added must not be.
func applyClientStatuses(host *entities.Host, remove, add []string) error {
	flag := func(st string) *bool {
		if st == entities.HostStatusClientDeleteProhibited {
			return &host.Status.ClientDeleteProhibited
		}
		return &host.Status.ClientUpdateProhibited
	}
	for _, st := range remove {
		if f := flag(st); *f {
			*f = false
		} else {
			return fmt.Errorf("%w: %s", entities.ErrEPPHostStatusNotSet, st)
		}
	}
	for _, st := range add {
		if f := flag(st); !*f {
			*f = true
		} else {
			return fmt.Errorf("%w: %s", entities.ErrEPPHostStatusAlreadySet, st)
		}
	}
	host.SetOKIfNeeded()
	host.UnsetOKIfNeeded()
	return host.ValidateStatus()
}

// applyAddresses removes, then adds, addresses: each removed must be the
// host's, and each added must not be.
func applyAddresses(host *entities.Host, remove, add []netip.Addr) error {
	for _, a := range remove {
		i := slices.Index(host.Addresses, a)
		if i < 0 {
			return fmt.Errorf("%w: %s", entities.ErrEPPHostAddressNotSet, a)
		}
		host.Addresses = slices.Delete(host.Addresses, i, i+1)
	}
	for _, a := range add {
		if slices.Contains(host.Addresses, a) {
			return fmt.Errorf("%w: %s", entities.ErrDuplicateHostAddress, a)
		}
		if len(host.Addresses) >= entities.MAX_ADDRESSES_PER_HOST {
			return entities.ErrMaxAddressesPerHostExceeded
		}
		host.Addresses = append(host.Addresses, a)
	}
	return nil
}

// placement is where a host name sits: under a TLD this registry serves
// (internal), with its superordinate domain if that is registered, or
// outside them (external).
type placement struct {
	internal      bool
	superordinate string
	domain        *entities.Domain
}

// admits checks that the registrar may have a host named name here: an
// internal host's superordinate domain must be registered, and sponsored by
// the registrar.
func (p *placement) admits(scope entities.RegistrarClID, name string) error {
	switch {
	case !p.internal:
		return nil
	case p.domain == nil:
		return fmt.Errorf("%w: %s, for %s", entities.ErrEPPSuperordinateDomainNotFound, p.superordinate, name)
	case p.domain.ClID.String() != scope.String():
		return fmt.Errorf("%w: domain %s, for host %s", entities.ErrEPPNotSponsor, p.superordinate, name)
	}
	return nil
}

// tldCache remembers which names are TLDs this registry serves, for the
// length of one command.
type tldCache map[string]bool

// place finds where name sits. Its TLD is the longest suffix of it that is
// a TLD this registry serves, so that a host under x.co.uk is placed under
// co.uk when both uk and co.uk are served; its superordinate domain is that
// TLD and one more label, which may be the host's whole name.
func (s *EPPHostService) place(ctx context.Context, name string, tlds tldCache) (*placement, error) {
	labels := strings.Split(name, ".")
	for k := 1; k < len(labels); k++ {
		tld := strings.Join(labels[k:], ".")
		served, err := s.served(ctx, tld, tlds)
		if err != nil {
			return nil, err
		}
		if !served {
			continue
		}
		p := &placement{internal: true, superordinate: strings.Join(labels[k-1:], ".")}
		dom, err := s.domains.GetDomainByName(ctx, p.superordinate, false)
		switch {
		case err == nil:
			p.domain = dom
		case !errors.Is(err, entities.ErrDomainNotFound):
			return nil, fmt.Errorf("look up domain %s: %w", p.superordinate, err)
		}
		return p, nil
	}
	return &placement{}, nil
}

// served reports whether tld is a TLD this registry serves.
func (s *EPPHostService) served(ctx context.Context, tld string, tlds tldCache) (bool, error) {
	if served, ok := tlds[tld]; ok {
		return served, nil
	}
	_, err := s.tlds.GetByName(ctx, tld, false)
	switch {
	case err == nil:
		tlds[tld] = true
	case errors.Is(err, entities.ErrTLDNotFound):
		tlds[tld] = false
	default:
		return false, fmt.Errorf("look up TLD %s: %w", tld, err)
	}
	return tlds[tld], nil
}
