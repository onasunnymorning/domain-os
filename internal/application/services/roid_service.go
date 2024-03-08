package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// RoidService implements the RoidService interface
type RoidService struct {
	idGenerator repositories.IDGenerator
}

// NewRoidService returns a new RoidService
func NewRoidService(idGenerator repositories.IDGenerator) *RoidService {
	return &RoidService{
		idGenerator: idGenerator,
	}
}

// GenerateRoid generates a new Roid
func (r *RoidService) GenerateRoid(objectType string) (entities.RoidType, error) {
	return entities.NewRoidType(r.idGenerator.GenerateID(), objectType)
}

// ListNode returns the Node ID of the IDGenerator so we can register it and ensure its uniqueness
func (r *RoidService) ListNode() int64 {
	return r.idGenerator.ListNode()
}
