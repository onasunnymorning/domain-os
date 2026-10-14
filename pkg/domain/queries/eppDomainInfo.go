package queries

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// EPPDomainInfo is what a <domain:info> reports: the domain, with its
// nameservers, and the names of the host objects subordinate to it.
type EPPDomainInfo struct {
	Domain           *entities.Domain
	SubordinateHosts []string
}
