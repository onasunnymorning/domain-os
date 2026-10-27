package memstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestListHostsByNameBreaksTiesByROID(t *testing.T) {
	r := NewRegistry("TEST")
	at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return at } // every host created at the same instant
	hosts := r.Hosts()
	ctx := context.Background()
	var batch []*entities.Host
	for _, h := range []struct{ roid, clid string }{{"30_HOST-TEST", "rar-c"}, {"10_HOST-TEST", "rar-a"}, {"20_HOST-TEST", "rar-b"}} {
		batch = append(batch, &entities.Host{RoID: entities.RoidType(h.roid), Name: "ns1.example.tld", ClID: entities.ClIDType(h.clid)})
	}
	require.NoError(t, hosts.BulkCreate(ctx, batch))

	for i := 0; i < 50; i++ {
		got, err := hosts.ListHostsByName(ctx, "ns1.example.tld")
		require.NoError(t, err)
		require.Len(t, got, 3)
		require.Equal(t, []entities.RoidType{"10_HOST-TEST", "20_HOST-TEST", "30_HOST-TEST"},
			[]entities.RoidType{got[0].RoID, got[1].RoID, got[2].RoID}, "the same order on every read")
	}
}
