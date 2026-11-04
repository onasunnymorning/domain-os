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

// Launch applications over EPP (RFC 8334, ADR 0024 §3). In a launch phase
// whose registration model is application, Create files an application
// instead of registering (fileApplication); <launch:info>, <launch:update>
// and <launch:delete> then read, change and withdraw it, by its
// applicationID, for the registrar that filed it. Allocation is the
// operator's, through the admin API, not a registrar's command.

// errApplicationsNotConfigured is a create in an application phase on a
// registry wired without an application store or an ID minter: a
// deployment fault, answered 2400.
var errApplicationsNotConfigured = errors.New("application phases are not configured on this server")

// fileApplication files an admitted create as an application: a new ID,
// the create's contacts, nameservers, DS records, period and code, and the
// quote the registration will be billed at allocation. The store refuses a
// second open application by the registrar for the name in the phase
// (entities.ErrApplicationExists).
func (s *EPPDomainService) fileApplication(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand, adm *createAdmission) (*entities.LaunchApplication, error) {
	if s.applications == nil || s.roids == nil {
		return nil, errApplicationsNotConfigured
	}
	id, err := s.roids.GenerateRoid(entities.RoidTypeApplication)
	if err != nil {
		return nil, fmt.Errorf("mint an application ID: %w", err)
	}
	app, err := entities.NewLaunchApplication(id, adm.name.String(), scope.String(), cmd.AuthInfo, adm.phase, adm.c.years, s.now())
	if err != nil {
		return nil, err
	}
	app.RegistrantID, app.AdminID, app.TechID, app.BillingID = entities.ClIDType(adm.c.registrant), entities.ClIDType(adm.c.admin), entities.ClIDType(adm.c.tech), entities.ClIDType(adm.c.billing)
	if err := app.SetHostNames(cmd.HostNames); err != nil {
		return nil, err
	}
	if err := app.SetDSRecords(adm.ds); err != nil {
		return nil, err
	}
	// The phase as resolved was read from the TLD's phase list, without its
	// prices; the quote needs them.
	priced, err := s.phases.GetPhaseByTLDAndName(ctx, adm.phase.TLDName.String(), adm.phase.Name.String())
	if err != nil {
		return nil, fmt.Errorf("read phase %s of %s: %w", adm.phase.Name, adm.phase.TLDName, err)
	}
	quote, err := s.register.QuoteRegistration(ctx, adm.name.String(), scope.String(), adm.c.years, priced)
	if err != nil {
		return nil, fmt.Errorf("quote %s in %s: %w", adm.name, adm.phase.Name, err)
	}
	app.Quote = entities.ApplicationQuote{Currency: quote.Price.Currency().Code, Amount: quote.Price.Amount(), Class: quote.Class}
	if err := app.Validate(); err != nil {
		return nil, err
	}
	stored, err := s.applications.CreateApplication(ctx, app)
	if errors.Is(err, entities.ErrDomainAlreadyExists) {
		return nil, ErrDomainExists // registered since it was checked
	}
	return stored, err
}

// InfoApplication implements interfaces.EPPDomainService.
func (s *EPPDomainService) InfoApplication(ctx context.Context, scope entities.RegistrarClID, name string, ref commands.EPPLaunchRef) (*entities.LaunchApplication, error) {
	return s.sponsoredApplication(ctx, scope, name, ref)
}

// UpdateApplication implements interfaces.EPPDomainService. It checks, in
// order: the application is the registrar's, for the name, in the phase
// given; it is not final; the update changes no statuses and restores
// nothing; nameservers are host objects and DNSSEC data DS data, without
// urgent or maxSigLife; each contact and nameserver removed is the
// application's and each added is not, at most one contact of each type,
// each new contact and nameserver the registrar's own, within the phase's
// contact policy; the DS records as a domain update's; and a new code keeps
// to the registry's rules. The checks and the write are made on one reading
// of the application, up to updateAttempts times.
func (s *EPPDomainService) UpdateApplication(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateDomainCommand) (*entities.LaunchApplication, error) {
	for attempt := 1; ; attempt++ {
		app, err := s.updateApplication(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrApplicationChanged) || attempt == updateAttempts {
			return app, err
		}
	}
}

