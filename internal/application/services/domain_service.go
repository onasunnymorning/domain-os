package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// DomainService immplements the DomainService interface
type DomainService struct {
	domainRepository repositories.DomainRepository
	hostRepository   repositories.HostRepository
	roidService      RoidService
}

// NewDomainService returns a new instance of a DomainService
func NewDomainService(dRepo repositories.DomainRepository, hRepo repositories.HostRepository, roidService RoidService) *DomainService {
	return &DomainService{
		domainRepository: dRepo,
		hostRepository:   hRepo,
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

// UpdateDomain Updates a new domain from a create domain command
func (s *DomainService) UpdateDomain(ctx context.Context, name string, upDom *commands.UpdateDomainCommand) (*entities.Domain, error) {
	// Look up the domain
	dom, err := s.domainRepository.GetDomainByName(ctx, name, false)
	if err != nil {
		return nil, err
	}
	// Make the changes
	dom.OriginalName = entities.DomainName(upDom.OriginalName)
	dom.UName = entities.DomainName(upDom.UName)
	dom.RegistrantID = entities.ClIDType(upDom.RegistrantID)
	dom.AdminID = entities.ClIDType(upDom.AdminID)
	dom.TechID = entities.ClIDType(upDom.TechID)
	dom.BillingID = entities.ClIDType(upDom.BillingID)
	dom.CrRr = entities.ClIDType(upDom.CrRr)
	dom.UpRr = entities.ClIDType(upDom.UpRr)
	dom.ExpiryDate = upDom.ExpiryDate
	dom.AuthInfo = entities.AuthInfoType(upDom.AuthInfo)
	dom.CreatedAt = upDom.CreatedAt
	dom.UpdatedAt = upDom.UpdatedAt
	dom.Status = upDom.Status
	dom.RGPStatus = upDom.RGPStatus

	// Validate the domain
	if err := dom.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// Save and return
	updatedDomain, err := s.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	return updatedDomain, nil
}

// GetDomainByName retrieves a domain by its name from the repository
func (s *DomainService) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	return s.domainRepository.GetDomainByName(ctx, name, preloadHosts)
}

// DeleteDomainByName deletes a domain by its name from the repository
func (s *DomainService) DeleteDomainByName(ctx context.Context, name string) error {
	return s.domainRepository.DeleteDomainByName(ctx, name)
}

// ListDomains returns a list of domains
func (s *DomainService) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListDomains(ctx, pageSize, cursor)
}

// AddHostToDomain adds a host to a domain
func (s *DomainService) AddHostToDomain(ctx context.Context, name string, roid string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, true)
	if err != nil {
		return err
	}
	domRoidInt, err := dom.RoID.Int64()
	if err != nil {
		return err
	}

	// Get the host
	hRoid := entities.RoidType(roid)
	if err := hRoid.Validate(); err != nil {
		return err
	}
	if hRoid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return entities.ErrInvalidHostRoID
	}
	hostRoidInt, err := hRoid.Int64()
	if err != nil {
		return err
	}

	host, err := s.hostRepository.GetHostByRoid(ctx, hostRoidInt)
	if err != nil {
		return err
	}

	// Add the host to the domain
	_, err = dom.AddHost(host)
	if err != nil {
		return err
	}

	// If no error, save the association to the DB and return
	return s.domainRepository.AddHostToDomain(ctx, domRoidInt, hostRoidInt)

}

// RemoveHostFromDomain removes a host from a domain
func (s *DomainService) RemoveHostFromDomain(ctx context.Context, name string, roid string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, true)
	if err != nil {
		return err
	}
	fmt.Println(dom)
	domRoidInt, err := dom.RoID.Int64()
	if err != nil {
		return err
	}

	// Get the host
	hRoid := entities.RoidType(roid)
	if err := hRoid.Validate(); err != nil {
		return err
	}
	if hRoid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return entities.ErrInvalidHostRoID
	}
	hostRoidInt, err := hRoid.Int64()
	if err != nil {
		return err
	}

	host, err := s.hostRepository.GetHostByRoid(ctx, hostRoidInt)
	if err != nil {
		return err
	}

	fmt.Println()
	// Remove the host from the domain
	err = dom.RemoveHost(host)
	if err != nil {
		return err
	}

	// If no error, save the association to the DB and return
	return s.domainRepository.RemoveHostFromDomain(ctx, domRoidInt, hostRoidInt)
}
