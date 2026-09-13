package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/escrowkeys"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EscrowKeyService manages the escrow key registry: parties, key versions and
// arrangements (issue #429, ADR-0009).
//
// Every method takes the caller's typed key scope right after ctx (ADR-0006).
// Visibility is enforced by the repositories in SQL; the "may change" rule is
// the scope's CanManage and is checked here before any write. Authentication
// scopes (who may call a mutating method at all) are the controller's job.
//
// Private and symmetric material passes through ImportPrivateKey and
// GenerateSymmetricKey only on its way to the key store. It is never logged,
// persisted outside the store, or returned.
type EscrowKeyService struct {
	parties      repositories.EscrowPartyRepository
	versions     repositories.EscrowKeyVersionRepository
	arrangements repositories.EscrowArrangementRepository
	audit        repositories.EscrowKeyAuditRepository
	tlds         repositories.TLDRepository
	store        interfaces.EscrowSecretStore     // nil: private material cannot be imported
	probes       interfaces.EscrowKeyProbeStarter // nil: probes are not started automatically
	resolver     *escrowkeys.Resolver
	now          func() time.Time
}

// NewEscrowKeyService creates the service. store and probes may be nil.
func NewEscrowKeyService(
	parties repositories.EscrowPartyRepository,
	versions repositories.EscrowKeyVersionRepository,
	arrangements repositories.EscrowArrangementRepository,
	audit repositories.EscrowKeyAuditRepository,
	tlds repositories.TLDRepository,
	store interfaces.EscrowSecretStore,
	probes interfaces.EscrowKeyProbeStarter,
) *EscrowKeyService {
	return &EscrowKeyService{
		parties: parties, versions: versions, arrangements: arrangements, audit: audit, tlds: tlds,
		store: store, probes: probes, resolver: escrowkeys.NewResolver(versions, arrangements),
		now: func() time.Time { return time.Now().UTC() },
	}
}

// EscrowPartyDetail is a party with its versions and where it is used.
type EscrowPartyDetail struct {
	Party    *entities.EscrowParty
	Versions []*entities.EscrowKeyVersion
	UsedBy   []*entities.EscrowArrangement
}

func (s *EscrowKeyService) auditContext(ctx context.Context) entities.EscrowKeyAuditContext {
	a := entities.EscrowKeyAuditContext{At: s.now()}
	a.Actor, _ = appcontext.UserID(ctx)
	a.TraceID, _ = appcontext.TraceID(ctx)
	a.CorrelationID, _ = appcontext.CorrelationID(ctx)
	return a
}

// ---------------------------------------------------------------------------
// Parties
// ---------------------------------------------------------------------------

// CreateParty registers a party owned by the scope.
func (s *EscrowKeyService) CreateParty(ctx context.Context, scope entities.EscrowKeyScope, cmd commands.CreateEscrowPartyCommand) (*entities.EscrowParty, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	actx := s.auditContext(ctx)
	p, err := entities.NewEscrowParty(scope.Owner(), cmd.Name, entities.EscrowPartyKind(strings.ToUpper(strings.TrimSpace(cmd.Kind))),
		entities.EscrowPartySide(strings.ToLower(strings.TrimSpace(cmd.Side))), actx.Actor, actx.At)
	if err != nil {
		return nil, err
	}
	ev, err := entities.NewEscrowPartyAuditEvent(actx, p, entities.EscrowAuditPartyCreated)
	if err != nil {
		return nil, err
	}
	if err := s.parties.Create(ctx, p, ev); err != nil {
		return nil, fmt.Errorf("create escrow party: %w", err)
	}
	return p, nil
}

// ListParties lists the parties visible in the scope.
func (s *EscrowKeyService) ListParties(ctx context.Context, scope entities.EscrowKeyScope, q queries.ListItemsQuery) ([]*entities.EscrowParty, string, error) {
	return s.parties.List(ctx, scope, q)
}

