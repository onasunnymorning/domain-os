package repositories

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TMCHRepository stores the three TMCH data sets (ADR 0024 §4, roadmap 9a):
// the CA certificate bundle, the SMD revocation list and the DNL list. Each
// set is replaced whole, with the outbox events of the replacement written
// in the same transaction (INV-01); a read of a set never fetched returns
// entities.ErrTMCHDataMissing.
//
// A Replace decides under the set's lock whether the fetched set supersedes
// the stored one (entities.TMCHSetStatus.Supersedes). When it does not, it
// records the fetch time and returns entities.ErrTMCHListStale without
// writing or emitting anything, so two refreshes at once, or a slow one
// behind a fast one, cannot store a generation twice or put an older one
// back.
//
// The lookups are what sunrise (9b) and claims (9c) will read on the EPP
// path: one indexed row, never the whole list.
type TMCHRepository interface {
	// ReplaceCertificates stores the bundle in place of the one stored.
	ReplaceCertificates(ctx context.Context, bundle *entities.TMCHCertificates, events []entities.DomainEvent) error
	// GetCertificates returns the stored bundle.
	GetCertificates(ctx context.Context) (*entities.TMCHCertificates, error)

	// ReplaceSMDRL stores the revocation list in place of the one stored.
	ReplaceSMDRL(ctx context.Context, list *entities.SMDRevocationList, events []entities.DomainEvent) error
	// IsSMDRevoked reports whether the signed mark with this ID is on the
	// stored revocation list.
	IsSMDRevoked(ctx context.Context, smdID string) (bool, error)

	// ReplaceDNL stores the DNL list in place of the one stored.
	ReplaceDNL(ctx context.Context, list *entities.DNLList, events []entities.DomainEvent) error
	// LookupDNL returns the stored entry for a label (compared lower-cased),
	// nil when the label is not listed.
	LookupDNL(ctx context.Context, label string) (*entities.DNLEntry, error)

	// MarkFetched records that a set was fetched at fetchedAt and found
	// not to supersede the stored one; nothing else changes. A set never
	// stored is left alone.
	MarkFetched(ctx context.Context, set string, fetchedAt time.Time) error

	// Status reports each set's freshness; a set never fetched is nil.
	Status(ctx context.Context) (*entities.TMCHStatus, error)
}
