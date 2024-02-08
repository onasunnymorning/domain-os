package postgres

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// ToDBSpec5Label converts a Spec5Label struct to a DBSpec5Label struct
func ToDBSpec5Label(label *entities.Spec5Label) *Spec5Label {
	return &Spec5Label{
		Label:     label.Label,
		Type:      label.Type,
		CreatedAt: label.CreatedAt,
	}
}

// ToSpec5Label converts a DBSpec5Label struct to a Spec5Label struct
func ToSpec5Label(label *Spec5Label) *entities.Spec5Label {
	return &entities.Spec5Label{
		Label:     label.Label,
		Type:      label.Type,
		CreatedAt: label.CreatedAt,
	}
}
