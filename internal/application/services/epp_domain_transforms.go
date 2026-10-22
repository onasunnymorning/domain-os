package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// clientDomainStatuses are the domain statuses a client adds and removes
// (RFC 5731 §2.3). The others are the server's.
var clientDomainStatuses = []string{
	entities.DomainStatusClientDeleteProhibited, entities.DomainStatusClientHold, entities.DomainStatusClientRenewProhibited,
	entities.DomainStatusClientTransferProhibited, entities.DomainStatusClientUpdateProhibited,
}

// Update changes a domain the session's registrar sponsors (RFC 5731
// §3.2.5 and RFC 5910 §5.2.5). It removes, then adds, statuses, contacts,
// nameservers and DS records, then makes the changes in <domain:chg>,
// checking in order:
//   - the domain is the registrar's (2201 otherwise, ADR 0006);
//   - every status is a client status;
//   - the domain has no pending action, no serverUpdateProhibited, and no
//     clientUpdateProhibited unless the update removes it;
//   - nameservers are host objects, and DNSSEC data DS data, without
//     urgent or maxSigLife;
//   - each status, contact, nameserver and DS record removed is the
//     domain's, and each added is not, at most one contact of each type,
//     each new contact and nameserver the registrar's own, at most
//     entities.MaxHostsPerDomain nameservers and MaxDSRecords DS records,
//     each new DS record well formed and accepted;
//   - the registrant, if changed, is the registrar's contact, and the
//     contacts satisfy the phase's contact policy;
//   - the authorization information, if changed, keeps to the registry's
//     rules.
//
// The checks and the write are made on one reading of the domain: if
// another command changes it in between, the update is read and checked
// again, up to updateAttempts times.
func (s *EPPDomainService) Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateDomainCommand) (*entities.Domain, error) {
	for attempt := 1; ; attempt++ {
		dom, err := s.update(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return dom, err
		}
	}
}

func (s *EPPDomainService) update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateDomainCommand) (*entities.Domain, error) {
	dom, err := s.sponsoredDomain(ctx, scope, cmd.Name)
	if err != nil {
		return nil, err
	}
	for _, st := range slices.Concat(cmd.RemoveStatuses, cmd.AddStatuses) {
		if !slices.Contains(clientDomainStatuses, st) {
			return nil, fmt.Errorf("%w: %s; a client sets only %s", entities.ErrEPPStatusNotClient, st, strings.Join(clientDomainStatuses, ", "))
		}
	}
	switch {
	case dom.Status.HasPendings():
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPObjectPending, dom.Name)
	case dom.Status.ServerUpdateProhibited:
		return nil, fmt.Errorf("%w: serverUpdateProhibited", entities.ErrDomainUpdateNotAllowed)
	case dom.Status.ClientUpdateProhibited && !slices.Contains(cmd.RemoveStatuses, entities.DomainStatusClientUpdateProhibited):
		return nil, fmt.Errorf("%w: clientUpdateProhibited; remove it first", entities.ErrDomainUpdateNotAllowed)
	case cmd.HostAttributes:
		return nil, entities.ErrEPPHostAttributes
	}
	if d := cmd.DNSSEC; d != nil {
		switch {
		case d.Urgent:
			return nil, entities.ErrEPPDNSSECUrgent
		case d.MaxSigLife != 0:
			return nil, entities.ErrEPPMaxSigLife
		case d.KeyData:
			return nil, entities.ErrEPPKeyData
		}
	}

	before := dom.DeepCopy()
	if err := applyDomainStatuses(dom, cmd.RemoveStatuses, cmd.AddStatuses); err != nil {
		return nil, err
	}
	if err := s.applyDomainContacts(ctx, scope, dom, cmd); err != nil {
		return nil, err
	}
	if err := s.applyNameservers(ctx, scope, dom, cmd.RemoveHostNames, cmd.AddHostNames); err != nil {
		return nil, err
	}
	if cmd.DNSSEC != nil {
		if err := applyDS(dom, cmd.DNSSEC); err != nil {
			return nil, err
		}
	}
	// RFC 9154 §5.2: an empty code or <domain:null/> unsets the domain's
	// code; a code given must be random enough, and is kept hashed.
	switch {
	case cmd.AuthInfoNull || (cmd.AuthInfo != nil && *cmd.AuthInfo == ""):
		dom.AuthInfo = ""
	case cmd.AuthInfo != nil:
		if err := entities.ValidateSecureAuthInfo(*cmd.AuthInfo); err != nil {
			return nil, err
		}
		h, err := entities.HashAuthInfo(*cmd.AuthInfo)
		if err != nil {
			return nil, err
		}
		dom.AuthInfo = h
	}

	dom.UpRr = entities.ClIDType(scope.String())
	dom.SetUnsetInactiveStatus()
	dom.SetOKStatusIfNeeded()
	dom.UnSetOKStatusIfNeeded()
	if err := dom.Validate(); err != nil {
		return nil, err
	}
	return s.register.SaveDomainUpdate(ctx, before, dom, cmd)
}

