package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHostFail(t *testing.T) {
	name := "--example.com"
	roid := "12345"
	clid := "67890"

	host, err := NewHost(name, roid, clid)
	require.Equal(t, err, ErrInvalidLabelDash, "expected error")
	require.Nil(t, host, "expected nil")
}
func TestNewHost(t *testing.T) {
	name := "example.com"
	roid := "12345"
	clid := "67890"

	host, err := NewHost(name, roid, clid)
	require.NoError(t, err)
	require.NotNil(t, host)
	require.Equal(t, name, host.Name.String())
	require.Equal(t, RoidType(roid), host.RoID)
	require.Equal(t, ClIDType(clid), host.ClID)
	require.Equal(t, ClIDType(clid), host.CrRr)
	require.True(t, host.HostStatus.OK)
}

func TestAddHostAddress(t *testing.T) {
	testcases := []struct {
		name      string
		addresses []string
		err       error
	}{
		{
			name:      "valid addresses",
			addresses: []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			err:       nil,
		},
		{
			name:      "invalid address",
			addresses: []string{"..195"},
			err:       ErrInvalidIP,
		},
		{
			name:      "duplicate ipv4 address",
			addresses: []string{"195.238.2.21", "195.238.2.21"},
			err:       ErrDuplicateHostAddress,
		},
		{
			name:      "duplicate ipv6 address",
			addresses: []string{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", "2001:db8:85a3::8a2e:370:7334"},
			err:       ErrDuplicateHostAddress,
		},
		{
			name:      "max addresses exceeded",
			addresses: []string{"195.238.2.21", "2001:db8:85a3::8a2e:370:7334", "2001:db8:85a3::8a2e:370:7335", "195.238.2.24", "195.238.2.25", "195.238.2.26", "195.238.2.27", "195.238.2.28", "195.238.2.29", "195.238.2.30", "195.238.2.31"},
			err:       ErrMaxAddressesPerHostExceeded,
		},
		{
			name:      "10 addresses",
			addresses: []string{"195.238.2.21", "2001:db8:85a3::8a2e:370:7334", "2001:db8:85a3::8a2e:370:7335", "195.238.2.24", "195.238.2.25", "195.238.2.26", "195.238.2.27", "195.238.2.28", "195.238.2.29", "195.238.2.30"},
			err:       nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			var err error
			for _, addr := range tc.addresses {
				err = host.AddAddress(addr)
			}
			require.Equal(t, tc.err, err)
		})
	}
}

func TestRemoveHostAddress(t *testing.T) {
	testcases := []struct {
		name            string
		addresses       []string
		removeAddresses []string
		err             error
		lenght          int
	}{
		{
			name:            "valid addresses",
			addresses:       []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			removeAddresses: []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			err:             nil,
			lenght:          0,
		},
		{
			name:            "none to begin with",
			addresses:       []string{""},
			removeAddresses: []string{"195.238.2.21"},
			err:             ErrHostAddressNotFound,
			lenght:          0,
		},
		{
			name:            "one remaining address",
			addresses:       []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			removeAddresses: []string{"2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			err:             nil,
			lenght:          1,
		},
		{
			name:            "invalid address",
			addresses:       []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			removeAddresses: []string{"..195"},
			err:             ErrInvalidIP,
			lenght:          2,
		},
		{
			name:            "remove to many",
			addresses:       []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			removeAddresses: []string{"195.238.2.21", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "195.238.2.22"},
			err:             ErrHostAddressNotFound,
			lenght:          0,
		},
		{
			name:            "remove unknown address",
			addresses:       []string{"195.238.2.21"},
			removeAddresses: []string{"195.238.2.22"},
			err:             ErrHostAddressNotFound,
			lenght:          1,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			var err error
			for _, addr := range tc.addresses {
				err = host.AddAddress(addr)
			}
			for _, addr := range tc.removeAddresses {
				err = host.RemoveAddress(addr)
			}
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.lenght, len(host.Addresses))
		})
	}
}

func TestCanBeDeleted(t *testing.T) {
	testcases := []struct {
		name   string
		s      HostStatus
		result bool
	}{
		{
			name:   "ok",
			s:      HostStatus{OK: true},
			result: true,
		},
		{
			name:   "ClientDeleteProhibited",
			s:      HostStatus{ClientDeleteProhibited: true},
			result: false,
		},
		{
			name:   "ServerDeleteProhibited",
			s:      HostStatus{ServerDeleteProhibited: true},
			result: false,
		},
		{
			name:   "other status",
			s:      HostStatus{ClientUpdateProhibited: true},
			result: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.s
			require.Equal(t, tc.result, host.CanBeDeleted())
		})
	}
}

