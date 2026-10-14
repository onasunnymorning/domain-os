package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// Refusals of EPP domain commands. The EPP adapter maps each to a result
// code; the codes are listed in docs/epp/POLICY_REFERENCE.md.
var (
	// ErrEPPNotSponsor: the object is sponsored by another registrar
	// (ADR 0006: 2201).
	ErrEPPNotSponsor = errors.New("the object is sponsored by another registrar")
	// ErrEPPTLDNotServed: the name is under no TLD this registry serves.
	ErrEPPTLDNotServed = errors.New("the TLD is not served by this registry")
	// ErrEPPTLDNotOpen: the TLD has no general-availability phase in force.
	ErrEPPTLDNotOpen = errors.New("the TLD is not open for registration")
	// ErrEPPInvalidPeriod: the registration period is outside 1 year to the
	// phase's maximum horizon.
	ErrEPPInvalidPeriod = errors.New("the registration period is out of range")
	// ErrEPPContactRequired: the phase's contact policy requires a contact
	// that was not given.
	ErrEPPContactRequired = errors.New("a contact the registry requires is missing")
	// ErrEPPContactProhibited: a contact was given that the phase's contact
	// policy does not accept.
	ErrEPPContactProhibited = errors.New("a contact the registry does not accept was given")
)

// subordinateHostPage is how many hosts one listing of a domain's
// subordinate hosts reads.
const subordinateHostPage = 100

// Info returns a domain the session's registrar sponsors, with its
// nameservers and the host objects subordinate to it (RFC 5731 §3.1.2). A
// domain sponsored by another registrar is ErrEPPNotSponsor, and a missing
// one entities.ErrDomainNotFound.
func (s *EPPDomainService) Info(ctx context.Context, scope entities.RegistrarClID, name string) (*queries.EPPDomainInfo, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	dn, err := entities.NewDomainName(name)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	dom, err := s.domains.GetDomainByName(ctx, dn.String(), true)
	if err != nil {
		return nil, err
	}
	if dom.ClID != entities.ClIDType(scope) {
		return nil, ErrEPPNotSponsor
	}
	sub, err := s.subordinateHosts(ctx, dom.ClID.String(), dn.String())
	if err != nil {
		return nil, err
	}
	return &queries.EPPDomainInfo{Domain: dom, SubordinateHosts: sub}, nil
}

// subordinateHosts lists the host objects named under domain. They are
// sponsored by the domain's sponsor (RFC 5732 §1.1).
func (s *EPPDomainService) subordinateHosts(ctx context.Context, clid, domain string) ([]string, error) {
	suffix := "." + domain
	var names []string
	cursor := ""
	for {
		page, next, err := s.hosts.ListHosts(ctx, queries.ListItemsQuery{
			PageSize: subordinateHostPage, PageCursor: cursor,
			Filter: queries.ListHostsFilter{ClidEquals: clid, NameLike: suffix},
		})
		if err != nil {
			return nil, fmt.Errorf("list hosts under %s: %w", domain, err)
		}
		for _, h := range page {
			if strings.HasSuffix(h.Name.String(), suffix) {
				names = append(names, h.Name.String())
			}
		}
		if next == "" || len(page) == 0 {
			return names, nil
		}
		cursor = next
	}
}

