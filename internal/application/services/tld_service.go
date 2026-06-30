package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

var (
	ErrCannotDeleteTLDWithActivePhases = errors.New("cannot delete TLD with active phases")
	ErrInvalidCreateTLDCommand         = errors.New("invalid create TLD command")
)

// TLDService implements the TLDService interface
type TLDService struct {
	tldRepository repositories.TLDRepository
	dnsRecRepo    repositories.TLDDNSRecordRepository
	rarRepo       repositories.RegistrarRepository
	accRepo       repositories.AccreditationRepository
	ryRepo        repositories.RegistryOperatorRepository
	eventPub      repositories.EventPublisher
}

// NewTLDService returns a new TLDService.
// The optional dependencies (rarRepo, accRepo, ryRepo, eventPub) enable auto-provisioning
// of operator registrar accounts (9998/9999) when creating a TLD. If any are nil,
// auto-provisioning is silently skipped.
func NewTLDService(
	tldRepo repositories.TLDRepository,
	dnsRecRepo repositories.TLDDNSRecordRepository,
	opts ...TLDServiceOption,
) *TLDService {
	svc := &TLDService{
		tldRepository: tldRepo,
		dnsRecRepo:    dnsRecRepo,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// TLDServiceOption is a functional option for TLDService.
type TLDServiceOption func(*TLDService)

// WithOperatorRegistrarDeps configures the dependencies needed for auto-creating
// operator registrar accounts (9998/9999) when a TLD is created.
func WithOperatorRegistrarDeps(
	rarRepo repositories.RegistrarRepository,
	accRepo repositories.AccreditationRepository,
	ryRepo repositories.RegistryOperatorRepository,
	eventPub repositories.EventPublisher,
) TLDServiceOption {
	return func(svc *TLDService) {
		svc.rarRepo = rarRepo
		svc.accRepo = accRepo
		svc.ryRepo = ryRepo
		svc.eventPub = eventPub
	}
}

// CreateTLD creates a new top-level domain (TLD) based on the provided command.
// It validates the command and attempts to create the TLD in the repository.
// If successful, it retrieves and returns the created TLD.
// When CreateOperatorRegistrars is true and the required dependencies are configured,
// it also creates the 9998-{tld} and 9999-{tld} registrar accounts and accredits them.
func (svc *TLDService) CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*entities.TLD, error) {
	newTLD, err := entities.NewTLD(cmd.Name, cmd.RyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	newTLD.AllowEscrowImport = cmd.AllowEscrowImport

	err = svc.tldRepository.Create(ctx, newTLD)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	createdTLD, err := svc.tldRepository.GetByName(ctx, strings.ToLower(cmd.Name), false)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	// Auto-provision operator registrar accounts (9998/9999) if requested and deps are available
	if cmd.CreateOperatorRegistrars && svc.canProvisionOperatorRegistrars() {
		svc.provisionOperatorRegistrars(ctx, createdTLD, cmd.RyID)
	}

	svc.publishTLDEvent(ctx, "tld.created", createdTLD.Name.String(), fmt.Sprintf("TLD %s created", createdTLD.Name.String()), cmd, createdTLD, nil)

	return createdTLD, nil
}


// GetTLDByName gets a TLD by name
func (svc *TLDService) GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	// domain names are case insensitive and we always store them as lowercase
	return svc.tldRepository.GetByName(ctx, strings.ToLower(name), preloadAll)
}

// ListTLDs lists all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (svc *TLDService) ListTLDs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	return svc.tldRepository.List(ctx, params)
}

// DeleteTLDByName deletes a TLD by name. To prevent accidental deletions, we check if there are no active phases for the TLD before deleting it.
func (svc *TLDService) DeleteTLDByName(ctx context.Context, name string) error {
	tld, err := svc.tldRepository.GetByName(ctx, name, false)
	if err != nil {
		if err == entities.ErrTLDNotFound {
			// if there is no TLD with the given name, nothing to do, be idempotent
			return nil
		}
		return err
	}

	if len(tld.GetCurrentPhases()) != 0 {
		return ErrCannotDeleteTLDWithActivePhases
	}
	err = svc.tldRepository.DeleteByName(ctx, name)
	if err != nil {
		return err
	}

	svc.publishTLDEvent(ctx, "tld.deleted", name, fmt.Sprintf("TLD %s deleted", name), nil, nil, tld)
	return nil
}

// GetTLDHeader gets a TLD header
func (s *TLDService) GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error) {
	// Collect the DNSRecords for the TLD
	rec, err := s.dnsRecRepo.GetByZone(ctx, name)
	if err != nil {
		return nil, err
	}
	// Create our return object
	var tldHeader entities.TLDHeader

	// Convert them to dns.RR records
	for _, r := range rec {
		// Convert the DNSRecord to a dns.RR
		rr, err := r.ToRR()
		if err != nil {
			return nil, err
		}
		// Append the RR to the appropriate slice or set soa
		switch r.Type {
		case "SOA":
			s, ok := rr.(*dns.SOA)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a SOA record: %s", rr.String())
			}
			tldHeader.Soa = *s
		case "NS":
			ns, ok := rr.(*dns.NS)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a NS record: %s", rr.String())
			}
			tldHeader.Ns = append(tldHeader.Ns, *ns)
		case "A":
			_, ok := rr.(*dns.A)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not an A record: %s", rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		case "AAAA":
			_, ok := rr.(*dns.AAAA)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not an AAAA record: %s", rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		case "DS":
			ds, ok := rr.(*dns.DS)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a DS record: %s", rr.String())
			}
			tldHeader.Ds = append(tldHeader.Ds, *ds)
		case "DNSKEY":
			dnskey, ok := rr.(*dns.DNSKEY)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a DNSKEY record: %s", rr.String())
			}
			tldHeader.DNSKey = append(tldHeader.DNSKey, *dnskey)
		}
	}

	return &tldHeader, nil
}

