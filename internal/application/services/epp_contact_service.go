package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Why a <contact:check> reports an ID unavailable (<contact:reason>, at
// most 32 characters).
const (
	ContactCheckReasonInvalid = "Invalid contact ID"
	ContactCheckReasonInUse   = "In use"
)

// clientContactStatuses are the contact statuses a client adds and removes
// (RFC 5733 §2.2). The others are the server's.
var clientContactStatuses = []string{
	string(entities.ContactStatusClientDeleteProhibited),
	string(entities.ContactStatusClientTransferProhibited),
	string(entities.ContactStatusClientUpdateProhibited),
}

// maxEmailLocalPart is the longest local part of an email address
// (RFC 5321 §4.5.3.1.1).
const maxEmailLocalPart = 64

// EPPContactService answers registrars' contact commands over EPP
// (RFC 5733, docs/epp/ARCHITECTURE.md §4).
//
// A contact ID is unique in the registry, whoever sponsors it, so a check is
// the same for every registrar. Info, update and delete are answered only
// for the sponsor: anyone else gets 2201 (ADR 0006), with or without the
// contact's authorization information.
//
// Phone extensions (the x attribute) are refused: the registry does not
// store them.
type EPPContactService struct {
	contacts repositories.ContactRepository
	roids    interfaces.RoidService
	events   repositories.EventPublisher
}

// EPPContactServiceDeps are what EPPContactService reads and writes
// through.
type EPPContactServiceDeps struct {
	Contacts repositories.ContactRepository
	// Roids mints the ROIDs of created contacts (ADR 0017).
	Roids interfaces.RoidService
	// Events publishes the contacts' domain events; nil publishes none.
	Events repositories.EventPublisher
}

// NewEPPContactService returns the service.
func NewEPPContactService(d EPPContactServiceDeps) *EPPContactService {
	return &EPPContactService{contacts: d.Contacts, roids: d.Roids, events: d.Events}
}

// Check reports, for each ID in order, whether a contact can be created
// with it: it is unavailable if it is not a valid ID or a contact has it.
func (s *EPPContactService) Check(ctx context.Context, ids []string) ([]*queries.ContactCheckResult, error) {
	results := make([]*queries.ContactCheckResult, len(ids))
	for i, raw := range ids {
		r := &queries.ContactCheckResult{ID: raw}
		results[i] = r
		id, err := entities.NewClIDType(raw)
		if err != nil {
			r.Reason = ContactCheckReasonInvalid
			continue
		}
		switch _, err := s.contacts.GetContactByID(ctx, id.String()); {
		case err == nil:
			r.Reason = ContactCheckReasonInUse
		case errors.Is(err, entities.ErrContactNotFound):
			r.Available = true
		default:
			return nil, fmt.Errorf("look up contact %s: %w", id, err)
		}
	}
	return results, nil
}

// Info returns the registrar's contact. Its linked status says whether any
// domain uses it now.
func (s *EPPContactService) Info(ctx context.Context, scope entities.RegistrarClID, id string) (*entities.Contact, error) {
	c, err := s.sponsored(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	links, err := s.links(ctx, c)
	if err != nil {
		return nil, err
	}
	c.Status.Linked = links > 0
	return c, nil
}

// Create creates a contact for the registrar, checking in order: the ID,
// the postal addresses (at most one of each type), the phone numbers, the
// email address and the authorization information are valid; and no
// contact has the ID.
func (s *EPPContactService) Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateContactCommand) (*entities.Contact, error) {
	id, err := entities.NewClIDType(cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, cmd.ID)
	}
	c := &entities.Contact{ID: id}
	for _, pi := range cmd.PostalInfo {
		if err := applyPostalInfo(c, pi); err != nil {
			return nil, err
		}
	}
	for _, ph := range []struct {
		name  string
		given *commands.EPPPhone
		set   *entities.E164Type
	}{{"voice", cmd.Voice, &c.Voice}, {"fax", cmd.Fax, &c.Fax}} {
		if ph.given == nil {
			continue
		}
		if ph.given.Number == "" {
			return nil, fmt.Errorf("%w: <contact:%s>", entities.ErrEPPEmptyPhone, ph.name)
		}
		if err := applyPhone(ph.set, ph.name, ph.given); err != nil {
			return nil, err
		}
	}
	email, err := contactEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	authInfo, err := entities.NewAuthInfoType(cmd.AuthInfo)
	if err != nil {
		return nil, err
	}
	if cmd.Disclose != nil {
		applyDisclose(&c.Disclose, cmd.Disclose)
	}

	if _, err := s.contacts.GetContactByID(ctx, id.String()); err == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrContactAlreadyExists, id)
	} else if !errors.Is(err, entities.ErrContactNotFound) {
		return nil, fmt.Errorf("look up contact %s: %w", id, err)
	}
	roid, err := s.roids.GenerateRoid("contact")
	if err != nil {
		return nil, fmt.Errorf("mint contact ROID: %w", err)
	}
	c.RoID, c.Email, c.AuthInfo = roid, email, authInfo
	c.ClID, c.CrRr = entities.ClIDType(scope.String()), entities.ClIDType(scope.String())
	c.SetOKStatusIfNeeded()
	if _, err := c.IsValid(); err != nil {
		return nil, err
	}
	created, err := s.contacts.CreateContact(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("create contact %s: %w", id, err)
	}
	publishContactEvent(ctx, s.events, "contact.created", created.ID.String(), fmt.Sprintf("Contact %s created", created.ID), cmd, created, nil)
	return created, nil
}

