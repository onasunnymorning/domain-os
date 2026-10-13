package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------------------------------------------------------------------------
// EPP credentials — ADR 0014, docs/epp/ARCHITECTURE.md §2.
//
// A registrar's logins, passwords, certificate pins and allowed networks are
// one aggregate (entities.EPPRegistrarAccess). UpdateAccess locks the
// registrar's row, loads the aggregate, applies the change and writes it back
// together with the change's outbox rows, in one transaction. Like the escrow
// key registry, it writes domain_events itself rather than through
// EventPublisher, so a credential change never commits without its event
// (INV-01, #408).
// ---------------------------------------------------------------------------

// EPPLifecycleColumns are the columns every credential shares. It is
// exported because GORM ignores unexported embedded structs.
type EPPLifecycleColumns struct {
	CreatedAt    time.Time `gorm:"not null;autoCreateTime:false"`
	CreatedBy    string
	RetireAt     *time.Time
	RetireKind   string
	RetiredBy    string
	LastUsedAt   *time.Time
	LastUsedFrom string
}

func toLifecycleColumns(l entities.EPPCredentialLifecycle) EPPLifecycleColumns {
	return EPPLifecycleColumns{
		CreatedAt: l.CreatedAt, CreatedBy: l.CreatedBy,
		RetireAt: l.RetireAt, RetireKind: string(l.RetireKind), RetiredBy: l.RetiredBy,
		LastUsedAt: l.LastUsedAt, LastUsedFrom: l.LastUsedFrom,
	}
}

func (c EPPLifecycleColumns) toLifecycle(id uuid.UUID) entities.EPPCredentialLifecycle {
	return entities.EPPCredentialLifecycle{
		ID: id, CreatedAt: c.CreatedAt.UTC(), CreatedBy: c.CreatedBy,
		RetireAt: utcPtr(c.RetireAt), RetireKind: entities.EPPRetireKind(c.RetireKind), RetiredBy: c.RetiredBy,
		LastUsedAt: utcPtr(c.LastUsedAt), LastUsedFrom: c.LastUsedFrom,
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// EPPLoginRecord is the GORM model for epp_logins. LoginID is the clID a
// registrar's system sends in <login>; it is unique across registrars.
type EPPLoginRecord struct {
	LoginID    string    `gorm:"primaryKey;size:16"`
	ClID       string    `gorm:"not null;size:16;index"`
	Role       string    `gorm:"not null"`
	Enabled    bool      `gorm:"not null"`
	SessionCap int       `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"not null;autoCreateTime:false"`
	CreatedBy  string
	// The domain sets UpdatedAt; GORM must not overwrite it on save.
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:false"`
}

// TableName specifies the table name for EPPLoginRecord.
func (EPPLoginRecord) TableName() string { return "epp_logins" }

// EPPPasswordRecord is the GORM model for epp_passwords.
type EPPPasswordRecord struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	LoginID string    `gorm:"not null;size:16;index"`
	Hash    string    `gorm:"not null"`
	EPPLifecycleColumns
}

// TableName specifies the table name for EPPPasswordRecord.
func (EPPPasswordRecord) TableName() string { return "epp_passwords" }

// EPPCertificatePinRecord is the GORM model for epp_certificate_pins.
type EPPCertificatePinRecord struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ClID       string    `gorm:"not null;size:16;index"`
	SPKISHA256 string    `gorm:"column:spki_sha256;not null;size:64;index"`
	Subject    string
	Issuer     string
	Serial     string
	NotBefore  time.Time `gorm:"not null"`
	NotAfter   time.Time `gorm:"not null"`
	// Logins is a JSON array of login IDs; empty means every login.
	Logins string `gorm:"type:text;not null;default:'[]'"`
	EPPLifecycleColumns
}

// TableName specifies the table name for EPPCertificatePinRecord.
func (EPPCertificatePinRecord) TableName() string { return "epp_certificate_pins" }

// EPPAllowedNetworkRecord is the GORM model for epp_allowed_networks.
type EPPAllowedNetworkRecord struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	ClID   string    `gorm:"not null;size:16;index"`
	Prefix string    `gorm:"not null"`
	Note   string
	EPPLifecycleColumns
}

// TableName specifies the table name for EPPAllowedNetworkRecord.
func (EPPAllowedNetworkRecord) TableName() string { return "epp_allowed_networks" }

// EPPCredentialRepository implements repositories.EPPCredentialRepository.
type EPPCredentialRepository struct {
	db *gorm.DB
}

// NewEPPCredentialRepository returns a repository over db.
func NewEPPCredentialRepository(db *gorm.DB) *EPPCredentialRepository {
	return &EPPCredentialRepository{db: db}
}

// GetAccess returns a registrar's access.
func (r *EPPCredentialRepository) GetAccess(ctx context.Context, clid string) (*entities.EPPRegistrarAccess, error) {
	db := r.db.WithContext(ctx)
	if err := db.Select("cl_id").Where("cl_id = ?", clid).Take(&Registrar{}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrRegistrarNotFound
		}
		return nil, fmt.Errorf("get registrar %s: %w", clid, err)
	}
	return loadEPPAccess(db, clid)
}

