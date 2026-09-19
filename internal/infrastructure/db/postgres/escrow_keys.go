package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Escrow key registry — issue #429, ADR-0009.
//
// Scope is enforced here, in SQL (ADR-0006). Rows carry owner_kind and
// tenant_id; an operator scope reads `owner_kind = 'platform' OR tenant_id = ?`
// and the platform scope reads `owner_kind = 'platform'`.
//
// The repository interfaces live in pkg/domain/repositories; this package cannot
// assert them at compile time because that package still imports this one
// (INV-14 exception fx_interface.go, #400). The composition roots do.
//
// Every mutation writes its audit row and its outbox row (domain_events) in the
// same transaction as the change. That is deliberately not EventPublisher: its
// insert is separate from the business write and its errors are swallowed by
// callers (INV-01 defects 1 and 2, #408), and a key revocation must never
// commit without its audit trail, nor an event describe a change that rolled
// back.
// ---------------------------------------------------------------------------

// EscrowPartyRecord is the GORM model for escrow_parties.
type EscrowPartyRecord struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OwnerKind string    `gorm:"not null"`
	TenantID  string    `gorm:"not null;default:''"`
	Name      string    `gorm:"not null"`
	Kind      string    `gorm:"not null"`
	Side      string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	CreatedBy string
}

// TableName specifies the table name for EscrowPartyRecord.
func (EscrowPartyRecord) TableName() string { return "escrow_parties" }

// EscrowKeyVersionRecord is the GORM model for escrow_key_versions.
type EscrowKeyVersionRecord struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey"`
	PartyID              uuid.UUID `gorm:"type:uuid;not null"`
	OwnerKind            string    `gorm:"not null"`
	TenantID             string    `gorm:"not null;default:''"`
	Purpose              string    `gorm:"not null"`
	Version              int       `gorm:"not null"`
	Fingerprint          string    `gorm:"not null;size:64"`
	ArmoredPublicKey     string    `gorm:"type:text"`
	SecretName           string
	SecretBackendVersion string
	State                string `gorm:"not null"`
	NotBefore            *time.Time
	NotAfter             *time.Time
	KeyExpiresAt         *time.Time
	LastProbeAt          *time.Time
	LastProbeOK          bool `gorm:"not null;default:false"`
	ActivatedAt          *time.Time
	DeactivatedAt        *time.Time
	RevokedAt            *time.Time
	RevocationReason     string
	Compromised          bool `gorm:"not null;default:false"`
	DestroyedAt          *time.Time
	CreatedAt            time.Time `gorm:"not null"`
	CreatedBy            string
}

// TableName specifies the table name for EscrowKeyVersionRecord.
func (EscrowKeyVersionRecord) TableName() string { return "escrow_key_versions" }

// EscrowArrangementRecord is the GORM model for escrow_arrangements.
type EscrowArrangementRecord struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Level            string     `gorm:"not null"`
	TenantID         string     `gorm:"not null;default:''"`
	TLD              string     `gorm:"not null;default:''"`
	Direction        string     `gorm:"not null"`
	DepositorPartyID *uuid.UUID `gorm:"type:uuid"`
	ReceiverPartyID  *uuid.UUID `gorm:"type:uuid"`
	Revision         int        `gorm:"not null"`
	CreatedAt        time.Time  `gorm:"not null"`
	CreatedBy        string
	SupersededAt     *time.Time
}

// TableName specifies the table name for EscrowArrangementRecord.
func (EscrowArrangementRecord) TableName() string { return "escrow_arrangements" }

// EscrowKeyAuditEventRecord is the GORM model for escrow_key_audit_events.
type EscrowKeyAuditEventRecord struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OwnerKind     string     `gorm:"not null"`
	TenantID      string     `gorm:"not null;default:''"`
	Action        string     `gorm:"not null"`
	Subject       string     `gorm:"not null"`
	SubjectID     uuid.UUID  `gorm:"type:uuid;not null"`
	PartyID       *uuid.UUID `gorm:"type:uuid"`
	Purpose       string
	Version       int
	Fingerprint   string
	StateBefore   string
	StateAfter    string
	TLD           string
	Reason        string
	Compromised   bool `gorm:"not null;default:false"`
	Actor         string
	TraceID       string
	CorrelationID string
	At            time.Time `gorm:"not null"`
}