// Update changes the registrar's contact: it removes, then adds, statuses,
// and changes the values given. It checks, in order: the contact has no
// pending action and no update prohibition (a clientUpdateProhibited the
// command removes excepted); each status removed or added is a client
// status, and set or not set as the command assumes; and every value
// changed is valid, a postal address of a type the contact does not have
// yet being given whole.
//
// As for hosts, the checks and the write are made on one reading of the
// contact, and an update that another command overtakes is read and
// checked again, up to updateAttempts times.
func (s *EPPContactService) Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateContactCommand) (*entities.Contact, error) {
	for attempt := 1; ; attempt++ {
		c, err := s.update(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrContactChanged) || attempt == updateAttempts {
			return c, err
		}
	}
}

func (s *EPPContactService) update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateContactCommand) (*entities.Contact, error) {
	c, err := s.sponsored(ctx, scope, cmd.ID)
	if err != nil {
		return nil, err
	}
	for _, st := range slices.Concat(cmd.RemoveStatuses, cmd.AddStatuses) {
		if !slices.Contains(clientContactStatuses, st) {
			return nil, fmt.Errorf("%w: %s; a client sets only %s", entities.ErrEPPStatusNotClient, st, strings.Join(clientContactStatuses, ", "))
		}
	}
	switch {
	case c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate:
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPObjectPending, c.ID)
	case c.Status.ServerUpdateProhibited:
		return nil, fmt.Errorf("%w: serverUpdateProhibited", entities.ErrContactUpdateNotAllowed)
	case c.Status.ClientUpdateProhibited && !slices.Contains(cmd.RemoveStatuses, string(entities.ContactStatusClientUpdateProhibited)):
		return nil, fmt.Errorf("%w: clientUpdateProhibited; remove it first", entities.ErrContactUpdateNotAllowed)
	}

	before := copyContactValue(c)
	if err := applyClientContactStatuses(c, cmd.RemoveStatuses, cmd.AddStatuses); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, pi := range cmd.PostalInfo {
		if seen[pi.Type] {
			return nil, fmt.Errorf("%w: %s", entities.ErrEPPDuplicatePostalInfo, pi.Type)
		}
		seen[pi.Type] = true
		if err := changePostalInfo(c, pi); err != nil {
			return nil, err
		}
	}
	// In an update, an empty number removes it.
	if cmd.Voice != nil {
		if err := applyPhone(&c.Voice, "voice", cmd.Voice); err != nil {
			return nil, err
		}
	}
	if cmd.Fax != nil {
		if err := applyPhone(&c.Fax, "fax", cmd.Fax); err != nil {
			return nil, err
		}
	}
	if cmd.Email != nil {
		if c.Email, err = contactEmail(*cmd.Email); err != nil {
			return nil, err
		}
	}
	if cmd.AuthInfo != nil {
		a, err := entities.NewAuthInfoType(*cmd.AuthInfo)
		if err != nil {
			return nil, err
		}
		c.AuthInfo = a
	}
	if cmd.Disclose != nil {
		applyDisclose(&c.Disclose, cmd.Disclose)
	}

	c.UpRr = entities.ClIDType(scope.String())
	if _, err := c.IsValid(); err != nil {
		return nil, err
	}
	updated, err := s.contacts.UpdateContactIfUnchanged(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("update contact %s: %w", c.ID, err)
	}
	publishContactEvent(ctx, s.events, "contact.updated", updated.ID.String(), fmt.Sprintf("Contact %s updated", updated.ID), cmd, updated, before)
	return updated, nil
}

