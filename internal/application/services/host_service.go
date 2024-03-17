package services

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// HostService is the implementation of the HostService interface
type HostService struct {
	hostRepository    repositories.HostRepository
	addressRepository repositories.HostAddressRepository
	roidService       RoidService
}

// NewHostService creates a new instance of HostService
func NewHostService(hostRepository repositories.HostRepository, addressRepository repositories.HostAddressRepository, roidService RoidService) *HostService {
	return &HostService{
		hostRepository:    hostRepository,
		addressRepository: addressRepository,
		roidService:       roidService,
	}
}

// CreateHost creates a new host including its optional addresses
func (s *HostService) CreateHost(ctx context.Context, cmd *commands.CreateHostCommand) (*entities.Host, error) {
	roid, err := s.roidService.GenerateRoid("host")
	if err != nil {
		return nil, err
	}
	host, err := entities.NewHost(cmd.Name, roid.String(), cmd.ClID.String())
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidHost, err)
	}
	// Add the addresses
	for _, a := range cmd.Addresses {
		_, err := host.AddAddress(a)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidHost, err)
		}
	}
	// Set the optional paramerters
	if cmd.CrRr != "" {
		host.CrRr = cmd.CrRr
	}
	if cmd.UpRr != "" {
		host.UpRr = cmd.UpRr
	}
	if !cmd.Status.IsNil() {
		host.Status = cmd.Status
	}

	// Check if we still have a valid host
	if err := host.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidHost, err)
	}

	dbHost, err := s.hostRepository.CreateHost(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, a := range host.Addresses {
		roidInt64, _ := dbHost.RoID.Int64()
		_, err := s.addressRepository.CreateHostAddress(ctx, roidInt64, &a)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidHost, err)
		}
	}

	return dbHost, nil
}

// GetHostByRoid gets a host by its roid in string format
func (s *HostService) GetHostByRoID(ctx context.Context, roidString string) (*entities.Host, error) {
	roid := entities.RoidType(roidString)
	if err := roid.Validate(); err != nil {
		return nil, err
	}
	if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return nil, entities.ErrInvalidRoid
	}
	roidInt, err := roid.Int64()
	if err != nil {
		return nil, err
	}
	return s.hostRepository.GetHostByRoid(ctx, roidInt)
}

// DeleteHostByRoid deletes a host by its roid in string format
func (s *HostService) DeleteHostByRoID(ctx context.Context, roidString string) error {
	roid := entities.RoidType(roidString)
	if err := roid.Validate(); err != nil {
		return err
	}
	if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return entities.ErrInvalidRoid
	}
	roidInt, err := roid.Int64()
	if err != nil {
		return err
	}
	return s.hostRepository.DeleteHostByRoid(ctx, roidInt)
}

// ListHosts lists hosts
func (s *HostService) ListHosts(ctx context.Context, pageSize int, cursor string) ([]*entities.Host, error) {
	return s.hostRepository.ListHosts(ctx, pageSize, cursor)
}

// AddHostAddress adds an ip address to an existing host
func (s *HostService) AddHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error) {
	roid := entities.RoidType(roidString)
	if err := roid.Validate(); err != nil {
		return nil, err
	}
	if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return nil, entities.ErrInvalidRoid
	}
	roidInt64, err := roid.Int64()
	if err != nil {
		return nil, err
	}
	// Get the host
	host, err := s.hostRepository.GetHostByRoid(ctx, roidInt64)
	if err != nil {
		return nil, err
	}

	// Add the addresses
	a, err := host.AddAddress(ip)
	if err != nil {
		// If its already there we return the host and make this idempotent
		if errors.Is(err, entities.ErrDuplicateHostAddress) {
			return host, nil
		}
		return nil, err
	}

	// Save the address to the repository
	_, err = s.addressRepository.CreateHostAddress(ctx, roidInt64, a)
	if err != nil {
		return nil, err
	}

	return host, nil
}

// RemoveAddress removes and ip address from an existing host
func (s *HostService) RemoveHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error) {
	roid := entities.RoidType(roidString)
	if err := roid.Validate(); err != nil {
		return nil, err
	}
	if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return nil, entities.ErrInvalidRoid
	}
	roidInt64, err := roid.Int64()
	if err != nil {
		return nil, err
	}
	// Get the host
	host, err := s.hostRepository.GetHostByRoid(ctx, roidInt64)
	if err != nil {
		return nil, err
	}

	// Add the addresses
	a, err := host.RemoveAddress(ip)
	if err != nil {
		// If its not there, we return the host and make this idempotent
		if errors.Is(err, entities.ErrHostAddressNotFound) {
			return host, nil
		}
		return nil, err
	}

	// Save the address to the repository
	err = s.addressRepository.DeleteHostAddressByHostRoidAndAddress(ctx, roidInt64, a)
	if err != nil {
		return nil, err
	}

	return host, nil
}