// TableName specifies the table name for EscrowKeyAuditEventRecord.
func (EscrowKeyAuditEventRecord) TableName() string { return "escrow_key_audit_events" }

// escrowKeyManualIndexes are appended to AutoMigrate's manual index list.
var escrowKeyManualIndexes = []string{
	// escrow_parties: scoped listing, newest first
	"CREATE INDEX IF NOT EXISTS idx_escrow_parties_owner_created ON escrow_parties (owner_kind, tenant_id, created_at DESC, id DESC)",
	// escrow_key_versions: one number per party and purpose; a concurrent import loses cleanly
	"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_key_versions_party_purpose_version ON escrow_key_versions (party_id, purpose, version)",
	// escrow_key_versions: a party cannot hold the same key twice for one purpose. Per party, not
	// per owner: one registry backend may legitimately be recorded as two parties by one operator.
	"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_key_versions_party_purpose_fingerprint ON escrow_key_versions (party_id, purpose, fingerprint)",
	// escrow_key_versions: scoped lookups by owner
	"CREATE INDEX IF NOT EXISTS idx_escrow_key_versions_owner ON escrow_key_versions (owner_kind, tenant_id)",
	// escrow_arrangements: at most one live row per level/tenant/TLD/direction
	"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_arrangements_live ON escrow_arrangements (level, tenant_id, tld, direction) WHERE superseded_at IS NULL",
	// escrow_arrangements: revisions are unique within their slot
	"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_arrangements_revision ON escrow_arrangements (level, tenant_id, tld, direction, revision)",
	// escrow_key_audit_events: a party's history, newest first
	"CREATE INDEX IF NOT EXISTS idx_escrow_key_audit_party_at ON escrow_key_audit_events (party_id, at DESC, id DESC)",
}

// ---------- scope ----------

func escrowOwnerScope(q *gorm.DB, scope entities.EscrowKeyScope) (*gorm.DB, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if scope.IsPlatform() {
		return q.Where("owner_kind = ?", string(entities.EscrowKeyOwnerPlatform)), nil
	}
	return q.Where("(owner_kind = ? OR (owner_kind = ? AND tenant_id = ?))",
		string(entities.EscrowKeyOwnerPlatform), string(entities.EscrowKeyOwnerOperator), scope.Operator().String()), nil
}

func ownerColumns(o entities.EscrowKeyOwner) (string, string) {
	return string(o.Kind), o.Operator.String()
}

func ownerFromColumns(kind, tenant string) entities.EscrowKeyOwner {
	return entities.EscrowKeyOwner{Kind: entities.EscrowKeyOwnerKind(kind), Operator: entities.OperatorID(tenant)}
}

func isUniqueViolation(err error) bool {
	var perr *pgconn.PgError
	return errors.As(err, &perr) && perr.Code == "23505"
}

func uniqueViolationConstraint(err error) string {
	var perr *pgconn.PgError
	if errors.As(err, &perr) {
		return perr.ConstraintName
	}
	return ""
}

// ---------- mappers ----------

func toDBEscrowParty(p *entities.EscrowParty) *EscrowPartyRecord {
	kind, tenant := ownerColumns(p.Owner)
	return &EscrowPartyRecord{ID: p.ID, OwnerKind: kind, TenantID: tenant, Name: p.Name, Kind: string(p.Kind), Side: string(p.Side), CreatedAt: p.CreatedAt, CreatedBy: p.CreatedBy}
}

func fromDBEscrowParty(r *EscrowPartyRecord) *entities.EscrowParty {
	return &entities.EscrowParty{ID: r.ID, Owner: ownerFromColumns(r.OwnerKind, r.TenantID), Name: r.Name,
		Kind: entities.EscrowPartyKind(r.Kind), Side: entities.EscrowPartySide(r.Side), CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy}
}

func toDBEscrowKeyVersion(v *entities.EscrowKeyVersion) *EscrowKeyVersionRecord {
	kind, tenant := ownerColumns(v.Owner)
	rec := &EscrowKeyVersionRecord{
		ID: v.ID, PartyID: v.PartyID, OwnerKind: kind, TenantID: tenant, Purpose: string(v.Purpose), Version: v.Version,
		Fingerprint: v.Fingerprint, ArmoredPublicKey: v.ArmoredPublicKey, State: string(v.State),
		NotBefore: v.NotBefore, NotAfter: v.NotAfter, KeyExpiresAt: v.KeyExpiresAt,
		LastProbeAt: v.LastProbeAt, LastProbeOK: v.LastProbeOK,
		ActivatedAt: v.ActivatedAt, DeactivatedAt: v.DeactivatedAt, RevokedAt: v.RevokedAt,
		RevocationReason: v.RevocationReason, Compromised: v.Compromised, DestroyedAt: v.DestroyedAt,
		CreatedAt: v.CreatedAt, CreatedBy: v.CreatedBy,
	}
	if v.SecretRef != nil {
		rec.SecretName, rec.SecretBackendVersion = v.SecretRef.Name, v.SecretRef.BackendVersionID
	}
	return rec
}

