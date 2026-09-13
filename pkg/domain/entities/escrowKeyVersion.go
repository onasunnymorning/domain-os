package entities

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowKeyVersionState is where a key version is in its lifecycle.
//
//	STAGED ──Activate──▶ ACTIVE ──Deactivate──▶ HISTORICAL
//	  └──────── Revoke (from any live state) ────────────▶ REVOKED
//	STAGED | HISTORICAL | REVOKED ──Destroy──▶ DESTROYED
//
// "No longer used for new deposits" (HISTORICAL) is a different state from
// "unavailable" (REVOKED): escrow must keep opening retained deposits long
// after a key stopped being advertised. Compromise always goes through Revoke,
// never through the ordinary retirement path.
type EscrowKeyVersionState string

const (
	EscrowKeyStaged     EscrowKeyVersionState = "STAGED"
	EscrowKeyActive     EscrowKeyVersionState = "ACTIVE"
	EscrowKeyHistorical EscrowKeyVersionState = "HISTORICAL"
	EscrowKeyRevoked    EscrowKeyVersionState = "REVOKED"
	EscrowKeyDestroyed  EscrowKeyVersionState = "DESTROYED"
)

const pgpPublicKeyArmorHeaderPrefix = "-----BEGIN PGP PUBLIC KEY BLOCK-----"

// EscrowSecretRef locates material in the secret backend. It is a reference,
// never a value: it is safe in workflow history, logs and API responses.
//
// BackendVersionID is the backend's own version of the stored value. It is
// deliberately separate from EscrowKeyVersion.Version: rewrapping a secret or
// changing its passphrase changes the backend version but not the key.
type EscrowSecretRef struct {
	Name             string `json:"name"`
	BackendVersionID string `json:"backendVersionId,omitempty"`
}

// EscrowKeyVersion is one cryptographic key and its lifecycle.
type EscrowKeyVersion struct {
	ID      uuid.UUID
	PartyID uuid.UUID
	Owner   EscrowKeyOwner // copied from the party so every query can scope in SQL
	Purpose EscrowKeyPurpose
	Version int // 1-based, monotonic per party and purpose

	Fingerprint      string
	ArmoredPublicKey string           // empty for symmetric material
	SecretRef        *EscrowSecretRef // nil for public material

	State EscrowKeyVersionState
	// NotBefore/NotAfter optionally bound which deposits (by receipt time) the
	// version applies to, independent of state.
	NotBefore    *time.Time
	NotAfter     *time.Time
	KeyExpiresAt *time.Time // from the key itself, when it carries one

	LastProbeAt *time.Time
	LastProbeOK bool

	ActivatedAt      *time.Time
	DeactivatedAt    *time.Time
	RevokedAt        *time.Time
	RevocationReason string
	Compromised      bool
	DestroyedAt      *time.Time

	CreatedAt time.Time
	CreatedBy string
}

// EscrowKeyVersionSpec is what a caller knows when registering a version. The
// fingerprint is computed by the caller from the material (the domain does not
// parse OpenPGP or hold secrets).
type EscrowKeyVersionSpec struct {
	Party            *EscrowParty
	Purpose          EscrowKeyPurpose
	Version          int
	Fingerprint      string
	ArmoredPublicKey string
	SecretRef        *EscrowSecretRef
	NotBefore        *time.Time
	NotAfter         *time.Time
	KeyExpiresAt     *time.Time
	CreatedBy        string
	At               time.Time
	// ID may be chosen up front so a secret can be stored under it before the
	// row exists; uuid.Nil means generate one.
	ID uuid.UUID
}

