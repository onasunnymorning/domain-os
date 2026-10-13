package services

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPSessionService tracks live EPP sessions in the cluster-wide registry and
// lets operators list and end them. The EPP server uses it as an
// interfaces.EPPSessionTracker, the admin API as an
// interfaces.EPPSessionService.
type EPPSessionService struct {
	registry repositories.EPPSessionRegistry
	caps     entities.EPPSessionCaps
	lease    time.Duration
	events   repositories.EventPublisher
	now      func() time.Time
}

// NewEPPSessionService returns a service over registry. The EPP server
// passes its caps and lease; the admin API, which only lists and kills, can
// pass zero values. events may be nil.
func NewEPPSessionService(registry repositories.EPPSessionRegistry, caps entities.EPPSessionCaps, lease time.Duration, events repositories.EventPublisher) *EPPSessionService {
	return &EPPSessionService{registry: registry, caps: caps, lease: lease, events: events, now: time.Now}
}

// Caps are the session caps in force.
func (s *EPPSessionService) Caps() entities.EPPSessionCaps { return s.caps }

// Open registers sess under its source address's cap.
func (s *EPPSessionService) Open(ctx context.Context, sess *entities.EPPSession) error {
	return s.registry.Open(ctx, sess, s.lease, s.caps.PerSource)
}

// Authenticate records who sess logged in as, under the registrar's cap and
// the login's own cap or the default.
func (s *EPPSessionService) Authenticate(ctx context.Context, sess *entities.EPPSession, login *entities.EPPAuthenticatedLogin) error {
	sess.Authenticate(login, s.now())
	return s.registry.Authenticate(ctx, sess, s.lease, s.caps.PerRegistrar, s.caps.ForLogin(login.SessionCap))
}

// Renew extends the leases of sessions.
func (s *EPPSessionService) Renew(ctx context.Context, sessions []*entities.EPPSession) error {
	return s.registry.Renew(ctx, sessions, s.lease)
}

// Close removes sess.
func (s *EPPSessionService) Close(ctx context.Context, sess *entities.EPPSession) error {
	return s.registry.Close(ctx, sess)
}

// KillRequests delivers the IDs of sessions to end.
func (s *EPPSessionService) KillRequests(ctx context.Context) (<-chan string, error) {
	return s.registry.KillRequests(ctx)
}

// List returns a page of live sessions.
func (s *EPPSessionService) List(ctx context.Context, q queries.ListItemsQuery) ([]*entities.EPPSession, string, error) {
	return s.registry.List(ctx, q)
}

// Get returns a live session.
func (s *EPPSessionService) Get(ctx context.Context, id string) (*entities.EPPSession, error) {
	return s.registry.Get(ctx, id)
}

// Kill asks the node holding the session to end it.
func (s *EPPSessionService) Kill(ctx context.Context, id string) (*entities.EPPSession, error) {
	sess, err := s.registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.registry.RequestKill(ctx, id); err != nil {
		return nil, err
	}
	s.announce(ctx, []*entities.EPPSession{sess}, "")
	return sess, nil
}

// KillMatching ends every session f selects. An empty filter is refused: it
// would end every session on the platform.
func (s *EPPSessionService) KillMatching(ctx context.Context, f queries.ListEPPSessionsFilter) ([]*entities.EPPSession, error) {
	if f.IsEmpty() {
		return nil, entities.ErrInvalidEPPSessionFilter
	}
	var all []*entities.EPPSession
	q := queries.ListItemsQuery{PageSize: 500, Filter: f}
	for {
		page, next, err := s.registry.List(ctx, q)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		q.PageCursor = next
	}
	if len(all) == 0 {
		return all, nil
	}
	ids := make([]string, len(all))
	for i, sess := range all {
		ids[i] = sess.ID
	}
	if err := s.registry.RequestKill(ctx, ids...); err != nil {
		return nil, err
	}
	s.announce(ctx, all, f.ToQueryParams())
	return all, nil
}

// announce records each kill in the outbox, as an audit of operator action.
// It is best effort: the kill has already been requested.
func (s *EPPSessionService) announce(ctx context.Context, killed []*entities.EPPSession, filter string) {
	if s.events == nil {
		return
	}
	actor, _ := appcontext.UserID(ctx)
	events := make([]entities.DomainEvent, 0, len(killed))
	for _, sess := range killed {
		subject := string(sess.ClID)
		if subject == "" {
			subject = sess.ClientIP.String()
		}
		e := entities.NewDomainEvent(eppEventSource, entities.EventEPPSessionKilled, subject,
			fmt.Sprintf("EPP session %s ended by an operator", sess.ID),
			entities.EPPSessionKilledData{SessionID: sess.ID, ClID: sess.ClID, LoginID: sess.LoginID, ClientIP: sess.ClientIP.String(), Node: sess.Node, Filter: filter})
		e.Actor = actor
		if v, ok := appcontext.TraceID(ctx); ok {
			e.TraceID = v
		}
		events = append(events, e)
	}
	_ = s.events.Publish(ctx, events...)
}
