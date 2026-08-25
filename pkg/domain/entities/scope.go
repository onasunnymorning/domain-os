package entities

import "errors"

var (
	// ErrInvalidOperatorID is returned when an operator scope does not have a valid ClIDType shape.
	ErrInvalidOperatorID = errors.New("invalid operator scope")
	// ErrInvalidRegistrarClID is returned when a registrar scope does not have a valid ClIDType shape.
	ErrInvalidRegistrarClID = errors.New("invalid registrar scope")
)

// ---------------------------------------------------------------------------
// Tenant scope types — see docs/adr/0006-tenancy-model.md (ADR-0006) and INV-02.
//
// This registry is a two-sided marketplace and therefore has two tenant kinds,
// one per side:
//
//   - OperatorID    — the supply side. A RegistryOperator manages TLDs.
//   - RegistrarClID — the consumer side. A Registrar sponsors domains, contacts
//     and hosts, and transacts over EPP.
//
// Both are ClIDType-shaped, which is exactly why they are declared as distinct
// defined types rather than aliases: an operator scope, a registrar scope and a
// plain object ClID are all strings of the same shape but mean three different
// things, and the compiler is the cheapest place to keep them apart.
//
// These types are for scope *parameters* — the "who is asking" argument that
// follows ctx on a tenant-scoped method. Entity fields keep ClIDType: a
// Domain's ClID is the sponsoring registrar of that record, not the scope of
// the caller reading it.
// ---------------------------------------------------------------------------

// OperatorID is the administrative-plane tenant scope: a RegistryOperator's
// RyID. An operator sees their TLDs and everything within them, derived
// through the TLD join (RegistryOperator.RyID -> TLD.RyID -> Domain.TLDName).
// RyID is deliberately never denormalized below TLD, so reassigning a TLD to
// another operator stays a one-row update.
type OperatorID ClIDType

// NewOperatorID creates a validated OperatorID. It reuses ClIDType validation:
// an operator scope that cannot be a RyID is not an operator scope.
func NewOperatorID(id string) (OperatorID, error) {
	c, err := NewClIDType(id)
	if err != nil {
		return OperatorID(""), errors.Join(ErrInvalidOperatorID, err)
	}
	return OperatorID(c), nil
}

// String implements the Stringer interface.
func (o OperatorID) String() string {
	return string(o)
}

// Validate checks that the OperatorID has a valid ClIDType shape.
func (o OperatorID) Validate() error {
	c := ClIDType(o)
	if err := c.Validate(); err != nil {
		return errors.Join(ErrInvalidOperatorID, err)
	}
	return nil
}

// IsZero reports whether the scope is unset. A zero scope is never a valid
// scope — scope is required, not optional. See ADR-0006.
func (o OperatorID) IsZero() bool {
	return o == ""
}

// RegistrarClID is the transactional-plane tenant scope: a Registrar's ClID as
// authenticated on an EPP session (or, later, a registrar portal session). A
// registrar sees and manages only the objects it sponsors — the ClID column on
// Domain, Contact and Host is the enforcement key — and may only transact in
// TLDs it is accredited for.
//
// Declared and documented here but not yet threaded through any call path: the
// EPP transactional verbs it governs do not exist yet. It exists so that the
// EPP buildout and any registrar-facing slice start typed rather than being
// retrofitted. See ADR-0006 and INV-02.
type RegistrarClID ClIDType

// NewRegistrarClID creates a validated RegistrarClID. It reuses ClIDType
// validation: a registrar scope that cannot be a registrar ClID is not a
// registrar scope.
func NewRegistrarClID(clID string) (RegistrarClID, error) {
	c, err := NewClIDType(clID)
	if err != nil {
		return RegistrarClID(""), errors.Join(ErrInvalidRegistrarClID, err)
	}
	return RegistrarClID(c), nil
}

// String implements the Stringer interface.
func (r RegistrarClID) String() string {
	return string(r)
}

// Validate checks that the RegistrarClID has a valid ClIDType shape.
func (r RegistrarClID) Validate() error {
	c := ClIDType(r)
	if err := c.Validate(); err != nil {
		return errors.Join(ErrInvalidRegistrarClID, err)
	}
	return nil
}

// IsZero reports whether the scope is unset. A zero scope is never a valid
// scope — scope is required, not optional. See ADR-0006.
func (r RegistrarClID) IsZero() bool {
	return r == ""
}