// NewEscrowKeyVersion validates and creates a STAGED version.
func NewEscrowKeyVersion(spec EscrowKeyVersionSpec) (*EscrowKeyVersion, error) {
	if spec.Party == nil {
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("party is required"))
	}
	if err := spec.Party.AllowsPurpose(spec.Purpose); err != nil {
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, err)
	}
	policy := escrowKeyPurposePolicies[spec.Purpose]
	if spec.Version < 1 {
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("version must be 1 or greater"))
	}
	fp, err := NormalizeEscrowKeyFingerprint(policy.Material, spec.Fingerprint)
	if err != nil {
		return nil, err
	}
	armored := strings.TrimSpace(spec.ArmoredPublicKey)
	switch policy.Material {
	case EscrowKeyMaterialOpenPGPPublic, EscrowKeyMaterialOpenPGPPrivate:
		if !strings.HasPrefix(armored, pgpPublicKeyArmorHeaderPrefix) {
			return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("an ASCII-armored OpenPGP public key is required"))
		}
	case EscrowKeyMaterialSymmetric:
		if armored != "" {
			return nil, errors.Join(ErrInvalidEscrowKeyVersion, ErrEscrowKeyMaterialNotApplicable)
		}
	}
	needsSecret := policy.Material != EscrowKeyMaterialOpenPGPPublic
	switch {
	case needsSecret && (spec.SecretRef == nil || strings.TrimSpace(spec.SecretRef.Name) == ""):
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("a secret reference is required for this purpose"))
	case !needsSecret && spec.SecretRef != nil:
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, ErrEscrowKeyMaterialNotApplicable)
	}
	if spec.NotBefore != nil && spec.NotAfter != nil && !spec.NotAfter.After(*spec.NotBefore) {
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, ErrEscrowKeyInvalidWindow)
	}
	if spec.At.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("creation time is required"))
	}
	id := spec.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	var ref *EscrowSecretRef
	if spec.SecretRef != nil {
		r := EscrowSecretRef{Name: strings.TrimSpace(spec.SecretRef.Name), BackendVersionID: strings.TrimSpace(spec.SecretRef.BackendVersionID)}
		ref = &r
	}
	return &EscrowKeyVersion{
		ID:               id,
		PartyID:          spec.Party.ID,
		Owner:            spec.Party.Owner,
		Purpose:          spec.Purpose,
		Version:          spec.Version,
		Fingerprint:      fp,
		ArmoredPublicKey: armored,
		SecretRef:        ref,
		State:            EscrowKeyStaged,
		NotBefore:        utcPtr(spec.NotBefore),
		NotAfter:         utcPtr(spec.NotAfter),
		KeyExpiresAt:     utcPtr(spec.KeyExpiresAt),
		CreatedAt:        RoundTime(spec.At.UTC()),
		CreatedBy:        strings.TrimSpace(spec.CreatedBy),
	}, nil
}

// Policy returns the purpose policy of the version.
func (v *EscrowKeyVersion) Policy() EscrowKeyPurposePolicy {
	return escrowKeyPurposePolicies[v.Purpose]
}

// Clone returns a copy, so a caller can keep the before-state of a transition.
func (v *EscrowKeyVersion) Clone() *EscrowKeyVersion {
	c := *v
	if v.SecretRef != nil {
		r := *v.SecretRef
		c.SecretRef = &r
	}
	return &c
}

func (v *EscrowKeyVersion) transitionError(action string) error {
	return fmt.Errorf("%w: cannot %s a %s version", ErrEscrowKeyInvalidTransition, action, v.State)
}

// RecordProbe records the outcome of a worker proving it can fetch and use the
// material. It does not change state.
func (v *EscrowKeyVersion) RecordProbe(ok bool, at time.Time) error {
	switch v.State {
	case EscrowKeyStaged, EscrowKeyActive, EscrowKeyHistorical:
	default:
		return v.transitionError("probe")
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("probe time is required"))
	}
	t := at.UTC()
	v.LastProbeAt, v.LastProbeOK = &t, ok
	return nil
}

// Activate makes a STAGED version usable for new work. Purposes whose material
// lives on a worker need a successful probe first. Use PlanEscrowKeyActivation
// to apply the purpose's MaxActive rule across sibling versions.
func (v *EscrowKeyVersion) Activate(at time.Time) error {
	if v.State != EscrowKeyStaged {
		return v.transitionError("activate")
	}
	if v.Policy().RequiresProbe && !v.LastProbeOK {
		return ErrEscrowKeyNotProbed
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("activation time is required"))
	}
	t := at.UTC()
	v.State, v.ActivatedAt = EscrowKeyActive, &t
	return nil
}

// Deactivate stops using an ACTIVE version for new work while keeping it for
// what the purpose's HistoricalByReceipt rule allows.
func (v *EscrowKeyVersion) Deactivate(at time.Time) error {
	if v.State != EscrowKeyActive {
		return v.transitionError("deactivate")
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("deactivation time is required"))
	}
	t := at.UTC()
	v.State, v.DeactivatedAt = EscrowKeyHistorical, &t
	return nil
}

// Revoke withdraws a version from every use immediately, including retries of
// runs that already selected it. A reason is required; compromised records that
// the private material must be treated as known to someone else.
func (v *EscrowKeyVersion) Revoke(reason string, compromised bool, at time.Time) error {
	switch v.State {
	case EscrowKeyStaged, EscrowKeyActive, EscrowKeyHistorical:
	default:
		return v.transitionError("revoke")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("a revocation reason is required"))
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("revocation time is required"))
	}
	t := at.UTC()
	v.State, v.RevokedAt, v.RevocationReason, v.Compromised = EscrowKeyRevoked, &t, reason, compromised
	return nil
}

// Destroy records that the material has been deleted from the backend. The
// version's metadata, fingerprint and public key are kept forever so runs that
// used it stay explainable. An ACTIVE version must be deactivated or revoked
// first.
func (v *EscrowKeyVersion) Destroy(at time.Time) error {
	switch v.State {
	case EscrowKeyStaged, EscrowKeyHistorical, EscrowKeyRevoked:
	default:
		return v.transitionError("destroy")
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowKeyVersion, errors.New("destruction time is required"))
	}
	t := at.UTC()
	v.State, v.DestroyedAt = EscrowKeyDestroyed, &t
	return nil
}

