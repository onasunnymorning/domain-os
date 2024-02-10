package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestHost_Tablename(t *testing.T) {
	host := Host{}
	require.Equal(t, "hosts", host.TableName())
}

func getValidHost() *entities.Host {
	return &entities.Host{
		RoID:        entities.RoidType("12345_HOST-APEX"),
		Name:        "my-host.com",
		ClID:        entities.ClIDType("my-registrar-id"),
		CrRr:        entities.ClIDType("my-registrar-id"),
		UpRr:        entities.ClIDType("my-registrar-id"),
		InBailiwick: true,
		HostStatus: entities.HostStatus{
			ServerDeleteProhibited: true,
		},
	}
}

func TestHost_ToDBHost(t *testing.T) {
	host := getValidHost()
	dbHost := ToDBHost(*host)

	require.Equal(t, int64(12345), dbHost.RoID)
	require.Equal(t, "my-host.com", dbHost.Name)
	require.Equal(t, "my-registrar-id", dbHost.ClID)
	require.Equal(t, "my-registrar-id", dbHost.CrRr)
	require.Equal(t, "my-registrar-id", dbHost.UpRr)
	require.True(t, dbHost.InBailiwick)
	require.True(t, dbHost.ServerDeleteProhibited)
}

func TestHost_FromDBHost(t *testing.T) {
	host := getValidHost()
	dbHost := ToDBHost(*host)

	dbHost.Addresses = []HostAddress{
		{
			ID:       1,
			Version:  4,
			IP:       "195.238.2.21",
			HostRoID: dbHost.RoID,
		},
		{
			ID:       2,
			Version:  6,
			IP:       "2001:db8:85a3::8a2e:370:7334",
			HostRoID: dbHost.RoID,
		},
	}

	host = ToHost(dbHost)

	require.Equal(t, "12345_HOST-APEX", host.RoID.String())
	require.Equal(t, "my-host.com", host.Name.String())
	require.Equal(t, "my-registrar-id", host.ClID.String())
	require.Equal(t, "my-registrar-id", host.CrRr.String())
	require.Equal(t, "my-registrar-id", host.UpRr.String())
	require.True(t, host.InBailiwick)
	require.True(t, host.ServerDeleteProhibited)
	require.Len(t, host.Addresses, 2)
	require.Equal(t, host.Addresses[0].String(), "195.238.2.21")
	require.Equal(t, host.Addresses[1].String(), "2001:db8:85a3::8a2e:370:7334")
	require.True(t, host.Addresses[0].Is4())
	require.True(t, host.Addresses[1].Is6())

}