func TestCanBeUpdated(t *testing.T) {
	testcases := []struct {
		name   string
		s      HostStatus
		result bool
	}{
		{
			name:   "ok",
			s:      HostStatus{OK: true},
			result: true,
		},
		{
			name:   "ClientUpdateProhibited",
			s:      HostStatus{ClientUpdateProhibited: true},
			result: false,
		},
		{
			name:   "ServerUpdateProhibited",
			s:      HostStatus{ServerUpdateProhibited: true},
			result: false,
		},
		{
			name:   "other status",
			s:      HostStatus{ClientDeleteProhibited: true},
			result: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.s
			require.Equal(t, tc.result, host.CanBeUpdated())
		})
	}
}

func TestValidateHostStatus(t *testing.T) {
	testcases := []struct {
		name string
		s    HostStatus
		err  error
	}{
		{
			name: "Pending combo1",
			s: HostStatus{
				PendingCreate: true,
				PendingDelete: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "Pending combo2",
			s: HostStatus{
				PendingUpdate:   true,
				PendingTransfer: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "Pending combo3",
			s: HostStatus{
				PendingUpdate: true,
				PendingCreate: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "Pending Update with UpdateProhibited",
			s: HostStatus{
				PendingUpdate:          true,
				ClientUpdateProhibited: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "Pending Delete with DeleteProhibited",
			s: HostStatus{
				PendingDelete:          true,
				ClientDeleteProhibited: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "Ok with a prohibited",
			s: HostStatus{
				OK:                     true,
				ClientDeleteProhibited: true,
			},
			err: ErrHostStatusIncompatible,
		},
		{
			name: "OK not set when supposed to",
			s:    HostStatus{},
			err:  ErrOKStatusMustBeSet,
		},
		{
			name: "Happy path",
			s: HostStatus{
				OK: true,
			},
			err: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.s
			err := host.ValidateStatus()
			require.Equal(t, tc.err, err)
		})
	}

}

func TestUnsetOKIfNeeded(t *testing.T) {
	testcases := []struct {
		name string
		hs   HostStatus
		ok   bool
	}{
		{
			name: "unset OK",
			hs: HostStatus{
				PendingCreate: true,
			},
			ok: false,
		},
		{
			name: "OK stays set",
			hs: HostStatus{
				OK: true,
			},
			ok: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.hs
			host.UnsetOKIfNeeded()
			require.Equal(t, tc.ok, host.HostStatus.OK)
		})
	}
}

func TestSetHostStatus(t *testing.T) {
	testcases := []struct {
		name string
		hs   HostStatus
		s    string
		err  error
		ok   bool
	}{
		{
			name: " set OK",
			hs:   HostStatus{},
			s:    HostStatusOK,
			err:  nil,
			ok:   true,
		},
		{
			name: " set Linked with OK",
			hs:   HostStatus{OK: true},
			s:    HostStatusLinked,
			err:  nil,
			ok:   true,
		},
		{
			name: " set PendingCreate",
			hs:   HostStatus{OK: true},
			s:    HostStatusPendingCreate,
			err:  nil,
			ok:   false,
		},
		{
			name: " set PendingDelete",
			hs:   HostStatus{OK: true},
			s:    HostStatusPendingDelete,
			err:  nil,
			ok:   false,
		},
		{
			name: " set PendingUpdate",
			hs:   HostStatus{OK: true},
			s:    HostStatusPendingUpdate,
			err:  nil,
			ok:   false,
		},
		{
			name: " set PendingTransfer",
			hs:   HostStatus{OK: true},
			s:    HostStatusPendingTransfer,
			err:  nil,
			ok:   false,
		},
		{
			name: " set ClientDeleteProhibited",
			hs:   HostStatus{OK: true},
			s:    HostStatusClientDeleteProhibited,
			err:  nil,
			ok:   false,
		},
		{
			name: " set ClientUpdateProhibited",
			hs:   HostStatus{OK: true},
			s:    HostStatusClientUpdateProhibited,
			err:  nil,
			ok:   false,
		},
		{
			name: " set ServerDeleteProhibited",
			hs:   HostStatus{OK: true},
			s:    HostStatusServerDeleteProhibited,
			err:  nil,
			ok:   false,
		},
		{
			name: " set ServerUpdateProhibited",
			hs:   HostStatus{OK: true},
			s:    HostStatusServerUpdateProhibited,
			err:  nil,
			ok:   false,
		},
		{
			name: " set invalid status combination",
			hs:   HostStatus{PendingDelete: true},
			s:    HostStatusClientDeleteProhibited,
			err:  ErrHostStatusIncompatible,
			ok:   false,
		},
		{
			name: " set invalid status",
			hs:   HostStatus{OK: true},
			s:    "invalid",
			err:  ErrUnknownHostStatus,
			ok:   true,
		},
		{
			name: " set Prohibition when update is prohinited",
			hs:   HostStatus{ClientUpdateProhibited: true},
			s:    HostStatusClientUpdateProhibited,
			err:  nil,
			ok:   false,
		},
		{
			name: " set somehting when update is prohinited",
			hs:   HostStatus{ClientUpdateProhibited: true},
			s:    HostStatusPendingTransfer,
			err:  ErrHostUpdateProhibited,
			ok:   false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.hs
			err := host.SetStatus(tc.s)
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.ok, host.HostStatus.OK)
		})
	}
}

func TestUnsetHostStatus(t *testing.T) {
	testcases := []struct {
		name string
		hs   HostStatus
		s    string
		err  error
	}{
		{
			name: " unset OK",
			hs:   HostStatus{OK: true},
			s:    HostStatusOK,
			err:  nil,
		},
		{
			name: " unset Linked with OK",
			hs:   HostStatus{OK: true, Linked: true},
			s:    HostStatusLinked,
			err:  nil,
		},
		{
			name: " unset PendingCreate",
			hs:   HostStatus{OK: true, PendingCreate: true},
			s:    HostStatusPendingCreate,
			err:  nil,
		},
		{
			name: " unset PendingDelete",
			hs:   HostStatus{OK: true, PendingDelete: true},
			s:    HostStatusPendingDelete,
			err:  nil,
		},
		{
			name: " unset PendingUpdate",
			hs:   HostStatus{OK: true, PendingUpdate: true},
			s:    HostStatusPendingUpdate,
			err:  nil,
		},
		{
			name: " unset PendingTransfer",
			hs:   HostStatus{OK: true, PendingTransfer: true},
			s:    HostStatusPendingTransfer,
			err:  nil,
		},
		{
			name: " unset ClientDeleteProhibited",
			hs:   HostStatus{OK: true, ClientDeleteProhibited: true},
			s:    HostStatusClientDeleteProhibited,
			err:  nil,
		},
		{
			name: " unset ClientUpdateProhibited",
			hs:   HostStatus{OK: true, ClientUpdateProhibited: true},
			s:    HostStatusClientUpdateProhibited,
			err:  nil,
		},
		{
			name: " unset ServerDeleteProhibited",
			hs:   HostStatus{OK: true, ServerDeleteProhibited: true},
			s:    HostStatusServerDeleteProhibited,
			err:  nil,
		},
		{
			name: " unset ServerUpdateProhibited",
			hs:   HostStatus{OK: true, ServerUpdateProhibited: true},
			s:    HostStatusServerUpdateProhibited,
			err:  nil,
		},
		{
			name: " unset invalid status combination",
			hs:   HostStatus{PendingDelete: true, ClientDeleteProhibited: true},
			s:    HostStatusServerDeleteProhibited,
			err:  ErrHostStatusIncompatible,
		},
		{
			name: " unset invalid status",
			hs:   HostStatus{OK: true},
			s:    "invalid",
			err:  ErrUnknownHostStatus,
		},
		{
			name: " unset Prohibition when set",
			hs:   HostStatus{ClientUpdateProhibited: true},
			s:    HostStatusClientUpdateProhibited,
			err:  nil,
		},
		{
			name: " unset somehting when update is prohinited",
			hs:   HostStatus{ClientUpdateProhibited: true, PendingTransfer: true},
			s:    HostStatusPendingTransfer,
			err:  ErrHostUpdateProhibited,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := NewHost("example.com", "12345", "67890")
			host.HostStatus = tc.hs
			err := host.UnsetStatus(tc.s)
			require.Equal(t, tc.err, err)
		})
	}
}