// Renew renews a domain the session's registrar sponsors (RFC 5731
// §3.2.3), checking in order: the domain is the registrar's; the current
// expiry date given is the domain's, so that a renew repeated by mistake is
// refused; the period, one year if none was given, is whole years; and the
// domain may be renewed, by its statuses and the phase's maximum horizon
// (ten years by default). Pricing and the renew grace period are
// DomainService's.
func (s *EPPDomainService) Renew(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPRenewDomainCommand) (*entities.Domain, error) {
	for attempt := 1; ; attempt++ {
		dom, err := s.renew(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return dom, err
		}
	}
}

func (s *EPPDomainService) renew(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPRenewDomainCommand) (*entities.Domain, error) {
	dom, err := s.sponsoredDomain(ctx, scope, cmd.Name)
	if err != nil {
		return nil, err
	}
	if exp := dom.ExpiryDate.UTC().Format("2006-01-02"); cmd.CurExpDate != exp {
		return nil, fmt.Errorf("%w: it is %s", entities.ErrEPPCurExpDate, exp)
	}
	years := 1
	if cmd.Period != 0 {
		years = cmd.Period
		if cmd.PeriodUnit == "m" {
			if cmd.Period%12 != 0 {
				return nil, entities.ErrEPPPeriodNotWholeYears
			}
			years = cmd.Period / 12
		}
	}
	return s.register.RenewDomainAsRead(ctx, dom, years)
}

// Delete starts the end of life of a domain the session's registrar
// sponsors (RFC 5731 §3.2.2, RFC 3915): it becomes pendingDelete, in its
// redemption period, or is purged at once within its add grace period. It
// checks in order: the domain is the registrar's; it has no pending action
// and no delete prohibition; and no host object is subordinate to it.
func (s *EPPDomainService) Delete(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Domain, error) {
	for attempt := 1; ; attempt++ {
		dom, err := s.delete(ctx, scope, name)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return dom, err
		}
	}
}

func (s *EPPDomainService) delete(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Domain, error) {
	dom, err := s.sponsoredDomain(ctx, scope, name)
	if err != nil {
		return nil, err
	}
	switch {
	case dom.Status.HasPendings():
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPObjectPending, dom.Name)
	case !dom.CanBeDeleted():
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPDeleteProhibited, dom.Name)
	}
	sub, err := s.subordinateHosts(ctx, dom.ClID.String(), dom.Name.String())
	if err != nil {
		return nil, err
	}
	if len(sub) > 0 {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPSubordinateHosts, strings.Join(sub, ", "))
	}
	return s.register.MarkDomainForDeletionAsRead(ctx, dom)
}

// sponsoredDomain returns the domain named name, with its nameservers and
// DS records, if the registrar sponsors it: another registrar's is
// ErrEPPNotSponsor (ADR 0006), and none ErrDomainNotFound.
func (s *EPPDomainService) sponsoredDomain(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Domain, error) {
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
		return nil, entities.ErrEPPNotSponsor
	}
	return dom, nil
}

// applyDomainStatuses removes, then adds, client statuses: each removed
// must be set, and each added must not be.
func applyDomainStatuses(dom *entities.Domain, remove, add []string) error {
	flag := func(st string) *bool {
		switch st {
		case entities.DomainStatusClientDeleteProhibited:
			return &dom.Status.ClientDeleteProhibited
		case entities.DomainStatusClientHold:
			return &dom.Status.ClientHold
		case entities.DomainStatusClientRenewProhibited:
			return &dom.Status.ClientRenewProhibited
		case entities.DomainStatusClientTransferProhibited:
			return &dom.Status.ClientTransferProhibited
		default:
			return &dom.Status.ClientUpdateProhibited
		}
	}
	for _, st := range remove {
		if f := flag(st); *f {
			*f = false
		} else {
			return fmt.Errorf("%w: %s", entities.ErrEPPStatusNotSet, st)
		}
	}
	for _, st := range add {
		if f := flag(st); !*f {
			*f = true
		} else {
			return fmt.Errorf("%w: %s", entities.ErrEPPStatusAlreadySet, st)
		}
	}
	return nil
}

