package rest

import "github.com/onasunnymorning/domain-os/internal/application/interfaces"

// DomainController
type DomainController struct {
	domainService interfaces.DomainService
}

func NewDomainController(domService interfaces.DomainService) *DomainController {
	controller := &DomainController{
		domainService: domService,
	}

	// ADD ROUTES

	return controller
}
