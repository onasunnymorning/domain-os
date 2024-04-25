package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRDEHost_ToEntity(t *testing.T) {
	tests := []struct {
		name     string
		rdeHost  *RDEHost
		expected *Host
		err      error
	}{
		{
			name: "valid host",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: &Host{
				Name:      "example.com",
				RoID:      "12345",
				ClID:      "client1",
				CrRr:      "admin",
				UpRr:      "admin",
				CreatedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			err: nil,
		},
		{
			name: "invalid Roid",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_DOM-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrInvalidHostRoID,
		},
		{
			name: "invalid Name",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "invalid.domain---name.com",
				RoID:   "12345_DOM-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrInvalidLabelDoubleDash,
		},
		{
			name: "invalid Ip",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_DOM-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "1923.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrInvalidIP,
		},
		{
			name: "invalid CrRR",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "thissssisssstooooooooolooooooongthissssisssstooooooooo",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrInvalidClIDType,
		},
		{
			name: "invalid UpRR",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "thissssisssstooooooooolooooooongthissssisssstooooooooo",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "ok"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrInvalidClIDType,
		},
		{
			name: "invalid status combination",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "pendingTransfer"}, {S: "pendingCreate"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrHostStatusIncompatible,
		},
		{
			name: "invalid status ",
			// Create a sample RDEHost
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				Status: []RDEHostStatus{{S: "linked"}, {S: "invalid"}},
				Addr:   []RDEHostAddr{{IP: "192.168.0.1"}, {IP: "192.168.0.2"}},
			},
			expected: nil,
			err:      ErrUnknownHostStatus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.rdeHost.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.rdeHost.Name, actual.Name.String())
				require.Equal(t, tc.rdeHost.RoID, actual.RoID.String())
				require.Equal(t, tc.rdeHost.ClID, actual.ClID.String())
				require.Equal(t, tc.rdeHost.CrRr, actual.CrRr.String())
				require.Equal(t, tc.rdeHost.UpRr, actual.UpRr.String())
				require.Equal(t, len(tc.rdeHost.Addr), len(actual.Addresses))
				// Test status
				require.Equal(t, tc.expected.CreatedAt, actual.CreatedAt)
				require.Equal(t, tc.expected.UpdatedAt, actual.UpdatedAt)
			}
		})
	}

}

func TestRDEHost_ToCSV(t *testing.T) {
	tests := []struct {
		name     string
		rdeHost  *RDEHost
		expected []string
	}{
		{
			name: "valid host",
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrRr:   "admin",
				UpRr:   "admin",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
			},
			expected: []string{"example.com", "12345_HOST-APEX", "client1", "admin", "2022-01-01T00:00:00Z", "admin", "2022-01-02T00:00:00Z"},
		},
		{
			name: "empty fields",
			rdeHost: &RDEHost{
				Name:   "example.com",
				RoID:   "12345_HOST-APEX",
				ClID:   "client1",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
			},
			expected: []string{"example.com", "12345_HOST-APEX", "client1", "", "2022-01-01T00:00:00Z", "", "2022-01-02T00:00:00Z"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.rdeHost.ToCSV()
			require.Equal(t, tc.expected, actual)
			require.Equal(t, len(RDE_HOST_CSV_HEADER), len(actual))
		})
	}
}
