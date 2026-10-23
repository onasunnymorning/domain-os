package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// DomainTransferRecord is one transfer request of a domain
// (entities.DomainTransfer). A domain's transfers are kept after it is
// purged, like its events.
type DomainTransferRecord struct {
	ID             int64     `gorm:"primaryKey"`
	DomainRoID     int64     `gorm:"not null;index:idx_domain_transfers_domain,priority:1"`
	DomainName     string    `gorm:"not null"`
	Status         string    `gorm:"not null;index:idx_domain_transfers_pending,priority:1"`
	RequestingClID string    `gorm:"not null"`
	RequestedAt    time.Time `gorm:"not null"`
	LosingClID     string    `gorm:"not null"`
	ActingClID     string    `gorm:"not null"`
	ActionDate     time.Time `gorm:"not null;index:idx_domain_transfers_pending,priority:2"`
	Years          int       `gorm:"not null"`
	ExpiryDate     *time.Time
}

// TableName pins the table name.
func (DomainTransferRecord) TableName() string { return "domain_transfers" }

// PollMessageRecord is a message in a registrar's queue
// (entities.PollMessage), with the transfer it is about as it stood then.
type PollMessageRecord struct {
	ID        int64     `gorm:"primaryKey"`
	ClID      string    `gorm:"not null;index:idx_poll_messages_queue,priority:1"`
	CreatedAt time.Time `gorm:"not null"`
	Message   string    `gorm:"not null"`
	// The transfer, as <domain:trnData> shows it.
	TransferID     *int64 `gorm:"index"`
	DomainRoID     *int64
	DomainName     string
	TrStatus       string
	RequestingClID string
	RequestedAt    *time.Time
	LosingClID     string
	ActingClID     string
	ActionDate     *time.Time
	Years          int
	ExpiryDate     *time.Time
}

// TableName pins the table name.
func (PollMessageRecord) TableName() string { return "poll_messages" }

// DomainTransferRepository is the Postgres DomainTransferRepository and
// PollMessageRepository (pkg/domain/repositories).
type DomainTransferRepository struct {
	db         *gorm.DB
	repository entities.RepositoryID
}

// NewDomainTransferRepository returns the repository. repository is the
// installation's ROID repository ID (ADR 0017).
func NewDomainTransferRepository(db *gorm.DB, repository entities.RepositoryID) *DomainTransferRepository {
	return &DomainTransferRepository{db: db, repository: repository}
}

// RequestTransfer implements repositories.DomainTransferRepository.
func (r *DomainTransferRepository) RequestTransfer(ctx context.Context, d *entities.Domain, t *entities.DomainTransfer, msgs []*entities.PollMessage) (*entities.Domain, error) {
	dbDomain := ToDBDomain(d)
	stored := &Domain{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateDomainRowIfAt(tx, dbDomain, d.UpdatedAt); err != nil {
			return err
		}
		rec := toTransferRecord(t)
		if err := tx.Create(rec).Error; err != nil {
			return err
		}
		t.ID = rec.ID
		if err := queueMessages(tx, msgs, t.ID); err != nil {
			return err
		}
		return tx.Preload("Hosts").Preload("DSRecords", orderDS).First(stored, dbDomain.RoID).Error
	})
	if err != nil {
		return nil, r.notWritten(ctx, dbDomain.RoID, err)
	}
	return ToDomain(stored, r.repository), nil
}

// ResolveTransfer implements repositories.DomainTransferRepository. Locks
// are taken in the order the other domain and host writes take them: the
// domain's row, then hosts in ROID order.
func (r *DomainTransferRepository) ResolveTransfer(ctx context.Context, res *entities.TransferResolution) (*entities.Domain, error) {
	d, t := res.Domain, res.Transfer
	dbDomain := ToDBDomain(d)
	stored := &Domain{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateDomainRowIfAtFrom(tx, dbDomain, d.UpdatedAt, t.LosingClID.String()); err != nil {
			return err
		}
		rec := toTransferRecord(t)
		if err := written(tx.Model(&DomainTransferRecord{}).
			Where("id = ? AND status = ?", t.ID, string(entities.TransferStatusPending)).
			Select("status", "acting_cl_id", "action_date", "expiry_date").
			Updates(rec)); err != nil {
			return err
		}
		if t.Status.IsApproved() {
			if err := moveHostsWithDomain(tx, dbDomain, t, res.GainingHosts); err != nil {
				return err
			}
		}
		if err := queueMessages(tx, res.Messages, t.ID); err != nil {
			return err
		}
		return tx.Preload("Hosts").Preload("DSRecords", orderDS).First(stored, dbDomain.RoID).Error
	})
	if err != nil {
		return nil, r.notWritten(ctx, dbDomain.RoID, err)
	}
	return ToDomain(stored, r.repository), nil
}

