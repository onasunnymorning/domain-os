package commands

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// SaveRDAPPublicationPolicyCommand creates or replaces an RDAP publication
// policy (PUT /rdap/publication-policies/{name}). Every published role gives
// every field a rule; a role left out is not published. The entity
// validates it (INV-08).
type SaveRDAPPublicationPolicyCommand struct {
	Description    string                                                              `json:"description"`
	Roles          map[entities.RDAPRole]map[entities.RDAPField]entities.RDAPFieldRule `json:"roles" binding:"required"`
	ContactFormURI string                                                              `json:"contactFormUri"`
}

// AssignRDAPPublicationPolicyCommand assigns a TLD its RDAP publication
// policy (PUT /tlds/{name}/rdap/publication-policy). Empty removes the
// assignment, so the TLD has the built-in for its type.
type AssignRDAPPublicationPolicyCommand struct {
	Policy string `json:"policy"`
}

// SaveRDAPAccessPolicyCommand creates or replaces an RDAP access policy
// (PUT /rdap/access-policies/{name}). The entity validates it (INV-08).
type SaveRDAPAccessPolicyCommand struct {
	Description string                    `json:"description"`
	Limits      entities.RDAPAccessLimits `json:"limits" binding:"required"`
}

// AssignRDAPAccessNetworkCommand assigns a network an RDAP access policy
// (PUT /rdap/access-networks). An empty policy removes the assignment, so
// the network's clients have "anonymous".
type AssignRDAPAccessNetworkCommand struct {
	Network string `json:"network" binding:"required"`
	Policy  string `json:"policy"`
}
