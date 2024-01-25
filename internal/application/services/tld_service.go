package services

import (
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/mappers"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
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
func (svc *TLDService) CreateTLD(cmd *commands.CreateTLDCommand) (*commands.CreateTLDCommandResult, error) {
	newTLD, err := entities.NewTLD(cmd.Name)
	if err != nil {
		return nil, err
	}

	err = svc.tldRepository.Create(newTLD)
	if err != nil {
		return nil, err
	}

	var result commands.CreateTLDCommandResult
	result.Result = mappers.NewTLDResultFromTLD(newTLD)

	return &result, nil
}

// GetTLDByName gets a TLD by name
func (svc *TLDService) GetTLDByName(name string) (*entities.TLD, error) {
	// domain names are case insensitive and we always store them as lowercase
	return svc.tldRepository.GetByName(strings.ToLower(name))
}

// ListTLDs lists all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (svc *TLDService) ListTLDs(pageSize int, pageCursor string) ([]*entities.TLD, error) {
	return svc.tldRepository.List(pageSize, pageCursor)
}

// DeleteTLDByName deletes a TLD by name
func (svc *TLDService) DeleteTLDByName(name string) error {
	return svc.tldRepository.DeleteByName(name)
}