// Delete deletes the registrar's contact, unless it has a pending action or
// a delete prohibition, or a domain uses it.
func (s *EPPContactService) Delete(ctx context.Context, scope entities.RegistrarClID, id string) error {
	c, err := s.sponsored(ctx, scope, id)
	if err != nil {
		return err
	}
	switch {
	case c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate:
		return fmt.Errorf("%w: %s", entities.ErrEPPObjectPending, c.ID)
	case !c.CanBeDeleted():
		return fmt.Errorf("%w: %s", entities.ErrEPPDeleteProhibited, c.ID)
	}
	links, err := s.links(ctx, c)
	if err != nil {
		return err
	}
	if links > 0 {
		return fmt.Errorf("%w: %s", entities.ErrEPPContactLinked, c.ID)
	}
	if err := s.contacts.DeleteContactByID(ctx, c.ID.String()); err != nil {
		return fmt.Errorf("delete contact %s: %w", c.ID, err)
	}
	publishContactEvent(ctx, s.events, "contact.deleted", c.ID.String(), fmt.Sprintf("Contact %s deleted", c.ID), nil, nil, c)
	return nil
}

// sponsored returns the contact with id if the registrar sponsors it:
// another registrar's is ErrEPPNotSponsor (ADR 0006), and none
// ErrContactNotFound.
func (s *EPPContactService) sponsored(ctx context.Context, scope entities.RegistrarClID, raw string) (*entities.Contact, error) {
	id, err := entities.NewClIDType(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, raw)
	}
	c, err := s.contacts.GetContactByID(ctx, id.String())
	switch {
	case errors.Is(err, entities.ErrContactNotFound):
		return nil, fmt.Errorf("%w: %s", entities.ErrContactNotFound, id)
	case err != nil:
		return nil, fmt.Errorf("look up contact %s: %w", id, err)
	case c.ClID.String() != scope.String():
		return nil, fmt.Errorf("%w: contact %s", entities.ErrEPPNotSponsor, id)
	}
	return c, nil
}

// links counts the domains that use c.
func (s *EPPContactService) links(ctx context.Context, c *entities.Contact) (int64, error) {
	n, err := s.contacts.CountContactLinks(ctx, c.ID.String())
	if err != nil {
		return 0, fmt.Errorf("count domains using contact %s: %w", c.ID, err)
	}
	return n, nil
}

// copyContactValue copies a contact and its postal addresses, for the
// event's before state.
func copyContactValue(c *entities.Contact) *entities.Contact {
	cc := *c
	for i, pi := range c.PostalInfo {
		if pi != nil {
			p := *pi
			if pi.Address != nil {
				a := *pi.Address
				p.Address = &a
			}
			cc.PostalInfo[i] = &p
		}
	}
	return &cc
}

// contactEmail validates an email address: a single address (RFC 5322
// addr-spec, without a display name), with a local part of at most 64
// characters and a domain of at least two labels.
func contactEmail(raw string) (string, error) {
	a, err := mail.ParseAddress(raw)
	if err != nil || a.Address != raw {
		return "", fmt.Errorf("%w: %s", entities.ErrInvalidEmail, raw)
	}
	at := strings.LastIndex(raw, "@")
	local, domain := raw[:at], raw[at+1:]
	if len(local) > maxEmailLocalPart {
		return "", fmt.Errorf("%w: the local part is longer than %d characters", entities.ErrInvalidEmail, maxEmailLocalPart)
	}
	if dn, err := entities.NewDomainName(domain); err != nil || !strings.Contains(dn.String(), ".") {
		return "", fmt.Errorf("%w: %s", entities.ErrInvalidEmail, raw)
	}
	return strings.ToLower(raw), nil
}

// applyPhone sets a voice or fax number. An extension is refused.
func applyPhone(set *entities.E164Type, name string, given *commands.EPPPhone) error {
	if given.Ext != nil {
		return fmt.Errorf("%w: <contact:%s x=...>", entities.ErrEPPPhoneExtension, name)
	}
	if given.Number == "" {
		*set = ""
		return nil
	}
	n, err := entities.NewE164Type(given.Number)
	if err != nil {
		return fmt.Errorf("%w: <contact:%s>", err, name)
	}
	*set = *n
	return nil
}