func fromDBEscrowKeyVersion(r *EscrowKeyVersionRecord) *entities.EscrowKeyVersion {
	v := &entities.EscrowKeyVersion{
		ID: r.ID, PartyID: r.PartyID, Owner: ownerFromColumns(r.OwnerKind, r.TenantID), Purpose: entities.EscrowKeyPurpose(r.Purpose),
		Version: r.Version, Fingerprint: r.Fingerprint, ArmoredPublicKey: r.ArmoredPublicKey, State: entities.EscrowKeyVersionState(r.State),
		NotBefore: r.NotBefore, NotAfter: r.NotAfter, KeyExpiresAt: r.KeyExpiresAt,
		LastProbeAt: r.LastProbeAt, LastProbeOK: r.LastProbeOK,
		ActivatedAt: r.ActivatedAt, DeactivatedAt: r.DeactivatedAt, RevokedAt: r.RevokedAt,
		RevocationReason: r.RevocationReason, Compromised: r.Compromised, DestroyedAt: r.DestroyedAt,
		CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy,
	}
	if r.SecretName != "" {
		v.SecretRef = &entities.EscrowSecretRef{Name: r.SecretName, BackendVersionID: r.SecretBackendVersion}
	}
	return v
}

func toDBEscrowArrangement(a *entities.EscrowArrangement) *EscrowArrangementRecord {
	return &EscrowArrangementRecord{ID: a.ID, Level: string(a.Level), TenantID: a.Operator.String(), TLD: a.TLD, Direction: string(a.Direction),
		DepositorPartyID: a.DepositorPartyID, ReceiverPartyID: a.ReceiverPartyID, Revision: a.Revision,
		CreatedAt: a.CreatedAt, CreatedBy: a.CreatedBy, SupersededAt: a.SupersededAt}
}

func fromDBEscrowArrangement(r *EscrowArrangementRecord) *entities.EscrowArrangement {
	return &entities.EscrowArrangement{ID: r.ID, Level: entities.EscrowArrangementLevel(r.Level), Operator: entities.OperatorID(r.TenantID), TLD: r.TLD,
		Direction: entities.EscrowArrangementDirection(r.Direction), DepositorPartyID: r.DepositorPartyID, ReceiverPartyID: r.ReceiverPartyID,
		Revision: r.Revision, CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy, SupersededAt: r.SupersededAt}
}

func toDBEscrowKeyAuditEvent(e *entities.EscrowKeyAuditEvent) *EscrowKeyAuditEventRecord {
	kind, tenant := ownerColumns(e.Owner)
	return &EscrowKeyAuditEventRecord{ID: e.ID, OwnerKind: kind, TenantID: tenant, Action: string(e.Action), Subject: string(e.Subject),
		SubjectID: e.SubjectID, PartyID: e.PartyID, Purpose: string(e.Purpose), Version: e.Version, Fingerprint: e.Fingerprint,
		StateBefore: e.StateBefore, StateAfter: e.StateAfter, TLD: e.TLD, Reason: e.Reason, Compromised: e.Compromised,
		Actor: e.Actor, TraceID: e.TraceID, CorrelationID: e.CorrelationID, At: e.At}
}

func fromDBEscrowKeyAuditEvent(r *EscrowKeyAuditEventRecord) *entities.EscrowKeyAuditEvent {
	return &entities.EscrowKeyAuditEvent{ID: r.ID, Owner: ownerFromColumns(r.OwnerKind, r.TenantID), Action: entities.EscrowKeyAuditAction(r.Action),
		Subject: entities.EscrowKeyAuditSubject(r.Subject), SubjectID: r.SubjectID, PartyID: r.PartyID, Purpose: entities.EscrowKeyPurpose(r.Purpose),
		Version: r.Version, Fingerprint: r.Fingerprint, StateBefore: r.StateBefore, StateAfter: r.StateAfter, TLD: r.TLD,
		Reason: r.Reason, Compromised: r.Compromised, Actor: r.Actor, TraceID: r.TraceID, CorrelationID: r.CorrelationID, At: r.At}
}