// moveHostsWithDomain moves the hosts of an approved transfer, as
// entities.TransferResolution describes.
func moveHostsWithDomain(tx *gorm.DB, dbDomain *Domain, t *entities.DomainTransfer, gainingHosts []*entities.Host) error {
	losing, gaining := t.LosingClID.String(), t.RequestingClID.String()
	now := dbDomain.UpdatedAt

	// The links as they are, locked with their hosts.
	var links []struct {
		RoID int64
		Name string
		ClID string
	}
	if err := tx.Raw(`SELECT h.ro_id, h.name, h.cl_id FROM domain_hosts dh JOIN hosts h ON h.ro_id = dh.host_ro_id
		WHERE dh.domain_ro_id = ? ORDER BY h.ro_id FOR UPDATE OF h`, dbDomain.RoID).Scan(&links).Error; err != nil {
		return err
	}
	affected := make([]int64, 0, len(links))
	for _, l := range links {
		affected = append(affected, l.RoID)
	}

	// The losing registrar's hosts under the domain go with it, but for a
	// name the gaining registrar already has.
	var moved []int64
	if err := tx.Raw(`SELECT ro_id FROM hosts h WHERE h.cl_id = ? AND h.name LIKE ?
		AND NOT EXISTS (SELECT 1 FROM hosts g WHERE g.cl_id = ? AND g.name = h.name)
		ORDER BY ro_id FOR UPDATE`, losing, "%."+dbDomain.Name, gaining).Scan(&moved).Error; err != nil {
		return err
	}
	if len(moved) > 0 {
		if err := tx.Exec("UPDATE hosts SET cl_id = ?, up_rr = ?, updated_at = ? WHERE ro_id IN ?", gaining, gaining, now, moved).Error; err != nil {
			return err
		}
	}
	affected = unionIDs(affected, moved)

	// Each nameserver to re-link goes to the gaining registrar's host of
	// that name, created if it has none.
	byName := map[string]*entities.Host{}
	for _, h := range gainingHosts {
		byName[h.Name.String()] = h
	}
	for _, l := range links {
		template, relink := byName[l.Name]
		if !relink || slices.Contains(moved, l.RoID) {
			continue
		}
		var target Host
		// Locked, so the gaining registrar cannot delete it before it is
		// linked.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ? AND cl_id = ?", l.Name, gaining).First(&target).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			h := *template
			h.ClID, h.CrRr, h.UpRr = t.RequestingClID, t.RequestingClID, ""
			h.CreatedAt, h.UpdatedAt = now, now
			target = *ToDBHost(&h)
			if err := tx.Create(&target).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		}
		if target.RoID == l.RoID {
			continue
		}
		if err := tx.Exec("DELETE FROM domain_hosts WHERE domain_ro_id = ? AND host_ro_id = ?", dbDomain.RoID, l.RoID).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (?, ?)", dbDomain.RoID, target.RoID).Error; err != nil {
			return err
		}
		affected = unionIDs(affected, []int64{target.RoID})
	}
	return recomputeHostLinks(tx, affected, now)
}

// queueMessages inserts msgs, about transfer id.
func queueMessages(tx *gorm.DB, msgs []*entities.PollMessage, transferID int64) error {
	for _, m := range msgs {
		if m.Transfer != nil {
			m.Transfer.ID = transferID
		}
		rec := toPollRecord(m)
		if err := tx.Create(rec).Error; err != nil {
			return err
		}
		m.ID = rec.ID
	}
	return nil
}

// notWritten turns a conditional write's errNotWritten into
// ErrDomainChanged, or ErrDomainNotFound if the domain is gone.
func (r *DomainTransferRepository) notWritten(ctx context.Context, roid int64, err error) error {
	return (&DomainRepository{db: r.db, repository: r.repository}).notWritten(ctx, roid, err)
}

// LatestTransfer implements repositories.DomainTransferRepository.
func (r *DomainTransferRepository) LatestTransfer(ctx context.Context, roid entities.RoidType) (*entities.DomainTransfer, error) {
	id, err := roid.Int64()
	if err != nil {
		return nil, err
	}
	var rec DomainTransferRecord
	err = r.db.WithContext(ctx).Where("domain_ro_id = ?", id).Order("id DESC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entities.ErrTransferNotPending
	}
	if err != nil {
		return nil, err
	}
	return r.fromTransferRecord(&rec), nil
}

