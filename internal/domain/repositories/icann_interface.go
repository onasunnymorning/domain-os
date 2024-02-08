package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// ICANNRepository is the interface for the ICANNRepository
type ICANNRepository interface {
	// List the spec5 labels from the ICANN Spec5 Registry
	ListSpec5Labels() ([]*entities.Spec5Label, error)
}