// writeEscrowKeyAudit inserts the audit row and its outbox event inside tx.
func writeEscrowKeyAudit(tx *gorm.DB, audits ...*entities.EscrowKeyAuditEvent) error {
	for _, a := range audits {
		if a == nil {
			return errors.Join(entities.ErrInvalidEscrowKeyAuditEvent, errors.New("every key-registry change needs an audit event"))
		}
		if err := tx.Create(toDBEscrowKeyAuditEvent(a)).Error; err != nil {
			return fmt.Errorf("escrow key audit insert: %w", err)
		}
		event, err := ToDBDomainEvent(a.DomainEvent())
		if err != nil {
			return fmt.Errorf("escrow key outbox encode: %w", err)
		}
		if err := tx.Create(&event).Error; err != nil {
			return fmt.Errorf("escrow key outbox insert: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Parties
// ---------------------------------------------------------------------------

// GormEscrowPartyRepository implements repositories.EscrowPartyRepository.
type GormEscrowPartyRepository struct{ db *gorm.DB }

// NewEscrowPartyRepository creates a new GormEscrowPartyRepository.
func NewEscrowPartyRepository(db *gorm.DB) *GormEscrowPartyRepository {
	return &GormEscrowPartyRepository{db: db}
}

// Create inserts a party with its audit entry.
func (r *GormEscrowPartyRepository) Create(ctx context.Context, p *entities.EscrowParty, audit *entities.EscrowKeyAuditEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toDBEscrowParty(p)).Error; err != nil {
			return fmt.Errorf("EscrowParty.Create(id=%s): %w", p.ID, err)
		}
		return writeEscrowKeyAudit(tx, audit)
	})
}

// GetByID retrieves a party visible in the scope.
func (r *GormEscrowPartyRepository) GetByID(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowParty, error) {
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Where("id = ?", id), scope)
	if err != nil {
		return nil, err
	}
	var rec EscrowPartyRecord
	if err := q.First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowPartyNotFound
		}
		return nil, fmt.Errorf("EscrowParty.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowParty(&rec), nil
}

// List returns the parties visible in the scope, newest first (INV-11).
func (r *GormEscrowPartyRepository) List(ctx context.Context, scope entities.EscrowKeyScope, lq queries.ListItemsQuery) ([]*entities.EscrowParty, string, error) {
	pageSize := clampPageSize(lq.PageSize)
	q, err := escrowOwnerScope(r.db.WithContext(ctx), scope)
	if err != nil {
		return nil, "", err
	}
	if lq.PageCursor != "" {
		t, id, err := decodeObsCursor(lq.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("EscrowParty.List(cursor): %w", err)
		}
		q = q.Where("(created_at < ? OR (created_at = ? AND id < ?))", t, t, id)
	}
	var records []EscrowPartyRecord
	if err := q.Order("created_at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("EscrowParty.List: %w", err)
	}
	var next string
	if len(records) > pageSize {
		last := records[pageSize-1]
		next = encodeObsCursor(last.CreatedAt, last.ID)
		records = records[:pageSize]
	}
	out := make([]*entities.EscrowParty, len(records))
	for i := range records {
		out[i] = fromDBEscrowParty(&records[i])
	}
	return out, next, nil
}

// ---------------------------------------------------------------------------
// Key versions
// ---------------------------------------------------------------------------

// GormEscrowKeyVersionRepository implements repositories.EscrowKeyVersionRepository.
type GormEscrowKeyVersionRepository struct{ db *gorm.DB }

// NewEscrowKeyVersionRepository creates a new GormEscrowKeyVersionRepository.
func NewEscrowKeyVersionRepository(db *gorm.DB) *GormEscrowKeyVersionRepository {
	return &GormEscrowKeyVersionRepository{db: db}
}