// CountTLDs returns the number of TLDs
func (svc *TLDService) CountTLDs(ctx context.Context, filter queries.ListTldsFilter) (int64, error) {
	return svc.tldRepository.Count(ctx, filter)
}

// SetAllowEscrowImport sets the AllowEscrowImport flag for a TLD
// It will validate the action at the domain layer and return an error if the action is invalid
func (svc *TLDService) SetAllowEscrowImport(ctx context.Context, tldName string, allowEscrowImport bool) (*entities.TLD, error) {
	// Get the tld from the repository
	// Include the phases in the query!
	tld, err := svc.tldRepository.GetByName(ctx, tldName, false)
	if err != nil {
		return nil, err
	}

	var prevTLD entities.TLD
	if tld != nil {
		prevTLD = *tld
	}

	// Use the tld method to set the flag
	err = tld.ToggleAllowEscrowImport(allowEscrowImport)
	if err != nil {
		return nil, err
	}

	// Save the tld back to the repository
	err = svc.tldRepository.Update(ctx, tld)
	if err != nil {
		return nil, err
	}

	svc.publishTLDEvent(ctx, "tld.allow_escrow_import_updated", tld.Name.String(), fmt.Sprintf("TLD %s allow escrow import set to %t", tld.Name.String(), allowEscrowImport), allowEscrowImport, tld, &prevTLD)

	return tld, nil
}

// canProvisionOperatorRegistrars returns true if all optional dependencies
// needed for operator registrar auto-provisioning are configured.
func (svc *TLDService) canProvisionOperatorRegistrars() bool {
	return svc.rarRepo != nil && svc.accRepo != nil && svc.ryRepo != nil && svc.eventPub != nil
}

// provisionOperatorRegistrars creates the 9998-{tld} (billable) and 9999-{tld}
// (non-billable) registrar accounts and accredits them for the given TLD.
// Errors are logged as warnings but do not fail the TLD creation.
func (svc *TLDService) provisionOperatorRegistrars(ctx context.Context, tld *entities.TLD, ryID string) {
	tldName := strings.ToLower(tld.Name.String())

	// Build a RegistrarService to use its Create method (with event publishing)
	rarService := NewRegistrarService(svc.rarRepo, svc.eventPub)

	type opRar struct {
		gurID int
		clID  string
		name  string
	}

	registrars := []opRar{
		{9998, fmt.Sprintf("9998-%s", tldName), fmt.Sprintf("%s - Reserved - Billable", tldName)},
		{9999, fmt.Sprintf("9999-%s", tldName), fmt.Sprintf("%s - Reserved - Non-Billable", tldName)},
	}

	for _, r := range registrars {
		// Create the registrar
		pi := svc.dummyPostalInfo()
		cmd := &commands.CreateRegistrarCommand{
			ClID:       r.clID,
			Name:       r.name,
			GurID:      r.gurID,
			Email:      "reserved@operator.local",
			PostalInfo: [2]*entities.RegistrarPostalInfo{pi},
			Status:     string(entities.RegistrarStatusOK),
			IANAStatus: entities.IANARegistrarStatusReserved,
		}

		_, err := rarService.Create(ctx, cmd)
		if err != nil {
			log.Printf("⚠️ auto-provision operator registrar %s for TLD %s: create failed (may already exist): %v", r.clID, tldName, err)
			continue
		}
		log.Printf("✅ auto-provisioned operator registrar %s (GurID %d) for TLD %s", r.clID, r.gurID, tldName)

		// Auto-accredit for this TLD
		if accErr := svc.accRepo.CreateAccreditation(ctx, tldName, r.clID); accErr != nil {
			log.Printf("⚠️ auto-accredit operator registrar %s for TLD %s failed: %v", r.clID, tldName, accErr)
		} else {
			log.Printf("✅ auto-accredited operator registrar %s for TLD %s", r.clID, tldName)
		}
	}
}

// dummyPostalInfo creates a placeholder postal info for operator registrar accounts.
func (svc *TLDService) dummyPostalInfo() *entities.RegistrarPostalInfo {
	a, _ := entities.NewAddress("Reserved", "US")
	pi, _ := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, a)
	return pi
}

func (svc *TLDService) publishTLDEvent(
	ctx context.Context,
	eventType string,
	subject string,
	msg string,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	if svc.eventPub == nil {
		return
	}
	data := map[string]interface{}{
		"tld_name":  subject,
		"timestamp": time.Now().UTC(),
	}

	domainEvent := entities.NewDomainEvent(
		"domain-os/api",
		eventType,
		subject,
		msg,
		data,
	)
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		domainEvent.TraceID = traceID
	}
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		domainEvent.CorrelationID = correlationID
	}
	domainEvent.Command = command
	domainEvent.BeforeState = previousState
	domainEvent.AfterState = newState
	if actor, ok := ctx.Value("userid").(string); ok {
		domainEvent.Actor = actor
	}

	if err := svc.eventPub.Publish(ctx, domainEvent); err != nil {
		log.Printf("failed to publish tld event %s: %v", eventType, err)
	}
}
