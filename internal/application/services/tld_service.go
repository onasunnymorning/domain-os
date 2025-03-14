package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

var (
	ErrCannotDeleteTLDWithActivePhases = errors.New("cannot delete TLD with active phases")
	ErrInvalidCreateTLDCommand         = errors.New("invalid create TLD command")
)

// TLDService implements the TLDService interface
type TLDService struct {
	tldRepository repositories.TLDRepository
	dnsRecRepo    repositories.TLDDNSRecordRepository
}

// NewTLDService returns a new TLDService
func NewTLDService(tldRepo repositories.TLDRepository, dnsRecRepo repositories.TLDDNSRecordRepository) *TLDService {
	return &TLDService{
		tldRepository: tldRepo,
		dnsRecRepo:    dnsRecRepo,
	}
}

// CreateTLD creates a new top-level domain (TLD) based on the provided command.
// It validates the command and attempts to create the TLD in the repository.
// If successful, it retrieves and returns the created TLD.
func (svc *TLDService) CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*entities.TLD, error) {
	newTLD, err := entities.NewTLD(cmd.Name, cmd.RyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	err = svc.tldRepository.Create(ctx, newTLD)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	createdTLD, err := svc.tldRepository.GetByName(ctx, strings.ToLower(cmd.Name), false)
	if err != nil {
		return nil, errors.Join(ErrInvalidCreateTLDCommand, err)
	}

	return createdTLD, nil
}

// GetTLDByName gets a TLD by name
func (svc *TLDService) GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	// domain names are case insensitive and we always store them as lowercase
	return svc.tldRepository.GetByName(ctx, strings.ToLower(name), false)
}

// ListTLDs lists all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (svc *TLDService) ListTLDs(ctx context.Context, params queries.ListTldsQuery) ([]*entities.TLD, error) {
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
	return svc.tldRepository.DeleteByName(ctx, name)
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
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a SOA record: %s" + rr.String())
			}
			tldHeader.Soa = *s
		case "NS":
			ns, ok := rr.(*dns.NS)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a NS record: %s" + rr.String())
			}
			tldHeader.Ns = append(tldHeader.Ns, *ns)
		case "A":
			_, ok := rr.(*dns.A)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not an A record: %s" + rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		case "AAAA":
			_, ok := rr.(*dns.AAAA)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not an AAAA record: %s" + rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		case "DS":
			ds, ok := rr.(*dns.DS)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a DS record: %s" + rr.String())
			}
			tldHeader.Ds = append(tldHeader.Ds, *ds)
		case "DNSKEY":
			dnskey, ok := rr.(*dns.DNSKEY)
			if !ok {
				return nil, fmt.Errorf("error converting TLDHeader to string: RR is not a DNSKEY record: %s" + rr.String())
			}
			tldHeader.DNSKey = append(tldHeader.DNSKey, *dnskey)
		}
	}

	return &tldHeader, nil
}

// CountTLDs returns the number of TLDs
func (svc *TLDService) CountTLDs(ctx context.Context) (int64, error) {
	return svc.tldRepository.Count(ctx)
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

	return tld, nil
}
