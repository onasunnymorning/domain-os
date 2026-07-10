package entities

import (
	"errors"
	"time"
)

var (
	// ErrTombstoneNotFound is returned when a tombstone is not found.
	ErrTombstoneNotFound = errors.New("domain tombstone not found")
	// ErrDuplicateTombstone is returned when a tombstone with the same ROID already exists.
	ErrDuplicateTombstone = errors.New("duplicate domain tombstone")
)

// DomainTombstone represents a preserved record of a purged domain. It is always
// created when a domain is purged (regardless of DropCatch status), and serves as
// the operator-facing archive record for historical audit and lifecycle browsing.
//
// The ROID uniquely identifies a specific incarnation of a domain name — if
// "example.com" is registered, purged, and re-registered, each incarnation gets
// its own tombstone with a distinct ROID.
type DomainTombstone struct {
	// RoID is the primary key — the ROID of the purged domain, unique per incarnation.
	RoID RoidType `json:"roid"`

	// Name is the domain name (A-label / punycode).
	Name DomainName `json:"name"`

	// UName is the Unicode representation of the domain name (IDN only).
	UName DomainName `json:"uname,omitempty"`

	// TLDName is the parent TLD.
	TLDName DomainName `json:"tld_name"`

	// RegistrarClID is the registrar's client ID at the time of purge.
	RegistrarClID string `json:"registrar_clid"`

	// RegisteredAt is the original domain creation date.
	RegisteredAt time.Time `json:"registered_at"`

	// ExpiredAt is when the domain expired (nil if admin-deleted before expiry).
	ExpiredAt *time.Time `json:"expired_at,omitempty"`

	// PurgedAt is when PurgeDomain() ran.
	PurgedAt time.Time `json:"purged_at"`

	// PurgeReason describes why the domain was purged (e.g., "expired", "admin_delete").
	PurgeReason string `json:"purge_reason"`

	// DropCatch indicates whether the DropCatch flag was set on the domain at purge time.
	DropCatch bool `json:"drop_catch"`

	// LastSnapshot is the full domain entity serialized as JSON at the time of purge.
	// This provides a self-contained view of the domain's final state without needing
	// to query the event stream.
	LastSnapshot interface{} `json:"last_snapshot,omitempty"`

	// CreatedAt is when this tombstone record was created.
	CreatedAt time.Time `json:"created_at"`
}
