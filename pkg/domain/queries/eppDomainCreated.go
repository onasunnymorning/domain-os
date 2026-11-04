package queries

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// EPPDomainCreated is what a <domain:create> made: the domain, registered
// at once (a GA or first-come create), or the application filed for the
// name in a launch phase that collects applications (RFC 8334 §3.3, ADR
// 0024 §3). Exactly one is set.
type EPPDomainCreated struct {
	Domain      *entities.Domain
	Application *entities.LaunchApplication
}
