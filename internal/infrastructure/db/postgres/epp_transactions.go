package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPTransactionRecord is a row of the EPP audit log (docs/epp/ARCHITECTURE.md
// §6). Rows are only ever inserted, and deleted once past retention.
type EPPTransactionRecord struct {
	SvTRID     string    `gorm:"column:sv_trid;primaryKey"`
	ClTRID     string    `gorm:"column:cl_trid;index:idx_epp_transactions_cltrid,priority:2"`
	SessionID  string    `gorm:"not null;index"`
	Node       string    `gorm:"not null"`
	ClID       string    `gorm:"column:clid;index:idx_epp_transactions_clid_received,priority:1;index:idx_epp_transactions_cltrid,priority:1"`
	LoginID    string    `gorm:"column:login_id"`
	ClientIP   string    `gorm:"not null"`
	Command    string    `gorm:"not null"`
	Objects    string    `gorm:"not null;default:'[]'"` // JSON array
	ResultCode int       `gorm:"not null"`
	ReceivedAt time.Time `gorm:"not null;index;index:idx_epp_transactions_clid_received,priority:2"`
	DurationUS int64     `gorm:"not null"`
	Request    string
	Response   string
}

// TableName pins the table name.
func (EPPTransactionRecord) TableName() string { return "epp_transactions" }

// EPPTransactionRepository is the Postgres repositories.EPPTransactionRepository.
type EPPTransactionRepository struct{ db *gorm.DB }

// NewEPPTransactionRepository returns a repository over db.
func NewEPPTransactionRepository(db *gorm.DB) *EPPTransactionRepository {
	return &EPPTransactionRepository{db: db}
}

// Save inserts the transactions, skipping any whose svTRID is already there.
func (r *EPPTransactionRepository) Save(ctx context.Context, txs ...*entities.EPPTransaction) error {
	if len(txs) == 0 {
		return nil
	}
	rows := make([]EPPTransactionRecord, 0, len(txs))
	for _, tx := range txs {
		objects, err := json.Marshal(tx.Objects)
		if err != nil {
			return fmt.Errorf("encode EPP transaction objects: %w", err)
		}
		if tx.Objects == nil {
			objects = []byte("[]")
		}
		ip := ""
		if tx.ClientIP.IsValid() {
			ip = tx.ClientIP.String()
		}
		rows = append(rows, EPPTransactionRecord{
			SvTRID: tx.SvTRID, ClTRID: tx.ClTRID, SessionID: tx.SessionID, Node: tx.Node,
			ClID: string(tx.ClID), LoginID: string(tx.LoginID), ClientIP: ip,
			Command: tx.Command, Objects: string(objects), ResultCode: tx.ResultCode,
			ReceivedAt: tx.ReceivedAt.UTC(), DurationUS: tx.Duration.Microseconds(),
			Request: tx.Request, Response: tx.Response,
		})
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "sv_trid"}}, DoNothing: true}).
		CreateInBatches(rows, 200).Error
	if err != nil {
		return fmt.Errorf("save EPP transactions: %w", err)
	}
	return nil
}

// DeleteReceivedBefore deletes the oldest transactions received before
// cutoff, at most limit of them.
func (r *EPPTransactionRepository) DeleteReceivedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res := r.db.WithContext(ctx).Exec(
		`DELETE FROM epp_transactions WHERE sv_trid IN (
			SELECT sv_trid FROM epp_transactions WHERE received_at < ? ORDER BY received_at LIMIT ?)`,
		cutoff.UTC(), limit)
	if res.Error != nil {
		return 0, fmt.Errorf("prune EPP transactions: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// ToEntity converts the row back, for tests and future readers.
func (rec *EPPTransactionRecord) ToEntity() (*entities.EPPTransaction, error) {
	tx := &entities.EPPTransaction{
		SvTRID: rec.SvTRID, ClTRID: rec.ClTRID, SessionID: rec.SessionID, Node: rec.Node,
		ClID: entities.ClIDType(rec.ClID), LoginID: entities.ClIDType(rec.LoginID),
		Command: rec.Command, ResultCode: rec.ResultCode, ReceivedAt: rec.ReceivedAt,
		Duration: time.Duration(rec.DurationUS) * time.Microsecond,
		Request:  rec.Request, Response: rec.Response,
	}
	if err := json.Unmarshal([]byte(rec.Objects), &tx.Objects); err != nil {
		return nil, fmt.Errorf("decode EPP transaction objects: %w", err)
	}
	if rec.ClientIP != "" {
		ip, err := netip.ParseAddr(rec.ClientIP)
		if err != nil {
			return nil, fmt.Errorf("decode EPP transaction client IP: %w", err)
		}
		tx.ClientIP = ip
	}
	return tx, nil
}
