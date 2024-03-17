package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type CreateDomainCommand struct {
	RoID         string                   `json:"RoID"`
	Name         string                   `json:"Name" binding:"required"`
	OriginalName string                   `json:"OriginalName"`
	UName        string                   `json:"UName"`
	RegistrantID string                   `json:"RegistrantID" binding:"required"`
	AdminID      string                   `json:"AdminID" binding:"required"`
	TechID       string                   `json:"TechID" binding:"required"`
	BillingID    string                   `json:"BillingID" binding:"required"`
	ClID         string                   `json:"ClID" binding:"required"`
	CrRr         string                   `json:"CrRr"`
	UpRr         string                   `json:"UpRr"`
	ExpiryDate   time.Time                `json:"ExpiryDate"`
	AuthInfo     string                   `json:"AuthInfo"`
	CreatedAt    time.Time                `json:"CreatedAt"`
	UpdatedAt    time.Time                `json:"UpdatedAt"`
	Status       entities.DomainStatus    `json:"Status"`
	RGPStatus    entities.DomainRGPStatus `json:"RGPStatus"`
}
