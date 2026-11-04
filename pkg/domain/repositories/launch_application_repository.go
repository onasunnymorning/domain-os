package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LaunchApplicationRepository stores launch applications
// (entities.LaunchApplication, ADR 0024 §3). A write that changes an
// application is conditional on the version it was read with, like the
// domain writes: another command's change in between is
// entities.ErrApplicationChanged, and the caller reads and checks again.
type LaunchApplicationRepository interface {
	// CreateApplication stores a, with its nameservers and DS records, and
	// returns it as stored. A registrar may hold one open (non-final)
	// application per name and phase: a second is
	// entities.ErrApplicationExists.
	CreateApplication(ctx context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error)
	// GetApplicationByID returns the application with ID id, or
	// entities.ErrApplicationNotFound.
	GetApplicationByID(ctx context.Context, id entities.RoidType) (*entities.LaunchApplication, error)
	// ListApplicationsByName returns every application for name, of every
	// registrar and status, oldest first.
	ListApplicationsByName(ctx context.Context, name string) ([]*entities.LaunchApplication, error)
	// ListApplicationsByPhase returns the applications filed in a phase of
	// a TLD, oldest first, those of one status when status is not empty.
	ListApplicationsByPhase(ctx context.Context, tld, phase string, status entities.ApplicationStatus) ([]*entities.LaunchApplication, error)
	// QueuePollMessages queues msgs, about applications, in their
	// registrars' queues (repositories.PollMessageRepository reads them),
	// in the transaction ctx carries if any, so that a status change and
	// its messages commit together.
	QueuePollMessages(ctx context.Context, msgs []*entities.PollMessage) error
	// CountApplicationsByPhase counts the applications filed in a phase of
	// a TLD, of every status: a phase with any cannot change its
	// registration model (entities.Phase.SetLaunchSettings).
	CountApplicationsByPhase(ctx context.Context, tld, phase string) (int64, error)
	// UpdateApplicationIfUnchanged writes a, with its nameservers and DS
	// records, if the stored application still has the version a was read
	// with (else entities.ErrApplicationChanged), and returns it as
	// stored. A missing application is entities.ErrApplicationNotFound.
	UpdateApplicationIfUnchanged(ctx context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error)
	// DeleteApplicationIfUnchanged removes a (a withdrawal) if the stored
	// application still has the version a was read with, else
	// entities.ErrApplicationChanged; a missing one is
	// entities.ErrApplicationNotFound.
	DeleteApplicationIfUnchanged(ctx context.Context, a *entities.LaunchApplication) error
}
