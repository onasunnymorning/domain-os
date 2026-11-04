package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// TMCHFetcher downloads the Clearinghouse's three data sets (ADR 0024 §4,
// roadmap 9a). *tmch.Feed satisfies it; tests use a fake. A fetcher that is
// not Configured has no URLs or credentials, and a refresh does nothing.
type TMCHFetcher interface {
	Configured() bool
	FetchCertificates(ctx context.Context) (*entities.TMCHCertificates, error)
	FetchSMDRL(ctx context.Context) (*entities.SMDRevocationList, error)
	FetchDNL(ctx context.Context) (*entities.DNLList, error)
}

// TMCHService keeps the platform's TMCH data fresh and reports on it. The
// refresh workflow and the admin API both go through it.
type TMCHService interface {
	// Configured reports whether this installation has a TMCH feed to
	// refresh from.
	Configured() bool
	// Status reports each set's freshness; a set never fetched is nil.
	Status(ctx context.Context) (*entities.TMCHStatus, error)
	// Refresh fetches every set and stores each one the Clearinghouse has
	// regenerated since it was last stored. One set failing does not stop
	// the others; the result says what each did, and the error joins the
	// failures (entities.ErrTMCHNotConfigured when there is no feed).
	Refresh(ctx context.Context) (*queries.TMCHRefreshResult, error)
}