// contactAddress builds an address from a command's. Postal lines are kept
// as the schema delivers them: entities.NormalizeString would drop a
// trailing dot ("Example Inc.").
func contactAddress(a *commands.EPPContactAddress) (*entities.Address, error) {
	cc, err := entities.NewCCType(a.CC)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, a.CC)
	}
	addr := &entities.Address{
		City:          entities.PostalLineType(a.City),
		StateProvince: entities.OptPostalLineType(a.SP),
		PostalCode:    entities.PCType(a.PC),
		CountryCode:   cc,
	}
	streets := []*entities.OptPostalLineType{&addr.Street1, &addr.Street2, &addr.Street3}
	for i, st := range a.Streets {
		*streets[i] = entities.OptPostalLineType(st) // the schema allows at most 3
	}
	return addr, nil
}

// applyPostalInfo adds a whole postal address to a contact being created.
func applyPostalInfo(c *entities.Contact, pi commands.EPPPostalInfo) error {
	i := postalInfoIndex(pi.Type)
	if c.PostalInfo[i] != nil {
		return fmt.Errorf("%w: %s", entities.ErrEPPDuplicatePostalInfo, pi.Type)
	}
	return changePostalInfo(c, pi)
}

// changePostalInfo changes the parts of a postal address that are given. A
// postal address of a type the contact does not have yet must be given
// whole: a name and an address.
func changePostalInfo(c *entities.Contact, pi commands.EPPPostalInfo) error {
	i := postalInfoIndex(pi.Type)
	next := entities.ContactPostalInfo{Type: entities.PostalInfoEnumType(pi.Type)}
	if cur := c.PostalInfo[i]; cur != nil {
		next = *cur
	} else if pi.Name == nil || pi.Addr == nil {
		return fmt.Errorf("%w: %s", entities.ErrEPPPostalInfoIncomplete, pi.Type)
	}
	if pi.Name != nil {
		next.Name = entities.PostalLineType(*pi.Name)
	}
	if pi.Org != nil {
		next.Org = entities.OptPostalLineType(*pi.Org)
	}
	if pi.Addr != nil {
		addr, err := contactAddress(pi.Addr)
		if err != nil {
			return err
		}
		next.Address = addr
	}
	if err := next.Address.Validate(next.Type); err != nil {
		return fmt.Errorf("%w: %s postal address", err, pi.Type)
	}
	if !next.IsValid() {
		return fmt.Errorf("%w: %s postal address; an int one is in 7-bit ASCII", entities.ErrInvalidContactPostalInfo, pi.Type)
	}
	c.PostalInfo[i] = &next
	return nil
}

// postalInfoIndex is where a contact keeps a postal address of type t:
// int first, loc second.
func postalInfoIndex(t string) int {
	if t == string(entities.PostalInfoEnumTypeLOC) {
		return 1
	}
	return 0
}

// applyClientContactStatuses removes, then adds, client statuses: each
// removed must be set, and each added must not be.
func applyClientContactStatuses(c *entities.Contact, remove, add []string) error {
	flag := func(st string) *bool {
		switch entities.ContactStatusType(st) {
		case entities.ContactStatusClientDeleteProhibited:
			return &c.Status.ClientDeleteProhibited
		case entities.ContactStatusClientTransferProhibited:
			return &c.Status.ClientTransferProhibited
		default:
			return &c.Status.ClientUpdateProhibited
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
	c.SetOKStatusIfNeeded()
	c.UnSetOKStatusIfNeeded()
	return nil
}

// applyDisclose sets the disclosure of the elements listed to the flag.
func applyDisclose(d *entities.ContactDisclose, x *commands.EPPDisclose) {
	byType := func(types []string, intFlag, locFlag *bool) {
		for _, t := range types {
			if t == string(entities.PostalInfoEnumTypeLOC) {
				*locFlag = x.Flag
			} else {
				*intFlag = x.Flag
			}
		}
	}
	byType(x.Names, &d.NameInt, &d.NameLoc)
	byType(x.Orgs, &d.OrgInt, &d.OrgLoc)
	byType(x.Addrs, &d.AddrInt, &d.AddrLoc)
	if x.Voice {
		d.Voice = x.Flag
	}
	if x.Fax {
		d.Fax = x.Flag
	}
	if x.Email {
		d.Email = x.Flag
	}
}