// Create inserts a version with its audit entry.
func (r *GormEscrowKeyVersionRepository) Create(ctx context.Context, v *entities.EscrowKeyVersion, audit *entities.EscrowKeyAuditEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toDBEscrowKeyVersion(v)).Error; err != nil {
			if isUniqueViolation(err) {
				if uniqueViolationConstraint(err) == "uq_escrow_key_versions_party_purpose_fingerprint" {
					return entities.ErrEscrowKeyDuplicateFingerprint
				}
				return entities.ErrEscrowKeyVersionConflict
			}
			return fmt.Errorf("EscrowKeyVersion.Create(id=%s): %w", v.ID, err)
		}
		return writeEscrowKeyAudit(tx, audit)
	})
}

// GetByID retrieves a version visible in the scope.
func (r *GormEscrowKeyVersionRepository) GetByID(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Where("id = ?", id), scope)
	if err != nil {
		return nil, err
	}
	var rec EscrowKeyVersionRecord
	if err := q.First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowKeyVersionNotFound
		}
		return nil, fmt.Errorf("EscrowKeyVersion.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowKeyVersion(&rec), nil
}

// GetByIDs returns the visible versions among ids.
func (r *GormEscrowKeyVersionRepository) GetByIDs(ctx context.Context, scope entities.EscrowKeyScope, ids []uuid.UUID) ([]*entities.EscrowKeyVersion, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Where("id IN ?", ids), scope)
	if err != nil {
		return nil, err
	}
	var records []EscrowKeyVersionRecord
	if err := q.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("EscrowKeyVersion.GetByIDs(n=%d): %w", len(ids), err)
	}
	return versionsFromRecords(records), nil
}

// ListByParty returns a party's versions, newest version first.
func (r *GormEscrowKeyVersionRepository) ListByParty(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) ([]*entities.EscrowKeyVersion, error) {
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Where("party_id = ?", partyID), scope)
	if err != nil {
		return nil, err
	}
	if purpose != "" {
		q = q.Where("purpose = ?", string(purpose))
	}
	var records []EscrowKeyVersionRecord
	if err := q.Order("purpose ASC, version DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("EscrowKeyVersion.ListByParty(party=%s): %w", partyID, err)
	}
	return versionsFromRecords(records), nil
}

// ActivePurposesByParty groups the ACTIVE versions of the given parties by
// party and purpose, in one query: the parties list needs this for every row
// at once, and asking per party would be a query per party.
func (r *GormEscrowKeyVersionRepository) ActivePurposesByParty(ctx context.Context, scope entities.EscrowKeyScope, partyIDs []uuid.UUID) (map[uuid.UUID][]entities.EscrowKeyPurpose, error) {
	if len(partyIDs) == 0 {
		return map[uuid.UUID][]entities.EscrowKeyPurpose{}, nil
	}
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Model(&EscrowKeyVersionRecord{}).
		Where("party_id IN ? AND state = ?", partyIDs, string(entities.EscrowKeyActive)), scope)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		PartyID uuid.UUID
		Purpose string
	}
	if err := q.Select("DISTINCT party_id, purpose").Order("purpose ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("EscrowKeyVersion.ActivePurposesByParty: %w", err)
	}
	active := make(map[uuid.UUID][]entities.EscrowKeyPurpose, len(rows))
	for _, row := range rows {
		active[row.PartyID] = append(active[row.PartyID], entities.EscrowKeyPurpose(row.Purpose))
	}
	return active, nil
}

// NextVersionNumber returns one more than the party and purpose's highest version.
func (r *GormEscrowKeyVersionRepository) NextVersionNumber(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) (int, error) {
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Model(&EscrowKeyVersionRecord{}).Where("party_id = ? AND purpose = ?", partyID, string(purpose)), scope)
	if err != nil {
		return 0, err
	}
	var highest *int
	if err := q.Select("MAX(version)").Scan(&highest).Error; err != nil {
		return 0, fmt.Errorf("EscrowKeyVersion.NextVersionNumber(party=%s): %w", partyID, err)
	}
	if highest == nil {
		return 1, nil
	}
	return *highest + 1, nil
}