func (s *EPPDomainService) updateApplication(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateDomainCommand) (*entities.LaunchApplication, error) {
	if cmd.Launch == nil {
		return nil, entities.ErrApplicationIDRequired
	}
	app, err := s.sponsoredApplication(ctx, scope, cmd.Name, *cmd.Launch)
	if err != nil {
		return nil, err
	}
	if err := app.CanChange(); err != nil {
		return nil, err
	}
	switch {
	case cmd.Restore != nil, len(cmd.AddStatuses)+len(cmd.RemoveStatuses) > 0:
		return nil, entities.ErrApplicationUpdateUnsupported
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
	phase, err := s.phases.GetPhaseByTLDAndName(ctx, app.TLDName.String(), app.PhaseName.String())
	if err != nil {
		return nil, fmt.Errorf("read phase %s of %s: %w", app.PhaseName, app.TLDName, err)
	}
	if err := s.applyApplicationContacts(ctx, scope, app, phase, cmd); err != nil {
		return nil, err
	}
	if err := s.applyApplicationNameservers(ctx, scope, app, cmd.RemoveHostNames, cmd.AddHostNames); err != nil {
		return nil, err
	}
	if cmd.DNSSEC != nil {
		d := entities.Domain{DSRecords: slices.Clone(app.DSRecords)}
		if err := applyDS(&d, cmd.DNSSEC); err != nil {
			return nil, err
		}
		app.DSRecords = d.DSRecords
	}
	// RFC 9154 §5.2, as for a domain: an empty code or <domain:null/>
	// unsets the code; a code given must be random enough, and is kept
	// hashed.
	switch {
	case cmd.AuthInfoNull || (cmd.AuthInfo != nil && *cmd.AuthInfo == ""):
		app.AuthInfo = ""
	case cmd.AuthInfo != nil:
		if err := entities.ValidateSecureAuthInfo(*cmd.AuthInfo); err != nil {
			return nil, err
		}
		h, err := entities.HashAuthInfo(*cmd.AuthInfo)
		if err != nil {
			return nil, err
		}
		app.AuthInfo = h
	}
	if err := app.Validate(); err != nil {
		return nil, err
	}
	return s.applications.UpdateApplicationIfUnchanged(ctx, app)
}

// applyApplicationContacts removes, then adds, contacts, and changes the
// registrant, as applyDomainContacts does for a domain; then applies the
// phase's contact policy.
func (s *EPPDomainService) applyApplicationContacts(ctx context.Context, scope entities.RegistrarClID, app *entities.LaunchApplication, phase *entities.Phase, cmd commands.EPPUpdateDomainCommand) error {
	slot := func(t string) (*entities.ClIDType, error) {
		switch t {
		case "admin":
			return &app.AdminID, nil
		case "tech":
			return &app.TechID, nil
		case "billing":
			return &app.BillingID, nil
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
		switch c.Type {
		case "admin":
			added.admin = c.ID
		case "tech":
			added.tech = c.ID
		case "billing":
			added.billing = c.ID
		}
	}
	if cmd.Registrant != nil {
		app.RegistrantID = entities.ClIDType(*cmd.Registrant)
		added.registrant = *cmd.Registrant
	}
	// Only contacts newly given are checked as the registrar's: the ones
	// the application keeps were checked when they were set.
	if err := s.checkContacts(ctx, scope, added); err != nil {
		return err
	}
	return checkContactPolicy(phase.Policy.ContactDataPolicy, eppCreate{
		registrant: app.RegistrantID.String(), admin: app.AdminID.String(), tech: app.TechID.String(), billing: app.BillingID.String(),
	})
}

// applyApplicationNameservers removes, then adds, nameservers: each removed
// must be the application's, each added one of the registrar's host
// objects and not already named, at most entities.MaxHostsPerDomain.
func (s *EPPDomainService) applyApplicationNameservers(ctx context.Context, scope entities.RegistrarClID, app *entities.LaunchApplication, remove, add []string) error {
	names := slices.Clone(app.HostNames)
	for _, raw := range remove {
		name := strings.ToLower(raw)
		i := slices.Index(names, name)
		if i < 0 {
			return fmt.Errorf("%w: %s", entities.ErrEPPNameserverNotSet, name)
		}
		names = slices.Delete(names, i, i+1)
	}
	for _, raw := range add {
		name := strings.ToLower(raw)
		if slices.Contains(names, name) {
			return fmt.Errorf("%w: %s is already a nameserver", entities.ErrDuplicateHost, name)
		}
		if _, err := s.hosts.GetHostByNameAndClID(ctx, name, scope.String()); err != nil {
			if errors.Is(err, entities.ErrHostNotFound) {
				return fmt.Errorf("%w: %s", entities.ErrHostNotFound, name)
			}
			return fmt.Errorf("look up host %s: %w", name, err)
		}
		names = append(names, name)
	}
	return app.SetHostNames(names)
}

// DeleteApplication implements interfaces.EPPDomainService: it withdraws
// the application, removing it, while it is not final.
func (s *EPPDomainService) DeleteApplication(ctx context.Context, scope entities.RegistrarClID, name string, ref commands.EPPLaunchRef) error {
	for attempt := 1; ; attempt++ {
		err := s.deleteApplication(ctx, scope, name, ref)
		if !errors.Is(err, entities.ErrApplicationChanged) || attempt == updateAttempts {
			return err
		}
	}
}

func (s *EPPDomainService) deleteApplication(ctx context.Context, scope entities.RegistrarClID, name string, ref commands.EPPLaunchRef) error {
	app, err := s.sponsoredApplication(ctx, scope, name, ref)
	if err != nil {
		return err
	}
	if err := app.CanChange(); err != nil {
		return err
	}
	return s.applications.DeleteApplicationIfUnchanged(ctx, app)
}

// sponsoredApplication returns the application ref names, if the
// registrar filed it (another registrar's is entities.ErrEPPNotSponsor,
// ADR 0006), it is for name, and it is in the phase ref gives, by name
// and by mode (entities.ErrApplicationMismatch). ref must name an
// application (entities.ErrApplicationIDRequired): registered names take
// no launch extension on info, update or delete.
func (s *EPPDomainService) sponsoredApplication(ctx context.Context, scope entities.RegistrarClID, name string, ref commands.EPPLaunchRef) (*entities.LaunchApplication, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if s.applications == nil {
		return nil, errApplicationsNotConfigured
	}
	dn, err := entities.NewDomainName(name)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	if ref.ApplicationID == "" {
		return nil, entities.ErrApplicationIDRequired
	}
	app, err := s.applications.GetApplicationByID(ctx, entities.RoidType(ref.ApplicationID))
	if err != nil {
		return nil, err
	}
	if app.ClID != entities.ClIDType(scope) {
		return nil, entities.ErrEPPNotSponsor
	}
	if app.Name != *dn {
		return nil, fmt.Errorf("%w: %s is for %s", entities.ErrApplicationMismatch, app.ID, app.Name)
	}
	if ref.Phase.Name != "" && app.PhaseName.String() != ref.Phase.Name {
		return nil, fmt.Errorf("%w: %s is in %s", entities.ErrApplicationMismatch, app.ID, app.PhaseName)
	}
	if ref.Phase.Value != "" {
		phase, err := s.phases.GetPhaseByTLDAndName(ctx, app.TLDName.String(), app.PhaseName.String())
		if err != nil {
			return nil, fmt.Errorf("read phase %s of %s: %w", app.PhaseName, app.TLDName, err)
		}
		if phase.Launch.Mode != entities.LaunchMode(ref.Phase.Value) {
			return nil, fmt.Errorf("%w: %s is in %s, a %s phase", entities.ErrApplicationMismatch, app.ID, app.PhaseName, phase.Launch.Mode)
		}
	}
	return app, nil
}
