package postgres

import (
	"net/netip"
)

func getValidHostAddresses() *HostAddress {
	return &HostAddress{
		Version: 4,
		IP:      "195.238.2.21",
	}
}

func getValidNetIPAddr() *netip.Addr {
	ip, _ := netip.ParseAddr("195.238.2.21")
	return &ip
}