// GetParty returns a visible party with its versions and usage.
func (s *EscrowKeyService) GetParty(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*EscrowPartyDetail, error) {
	p, err := s.parties.GetByID(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	versions, err := s.versions.ListByParty(ctx, scope, id, "")
	if err != nil {
		return nil, fmt.Errorf("list key versions: %w", err)
	}
	usedBy, err := s.arrangements.ListLiveReferencing(ctx, scope, id)
	if err != nil {
		return nil, fmt.Errorf("list party usage: %w", err)
	}
	return &EscrowPartyDetail{Party: p, Versions: versions, UsedBy: usedBy}, nil
}

// PartyAudit returns a party's audit trail, newest first.
func (s *EscrowKeyService) PartyAudit(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID, q queries.ListItemsQuery) ([]*entities.EscrowKeyAuditEvent, string, error) {
	if _, err := s.parties.GetByID(ctx, scope, id); err != nil {
		return nil, "", err
	}
	return s.audit.ListByParty(ctx, scope, id, q)
}

func (s *EscrowKeyService) manageableParty(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowParty, error) {
	p, err := s.parties.GetByID(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	if !scope.CanManage(p.Owner) {
		return nil, entities.ErrEscrowKeyOwnerMismatch
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Adding versions
// ---------------------------------------------------------------------------

// EscrowKeyVersionCreated reports a new version and, when one was started,
// the probe that will prove it usable.
type EscrowKeyVersionCreated struct {
	Version         *entities.EscrowKeyVersion
	ProbeWorkflowID string
}

func secretName(v *entities.EscrowKeyVersion) string {
	owner := "platform"
	if !v.Owner.IsPlatform() {
		owner = "op-" + v.Owner.Operator.String()
	}
	return fmt.Sprintf("escrow-keys/%s/%s/%s", owner, v.PartyID, v.ID)
}

func secretTags(v *entities.EscrowKeyVersion) map[string]string {
	return map[string]string{
		"domain-os/owner": v.Owner.String(), "domain-os/party": v.PartyID.String(),
		"domain-os/key-version": v.ID.String(), "domain-os/purpose": string(v.Purpose), "domain-os/fingerprint": v.Fingerprint,
	}
}

func (s *EscrowKeyService) nextVersion(ctx context.Context, scope entities.EscrowKeyScope, p *entities.EscrowParty, purpose entities.EscrowKeyPurpose, fingerprint string) (int, error) {
	existing, err := s.versions.ListByParty(ctx, scope, p.ID, purpose)
	if err != nil {
		return 0, fmt.Errorf("list key versions: %w", err)
	}
	for _, v := range existing {
		if v.Fingerprint == fingerprint {
			return 0, entities.ErrEscrowKeyDuplicateFingerprint
		}
	}
	return s.versions.NextVersionNumber(ctx, scope, p.ID, purpose)
}

// storeAndCreate writes material to the key store under the version's id, then
// the version row. A failed row leaves no orphan secret behind if it can help
// it: the secret is scheduled for deletion.
func (s *EscrowKeyService) storeAndCreate(ctx context.Context, spec entities.EscrowKeyVersionSpec, value []byte, action entities.EscrowKeyAuditAction) (*entities.EscrowKeyVersion, error) {
	if s.store == nil {
		return nil, entities.ErrEscrowKeyStoreNotConfigured
	}
	// Validate everything the entity can before any secret exists.
	spec.ID = uuid.New()
	spec.SecretRef = &entities.EscrowSecretRef{Name: "pending"}
	draft, err := entities.NewEscrowKeyVersion(spec)
	if err != nil {
		return nil, err
	}
	ref, err := s.store.Put(ctx, secretName(draft), secretTags(draft), value)
	if err != nil {
		return nil, fmt.Errorf("store key material: %w", err)
	}
	spec.SecretRef = &ref
	v, err := entities.NewEscrowKeyVersion(spec)
	if err != nil {
		s.discardSecret(ctx, ref)
		return nil, err
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(s.auditContext(ctx), nil, v, action)
	if err != nil {
		s.discardSecret(ctx, ref)
		return nil, err
	}
	if err := s.versions.Create(ctx, v, ev); err != nil {
		s.discardSecret(ctx, ref)
		return nil, fmt.Errorf("create key version: %w", err)
	}
	return v, nil
}

func (s *EscrowKeyService) discardSecret(ctx context.Context, ref entities.EscrowSecretRef) {
	if err := s.store.ScheduleDelete(ctx, ref); err != nil {
		// The secret is tagged with its would-be version id, so an orphan is
		// findable; the error text is the adapter's fixed text, never material.
		log.Printf("escrow keys: could not discard key material after a failed registration: %v", err)
	}
}

func (s *EscrowKeyService) startProbe(ctx context.Context, v *entities.EscrowKeyVersion) string {
	if s.probes == nil || !v.Policy().RequiresProbe {
		return ""
	}
	actor, _ := appcontext.UserID(ctx)
	id, err := s.probes.StartEscrowKeyProbe(ctx, v.Owner, v.ID, actor)
	if err != nil {
		// The version exists and can be probed again explicitly.
		log.Printf("escrow keys: could not start probe for key version %s: %v", v.ID, err)
		return ""
	}
	return id
}

// ImportPrivateKey imports an OpenPGP private key as a new STAGED version and
// starts its probe.
func (s *EscrowKeyService) ImportPrivateKey(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, cmd commands.ImportEscrowPrivateKeyCommand) (*EscrowKeyVersionCreated, error) {
	p, err := s.manageableParty(ctx, scope, partyID)
	if err != nil {
		return nil, err
	}
	purpose := entities.EscrowKeyPurpose(cmd.Purpose)
	policy, err := entities.EscrowKeyPurposePolicyFor(purpose)
	if err != nil {
		return nil, err
	}
	if policy.Material != entities.EscrowKeyMaterialOpenPGPPrivate {
		return nil, entities.ErrEscrowKeyMaterialNotApplicable
	}
	if err := p.AllowsPurpose(purpose); err != nil {
		return nil, err
	}
	ring, err := secrets.ParseAndUnlockArmoredPrivateKeys(cmd.ArmoredPrivateKey, cmd.Passphrase)
	if err != nil || len(ring) != 1 {
		return nil, entities.ErrEscrowKeyMaterialUnreadable
	}
	entity := ring[0]
	if purpose == entities.EscrowKeyPurposeDecryptInbound && !secrets.CanEncrypt(entity, s.now()) {
		return nil, errors.Join(entities.ErrEscrowKeyMaterialUnreadable, errors.New("the key has no usable encryption subkey"))
	}
	publicKey, err := secrets.ArmoredPublicKey(entity)
	if err != nil {
		return nil, entities.ErrEscrowKeyMaterialUnreadable
	}
	fingerprint := rdevalidate.FingerprintHex(entity)
	n, err := s.nextVersion(ctx, scope, p, purpose, fingerprint)
	if err != nil {
		return nil, err
	}
	// The stored value keeps the key exactly as imported, still protected by
	// its passphrase, so the store's encryption is not the only layer.
	value, err := secrets.EncodeOpenPGPSecret(cmd.ArmoredPrivateKey, cmd.Passphrase)
	if err != nil {
		return nil, entities.ErrEscrowKeyMaterialUnreadable
	}
	actx := s.auditContext(ctx)
	v, err := s.storeAndCreate(ctx, entities.EscrowKeyVersionSpec{
		Party: p, Purpose: purpose, Version: n, Fingerprint: fingerprint, ArmoredPublicKey: publicKey,
		NotBefore: cmd.NotBefore, NotAfter: cmd.NotAfter, KeyExpiresAt: secrets.KeyExpiry(entity),
		CreatedBy: actx.Actor, At: actx.At,
	}, value, entities.EscrowAuditVersionImported)
	if err != nil {
		return nil, err
	}
	return &EscrowKeyVersionCreated{Version: v, ProbeWorkflowID: s.startProbe(ctx, v)}, nil
}

// AddPublicKey adds a counterparty's public key as a new STAGED version.
func (s *EscrowKeyService) AddPublicKey(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, cmd commands.AddEscrowPublicKeyCommand) (*EscrowKeyVersionCreated, error) {
	p, err := s.manageableParty(ctx, scope, partyID)
	if err != nil {
		return nil, err
	}
	purpose := entities.EscrowKeyPurpose(cmd.Purpose)
	policy, err := entities.EscrowKeyPurposePolicyFor(purpose)
	if err != nil {
		return nil, err
	}
	if policy.Material != entities.EscrowKeyMaterialOpenPGPPublic {
		return nil, entities.ErrEscrowKeyMaterialNotApplicable
	}
	entity, err := rdevalidate.ParseArmoredPublicKey(cmd.ArmoredPublicKey)
	if err != nil {
		return nil, errors.Join(entities.ErrEscrowKeyMaterialUnreadable, errors.New("expected exactly one ASCII-armored OpenPGP public key"))
	}
	fingerprint := rdevalidate.FingerprintHex(entity)
	n, err := s.nextVersion(ctx, scope, p, purpose, fingerprint)
	if err != nil {
		return nil, err
	}
	actx := s.auditContext(ctx)
	v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{
		Party: p, Purpose: purpose, Version: n, Fingerprint: fingerprint, ArmoredPublicKey: cmd.ArmoredPublicKey,
		NotBefore: cmd.NotBefore, NotAfter: cmd.NotAfter, KeyExpiresAt: secrets.KeyExpiry(entity),
		CreatedBy: actx.Actor, At: actx.At,
	})
	if err != nil {
		return nil, err
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(actx, nil, v, entities.EscrowAuditVersionPublicKeyAdded)
	if err != nil {
		return nil, err
	}
	if err := s.versions.Create(ctx, v, ev); err != nil {
		return nil, fmt.Errorf("create key version: %w", err)
	}
	return &EscrowKeyVersionCreated{Version: v}, nil
}

// GenerateSymmetricKey creates a random symmetric key version in the key store
// and starts its probe. The material is never returned.
func (s *EscrowKeyService) GenerateSymmetricKey(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, cmd commands.GenerateEscrowSymmetricKeyCommand) (*EscrowKeyVersionCreated, error) {
	p, err := s.manageableParty(ctx, scope, partyID)
	if err != nil {
		return nil, err
	}
	purpose := entities.EscrowKeyPurpose(cmd.Purpose)
	policy, err := entities.EscrowKeyPurposePolicyFor(purpose)
	if err != nil {
		return nil, err
	}
	if policy.Material != entities.EscrowKeyMaterialSymmetric || !policy.Generatable {
		return nil, entities.ErrEscrowKeyMaterialNotApplicable
	}
	if err := p.AllowsPurpose(purpose); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	fingerprint, err := rdesanitize.MasterKeyFingerprint(key)
	if err != nil {
		return nil, err
	}
	n, err := s.nextVersion(ctx, scope, p, purpose, fingerprint)
	if err != nil {
		return nil, err
	}
	value, err := secrets.EncodeSymmetricSecret(key)
	if err != nil {
		return nil, err
	}
	actx := s.auditContext(ctx)
	v, err := s.storeAndCreate(ctx, entities.EscrowKeyVersionSpec{
		Party: p, Purpose: purpose, Version: n, Fingerprint: fingerprint, CreatedBy: actx.Actor, At: actx.At,
	}, value, entities.EscrowAuditVersionGenerated)
	if err != nil {
		return nil, err
	}
	return &EscrowKeyVersionCreated{Version: v, ProbeWorkflowID: s.startProbe(ctx, v)}, nil
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// GetVersion returns a visible version.
func (s *EscrowKeyService) GetVersion(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
	return s.versions.GetByID(ctx, scope, id)
}

func (s *EscrowKeyService) manageableVersion(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
	v, err := s.versions.GetByID(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	if !scope.CanManage(v.Owner) {
		return nil, entities.ErrEscrowKeyOwnerMismatch
	}
	return v, nil
}

// Probe starts a probe of a version.
func (s *EscrowKeyService) Probe(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (string, error) {
	v, err := s.manageableVersion(ctx, scope, id)
	if err != nil {
		return "", err
	}
	if !v.UsableForRecordedRun() {
		return "", entities.ErrEscrowKeyInvalidTransition
	}
	if s.probes == nil {
		return "", errors.New("probes cannot be started from this process")
	}
	actor, _ := appcontext.UserID(ctx)
	return s.probes.StartEscrowKeyProbe(ctx, v.Owner, v.ID, actor)
}

// Activate activates a version, deactivating siblings as the purpose requires.
func (s *EscrowKeyService) Activate(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID, cmd commands.ActivateEscrowKeyVersionCommand) (*entities.EscrowKeyVersion, error) {
	v, err := s.manageableVersion(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	siblings, err := s.versions.ListByParty(ctx, scope, v.PartyID, v.Purpose)
	if err != nil {
		return nil, fmt.Errorf("list key versions: %w", err)
	}
	before := v.Clone()
	befores := map[uuid.UUID]*entities.EscrowKeyVersion{}
	for _, sib := range siblings {
		befores[sib.ID] = sib.Clone()
	}
	actx := s.auditContext(ctx)
	deactivated, err := entities.PlanEscrowKeyActivation(v, siblings, cmd.ConfirmReplace, actx.At)
	if err != nil {
		return nil, err
	}
	transitions := make([]entities.EscrowKeyVersionTransition, 0, len(deactivated)+1)
	audits := make([]*entities.EscrowKeyAuditEvent, 0, len(deactivated)+1)
	for _, d := range deactivated {
		ev, err := entities.NewEscrowKeyVersionAuditEvent(actx, befores[d.ID], d, entities.EscrowAuditVersionDeactivated)
		if err != nil {
			return nil, err
		}
		transitions = append(transitions, entities.EscrowKeyVersionTransition{Version: d, ExpectedState: befores[d.ID].State})
		audits = append(audits, ev)
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(actx, before, v, entities.EscrowAuditVersionActivated)
	if err != nil {
		return nil, err
	}
	transitions = append(transitions, entities.EscrowKeyVersionTransition{Version: v, ExpectedState: before.State})
	audits = append(audits, ev)
	if err := s.versions.ApplyTransitions(ctx, transitions, audits); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *EscrowKeyService) transition(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID, action entities.EscrowKeyAuditAction, change func(*entities.EscrowKeyVersion, time.Time) error) (*entities.EscrowKeyVersion, error) {
	v, err := s.manageableVersion(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	before := v.Clone()
	actx := s.auditContext(ctx)
	if err := change(v, actx.At); err != nil {
		return nil, err
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(actx, before, v, action)
	if err != nil {
		return nil, err
	}
	if err := s.versions.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{{Version: v, ExpectedState: before.State}}, []*entities.EscrowKeyAuditEvent{ev}); err != nil {
		return nil, err
	}
	return v, nil
}

// Deactivate stops using a version for new work.
func (s *EscrowKeyService) Deactivate(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
	return s.transition(ctx, scope, id, entities.EscrowAuditVersionDeactivated, func(v *entities.EscrowKeyVersion, at time.Time) error {
		return v.Deactivate(at)
	})
}

// Revoke withdraws a version from every use, including in-flight retries.
func (s *EscrowKeyService) Revoke(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID, cmd commands.RevokeEscrowKeyVersionCommand) (*entities.EscrowKeyVersion, error) {
	return s.transition(ctx, scope, id, entities.EscrowAuditVersionRevoked, func(v *entities.EscrowKeyVersion, at time.Time) error {
		return v.Revoke(cmd.Reason, cmd.Compromised, at)
	})
}

// Destroy schedules the material's deletion in the key store and then records
// the version as destroyed. The store goes first: a version that says it was
// destroyed must not still have material behind it.
func (s *EscrowKeyService) Destroy(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID, cmd commands.DestroyEscrowKeyVersionCommand) (*entities.EscrowKeyVersion, error) {
	v, err := s.manageableVersion(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.ReplaceAll(strings.TrimSpace(cmd.ConfirmFingerprint), " ", ""), v.Fingerprint) {
		return nil, errors.Join(entities.ErrInvalidEscrowKeyVersion, errors.New("confirmFingerprint must repeat the version's fingerprint"))
	}
	if err := v.Clone().Destroy(s.now()); err != nil {
		return nil, err
	}
	if v.SecretRef != nil {
		if s.store == nil {
			return nil, entities.ErrEscrowKeyStoreNotConfigured
		}
		if err := s.store.ScheduleDelete(ctx, *v.SecretRef); err != nil {
			return nil, fmt.Errorf("destroy key material: %w", err)
		}
	}
	return s.transition(ctx, scope, id, entities.EscrowAuditVersionDestroyed, func(v *entities.EscrowKeyVersion, at time.Time) error {
		return v.Destroy(at)
	})
}

// ---------------------------------------------------------------------------
// Arrangements
// ---------------------------------------------------------------------------

// EffectiveArrangement resolves a TLD's arrangement for an operator.
func (s *EscrowKeyService) EffectiveArrangement(ctx context.Context, scope entities.OperatorID, tld string) (entities.EffectiveEscrowArrangement, error) {
	if _, err := s.tlds.GetByNameForOperator(ctx, scope, tld); err != nil {
		return entities.EffectiveEscrowArrangement{}, err
	}
	return s.resolver.Effective(ctx, scope, tld)
}

// GetArrangement returns the live arrangement at a level: the scope's own
// default (tld empty) or a TLD override.
func (s *EscrowKeyService) GetArrangement(ctx context.Context, scope entities.EscrowKeyScope, tld string) (*entities.EscrowArrangement, error) {
	level, err := s.arrangementLevel(ctx, scope, tld)
	if err != nil {
		return nil, err
	}
	return s.arrangements.GetLive(ctx, scope, level, tld, entities.EscrowArrangementInbound)
}

// ListTLDOverrides lists an operator's TLD overrides.
func (s *EscrowKeyService) ListTLDOverrides(ctx context.Context, scope entities.OperatorID) ([]*entities.EscrowArrangement, error) {
	return s.arrangements.ListLiveTLDOverrides(ctx, scope, entities.EscrowArrangementInbound)
}

func (s *EscrowKeyService) arrangementLevel(ctx context.Context, scope entities.EscrowKeyScope, tld string) (entities.EscrowArrangementLevel, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	switch {
	case scope.IsPlatform() && tld != "":
		return "", errors.Join(entities.ErrInvalidEscrowArrangement, errors.New("TLD overrides belong to the operator"))
	case scope.IsPlatform():
		return entities.EscrowArrangementPlatform, nil
	case tld == "":
		return entities.EscrowArrangementOperator, nil
	default:
		if _, err := s.tlds.GetByNameForOperator(ctx, scope.Operator(), tld); err != nil {
			return "", err
		}
		return entities.EscrowArrangementTLD, nil
	}
}

// SetArrangement writes the next revision of the scope's default (tld empty)
// or of a TLD override.
func (s *EscrowKeyService) SetArrangement(ctx context.Context, scope entities.EscrowKeyScope, tld string, cmd commands.SetEscrowArrangementCommand) (*entities.EscrowArrangement, error) {
	level, err := s.arrangementLevel(ctx, scope, tld)
	if err != nil {
		return nil, err
	}
	lookup := func(id *uuid.UUID) (*entities.EscrowParty, error) {
		if id == nil {
			return nil, nil
		}
		// A party this scope cannot see is reported as not found, exactly as a
		// party that does not exist: nothing about another owner leaks.
		return s.parties.GetByID(ctx, scope, *id)
	}
	depositor, err := lookup(cmd.DepositorPartyID)
	if err != nil {
		return nil, err
	}
	receiver, err := lookup(cmd.ReceiverPartyID)
	if err != nil {
		return nil, err
	}
	previous, err := s.arrangements.GetLive(ctx, scope, level, tld, entities.EscrowArrangementInbound)
	if err != nil && !errors.Is(err, entities.ErrEscrowArrangementNotFound) {
		return nil, err
	}
	actx := s.auditContext(ctx)
	next, err := entities.NewEscrowArrangement(entities.EscrowArrangementSpec{
		Level: level, Operator: scope.Operator(), TLD: tld, Direction: entities.EscrowArrangementInbound,
		Depositor: depositor, Receiver: receiver, Previous: previous, CreatedBy: actx.Actor, At: actx.At,
	})
	if err != nil {
		return nil, err
	}
	ev, err := entities.NewEscrowArrangementAuditEvent(actx, next, entities.EscrowAuditArrangementChanged)
	if err != nil {
		return nil, err
	}
	if err := s.arrangements.Replace(ctx, previous, next, ev); err != nil {
		return nil, err
	}
	return next, nil
}

// RemoveArrangement removes the scope's default or a TLD override, so that
// level inherits again.
func (s *EscrowKeyService) RemoveArrangement(ctx context.Context, scope entities.EscrowKeyScope, tld string) error {
	current, err := s.GetArrangement(ctx, scope, tld)
	if err != nil {
		return err
	}
	actx := s.auditContext(ctx)
	if err := current.Supersede(actx.At); err != nil {
		return err
	}
	ev, err := entities.NewEscrowArrangementAuditEvent(actx, current, entities.EscrowAuditArrangementRemoved)
	if err != nil {
		return err
	}
	return s.arrangements.Remove(ctx, current, ev)
}
