package postgres

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestEPPTransactionRepository(t *testing.T) {
	db := getTestDB()
	repo := NewEPPTransactionRepository(db)
	ctx := context.Background()
	// A session ID of our own keeps this test's rows apart from any other's.
	session := uuid.NewString()
	t.Cleanup(func() { db.Where("session_id = ?", session).Delete(&EPPTransactionRecord{}) })

	old := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Microsecond)
	tx := func(id string, at time.Time) *entities.EPPTransaction {
		return &entities.EPPTransaction{
			SvTRID: session + "-" + id, ClTRID: "CL-" + id, SessionID: session, Node: "node-a",
			ClID: "registrar1", LoginID: "registrar1", ClientIP: netip.MustParseAddr("192.0.2.7"),
			Command: "domain:check", Objects: []string{"a.test", "b.test"}, ResultCode: 1000,
			ReceivedAt: at, Duration: 1500 * time.Microsecond,
			Request: "<epp/>", Response: "<epp><response/></epp>",
		}
	}
	first := tx("1", old)
	require.NoError(t, repo.Save(ctx, first, tx("2", old.Add(time.Second)), tx("3", time.Now())))
	require.NoError(t, repo.Save(ctx, first), "saving a record again is not an error: a retried batch may repeat one")

	var rec EPPTransactionRecord
	require.NoError(t, db.Where("sv_trid = ?", first.SvTRID).First(&rec).Error)
	got, err := rec.ToEntity()
	require.NoError(t, err)
	got.ReceivedAt = got.ReceivedAt.UTC()
	assert.Equal(t, first, got)

	var objectless EPPTransactionRecord
	none := tx("4", time.Now())
	none.Objects, none.ClientIP = nil, netip.Addr{}
	require.NoError(t, repo.Save(ctx, none))
	require.NoError(t, db.Where("sv_trid = ?", none.SvTRID).First(&objectless).Error)
	assert.Equal(t, "[]", objectless.Objects)

	// Prune in batches of one: the oldest goes first.
	n, err := repo.DeleteReceivedBefore(ctx, time.Now().Add(-24*time.Hour), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	var left []EPPTransactionRecord
	require.NoError(t, db.Where("session_id = ?", session).Order("received_at").Find(&left).Error)
	require.Len(t, left, 3)
	assert.Equal(t, session+"-2", left[0].SvTRID)
}