// ApplyTransitions applies every conditional transition and audit entry atomically.
func (r *GormEscrowKeyVersionRepository) ApplyTransitions(ctx context.Context, transitions []entities.EscrowKeyVersionTransition, audits []*entities.EscrowKeyAuditEvent) error {
	if len(transitions) == 0 {
		return nil
	}
	if len(audits) == 0 {
		return errors.Join(entities.ErrInvalidEscrowKeyAuditEvent, errors.New("every key-registry change needs an audit event"))
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, t := range transitions {
			v := t.Version
			kind, tenant := ownerColumns(v.Owner)
			rec := toDBEscrowKeyVersion(v)
			res := tx.Model(&EscrowKeyVersionRecord{}).
				Where("id = ? AND state = ? AND owner_kind = ? AND tenant_id = ?", v.ID, string(t.ExpectedState), kind, tenant).
				Updates(map[string]any{
					"state":                  rec.State,
					"secret_backend_version": rec.SecretBackendVersion,
					"last_probe_at":          rec.LastProbeAt,
					"last_probe_ok":          rec.LastProbeOK,
					"activated_at":           rec.ActivatedAt,
					"deactivated_at":         rec.DeactivatedAt,
					"revoked_at":             rec.RevokedAt,
					"revocation_reason":      rec.RevocationReason,
					"compromised":            rec.Compromised,
					"destroyed_at":           rec.DestroyedAt,
				})
			if res.Error != nil {
				return fmt.Errorf("EscrowKeyVersion.ApplyTransitions(id=%s): %w", v.ID, res.Error)
			}
			if res.RowsAffected != 1 {
				return entities.ErrEscrowKeyVersionConflict
			}
		}
		return writeEscrowKeyAudit(tx, audits...)
	})
}

func versionsFromRecords(records []EscrowKeyVersionRecord) []*entities.EscrowKeyVersion {
	out := make([]*entities.EscrowKeyVersion, len(records))
	for i := range records {
		out[i] = fromDBEscrowKeyVersion(&records[i])
	}
	return out
}

// ---------------------------------------------------------------------------
// Arrangements
// ---------------------------------------------------------------------------

// GormEscrowArrangementRepository implements repositories.EscrowArrangementRepository.
type GormEscrowArrangementRepository struct{ db *gorm.DB }

// NewEscrowArrangementRepository creates a new GormEscrowArrangementRepository.
func NewEscrowArrangementRepository(db *gorm.DB) *GormEscrowArrangementRepository {
	return &GormEscrowArrangementRepository{db: db}
}

// GetLive returns the live arrangement at one level.
func (r *GormEscrowArrangementRepository) GetLive(ctx context.Context, scope entities.EscrowKeyScope, level entities.EscrowArrangementLevel, tld string, direction entities.EscrowArrangementDirection) (*entities.EscrowArrangement, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	q := r.db.WithContext(ctx).Where("level = ? AND direction = ? AND superseded_at IS NULL", string(level), string(direction))
	switch level {
	case entities.EscrowArrangementPlatform:
		q = q.Where("tenant_id = '' AND tld = ''")
	case entities.EscrowArrangementOperator, entities.EscrowArrangementTLD:
		if scope.IsPlatform() {
			// The platform scope does not see into operators (ADR-0006).
			return nil, entities.ErrEscrowArrangementNotFound
		}
		q = q.Where("tenant_id = ?", scope.Operator().String())
		if level == entities.EscrowArrangementTLD {
			q = q.Where("tld = ?", tld)
		} else {
			q = q.Where("tld = ''")
		}
	default:
		return nil, entities.ErrInvalidEscrowArrangement
	}
	var rec EscrowArrangementRecord
	if err := q.First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowArrangementNotFound
		}
		return nil, fmt.Errorf("EscrowArrangement.GetLive(level=%s): %w", level, err)
	}
	return fromDBEscrowArrangement(&rec), nil
}

// ListLiveTLDOverrides returns an operator's live TLD-level rows, by TLD.
func (r *GormEscrowArrangementRepository) ListLiveTLDOverrides(ctx context.Context, scope entities.OperatorID, direction entities.EscrowArrangementDirection) ([]*entities.EscrowArrangement, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	var records []EscrowArrangementRecord
	err := r.db.WithContext(ctx).
		Where("level = ? AND tenant_id = ? AND direction = ? AND superseded_at IS NULL", string(entities.EscrowArrangementTLD), scope.String(), string(direction)).
		Order("tld ASC").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("EscrowArrangement.ListLiveTLDOverrides: %w", err)
	}
	return arrangementsFromRecords(records), nil
}

