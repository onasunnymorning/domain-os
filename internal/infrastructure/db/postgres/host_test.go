package postgres

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestHost_Tablename(t *testing.T) {
	host := Host{}
	require.Equal(t, "hosts", host.TableName())
}

func getValidHost(clid string, t *time.Time) *entities.Host {
	h := &entities.Host{
		RoID:        entities.RoidType(fmt.Sprintf("%d_HOST-APEX", gofakeit.Uint32())),
		Name:        entities.DomainName(gofakeit.DomainName()),
		ClID:        entities.ClIDType(clid),
		CrRr:        entities.ClIDType(clid),
		UpRr:        entities.ClIDType(clid),
		InBailiwick: true,
		Status: entities.HostStatus{
			ServerDeleteProhibited: true,
		},
	}
	if t != nil {
		h.CreatedAt = t.Round(time.Millisecond)
		h.UpdatedAt = t.Round(time.Millisecond)
	}
	return h
}

func TestHost_ToDBHost(t *testing.T) {
	ti := time.Now().UTC()
	host := getValidHost("myrarID", &ti)

	a, _ := netip.ParseAddr("195.238.2.21")
	host.Addresses = append(host.Addresses, a)
	a, _ = netip.ParseAddr("195.238.2.22")
	host.Addresses = append(host.Addresses, a)
	a, _ = netip.ParseAddr("2001:db8:85a3::8a2e:370:7334")
	host.Addresses = append(host.Addresses, a)

	dbHost := ToDBHost(host)

	roid, _ := host.RoID.Int64()
	require.Equal(t, roid, dbHost.RoID)
	require.Equal(t, host.Name.String(), dbHost.Name)
	require.Equal(t, host.ClID.String(), dbHost.ClID)
	require.Equal(t, host.CrRr, entities.ClIDType(*dbHost.CrRr))
	require.Equal(t, host.UpRr, entities.ClIDType(*dbHost.UpRr))
	require.True(t, dbHost.InBailiwick)
	require.True(t, dbHost.ServerDeleteProhibited)
	require.Equal(t, host.CreatedAt, dbHost.CreatedAt)
	require.Equal(t, host.UpdatedAt, dbHost.UpdatedAt)
	require.Len(t, dbHost.Addresses, len(host.Addresses))
}

func TestHost_ToHost(t *testing.T) {
	ti := time.Now().UTC()
	host := getValidHost("myrarOtherID", &ti)

	a, _ := netip.ParseAddr("195.238.2.21")
	host.Addresses = append(host.Addresses, a)
	a, _ = netip.ParseAddr("195.238.2.22")
	host.Addresses = append(host.Addresses, a)
	a, _ = netip.ParseAddr("2001:db8:85a3::8a2e:370:7334")
	host.Addresses = append(host.Addresses, a)

	dbHost := ToDBHost(host)
	convertedHost := ToHost(dbHost)

	require.Equal(t, host.RoID, convertedHost.RoID)
	require.Equal(t, host.Name, convertedHost.Name)
	require.Equal(t, host.ClID, convertedHost.ClID)
	require.Equal(t, host.CrRr, convertedHost.CrRr)
	require.Equal(t, host.UpRr, convertedHost.UpRr)
	require.Equal(t, host.InBailiwick, convertedHost.InBailiwick)
	require.Equal(t, host.Status, convertedHost.Status)
	require.Equal(t, host.CreatedAt, convertedHost.CreatedAt)
	require.Equal(t, host.UpdatedAt, convertedHost.UpdatedAt)
	require.Len(t, convertedHost.Addresses, len(host.Addresses))
}
