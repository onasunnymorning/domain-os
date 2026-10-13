package repositories

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EPPSessionRegistry holds every live EPP session across the nodes
// (docs/epp/ARCHITECTURE.md §3). A session is a lease: its node renews it,
// and it disappears when its lease runs out, so a crashed node's sessions
// expire on their own and counts cannot drift.
//
// Open and Authenticate check a cap and take the slot in one atomic step, so
// concurrent connects cannot overshoot it. Over a cap they return an
// *entities.EPPSessionLimitError. When the registry cannot answer they wrap
// entities.ErrEPPSessionsUnavailable.
type EPPSessionRegistry interface {
	// Open registers a session that has completed its TLS handshake, unless
	// its source address already holds perSource live sessions.
	Open(ctx context.Context, s *entities.EPPSession, lease time.Duration, perSource int) error
	// Authenticate records the login of an open session, unless its
	// registrar already holds perRegistrar sessions or its login perLogin.
	Authenticate(ctx context.Context, s *entities.EPPSession, lease time.Duration, perRegistrar, perLogin int) error
	// Renew extends the leases of sessions and rewrites what they record. A
	// session missing from the registry, for instance after an outage, is
	// put back without a cap check.
	Renew(ctx context.Context, sessions []*entities.EPPSession, lease time.Duration) error
	// Close removes a session.
	Close(ctx context.Context, s *entities.EPPSession) error
	// Get returns a live session, or entities.ErrEPPSessionNotFound.
	Get(ctx context.Context, id string) (*entities.EPPSession, error)
	// List returns the live sessions matching q.Filter, a
	// queries.ListEPPSessionsFilter, ordered by ID, one page at a time
	// (INV-11). The returned cursor is empty on the last page.
	List(ctx context.Context, q queries.ListItemsQuery) ([]*entities.EPPSession, string, error)
	// RequestKill asks the node holding each session to end it.
	RequestKill(ctx context.Context, ids ...string) error
	// KillRequests delivers the IDs of sessions to end until ctx is done.
	KillRequests(ctx context.Context) (<-chan string, error)
}
