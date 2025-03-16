package services

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// HostService is the implementation of the HostService interface
type HostService struct {
	hostRepository    repositories.HostRepository
	addressRepository repositories.HostAddressRepository
	roidService       interfaces.RoidService
}

// NewHostService creates a new instance of HostService
func NewHostService(hostRepository repositories.HostRepository, addressRepository repositories.HostAddressRepository, roidService interfaces.RoidService) *HostService {
	return &HostService{
		hostRepository:    hostRepository,
		addressRepository: addressRepository,
		roidService:       roidService,
	}
}

// CreateHost creates a new host including its optional addresses
func (s *HostService) CreateHost(ctx context.Context, cmd *commands.CreateHostCommand) (*entities.Host, error) {
	// Convert the command to a host and validate it
	host, err := s.createHostFromCreateHostCommand(cmd)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidHost, err)
	}

	dbHost, err := s.hostRepository.CreateHost(ctx, host)
	if err != nil {
		if errors.Is(err, entities.ErrHostAlreadyExists) {
			return nil, errors.Join(entities.ErrInvalidHost, err)
		}
		return nil, err
	}

	roidInt64, err := dbHost.RoID.Int64() // use the RoID that was just created
	if err != nil {
		return nil, fmt.Errorf("error converting system generated RoID of created host (%s) to int64", dbHost.RoID)
	}

	// Validate and save the addresses
	for _, a := range cmd.Addresses {
		addr, err := netip.ParseAddr(a)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidHost, err)
		}
		_, err = s.addressRepository.CreateHostAddress(ctx, roidInt64, &addr)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidHost, err)
		}
	}

	// Read the host with Addresses from the DB to make sure all is saved correctly
	dbHost, err = s.hostRepository.GetHostByRoid(ctx, roidInt64)
	if err != nil {
		return nil, err
	}

	return dbHost, nil
}

// BulkCreate creates multiple hosts in a single transaction. If addresses are provided, they will be created as well
// Should one of the hosts fail to be created, the operation fails and no hosts are created, the error will be returned
func (s *HostService) BulkCreate(ctx context.Context, cmds []*commands.CreateHostCommand) error {

	// Create a slice of hosts
	hosts := make([]*entities.Host, 0, len(cmds))
	for _, cmd := range cmds {
		host, err := s.createHostFromCreateHostCommand(cmd)
		if err != nil {
			return errors.Join(entities.ErrInvalidHost, err)
		}
		hosts = append(hosts, host)
	}

	// Create the hosts in the repository
	return s.hostRepository.BulkCreate(ctx, hosts)
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
func (s *HostService) ListHosts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error) {
	return s.hostRepository.ListHosts(ctx, params)
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

// GetHostByNameAndClID gets a host by its name and clid
func (s *HostService) GetHostByNameAndClID(ctx context.Context, name string, clid string) (*entities.Host, error) {
	return s.hostRepository.GetHostByNameAndClID(ctx, name, clid)
}

// createHostFromCreateHostCommand creates a new host from a CreateHostCommand.
// if a RoID is not provided, it will generate a new one, otherwise it will use the provided RoID
// if the RoID or any of the attributes are invalid, it will return an error
func (s *HostService) createHostFromCreateHostCommand(cmd *commands.CreateHostCommand) (*entities.Host, error) {
	var roid entities.RoidType
	var err error
	if cmd.RoID == "" {
		// Generate a Roid if none is provided
		roid, err = s.roidService.GenerateRoid("host")
		if err != nil {
			return nil, err
		}
	} else {
		// Validate the provided Roid
		roid = entities.RoidType(cmd.RoID)
		if err := roid.Validate(); err != nil {
			return nil, err
		}
		if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
			return nil, entities.ErrInvalidObjectIdentifier
		}
	}

	host, err := entities.NewHost(cmd.Name, roid.String(), cmd.ClID.String())
	if err != nil {
		return nil, err
	}
	// Add the addresses
	for _, a := range cmd.Addresses {
		_, err := host.AddAddress(a)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return host, nil
}
