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

// eppCreate is a create command with its period and contacts resolved.
type eppCreate struct {
	years                            int
	registrant, admin, tech, billing string
}

// The registry's DNSSEC policy for DS records registrars submit. A deposit
// imported from another registry keeps whatever it holds.
var (
	// AcceptedDSAlgorithms are the DNSSEC algorithms (IANA DNS Security
	// Algorithm Numbers) whose DS records are accepted: those RFC 8624 still
	// recommends or allows for signing, less the SHA-1 based RSASHA1 (5, 7),
	// whose use the IETF has since deprecated. RSAMD5 (1), DSA (3, 6) and
	// ECC-GOST (12) are refused as well.
	AcceptedDSAlgorithms = map[uint8]string{
		8: "RSASHA256", 10: "RSASHA512", 13: "ECDSAP256SHA256",
		14: "ECDSAP384SHA384", 15: "ED25519", 16: "ED448",
	}
	// AcceptedDSDigestTypes are the DS digest types accepted: RFC 8624
	// forbids SHA-1 (1) and GOST (3) for delegations.
	AcceptedDSDigestTypes = map[uint8]string{2: "SHA-256", 4: "SHA-384"}
)

// MaxDSRecords is the most DS records one domain may have.
const MaxDSRecords = 8

// Info returns a domain the session's registrar sponsors, with its
// nameservers and the host objects subordinate to it (RFC 5731 §3.1.2). A
// missing domain is entities.ErrDomainNotFound.
//
// Another registrar's domain is entities.ErrEPPNotSponsor (ADR 0006), unless
// the registrar gives the domain's transfer code (authInfo): RFC 9154 §5.3
// lets a gaining registrar check a code before it asks for a transfer. A
// code that matches gets the limited view; one that does not, or any code for
// a domain with none set, is entities.ErrEPPAuthInfoMismatch, so the
// registrar cannot tell whether a code is set.
func (s *EPPDomainService) Info(ctx context.Context, scope entities.RegistrarClID, name string, authInfo *string) (*queries.EPPDomainInfo, error) {
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
		switch {
		case authInfo == nil:
			return nil, entities.ErrEPPNotSponsor
		case !dom.AuthInfo.Matches(*authInfo):
			return nil, entities.ErrEPPAuthInfoMismatch
		}
		return &queries.EPPDomainInfo{Domain: dom, Limited: true}, nil
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
	names, err := s.hosts.ListHostNamesUnder(ctx, clid, domain)
	if err != nil {
		return nil, fmt.Errorf("list hosts under %s: %w", domain, err)
	}
	return names, nil
}

// createAdmission is a create the registry has checked in full
// (admitCreate): the name, the phase it is made in, its period and
// contacts, and its DS records. What is then made of it depends on the
// phase: a registration, or an application.
type createAdmission struct {
	name  *entities.DomainName
	phase *entities.Phase
	c     eppCreate
	ds    []entities.DSRecord
}

// Create registers a domain for the session's registrar (RFC 5731 §3.2.1),
// or files its application for the name when the launch phase a
// <launch:create> names collects applications (RFC 8334 §3.3, ADR 0024
// §3). It applies every rule in one order, so that of several problems
// with a command the first in this order is the one reported: the name;
// its TLD and phase; the registrar's accreditation for the TLD (ADR 0006);
// what the phase admits (a registration or an application, with no
// trademark evidence); the period; the phase's label rules; that the name
// is neither registered nor reserved; the contacts (each typed, at most
// one of each type, the phase's contact policy, each existing and the
// registrar's own); the nameservers (host objects, not attributes; no
// repeats; at most entities.MaxHostsPerDomain; each one of the registrar's
// host objects); the DNSSEC data (DS data only, no maxSigLife, at most
// MaxDSRecords, each well formed with an accepted algorithm and digest
// type, none twice); and the authorization password. Then, in an
// application phase, that the registrar has no open application for the
// name in it.
func (s *EPPDomainService) Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*queries.EPPDomainCreated, error) {
	adm, err := s.admitCreate(ctx, scope, cmd)
	if err != nil {
		return nil, err
	}
	if adm.phase.Launch.Model == entities.RegistrationModelApplication {
		app, err := s.fileApplication(ctx, scope, cmd, adm)
		if err != nil {
			return nil, err
		}
		return &queries.EPPDomainCreated{Application: app}, nil
	}
	dom, err := s.registerAdmitted(ctx, scope, cmd, adm)
	if err != nil {
		return nil, err
	}
	return &queries.EPPDomainCreated{Domain: dom}, nil
}

