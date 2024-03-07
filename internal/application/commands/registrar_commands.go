package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type CreateRegistrarCommand struct {
	ClID        string                           `json:"ClID" binding:"required"`
	Name        string                           `json:"Name" binding:"required"`
	Email       string                           `json:"Email" binding:"required"`
	PostalInfo  [2]*entities.RegistrarPostalInfo `json:"PostalInfo" binding:"required"`
	GurID       int                              `json:"GurID"`
	Voice       string                           `json:"Voice"`
	Fax         string                           `json:"Fax"`
	URL         string                           `json:"URL"`
	RdapBaseURL string                           `json:"RdapBaseURL"`
	WhoisInfo   *entities.WhoisInfo              `json:"WhoisInfo"`
}

type CreateRegistrarCommandResult struct {
	Result entities.Registrar
}
