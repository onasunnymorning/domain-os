package services

import (
	"context"
	"strconv"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPPollService answers <poll> (RFC 5730 §2.9.2.3): a registrar reads the
// oldest message in its queue and acknowledges it to see the next. A queue
// is the registrar's alone (ADR 0006): another registrar's messages are
// neither read nor acknowledged through it.
type EPPPollService struct {
	polls repositories.PollMessageRepository
}

// NewEPPPollService returns the service.
func NewEPPPollService(polls repositories.PollMessageRepository) *EPPPollService {
	return &EPPPollService{polls: polls}
}

// Request returns the registrar's oldest message and how many it has; nil
// and 0 when its queue is empty (1300). Reading a message does not remove
// it.
func (s *EPPPollService) Request(ctx context.Context, scope entities.RegistrarClID) (*entities.PollMessage, int64, error) {
	if err := scope.Validate(); err != nil {
		return nil, 0, err
	}
	return s.polls.OldestPollMessage(ctx, entities.ClIDType(scope))
}

// Ack removes message msgID from the registrar's queue and returns how many
// remain. An identifier that is not one of its messages is
// entities.ErrPollMessageNotFound (2303).
func (s *EPPPollService) Ack(ctx context.Context, scope entities.RegistrarClID, msgID string) (int64, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(msgID, 10, 64)
	if err != nil || id <= 0 {
		return 0, entities.ErrPollMessageNotFound
	}
	return s.polls.AckPollMessage(ctx, entities.ClIDType(scope), id)
}