// GetAccessByLogin returns the access of the registrar that owns loginID.
func (r *EPPCredentialRepository) GetAccessByLogin(ctx context.Context, loginID string) (*entities.EPPRegistrarAccess, error) {
	db := r.db.WithContext(ctx)
	var login EPPLoginRecord
	if err := db.Select("cl_id").Where("login_id = ?", loginID).Take(&login).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEPPLoginNotFound
		}
		return nil, fmt.Errorf("get EPP login %s: %w", loginID, err)
	}
	return loadEPPAccess(db, login.ClID)
}

// UpdateAccess applies fn to a registrar's access under a lock on the
// registrar's row, and saves the result with fn's events.
func (r *EPPCredentialRepository) UpdateAccess(ctx context.Context, clid string, fn func(*entities.EPPRegistrarAccess) ([]entities.DomainEvent, error)) (*entities.EPPRegistrarAccess, error) {
	var out *entities.EPPRegistrarAccess
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("cl_id").Where("cl_id = ?", clid).Take(&Registrar{}).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return entities.ErrRegistrarNotFound
			}
			return fmt.Errorf("lock registrar %s: %w", clid, err)
		}
		access, err := loadEPPAccess(tx, clid)
		if err != nil {
			return err
		}
		before := eppAccessIDs(access)

		events, err := fn(access)
		if err != nil {
			return err
		}
		if err := saveEPPAccess(tx, access, before); err != nil {
			return err
		}
		for _, e := range events {
			rec, err := ToDBDomainEvent(e)
			if err != nil {
				return fmt.Errorf("encode event %s: %w", e.Type, err)
			}
			if err := tx.Create(&rec).Error; err != nil {
				return fmt.Errorf("write event %s: %w", e.Type, err)
			}
		}
		out = access
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListUsableAllowedNetworks returns the networks of every registrar that are
// usable at now, deduplicated.
func (r *EPPCredentialRepository) ListUsableAllowedNetworks(ctx context.Context, now time.Time) ([]netip.Prefix, error) {
	var raw []string
	err := r.db.WithContext(ctx).Model(&EPPAllowedNetworkRecord{}).
		Where("retire_at IS NULL OR retire_at > ?", now).
		Distinct().Order("prefix").Pluck("prefix", &raw).Error
	if err != nil {
		return nil, fmt.Errorf("list EPP allowed networks: %w", err)
	}
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("decode EPP allowed network %q: %w", s, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// ListCompromisedSince returns the IDs of credentials of every kind retired
// as compromised at or after since.
func (r *EPPCredentialRepository) ListCompromisedSince(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, model := range []any{&EPPPasswordRecord{}, &EPPCertificatePinRecord{}, &EPPAllowedNetworkRecord{}} {
		var ids []uuid.UUID
		err := r.db.WithContext(ctx).Model(model).
			Where("retire_kind = ? AND retire_at >= ?", string(entities.EPPRetireCompromise), since).
			Pluck("id", &ids).Error
		if err != nil {
			return nil, fmt.Errorf("list compromised EPP credentials: %w", err)
		}
		out = append(out, ids...)
	}
	return out, nil
}

// eppAccessIDs records which rows existed when the access was loaded, so that
// saving inserts new rows rather than upserting them. An upsert of a new login
// would silently take over another registrar's login with the same ID.
type eppAccessKeys struct {
	logins      map[string]bool
	credentials map[uuid.UUID]bool
}

func eppAccessIDs(a *entities.EPPRegistrarAccess) eppAccessKeys {
	k := eppAccessKeys{logins: map[string]bool{}, credentials: map[uuid.UUID]bool{}}
	for _, l := range a.Logins {
		k.logins[string(l.LoginID)] = true
		for _, p := range l.Passwords {
			k.credentials[p.ID] = true
		}
	}
	for _, p := range a.CertificatePins {
		k.credentials[p.ID] = true
	}
	for _, n := range a.AllowedNetworks {
		k.credentials[n.ID] = true
	}
	return k
}

func loadEPPAccess(db *gorm.DB, clid string) (*entities.EPPRegistrarAccess, error) {
	access := &entities.EPPRegistrarAccess{ClID: entities.ClIDType(clid)}

	var logins []EPPLoginRecord
	if err := db.Where("cl_id = ?", clid).Order("created_at, login_id").Find(&logins).Error; err != nil {
		return nil, fmt.Errorf("load EPP logins of %s: %w", clid, err)
	}
	byID := make(map[string]*entities.EPPLogin, len(logins))
	ids := make([]string, 0, len(logins))
	for _, rec := range logins {
		l := &entities.EPPLogin{
			LoginID: entities.ClIDType(rec.LoginID), Role: entities.EPPLoginRole(rec.Role), Enabled: rec.Enabled,
			SessionCap: rec.SessionCap, CreatedAt: rec.CreatedAt.UTC(), CreatedBy: rec.CreatedBy, UpdatedAt: rec.UpdatedAt.UTC(),
		}
		access.Logins = append(access.Logins, l)
		byID[rec.LoginID] = l
		ids = append(ids, rec.LoginID)
	}
	if len(ids) > 0 {
		var passwords []EPPPasswordRecord
		if err := db.Where("login_id IN ?", ids).Order("created_at, id").Find(&passwords).Error; err != nil {
			return nil, fmt.Errorf("load EPP passwords of %s: %w", clid, err)
		}
		for _, rec := range passwords {
			l := byID[rec.LoginID]
			l.Passwords = append(l.Passwords, &entities.EPPPassword{EPPCredentialLifecycle: rec.toLifecycle(rec.ID), Hash: rec.Hash})
		}
	}

	var pins []EPPCertificatePinRecord
	if err := db.Where("cl_id = ?", clid).Order("created_at, id").Find(&pins).Error; err != nil {
		return nil, fmt.Errorf("load EPP certificate pins of %s: %w", clid, err)
	}
	for _, rec := range pins {
		var restrict []entities.ClIDType
		if err := json.Unmarshal([]byte(rec.Logins), &restrict); err != nil {
			return nil, fmt.Errorf("decode logins of EPP certificate pin %s: %w", rec.ID, err)
		}
		access.CertificatePins = append(access.CertificatePins, &entities.EPPCertificatePin{
			EPPCredentialLifecycle: rec.toLifecycle(rec.ID),
			SPKISHA256:             rec.SPKISHA256, Subject: rec.Subject, Issuer: rec.Issuer, Serial: rec.Serial,
			NotBefore: rec.NotBefore.UTC(), NotAfter: rec.NotAfter.UTC(), Logins: restrict,
		})
	}

	var networks []EPPAllowedNetworkRecord
	if err := db.Where("cl_id = ?", clid).Order("created_at, id").Find(&networks).Error; err != nil {
		return nil, fmt.Errorf("load EPP allowed networks of %s: %w", clid, err)
	}
	for _, rec := range networks {
		prefix, err := netip.ParsePrefix(rec.Prefix)
		if err != nil {
			return nil, fmt.Errorf("decode EPP allowed network %s: %w", rec.ID, err)
		}
		access.AllowedNetworks = append(access.AllowedNetworks, &entities.EPPAllowedNetwork{
			EPPCredentialLifecycle: rec.toLifecycle(rec.ID), Prefix: prefix, Note: rec.Note,
		})
	}
	return access, nil
}

// upsert inserts rec when it is new and saves it otherwise.
func upsert(tx *gorm.DB, isNew bool, rec any) error {
	if isNew {
		return tx.Create(rec).Error
	}
	return tx.Save(rec).Error
}

func saveEPPAccess(tx *gorm.DB, a *entities.EPPRegistrarAccess, before eppAccessKeys) error {
	clid := string(a.ClID)
	for _, l := range a.Logins {
		rec := EPPLoginRecord{
			LoginID: string(l.LoginID), ClID: clid, Role: string(l.Role), Enabled: l.Enabled,
			SessionCap: l.SessionCap, CreatedAt: l.CreatedAt, CreatedBy: l.CreatedBy, UpdatedAt: l.UpdatedAt,
		}
		if err := upsert(tx, !before.logins[rec.LoginID], &rec); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %s", entities.ErrEPPLoginExists, rec.LoginID)
			}
			return fmt.Errorf("save EPP login %s: %w", rec.LoginID, err)
		}
		for _, p := range l.Passwords {
			rec := EPPPasswordRecord{ID: p.ID, LoginID: string(l.LoginID), Hash: p.Hash, EPPLifecycleColumns: toLifecycleColumns(p.EPPCredentialLifecycle)}
			if err := upsert(tx, !before.credentials[p.ID], &rec); err != nil {
				return fmt.Errorf("save EPP password %s: %w", p.ID, err)
			}
		}
	}
	for _, p := range a.CertificatePins {
		restrict := p.Logins
		if restrict == nil {
			restrict = []entities.ClIDType{}
		}
		logins, err := json.Marshal(restrict)
		if err != nil {
			return fmt.Errorf("encode logins of EPP certificate pin %s: %w", p.ID, err)
		}
		rec := EPPCertificatePinRecord{
			ID: p.ID, ClID: clid, SPKISHA256: p.SPKISHA256, Subject: p.Subject, Issuer: p.Issuer, Serial: p.Serial,
			NotBefore: p.NotBefore, NotAfter: p.NotAfter, Logins: string(logins),
			EPPLifecycleColumns: toLifecycleColumns(p.EPPCredentialLifecycle),
		}
		if err := upsert(tx, !before.credentials[p.ID], &rec); err != nil {
			return fmt.Errorf("save EPP certificate pin %s: %w", p.ID, err)
		}
	}
	for _, n := range a.AllowedNetworks {
		rec := EPPAllowedNetworkRecord{ID: n.ID, ClID: clid, Prefix: n.Prefix.String(), Note: n.Note, EPPLifecycleColumns: toLifecycleColumns(n.EPPCredentialLifecycle)}
		if err := upsert(tx, !before.credentials[n.ID], &rec); err != nil {
			return fmt.Errorf("save EPP allowed network %s: %w", n.ID, err)
		}
	}
	return nil
}
