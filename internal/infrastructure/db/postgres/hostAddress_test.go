package postgres

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostAddress_Tablename(t *testing.T) {
	hostAddress := HostAddress{}
	require.Equal(t, "host_addresses", hostAddress.TableName())
}

func TestToHostAddress(t *testing.T) {
	dbHostAddress := &HostAddress{
		Address: "192.168.0.1",
	}
	expectedAddr, _ := netip.ParseAddr("192.168.0.1")

	actualAddr := ToHostAddress(dbHostAddress)

	require.Equal(t, expectedAddr, actualAddr)
}

func TestToDBHostAddress(t *testing.T) {
	addr, _ := netip.ParseAddr("192.168.0.1")
	hostRoID := int64(1)

	expectedHostAddress := &HostAddress{
		Address:  "192.168.0.1",
		Version:  4,
		HostRoID: hostRoID,
	}

	actualHostAddress := ToDBHostAddress(addr, hostRoID)

	require.Equal(t, expectedHostAddress, actualHostAddress)

}

func TestToDBHostAddressV6(t *testing.T) {
	addr, _ := netip.ParseAddr("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	hostRoID := int64(1)

	expectedHostAddress := &HostAddress{
		Address:  "2001:db8:85a3::8a2e:370:7334",
		Version:  6,
		HostRoID: hostRoID,
	}

	wrongExpectedHostAddress := &HostAddress{
		Address:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		Version:  6,
		HostRoID: hostRoID,
	}

	actualHostAddress := ToDBHostAddress(addr, hostRoID)

	require.NotEqual(t, wrongExpectedHostAddress, actualHostAddress)
	require.Equal(t, expectedHostAddress, actualHostAddress)
}
