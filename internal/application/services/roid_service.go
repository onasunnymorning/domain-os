package services

import (
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RoidService implements the RoidService interface
type RoidService struct {
	idGenerator repositories.IDGenerator
	repository  entities.RepositoryID
}

// NewRoidService returns a RoidService that mints ROIDs with the repository
// ID the process was configured with (config.LoadRepositoryID). A zero
// RoidService, as the read-only processes hold, mints nothing: its unset ID
// fails validation.
func NewRoidService(idGenerator repositories.IDGenerator, repository entities.RepositoryID) *RoidService {
	return &RoidService{
		idGenerator: idGenerator,
		repository:  repository,
	}
}

// GenerateRoid generates a new Roid
func (r *RoidService) GenerateRoid(objectType string) (entities.RoidType, error) {
	// Checked before an ID is drawn: a zero RoidService has no generator.
	if err := r.repository.Validate(); err != nil {
		return "", err
	}
	return entities.NewRoidType(r.idGenerator.GenerateID(), objectType, r.repository)
}

// RepositoryID returns the repository ID this service mints ROIDs with.
func (r *RoidService) RepositoryID() entities.RepositoryID {
	return r.repository
}

// ListNode returns the Node ID of the IDGenerator so we can register it and ensure its uniqueness
func (r *RoidService) ListNode() int64 {
	return r.idGenerator.ListNode()
}
