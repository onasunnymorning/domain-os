package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPPublicationPolicyService manages RDAP publication policies and their
// assignment to TLDs (ADR 0021).
type RDAPPublicationPolicyService interface {
	ListPolicies(ctx context.Context) ([]*entities.RDAPPublicationPolicy, error)
	GetPolicy(ctx context.Context, name string) (*entities.RDAPPublicationPolicy, error)
	SavePolicy(ctx context.Context, name string, cmd commands.SaveRDAPPublicationPolicyCommand) (*entities.RDAPPublicationPolicy, error)
	DeletePolicy(ctx context.Context, name string) error
	TLDPolicy(ctx context.Context, tld string) (*entities.RDAPTLDPublication, error)
	AssignPolicy(ctx context.Context, tld string, cmd commands.AssignRDAPPublicationPolicyCommand) (*entities.RDAPTLDPublication, error)
}