// admitCreate applies the rules of Create that hold for a registration and
// an application alike, in Create's order.
func (s *EPPDomainService) admitCreate(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*createAdmission, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	dn, err := entities.NewDomainName(cmd.Name)
	if err != nil || !strings.Contains(dn.String(), ".") {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	// The phase: the GA phase in force, or the launch phase a
	// <launch:create> names (ADR 0024). A launch phase that cannot be
	// registered or applied in is refused here, before anything else is
	// read.
	var ref entities.PhaseRef
	if cmd.Launch != nil {
		ref = launchRef(cmd.Launch.Phase)
	}
	t, err := s.tldForCheck(ctx, dn.ParentDomain(), ref)
	if err != nil {
		return nil, err
	}
	switch t.reason {
	case CheckReasonNotServed:
		return nil, entities.ErrEPPTLDNotServed
	case CheckReasonNotOpen:
		return nil, entities.ErrEPPTLDNotOpen
	}
	accredited, err := s.registrars.IsRegistrarAccreditedForTLD(ctx, dn.ParentDomain(), scope.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", entities.ErrCouldNotDetermineAccreditation, err)
	}
	if !accredited {
		return nil, entities.ErrRegistrarNotAccredited
	}
	if err := admitLaunchCreate(t.phase, cmd.Launch); err != nil {
		return nil, err
	}
	adm := &createAdmission{name: dn, phase: t.phase, c: eppCreate{registrant: cmd.RegistrantID}}
	if adm.c.years, err = createYears(cmd, t.phase.Lifecycle().MaxHorizonYears()); err != nil {
		return nil, err
	}
	if !t.phase.Policy.LabelIsAllowed(dn.Label()) {
		return nil, entities.ErrLabelNotValidInPhase
	}
	switch registered, reserved, err := s.registeredOrReserved(ctx, []string{dn.String()}); {
	case err != nil:
		return nil, err
	case registered[dn.String()]:
		return nil, entities.ErrDomainExists
	case reserved[dn.String()]:
		return nil, entities.ErrDomainBlocked
	}
	if err := adm.c.assignContacts(cmd.Contacts); err != nil {
		return nil, err
	}
	if err := checkContactPolicy(t.phase.Policy.ContactDataPolicy, adm.c); err != nil {
		return nil, err
	}
	if err := s.checkContacts(ctx, scope, adm.c); err != nil {
		return nil, err
	}
	if err := s.checkNameservers(ctx, scope, cmd); err != nil {
		return nil, err
	}
	if adm.ds, err = dsRecords(cmd.DSData, cmd.MaxSigLife, cmd.KeyData); err != nil {
		return nil, err
	}
	// RFC 9154 §5.1: a domain may be created without a code (the normal
	// case), and one given must be random enough.
	if cmd.AuthInfo != "" {
		if err := entities.ValidateSecureAuthInfo(cmd.AuthInfo); err != nil {
			return nil, err
		}
	}
	return adm, nil
}

// registerAdmitted registers an admitted create through the registration
// path (DomainService.RegisterDomain), which prices it, assigns its ROID,
// stores it and publishes its event.
func (s *EPPDomainService) registerAdmitted(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand, adm *createAdmission) (*entities.Domain, error) {
	// A launch registration names its phase; a GA one names none, and the
	// registration path resolves the GA phase in force again as it writes.
	var phaseName string
	if adm.phase.Type == entities.PhaseTypeLaunch {
		phaseName = adm.phase.Name.String()
	}
	dom, err := s.register.RegisterDomain(ctx, &commands.RegisterDomainCommand{
		Name: adm.name.String(), ClID: scope.String(), AuthInfo: cmd.AuthInfo,
		RegistrantID: adm.c.registrant, AdminID: adm.c.admin, TechID: adm.c.tech, BillingID: adm.c.billing,
		Years: adm.c.years, HostNames: cmd.HostNames, DSRecords: adm.ds, PhaseName: phaseName,
	})
	if err != nil {
		// Another registrar may have registered the name since it was
		// checked; that is "exists", whatever the registration path said.
		if _, lookupErr := s.domains.GetDomainByName(ctx, adm.name.String(), false); lookupErr == nil {
			return nil, entities.ErrDomainExists
		}
		return nil, err
	}
	return dom, nil
}

// admitLaunchCreate applies what a create may do under phase (ADR 0024
// §2). Without a <launch:create> the phase is the GA phase, whose zero
// settings admit a registration. With one: the phase must not need
// trademark evidence verified (sunrise, claims: ErrLaunchModeUnsupported,
// until roadmap 9a-9c); the type attribute, if given, must be what the
// phase makes (RFC 8334 §3.3: type="application" in a first-come phase,
// or type="registration" in an application phase, is
// ErrLaunchCreateTypeMismatch); and the create must carry no trademark
// evidence, which nothing verifies yet (ErrLaunchEvidenceUnsupported).
func admitLaunchCreate(phase *entities.Phase, launch *commands.EPPLaunchCreate) error {
	if launch == nil {
		return phase.Launch.AdmitsRegistration()
	}
	if phase.Launch.RequiresVerification() {
		return fmt.Errorf("%w: %s", entities.ErrLaunchModeUnsupported, phase.Name)
	}
	applications := phase.Launch.Model == entities.RegistrationModelApplication
	switch {
	case launch.Type == "application" && !applications:
		return fmt.Errorf("%w: %s registers at once", entities.ErrLaunchCreateTypeMismatch, phase.Name)
	case launch.Type == "registration" && applications:
		return fmt.Errorf("%w: %s collects applications", entities.ErrLaunchCreateTypeMismatch, phase.Name)
	}
	if launch.Marks+launch.Notices > 0 {
		return entities.ErrLaunchEvidenceUnsupported
	}
	return nil
}

// dsRecords applies the registry's DNSSEC policy to the DS data given, in
// this order: no maxSigLife, DS data rather than key data, at most
// MaxDSRecords, every record well formed, every algorithm and digest type
// accepted, and none twice.
func dsRecords(given []commands.EPPDSData, maxSigLife int, keyData bool) ([]entities.DSRecord, error) {
	switch {
	case maxSigLife != 0:
		return nil, entities.ErrEPPMaxSigLife
	case keyData:
		return nil, entities.ErrEPPKeyData
	case len(given) > MaxDSRecords:
		return nil, fmt.Errorf("%w: at most %d", entities.ErrEPPTooManyDS, MaxDSRecords)
	}
	// Every record's structure first, then the policy on what is accepted:
	// a malformed record is reported as malformed whatever else is wrong.
	out, err := wellFormedDS(given)
	if err != nil {
		return nil, err
	}
	if err := acceptedDS(out); err != nil {
		return nil, err
	}
	// SetDSRecords refuses duplicates; check here so the refusal comes in
	// this command's order, before anything is registered.
	if err := (&entities.Domain{}).SetDSRecords(out); err != nil {
		return nil, err
	}
	return out, nil
}

// wellFormedDS builds the DS records given, each well formed.
func wellFormedDS(given []commands.EPPDSData) ([]entities.DSRecord, error) {
	out := make([]entities.DSRecord, 0, len(given))
	for _, d := range given {
		if d.KeyTag < 0 || d.KeyTag > 65535 || d.Algorithm < 0 || d.Algorithm > 255 || d.DigestType < 0 || d.DigestType > 255 {
			return nil, fmt.Errorf("%w: a field is out of range", entities.ErrInvalidDSRecord)
		}
		rec, err := entities.NewDSRecord(uint16(d.KeyTag), uint8(d.Algorithm), uint8(d.DigestType), d.Digest) // #nosec G115 -- range-checked above
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// acceptedDS applies the registry's policy on algorithms and digest types.
func acceptedDS(records []entities.DSRecord) error {
	for _, rec := range records {
		if _, ok := AcceptedDSAlgorithms[rec.Algorithm]; !ok {
			return fmt.Errorf("%w: %d", entities.ErrEPPDSAlgorithm, rec.Algorithm)
		}
		if _, ok := AcceptedDSDigestTypes[rec.DigestType]; !ok {
			return fmt.Errorf("%w: %d", entities.ErrEPPDSDigestType, rec.DigestType)
		}
	}
	return nil
}

// createYears is the registration period in years: one if none was given,
// months only in whole years, and within 1 to maxYears.
func createYears(cmd commands.EPPCreateDomainCommand, maxYears int) (int, error) {
	years := 1
	if cmd.Period != 0 {
		years = cmd.Period
		if cmd.PeriodUnit == "m" {
			if cmd.Period%12 != 0 {
				return 0, entities.ErrEPPPeriodNotWholeYears
			}
			years = cmd.Period / 12
		}
	}
	if years < 1 || years > maxYears {
		return 0, fmt.Errorf("%w: 1 to %d years", entities.ErrEPPInvalidPeriod, maxYears)
	}
	return years, nil
}

// assignContacts puts each typed contact in its slot.
func (c *eppCreate) assignContacts(contacts []commands.EPPDomainContact) error {
	for _, ct := range contacts {
		var slot *string
		switch ct.Type {
		case "admin":
			slot = &c.admin
		case "tech":
			slot = &c.tech
		case "billing":
			slot = &c.billing
		default:
			return entities.ErrEPPContactTypeMissing
		}
		if *slot != "" {
			return fmt.Errorf("%w: %s", entities.ErrEPPDuplicateContactType, ct.Type)
		}
		*slot = ct.ID
	}
	return nil
}

// checkNameservers requires host objects, each named once, at most
// entities.MaxHostsPerDomain of them, each one of the registrar's.
func (s *EPPDomainService) checkNameservers(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) error {
	if cmd.HostAttributes {
		return entities.ErrEPPHostAttributes
	}
	seen := map[string]bool{}
	for _, h := range cmd.HostNames {
		name := strings.ToLower(h)
		if seen[name] {
			return fmt.Errorf("%w: %s is named more than once", entities.ErrDuplicateHost, name)
		}
		seen[name] = true
	}
	if len(cmd.HostNames) > entities.MaxHostsPerDomain {
		return entities.ErrMaxHostsPerDomainExceeded
	}
	for _, h := range cmd.HostNames {
		if _, err := s.hosts.GetHostByNameAndClID(ctx, strings.ToLower(h), scope.String()); err != nil {
			if errors.Is(err, entities.ErrHostNotFound) {
				return fmt.Errorf("%w: %s", entities.ErrHostNotFound, h)
			}
			return fmt.Errorf("look up host %s: %w", h, err)
		}
	}
	return nil
}

// checkContactPolicy applies the phase's contact policy to the contacts
// given: a mandatory one must be there, and a prohibited one must not.
// (entities.Domain.ApplyContactDataPolicy drops a prohibited contact
// silently; over EPP the registrar is told instead.)
func checkContactPolicy(p entities.ContactDataPolicy, c eppCreate) error {
	for _, c := range []struct {
		kind   string
		policy entities.ContactDataPolicyType
		id     string
	}{
		{"registrant", p.RegistrantContactDataPolicy, c.registrant},
		{"admin", p.AdminContactDataPolicy, c.admin},
		{"tech", p.TechContactDataPolicy, c.tech},
		{"billing", p.BillingContactDataPolicy, c.billing},
	} {
		switch {
		case c.policy == entities.ContactDataPolicyTypeMandatory && c.id == "":
			return fmt.Errorf("%w: %s", entities.ErrEPPContactRequired, c.kind)
		case c.policy == entities.ContactDataPolicyTypeProhibited && c.id != "":
			return fmt.Errorf("%w: %s", entities.ErrEPPContactProhibited, c.kind)
		}
	}
	return nil
}

// checkContacts requires every contact given to exist and to be sponsored
// by the session's registrar.
func (s *EPPDomainService) checkContacts(ctx context.Context, scope entities.RegistrarClID, c eppCreate) error {
	seen := map[string]bool{}
	for _, id := range []string{c.registrant, c.admin, c.tech, c.billing} {
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
			return fmt.Errorf("%w: contact %s", entities.ErrEPPNotSponsor, id)
		}
	}
	return nil
}