// applyDomainContacts removes, then adds, contacts, and changes the
// registrant; then applies the phase's contact policy.
func (s *EPPDomainService) applyDomainContacts(ctx context.Context, scope entities.RegistrarClID, dom *entities.Domain, cmd commands.EPPUpdateDomainCommand) error {
	slot := func(t string) (*entities.ClIDType, error) {
		switch t {
		case "admin":
			return &dom.AdminID, nil
		case "tech":
			return &dom.TechID, nil
		case "billing":
			return &dom.BillingID, nil
		}
		return nil, entities.ErrEPPContactTypeMissing
	}
	for _, c := range cmd.RemoveContacts {
		sl, err := slot(c.Type)
		if err != nil {
			return err
		}
		if sl.String() != c.ID {
			return fmt.Errorf("%w: %s %s", entities.ErrEPPDomainContactNotSet, c.Type, c.ID)
		}
		*sl = ""
	}
	added := eppCreate{}
	for _, c := range cmd.AddContacts {
		sl, err := slot(c.Type)
		if err != nil {
			return err
		}
		if *sl != "" {
			return fmt.Errorf("%w: %s", entities.ErrEPPDuplicateContactType, c.Type)
		}
		*sl = entities.ClIDType(c.ID)
	}
	if cmd.Registrant != nil {
		dom.RegistrantID = entities.ClIDType(*cmd.Registrant)
		added.registrant = *cmd.Registrant
	}
	// Only contacts newly given are checked as the registrar's: the ones
	// the domain keeps were checked when they were set.
	for _, c := range cmd.AddContacts {
		switch c.Type {
		case "admin":
			added.admin = c.ID
		case "tech":
			added.tech = c.ID
		case "billing":
			added.billing = c.ID
		}
	}
	if err := s.checkContacts(ctx, scope, added); err != nil {
		return err
	}
	t, err := s.tldForCheck(ctx, dom.Name.ParentDomain())
	if err != nil {
		return err
	}
	if t.phase == nil {
		return nil // no phase in force: no contact policy to apply
	}
	return checkContactPolicy(t.phase.Policy.ContactDataPolicy, eppCreate{
		registrant: dom.RegistrantID.String(), admin: dom.AdminID.String(), tech: dom.TechID.String(), billing: dom.BillingID.String(),
	})
}

// applyNameservers removes, then adds, nameservers: each removed must be
// the domain's, and each added one of the registrar's host objects that the
// domain does not have.
func (s *EPPDomainService) applyNameservers(ctx context.Context, scope entities.RegistrarClID, dom *entities.Domain, remove, add []string) error {
	for _, raw := range remove {
		name := strings.ToLower(raw)
		i := slices.IndexFunc(dom.Hosts, func(h *entities.Host) bool { return h.Name.String() == name })
		if i < 0 {
			return fmt.Errorf("%w: %s", entities.ErrEPPNameserverNotSet, name)
		}
		dom.Hosts = slices.Delete(dom.Hosts, i, i+1)
	}
	for _, raw := range add {
		name := strings.ToLower(raw)
		host, err := s.hosts.GetHostByNameAndClID(ctx, name, scope.String())
		if err != nil {
			if errors.Is(err, entities.ErrHostNotFound) {
				return fmt.Errorf("%w: %s", entities.ErrHostNotFound, name)
			}
			return fmt.Errorf("look up host %s: %w", name, err)
		}
		// Update prohibitions were checked above, against the statuses the
		// command leaves.
		if _, err := dom.AddHost(host, true); err != nil {
			return fmt.Errorf("%w: %s", err, name)
		}
	}
	return nil
}

// applyDS applies a <secDNS:update>: removes all DS records or those
// listed, each the domain's; then adds those listed, each well formed,
// accepted and new, to at most MaxDSRecords.
func applyDS(dom *entities.Domain, u *commands.EPPDNSSECUpdate) error {
	records := slices.Clone(dom.DSRecords)
	if u.RemoveAll {
		records = nil
	}
	remove, err := wellFormedDS(u.RemoveDS)
	if err != nil {
		return err
	}
	for _, r := range remove {
		i := slices.Index(records, r)
		if i < 0 {
			return fmt.Errorf("%w: key tag %d", entities.ErrEPPDSNotSet, r.KeyTag)
		}
		records = slices.Delete(records, i, i+1)
	}
	add, err := wellFormedDS(u.AddDS)
	if err != nil {
		return err
	}
	if err := acceptedDS(add); err != nil {
		return err
	}
	records = append(records, add...)
	if len(records) > MaxDSRecords {
		return fmt.Errorf("%w: at most %d", entities.ErrEPPTooManyDS, MaxDSRecords)
	}
	return dom.SetDSRecords(records)
}
