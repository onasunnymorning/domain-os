package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EPPSessionTracker is what the EPP server asks of the cluster-wide session
// registry (docs/epp/ARCHITECTURE.md §3).
type EPPSessionTracker interface {
	// Caps are the session caps in force.
	Caps() entities.EPPSessionCaps
	// Open registers a session that has completed its TLS handshake, under
	// its source address's cap.
	Open(ctx context.Context, s *entities.EPPSession) error
	// Authenticate records who s logged in as, under the registrar's and the
	// login's caps. Over a cap it returns an *entities.EPPSessionLimitError.
	Authenticate(ctx context.Context, s *entities.EPPSession, login *entities.EPPAuthenticatedLogin) error
	// Renew extends the leases of the node's sessions.
	Renew(ctx context.Context, sessions []*entities.EPPSession) error
	// Close removes a session.
	Close(ctx context.Context, s *entities.EPPSession) error
	// KillRequests delivers the IDs of sessions to end until ctx is done.
	KillRequests(ctx context.Context) (<-chan string, error)
}

// EPPSessionService lets operators see and end live EPP sessions.
type EPPSessionService interface {
	List(ctx context.Context, q queries.ListItemsQuery) ([]*entities.EPPSession, string, error)
	Get(ctx context.Context, id string) (*entities.EPPSession, error)
	// Kill asks the node holding the session to end it.
	Kill(ctx context.Context, id string) (*entities.EPPSession, error)
	// KillMatching ends every session the filter selects, which must not be
	// empty, and returns them.
	KillMatching(ctx context.Context, f queries.ListEPPSessionsFilter) ([]*entities.EPPSession, error)
}