// ListLiveReferencing returns the visible live rows naming the party on either side.
func (r *GormEscrowArrangementRepository) ListLiveReferencing(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID) ([]*entities.EscrowArrangement, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	q := r.db.WithContext(ctx).Where("superseded_at IS NULL AND (depositor_party_id = ? OR receiver_party_id = ?)", partyID, partyID)
	if scope.IsPlatform() {
		q = q.Where("level = ?", string(entities.EscrowArrangementPlatform))
	} else {
		q = q.Where("(level = ? OR tenant_id = ?)", string(entities.EscrowArrangementPlatform), scope.Operator().String())
	}
	var records []EscrowArrangementRecord
	if err := q.Order("level ASC, tld ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("EscrowArrangement.ListLiveReferencing(party=%s): %w", partyID, err)
	}
	return arrangementsFromRecords(records), nil
}

// Replace supersedes previous and inserts next atomically.
func (r *GormEscrowArrangementRepository) Replace(ctx context.Context, previous, next *entities.EscrowArrangement, audit *entities.EscrowKeyAuditEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if previous != nil {
			if err := supersedeArrangement(tx, previous, next.CreatedAt); err != nil {
				return err
			}
		}
		if err := tx.Create(toDBEscrowArrangement(next)).Error; err != nil {
			if isUniqueViolation(err) {
				return entities.ErrEscrowArrangementConflict
			}
			return fmt.Errorf("EscrowArrangement.Replace(id=%s): %w", next.ID, err)
		}
		return writeEscrowKeyAudit(tx, audit)
	})
}

// Remove supersedes a live row without a replacement.
func (r *GormEscrowArrangementRepository) Remove(ctx context.Context, current *entities.EscrowArrangement, audit *entities.EscrowKeyAuditEvent) error {
	if current.SupersededAt == nil {
		return errors.Join(entities.ErrInvalidEscrowArrangement, errors.New("supersede the arrangement before removing it"))
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := supersedeArrangement(tx, current, *current.SupersededAt); err != nil {
			return err
		}
		return writeEscrowKeyAudit(tx, audit)
	})
}

func supersedeArrangement(tx *gorm.DB, a *entities.EscrowArrangement, at time.Time) error {
	res := tx.Model(&EscrowArrangementRecord{}).
		Where("id = ? AND revision = ? AND tenant_id = ? AND superseded_at IS NULL", a.ID, a.Revision, a.Operator.String()).
		Update("superseded_at", at.UTC())
	if res.Error != nil {
		return fmt.Errorf("EscrowArrangement.supersede(id=%s): %w", a.ID, res.Error)
	}
	if res.RowsAffected != 1 {
		return entities.ErrEscrowArrangementConflict
	}
	return nil
}

func arrangementsFromRecords(records []EscrowArrangementRecord) []*entities.EscrowArrangement {
	out := make([]*entities.EscrowArrangement, len(records))
	for i := range records {
		out[i] = fromDBEscrowArrangement(&records[i])
	}
	return out
}

// ---------------------------------------------------------------------------
// Audit trail
// ---------------------------------------------------------------------------

// GormEscrowKeyAuditRepository implements repositories.EscrowKeyAuditRepository.
type GormEscrowKeyAuditRepository struct{ db *gorm.DB }

// NewEscrowKeyAuditRepository creates a new GormEscrowKeyAuditRepository.
func NewEscrowKeyAuditRepository(db *gorm.DB) *GormEscrowKeyAuditRepository {
	return &GormEscrowKeyAuditRepository{db: db}
}

// ListByParty returns a party's history, newest first (INV-11).
func (r *GormEscrowKeyAuditRepository) ListByParty(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, lq queries.ListItemsQuery) ([]*entities.EscrowKeyAuditEvent, string, error) {
	pageSize := clampPageSize(lq.PageSize)
	q, err := escrowOwnerScope(r.db.WithContext(ctx).Where("party_id = ?", partyID), scope)
	if err != nil {
		return nil, "", err
	}
	if lq.PageCursor != "" {
		t, id, err := decodeObsCursor(lq.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("EscrowKeyAudit.ListByParty(cursor): %w", err)
		}
		q = q.Where("(at < ? OR (at = ? AND id < ?))", t, t, id)
	}
	var records []EscrowKeyAuditEventRecord
	if err := q.Order("at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("EscrowKeyAudit.ListByParty(party=%s): %w", partyID, err)
	}
	var next string
	if len(records) > pageSize {
		last := records[pageSize-1]
		next = encodeObsCursor(last.At, last.ID)
		records = records[:pageSize]
	}
	out := make([]*entities.EscrowKeyAuditEvent, len(records))
	for i := range records {
		out[i] = fromDBEscrowKeyAuditEvent(&records[i])
	}
	return out, next, nil
}
