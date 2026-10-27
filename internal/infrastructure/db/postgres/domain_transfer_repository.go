package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"

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
	err := dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
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
	err := dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
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
			if err := moveHostsWithDomain(tx, dbDomain, t, d.Hosts, res.GainingHosts); err != nil {
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
// entities.TransferResolution describes. snapshot is the domain's
// nameservers as the plan (gainingHosts) was made from them.
//
// The domain's row is locked already, so its links, and the hosts under it
// (host writes that place a host under a domain lock its row first), cannot
// change. Every host touched is then locked in one statement, in ROID
// order, as the other domain and host writes lock them, so two approvals,
// or an approval and an update, cannot deadlock. A nameserver renamed or
// replaced since the plan was made, or a gaining registrar's host created
// or deleted meanwhile, is errNotWritten: the transfer is read and planned
// again.
func moveHostsWithDomain(tx *gorm.DB, dbDomain *Domain, t *entities.DomainTransfer, snapshot, gainingHosts []*entities.Host) error {
	losing, gaining := t.LosingClID.String(), t.RequestingClID.String()
	now := dbDomain.UpdatedAt

	type hostRef struct {
		RoID int64
		Name string
		ClID string
	}
	var links []hostRef
	if err := tx.Raw(`SELECT h.ro_id, h.name, h.cl_id FROM domain_hosts dh JOIN hosts h ON h.ro_id = dh.host_ro_id
		WHERE dh.domain_ro_id = ?`, dbDomain.RoID).Scan(&links).Error; err != nil {
		return err
	}
	// The links must be the nameservers the plan was made from.
	planned := map[int64]string{}
	for _, h := range snapshot {
		id, err := h.RoID.Int64()
		if err != nil {
			return err
		}
		planned[id] = h.Name.String()
	}
	if len(links) != len(planned) {
		return errNotWritten
	}
	for _, l := range links {
		if name, ok := planned[l.RoID]; !ok || name != l.Name {
			return errNotWritten
		}
	}

	// The losing registrar's hosts under the domain (RFC 5732 §1.1: the
	// domain itself, or any name below it), but for a name the gaining
	// registrar already has.
	var subordinate []int64
	if err := tx.Raw(`SELECT ro_id FROM hosts h WHERE h.cl_id = ? AND (h.name = ? OR `+hostUnderDomainSQL+`)
		AND NOT EXISTS (SELECT 1 FROM hosts g WHERE g.cl_id = ? AND g.name = h.name)`,
		losing, dbDomain.Name, dbDomain.Name, gaining).Scan(&subordinate).Error; err != nil {
		return err
	}
	// The gaining registrar's hosts already named as re-linked nameservers.
	byName := map[string]*entities.Host{}
	names := make([]string, 0, len(gainingHosts))
	for _, h := range gainingHosts {
		byName[h.Name.String()] = h
		names = append(names, h.Name.String())
	}
	var targets []hostRef
	if len(names) > 0 {
		if err := tx.Raw("SELECT ro_id, name, cl_id FROM hosts WHERE cl_id = ? AND name IN ?", gaining, names).Scan(&targets).Error; err != nil {
			return err
		}
	}

	// Lock them all, in ROID order.
	all := unionIDs(nil, subordinate)
	for _, l := range links {
		all = unionIDs(all, []int64{l.RoID})
	}
	for _, g := range targets {
		all = unionIDs(all, []int64{g.RoID})
	}
	if len(all) > 0 {
		var locked []int64
		if err := tx.Raw("SELECT ro_id FROM hosts WHERE ro_id IN ? ORDER BY ro_id FOR UPDATE", all).Scan(&locked).Error; err != nil {
			return err
		}
		if len(locked) != len(all) {
			return errNotWritten // one was deleted since it was read
		}
	}

	if len(subordinate) > 0 {
		if err := tx.Exec("UPDATE hosts SET cl_id = ?, up_rr = ?, updated_at = ? WHERE ro_id IN ?", gaining, gaining, now, subordinate).Error; err != nil {
			return err
		}
	}
	gainingByName := map[string]int64{}
	for _, g := range targets {
		gainingByName[g.Name] = g.RoID
	}
	affected := all
	for _, l := range links {
		template, relink := byName[l.Name]
		if !relink || slices.Contains(subordinate, l.RoID) {
			continue
		}
		target, ok := gainingByName[l.Name]
		if !ok {
			h := *template
			h.ClID, h.CrRr, h.UpRr = t.RequestingClID, t.RequestingClID, ""
			h.CreatedAt, h.UpdatedAt = now, now
			row := ToDBHost(&h)
			if err := tx.Create(row).Error; err != nil {
				if isUniqueViolation(err) {
					return errNotWritten // the gaining registrar created it meanwhile
				}
				return err
			}
			target = row.RoID
			gainingByName[l.Name] = target
		}
		if err := tx.Exec("DELETE FROM domain_hosts WHERE domain_ro_id = ? AND host_ro_id = ?", dbDomain.RoID, l.RoID).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (?, ?)", dbDomain.RoID, target).Error; err != nil {
			return err
		}
		affected = unionIDs(affected, []int64{target})
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
	err = dbFrom(ctx, r.db).Where("domain_ro_id = ?", id).Order("id DESC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entities.ErrTransferNotPending
	}
	if err != nil {
		return nil, err
	}
	return r.fromTransferRecord(&rec), nil
}

// ListDueTransfers implements repositories.DomainTransferRepository. It
// reads idx_domain_transfers_pending (status, action_date). A transfer of a
// domain since deleted stays pending in the history and is left out.
func (r *DomainTransferRepository) ListDueTransfers(ctx context.Context, dueBy, afterDate time.Time, afterID int64, limit int) ([]*entities.DomainTransfer, error) {
	var recs []DomainTransferRecord
	if err := dbFrom(ctx, r.db).
		Where("status = ? AND action_date <= ?", string(entities.TransferStatusPending), dueBy).
		Where("(action_date, id) > (?, ?)", afterDate, afterID).
		Where("EXISTS (SELECT 1 FROM domains d WHERE d.ro_id = domain_transfers.domain_ro_id)").
		Order("action_date, id").Limit(limit).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.DomainTransfer, len(recs))
	for i := range recs {
		out[i] = r.fromTransferRecord(&recs[i])
	}
	return out, nil
}

// OldestPollMessage implements repositories.PollMessageRepository.
func (r *DomainTransferRepository) OldestPollMessage(ctx context.Context, clid entities.ClIDType) (*entities.PollMessage, int64, error) {
	q := dbFrom(ctx, r.db).Model(&PollMessageRecord{}).Where("cl_id = ?", clid.String())
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return nil, 0, err
	}
	if n == 0 {
		return nil, 0, nil
	}
	var rec PollMessageRecord
	if err := dbFrom(ctx, r.db).Where("cl_id = ?", clid.String()).Order("id").First(&rec).Error; err != nil {
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
	err := dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
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
