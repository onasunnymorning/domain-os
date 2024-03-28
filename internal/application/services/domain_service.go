package services

import (
	"errors"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// DomainService immplements the DomainService interface
type DomainService struct {
	domainRepository repositories.DomainRepository
	roidService      RoidService
}

// NewDomainService returns a new instance of a DomainService
func NewDomainService(repo repositories.DomainRepository, roidService RoidService) *DomainService {
	return &DomainService{
		domainRepository: repo,
		roidService:      roidService,
	}
}

// CreateDomain creates a new domain from a create domain command
func (s *DomainService) CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) (*entities.Domain, error) {
	var roid entities.RoidType
	var err error
	if cmd.RoID == "" {
		// Generate a RoID if none is supplied
		roid, err = s.roidService.GenerateRoid("domain")
		if err != nil {
			return nil, err
		}
	} else {
		roid = entities.RoidType(cmd.RoID) // Validity will be checked in NewDomain
	}
	d, err := entities.NewDomain(roid.String(), cmd.Name, cmd.ClID, cmd.AuthInfo)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	// Set the optional fields
	if cmd.OriginalName != "" {
		d.OriginalName = entities.DomainName(strings.ToLower(cmd.OriginalName))
	}
	if cmd.UName != "" {
		d.UName = entities.DomainName(strings.ToLower(cmd.UName))
	}
	if cmd.RegistrantID != "" {
		d.RegistrantID = entities.ClIDType(cmd.RegistrantID)
	}
	if cmd.AdminID != "" {
		d.AdminID = entities.ClIDType(cmd.AdminID)
	}
	if cmd.TechID != "" {
		d.TechID = entities.ClIDType(cmd.TechID)
	}
	if cmd.BillingID != "" {
		d.BillingID = entities.ClIDType(cmd.BillingID)
	}
	if cmd.CrRr != "" {
		d.CrRr = entities.ClIDType(cmd.CrRr)
	}
	if cmd.UpRr != "" {
		d.UpRr = entities.ClIDType(cmd.UpRr)
	}
	if !cmd.ExpiryDate.IsZero() {
		d.ExpiryDate = cmd.ExpiryDate
	}
	if !cmd.CreatedAt.IsZero() {
		d.CreatedAt = cmd.CreatedAt
	}
	if !cmd.UpdatedAt.IsZero() {
		d.UpdatedAt = cmd.UpdatedAt
	}
	if !cmd.Status.IsNil() {
		d.Status = cmd.Status
	}
	if !cmd.RGPStatus.IsNil() {
		d.RGPStatus = cmd.RGPStatus
	}
	// Check if the domain is valid
	if err := d.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// Save the domain
	createdDomain, err := s.domainRepository.CreateDomain(ctx, d)
	if err != nil {
		return nil, err
	}

	return createdDomain, nil
}

// GetDomainByName retrieves a domain by its name from the repository
func (s *DomainService) GetDomainByName(ctx context.Context, name string) (*entities.Domain, error) {
	return s.domainRepository.GetDomainByName(ctx, name)
}

// DeleteDomainByName deletes a domain by its name from the repository
func (s *DomainService) DeleteDomainByName(ctx context.Context, name string) error {
	return s.domainRepository.DeleteDomainByName(ctx, name)
}

// ListDomains returns a list of domains
func (s *DomainService) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListDomains(ctx, pageSize, cursor)
}
