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
// passes its source cap and lease; the admin API, which only lists and
// kills, can pass zero values. events may be nil.
func NewEPPSessionService(registry repositories.EPPSessionRegistry, caps entities.EPPSessionCaps, lease time.Duration, events repositories.EventPublisher) *EPPSessionService {
	return &EPPSessionService{registry: registry, caps: caps, lease: lease, events: events, now: time.Now}
}

// Caps are the session caps in force.
func (s *EPPSessionService) Caps() entities.EPPSessionCaps { return s.caps }

// Open registers sess under its source address's cap.
func (s *EPPSessionService) Open(ctx context.Context, sess *entities.EPPSession) error {
	return s.registry.Open(ctx, sess, s.lease, s.caps.PerSource)
}

// Authenticate records who sess logged in as, under limits: the caps of the
// registrar's EPP policy, the login's own cap, and its family's.
func (s *EPPSessionService) Authenticate(ctx context.Context, sess *entities.EPPSession, login *entities.EPPAuthenticatedLogin, limits entities.EPPSessionLimits) error {
	sess.Authenticate(login, s.now())
	sess.Family = limits.Family
	return s.registry.Authenticate(ctx, sess, s.lease, limits)
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

// Kill asks the node holding the session to end it. The request is
// recorded in the outbox first, and is not sent if it cannot be recorded.
func (s *EPPSessionService) Kill(ctx context.Context, id string) (*entities.EPPSession, error) {
	sess, err := s.registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.record(ctx, []*entities.EPPSession{sess}, ""); err != nil {
		return nil, err
	}
	if err := s.registry.RequestKill(ctx, id); err != nil {
		return nil, err
	}
	return sess, nil
}

// KillMatching ends every session f selects. An empty filter is refused: it
// would end every session on the platform. Like Kill, the requests are
// recorded before they are sent.
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
	if err := s.record(ctx, all, f.ToQueryParams()); err != nil {
		return nil, err
	}
	ids := make([]string, len(all))
	for i, sess := range all {
		ids[i] = sess.ID
	}
	if err := s.registry.RequestKill(ctx, ids...); err != nil {
		return nil, err
	}
	return all, nil
}

// record writes one epp.session.kill_requested event per session, as the
// audit of operator action. Without an event publisher (the EPP server, which
// never kills on request) there is nothing to record.
func (s *EPPSessionService) record(ctx context.Context, sessions []*entities.EPPSession, filter string) error {
	if s.events == nil {
		return nil
	}
	actor, _ := appcontext.UserID(ctx)
	events := make([]entities.DomainEvent, 0, len(sessions))
	for _, sess := range sessions {
		subject := string(sess.ClID)
		if subject == "" {
			subject = sess.ClientIP.String()
		}
		e := entities.NewDomainEvent(eppEventSource, entities.EventEPPSessionKillRequested, subject,
			fmt.Sprintf("an operator asked for EPP session %s to be ended", sess.ID),
			entities.EPPSessionKillRequestedData{SessionID: sess.ID, ClID: sess.ClID, LoginID: sess.LoginID, ClientIP: sess.ClientIP.String(), Node: sess.Node, Filter: filter})
		e.Actor = actor
		if v, ok := appcontext.TraceID(ctx); ok {
			e.TraceID = v
		}
		events = append(events, e)
	}
	if err := s.events.Publish(ctx, events...); err != nil {
		return fmt.Errorf("record EPP session kill request: %w", err)
	}
	return nil
}