func (v *EscrowKeyVersion) inWindow(receivedAt time.Time) bool {
	if v.NotBefore != nil && receivedAt.Before(*v.NotBefore) {
		return false
	}
	if v.NotAfter != nil && !receivedAt.Before(*v.NotAfter) {
		return false
	}
	return true
}

// UsableForDeposit reports whether the version may be selected for a run over a
// deposit received at receivedAt: ACTIVE, or HISTORICAL when the purpose keeps
// historical versions for deposits received before deactivation — and inside
// the optional window either way.
func (v *EscrowKeyVersion) UsableForDeposit(receivedAt time.Time) bool {
	if !v.inWindow(receivedAt) {
		return false
	}
	switch v.State {
	case EscrowKeyActive:
		return true
	case EscrowKeyHistorical:
		return v.Policy().HistoricalByReceipt && v.DeactivatedAt != nil && receivedAt.Before(*v.DeactivatedAt)
	default:
		return false
	}
}

// UsableForNewWork reports whether the version may be selected for work that
// has no recorded selection yet, such as a new derivative.
func (v *EscrowKeyVersion) UsableForNewWork() bool {
	return v.State == EscrowKeyActive
}

// UsableForRecordedRun reports whether a retry of a run that already recorded
// this version may still use it: anything short of revocation or destruction.
// Emergency disablement always wins over workflow history.
func (v *EscrowKeyVersion) UsableForRecordedRun() bool {
	switch v.State {
	case EscrowKeyStaged, EscrowKeyActive, EscrowKeyHistorical:
		return true
	default:
		return false
	}
}

// Ref returns the reference a run carries through workflow history.
func (v *EscrowKeyVersion) Ref() EscrowKeyVersionRef {
	ref := EscrowKeyVersionRef{ID: v.ID, PartyID: v.PartyID, Purpose: v.Purpose, Version: v.Version, Fingerprint: v.Fingerprint}
	if v.SecretRef != nil {
		r := *v.SecretRef
		ref.SecretRef = &r
	}
	return ref
}

// EscrowKeyVersionTransition is one conditional state change for persistence:
// it applies only if the stored row is still in ExpectedState, so two
// concurrent changes to one version cannot both succeed.
type EscrowKeyVersionTransition struct {
	Version       *EscrowKeyVersion
	ExpectedState EscrowKeyVersionState
}

// EscrowKeyVersionRef identifies a selected version without any material. It
// is what workflow history, run records and activity inputs carry.
type EscrowKeyVersionRef struct {
	ID          uuid.UUID        `json:"id"`
	PartyID     uuid.UUID        `json:"partyId"`
	Purpose     EscrowKeyPurpose `json:"purpose"`
	Version     int              `json:"version"`
	Fingerprint string           `json:"fingerprint"`
	SecretRef   *EscrowSecretRef `json:"secretRef,omitempty"`
}

// PlanEscrowKeyActivation activates target and returns the siblings that must
// be deactivated in the same transaction to respect the purpose's MaxActive.
// siblings are the other versions of the same party and purpose. Replacing an
// active version requires confirmReplace, because for a bounded purpose it is a
// decision with consequences (for pseudonymisation: tokens stop joining).
func PlanEscrowKeyActivation(target *EscrowKeyVersion, siblings []*EscrowKeyVersion, confirmReplace bool, at time.Time) ([]*EscrowKeyVersion, error) {
	policy := target.Policy()
	var active []*EscrowKeyVersion
	for _, s := range siblings {
		if s.ID == target.ID {
			continue
		}
		if s.PartyID != target.PartyID || s.Purpose != target.Purpose {
			return nil, errors.Join(ErrInvalidEscrowKeyVersion, errors.New("siblings must share the party and purpose"))
		}
		if s.State == EscrowKeyActive {
			active = append(active, s)
		}
	}
	if err := target.Activate(at); err != nil {
		return nil, err
	}
	if policy.MaxActive == 0 || len(active)+1 <= policy.MaxActive {
		return nil, nil
	}
	if !confirmReplace {
		// Undo: the caller has not agreed to replace anything.
		target.State, target.ActivatedAt = EscrowKeyStaged, nil
		return nil, ErrEscrowKeyReplaceNotConfirmed
	}
	// Deactivate the oldest active versions until the bound holds.
	excess := len(active) + 1 - policy.MaxActive
	sort.SliceStable(active, func(i, j int) bool { return active[i].Version < active[j].Version })
	var deactivated []*EscrowKeyVersion
	for _, s := range active[:excess] {
		if err := s.Deactivate(at); err != nil {
			return nil, err
		}
		deactivated = append(deactivated, s)
	}
	return deactivated, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