// OldestPollMessage implements repositories.PollMessageRepository.
func (r *DomainTransferRepository) OldestPollMessage(ctx context.Context, clid entities.ClIDType) (*entities.PollMessage, int64, error) {
	q := r.db.WithContext(ctx).Model(&PollMessageRecord{}).Where("cl_id = ?", clid.String())
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return nil, 0, err
	}
	if n == 0 {
		return nil, 0, nil
	}
	var rec PollMessageRecord
	if err := r.db.WithContext(ctx).Where("cl_id = ?", clid.String()).Order("id").First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, nil // acknowledged since it was counted
		}
		return nil, 0, err
	}
	return r.fromPollRecord(&rec), n, nil
}

// AckPollMessage implements repositories.PollMessageRepository.
func (r *DomainTransferRepository) AckPollMessage(ctx context.Context, clid entities.ClIDType, id int64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND cl_id = ?", id, clid.String()).Delete(&PollMessageRecord{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return entities.ErrPollMessageNotFound
		}
		return tx.Model(&PollMessageRecord{}).Where("cl_id = ?", clid.String()).Count(&n).Error
	})
	return n, err
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func timeOf(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func toTransferRecord(t *entities.DomainTransfer) *DomainTransferRecord {
	roid, _ := t.DomainRoID.Int64()
	return &DomainTransferRecord{
		ID: t.ID, DomainRoID: roid, DomainName: t.DomainName.String(), Status: string(t.Status),
		RequestingClID: t.RequestingClID.String(), RequestedAt: t.RequestedAt, LosingClID: t.LosingClID.String(),
		ActingClID: t.ActingClID.String(), ActionDate: t.ActionDate, Years: t.Years, ExpiryDate: timePtr(t.ExpiryDate),
	}
}

func (r *DomainTransferRepository) fromTransferRecord(rec *DomainTransferRecord) *entities.DomainTransfer {
	roid, _ := entities.NewRoidType(rec.DomainRoID, entities.RoidTypeDomain, r.repository)
	return &entities.DomainTransfer{
		ID: rec.ID, DomainRoID: roid, DomainName: entities.DomainName(rec.DomainName), Status: entities.TransferStatus(rec.Status),
		RequestingClID: entities.ClIDType(rec.RequestingClID), RequestedAt: rec.RequestedAt, LosingClID: entities.ClIDType(rec.LosingClID),
		ActingClID: entities.ClIDType(rec.ActingClID), ActionDate: rec.ActionDate, Years: rec.Years, ExpiryDate: timeOf(rec.ExpiryDate),
	}
}

func toPollRecord(m *entities.PollMessage) *PollMessageRecord {
	rec := &PollMessageRecord{ClID: m.ClID.String(), CreatedAt: m.CreatedAt, Message: m.Message}
	if t := m.Transfer; t != nil {
		roid, _ := t.DomainRoID.Int64()
		id := t.ID
		rec.TransferID, rec.DomainRoID = &id, &roid
		rec.DomainName, rec.TrStatus = t.DomainName.String(), string(t.Status)
		rec.RequestingClID, rec.RequestedAt = t.RequestingClID.String(), timePtr(t.RequestedAt)
		rec.LosingClID, rec.ActingClID, rec.ActionDate = t.LosingClID.String(), t.ActingClID.String(), timePtr(t.ActionDate)
		rec.Years, rec.ExpiryDate = t.Years, timePtr(t.ExpiryDate)
	}
	return rec
}

func (r *DomainTransferRepository) fromPollRecord(rec *PollMessageRecord) *entities.PollMessage {
	m := &entities.PollMessage{ID: rec.ID, ClID: entities.ClIDType(rec.ClID), CreatedAt: rec.CreatedAt, Message: rec.Message}
	if rec.TransferID != nil {
		var roid entities.RoidType
		if rec.DomainRoID != nil {
			roid, _ = entities.NewRoidType(*rec.DomainRoID, entities.RoidTypeDomain, r.repository)
		}
		m.Transfer = &entities.DomainTransfer{
			ID: *rec.TransferID, DomainRoID: roid, DomainName: entities.DomainName(rec.DomainName), Status: entities.TransferStatus(rec.TrStatus),
			RequestingClID: entities.ClIDType(rec.RequestingClID), RequestedAt: timeOf(rec.RequestedAt), LosingClID: entities.ClIDType(rec.LosingClID),
			ActingClID: entities.ClIDType(rec.ActingClID), ActionDate: timeOf(rec.ActionDate), Years: rec.Years, ExpiryDate: timeOf(rec.ExpiryDate),
		}
	}
	return m
}
