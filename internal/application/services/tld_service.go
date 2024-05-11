package services

import (
	"context"
	"errors"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/mappers"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

var (
	ErrCannotDeleteTLDWithActivePhases = errors.New("cannot delete TLD with active phases")
)

// TLDService implements the TLDService interface
type TLDService struct {
	tldRepository repositories.TLDRepository
}

// NewTLDService returns a new TLDService
func NewTLDService(tldRepo repositories.TLDRepository) *TLDService {
	return &TLDService{
		tldRepository: tldRepo,
	}
}

// CreateTLD creates a new TLD
func (svc *TLDService) CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*commands.CreateTLDCommandResult, error) {
	newTLD, err := entities.NewTLD(cmd.Name)
	if err != nil {
		return nil, err
	}

	err = svc.tldRepository.Create(ctx, newTLD)
	if err != nil {
		return nil, err
	}

	var result commands.CreateTLDCommandResult
	result.Result = mappers.NewTLDResultFromTLD(newTLD)

	return &result, nil
}

// GetTLDByName gets a TLD by name
func (svc *TLDService) GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	// domain names are case insensitive and we always store them as lowercase
	return svc.tldRepository.GetByName(ctx, strings.ToLower(name), false)
}

// ListTLDs lists all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (svc *TLDService) ListTLDs(ctx context.Context, pageSize int, pageCursor string) ([]*entities.TLD, error) {
	return svc.tldRepository.List(ctx, pageSize, pageCursor)
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
