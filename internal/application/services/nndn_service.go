package services

import (
	"context"
	"errors"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// NNDNService implements the NNDNService interface
type NNDNService struct {
	nndnRepository repositories.NNDNRepository
}

// NewNNDNService returns a new instance of NNDNService
func NewNNDNService(nndnRepo repositories.NNDNRepository) *NNDNService {
	return &NNDNService{
		nndnRepository: nndnRepo,
	}
}

// CreateNNDN handles the creation of a new NNDN
func (svc *NNDNService) CreateNNDN(ctx context.Context, cmd *commands.CreateNNDNCommand) (*entities.NNDN, error) {
	newNNDN, err := entities.NewNNDN(cmd.Name)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidNNDN, err)
	}
	if cmd.Reason != "" {
		r, err := entities.NewClIDType(cmd.Reason)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidNNDN, err)
		}
		newNNDN.Reason = r
	}

	_, err = svc.nndnRepository.CreateNNDN(ctx, newNNDN)
	if err != nil {
		if errors.Is(err, entities.ErrDuplicateNNDN) {
			return nil, errors.Join(entities.ErrInvalidNNDN, err)
		}
		return nil, err
	}

	return newNNDN, nil
}

// GetNNDNByName retrieves an NNDN by its name
func (svc *NNDNService) GetNNDNByName(ctx context.Context, name string) (*entities.NNDN, error) {
	return svc.nndnRepository.GetNNDN(ctx, strings.ToLower(name))
}

// ListNNDNs retrieves a list of NNDNs with pagination support
func (svc *NNDNService) ListNNDNs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.NNDN, string, error) {
	return svc.nndnRepository.ListNNDNs(ctx, params)
}

// DeleteNNDNByName deletes an NNDN by its name
func (svc *NNDNService) DeleteNNDNByName(ctx context.Context, name string) error {
	return svc.nndnRepository.DeleteNNDN(ctx, name)
}
