package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// CreateDomainCommand is a command to create a domain
type CreateDomainCommand struct {
	RoID         string                   `json:"RoID"` // if not provided, it will be generated
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
	ExpiryDate   time.Time                `json:"ExpiryDate" binding:"required"`
	AuthInfo     string                   `json:"AuthInfo"  binding:"required"`
	CreatedAt    time.Time                `json:"CreatedAt"`
	UpdatedAt    time.Time                `json:"UpdatedAt"`
	Status       entities.DomainStatus    `json:"Status"`
	RGPStatus    entities.DomainRGPStatus `json:"RGPStatus"`
}

// UpdateDomainCommand is a command to update a domain. RoID and Name are not updatable, please delete and create a new domain if you need to change these fields
type UpdateDomainCommand struct {
	OriginalName string                   `json:"OriginalName"`
	UName        string                   `json:"UName"`
	RegistrantID string                   `json:"RegistrantID" binding:"required"`
	AdminID      string                   `json:"AdminID" binding:"required"`
	TechID       string                   `json:"TechID" binding:"required"`
	BillingID    string                   `json:"BillingID" binding:"required"`
	ClID         string                   `json:"ClID" binding:"required"`
	CrRr         string                   `json:"CrRr"`
	UpRr         string                   `json:"UpRr"`
	ExpiryDate   time.Time                `json:"ExpiryDate" binding:"required"`
	AuthInfo     string                   `json:"AuthInfo"  binding:"required"`
	CreatedAt    time.Time                `json:"CreatedAt"`
	UpdatedAt    time.Time                `json:"UpdatedAt"`
	Status       entities.DomainStatus    `json:"Status"`
	RGPStatus    entities.DomainRGPStatus `json:"RGPStatus"`
}
