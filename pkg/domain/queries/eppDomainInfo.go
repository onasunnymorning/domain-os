package queries

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// EPPDomainInfo is what a <domain:info> reports: the domain, with its
// nameservers, and the names of the host objects subordinate to it.
type EPPDomainInfo struct {
	Domain           *entities.Domain
	SubordinateHosts []string
	// Limited is the view of a registrar that does not sponsor the domain
	// but gave its transfer code (RFC 9154 §5.3): enough to check a
	// transfer, and no contacts, nameservers or authorization information.
	Limited bool
}