// Create registers a domain for the session's registrar (RFC 5731 §3.2.1).
// Before registering, it checks, in order: the name, its TLD and phase, the
// registrar's accreditation for the TLD (ADR 0006), the period, that the
// name is neither registered nor reserved, the contacts against the
// phase's contact policy, that every contact exists and is the registrar's
// own, and that every nameserver is one of the registrar's host objects.
func (s *EPPDomainService) Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*entities.Domain, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	dn, err := entities.NewDomainName(cmd.Name)
	if err != nil || !strings.Contains(dn.String(), ".") {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	t, err := s.tldForCheck(ctx, dn.ParentDomain())
	if err != nil {
		return nil, err
	}
	switch t.reason {
	case CheckReasonNotServed:
		return nil, ErrEPPTLDNotServed
	case CheckReasonNotOpen:
		return nil, ErrEPPTLDNotOpen
	}
	accredited, err := s.registrars.IsRegistrarAccreditedForTLD(ctx, dn.ParentDomain(), scope.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCouldNotDetermineAccreditation, err)
	}
	if !accredited {
		return nil, ErrRegistrarNotAccredited
	}
	if cmd.Years < 1 || cmd.Years > t.phase.Policy.MaxHorizon {
		return nil, fmt.Errorf("%w: 1 to %d years", ErrEPPInvalidPeriod, t.phase.Policy.MaxHorizon)
	}
	if !t.phase.Policy.LabelIsAllowed(dn.Label()) {
		return nil, entities.ErrLabelNotValidInPhase
	}
	switch reason, err := s.registeredOrReserved(ctx, dn.String()); {
	case err != nil:
		return nil, err
	case reason == CheckReasonInUse:
		return nil, ErrDomainExists
	case reason == CheckReasonReserved:
		return nil, ErrDomainBlocked
	}
	if err := checkContactPolicy(t.phase.Policy.ContactDataPolicy, cmd); err != nil {
		return nil, err
	}
	if err := s.checkContacts(ctx, scope, cmd); err != nil {
		return nil, err
	}
	if len(cmd.HostNames) > entities.MaxHostsPerDomain {
		return nil, entities.ErrMaxHostsPerDomainExceeded
	}
	for _, h := range cmd.HostNames {
		if _, err := s.hosts.GetHostByNameAndClID(ctx, strings.ToLower(h), scope.String()); err != nil {
			if errors.Is(err, entities.ErrHostNotFound) {
				return nil, fmt.Errorf("%w: %s", entities.ErrHostNotFound, h)
			}
			return nil, fmt.Errorf("look up host %s: %w", h, err)
		}
	}
	if _, err := entities.NewAuthInfoType(cmd.AuthInfo); err != nil {
		return nil, err
	}

	dom, err := s.register.RegisterDomain(ctx, &commands.RegisterDomainCommand{
		Name: dn.String(), ClID: scope.String(), AuthInfo: cmd.AuthInfo,
		RegistrantID: cmd.RegistrantID, AdminID: cmd.AdminID, TechID: cmd.TechID, BillingID: cmd.BillingID,
		Years: cmd.Years, HostNames: cmd.HostNames,
	})
	if err != nil {
		// Another registrar may have registered the name since it was
		// checked; that is "exists", whatever the registration path said.
		if _, lookupErr := s.domains.GetDomainByName(ctx, dn.String(), false); lookupErr == nil {
			return nil, ErrDomainExists
		}
		return nil, err
	}
	return dom, nil
}

// checkContactPolicy applies the phase's contact policy to the contacts
// given: a mandatory one must be there, and a prohibited one must not.
// (entities.Domain.ApplyContactDataPolicy drops a prohibited contact
// silently; over EPP the registrar is told instead.)
func checkContactPolicy(p entities.ContactDataPolicy, cmd commands.EPPCreateDomainCommand) error {
	for _, c := range []struct {
		kind   string
		policy entities.ContactDataPolicyType
		id     string
	}{
		{"registrant", p.RegistrantContactDataPolicy, cmd.RegistrantID},
		{"admin", p.AdminContactDataPolicy, cmd.AdminID},
		{"tech", p.TechContactDataPolicy, cmd.TechID},
		{"billing", p.BillingContactDataPolicy, cmd.BillingID},
	} {
		switch {
		case c.policy == entities.ContactDataPolicyTypeMandatory && c.id == "":
			return fmt.Errorf("%w: %s", ErrEPPContactRequired, c.kind)
		case c.policy == entities.ContactDataPolicyTypeProhibited && c.id != "":
			return fmt.Errorf("%w: %s", ErrEPPContactProhibited, c.kind)
		}
	}
	return nil
}

// checkContacts requires every contact given to exist and to be sponsored
// by the session's registrar.
func (s *EPPDomainService) checkContacts(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) error {
	seen := map[string]bool{}
	for _, id := range []string{cmd.RegistrantID, cmd.AdminID, cmd.TechID, cmd.BillingID} {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		c, err := s.contacts.GetContactByID(ctx, id)
		if err != nil {
			if errors.Is(err, entities.ErrContactNotFound) {
				return fmt.Errorf("%w: %s", entities.ErrContactNotFound, id)
			}
			return fmt.Errorf("look up contact %s: %w", id, err)
		}
		if c.ClID != entities.ClIDType(scope) {
			return fmt.Errorf("%w: contact %s", ErrEPPNotSponsor, id)
		}
	}
	return nil
}
