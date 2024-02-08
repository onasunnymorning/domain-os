package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type CreateRegistrarCommand struct {
	ClID        string                           `json:"clid" binding:"required"`
	Name        string                           `json:"name" binding:"required"`
	Email       string                           `json:"email" binding:"required"`
	PostalInfo  [2]*entities.RegistrarPostalInfo `json:"postalInfo" binding:"required"`
	GurID       int                              `json:"gurid"`
	Voice       string                           `json:"voice"`
	Fax         string                           `json:"fax"`
	URL         string                           `json:"url"`
	RdapBaseURL string                           `json:"rdapBaseUrl"`
	WhoisInfo   *entities.WhoisInfo              `json:"whoisInfo"`
}

type CreateRegistrarCommandResult struct {
	Result entities.Registrar
}
