package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainStatus_Validate(t *testing.T) {
	testcases := []struct {
		Name    string
		ds      DomainStatus
		wantErr error
	}{
		{
			Name:    "nil",
			ds:      DomainStatus{},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "pending + ok",
			ds: DomainStatus{
				OK:            true,
				PendingCreate: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "prohibition + ok",
			ds: DomainStatus{
				OK:         true,
				ServerHold: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "inactive and ok",
			ds: DomainStatus{
				Inactive: true,
				OK:       true,
			},
			wantErr: nil,
		},
		{
			Name:    "ok missing",
			ds:      DomainStatus{},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "pendingDelete with delete prohibition",
			ds: DomainStatus{
				PendingDelete:          true,
				ClientDeleteProhibited: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "pendingUpdate with update prohibition",
			ds: DomainStatus{
				PendingUpdate:          true,
				ServerUpdateProhibited: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "pendingRenew with renew prohibition",
			ds: DomainStatus{
				PendingRenew:          true,
				ClientRenewProhibited: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "pendingTransfer with transfer prohibition",
			ds: DomainStatus{
				PendingTransfer:          true,
				ServerTransferProhibited: true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			Name: "more than one pending",
			ds: DomainStatus{
				PendingTransfer: true,
				PendingRenew:    true,
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
	}

	for _, tc := range testcases {
		require.Equal(t, tc.wantErr, tc.ds.Validate())
	}

}

func TestDomainsStatus_IsNil(t *testing.T) {
	d := Domain{}
	require.True(t, d.Status.IsNil())
}

func TestDomainStatus_HasProhibitions(t *testing.T) {
	testcases := []struct {
		name string
		ds   DomainStatus
		want bool
	}{
		{
			name: "no prohibitions",
			ds:   DomainStatus{},
			want: false,
		},
		{
			name: "CliendDeleteProhibited",
			ds: DomainStatus{
				ClientDeleteProhibited: true,
			},
			want: true,
		},
		{
			name: "ServerUpdateProhibited",
			ds: DomainStatus{
				ServerUpdateProhibited: true,
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		d := Domain{
			Status: tc.ds,
		}
		require.Equal(t, tc.want, d.Status.HasProhibitions())
	}
}

func TestDomainStatus_HasPendings(t *testing.T) {
	testcases := []struct {
		name string
		ds   DomainStatus
		want bool
	}{
		{
			name: "no pendings",
			ds:   DomainStatus{},
			want: false,
		},
		{
			name: "PendingDelete",
			ds: DomainStatus{
				PendingDelete: true,
			},
			want: true,
		},
		{
			name: "PendingTransfer",
			ds: DomainStatus{
				PendingTransfer: true,
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		d := Domain{
			Status: tc.ds,
		}
		require.Equal(t, tc.want, d.Status.HasPendings())
	}
}

func TestDomainStatus_SetOK(t *testing.T) {
	testcases := []struct {
		Name   string
		ds     DomainStatus
		wantOK bool
	}{
		{
			Name:   "empty",
			ds:     DomainStatus{},
			wantOK: true,
		},
		{
			Name: "inactive",
			ds: DomainStatus{
				Inactive: true,
			},
			wantOK: true,
		},
		{
			Name: "PendingDelete",
			ds: DomainStatus{
				PendingDelete: true,
			},
			wantOK: false,
		},
		{
			Name: "PendingTransfer",
			ds: DomainStatus{
				PendingTransfer: true,
			},
			wantOK: false,
		},
		{
			Name: "ClientHold",
			ds: DomainStatus{
				ClientHold: true,
			},
			wantOK: false,
		},
		{
			Name: "ServerHold",
			ds: DomainStatus{
				ServerHold: true,
			},
			wantOK: false,
		},
	}

	for _, tc := range testcases {
		d := Domain{
			Status: tc.ds,
		}
		d.SetOKStatusIfNeeded()
		require.Equal(t, tc.wantOK, d.Status.OK)
	}
}

func TestDomainStatus_UnSetOK(t *testing.T) {
	testcases := []struct {
		Name   string
		ds     DomainStatus
		wantOK bool
	}{
		{
			Name: "empty",
			ds: DomainStatus{
				OK: true,
			},
			wantOK: true,
		},
		{
			Name: "inactive",
			ds: DomainStatus{
				OK:       true,
				Inactive: true,
			},
			wantOK: true,
		},
		{
			Name: "PendingDelete",
			ds: DomainStatus{
				OK:            true,
				PendingDelete: true,
			},
			wantOK: false,
		},
		{
			Name: "PendingTransfer",
			ds: DomainStatus{
				OK:              true,
				PendingTransfer: true,
			},
			wantOK: false,
		},
		{
			Name: "ClientHold",
			ds: DomainStatus{
				OK:         true,
				ClientHold: true,
			},
			wantOK: false,
		},
		{
			Name: "ServerHold",
			ds: DomainStatus{
				OK:         true,
				ServerHold: true,
			},
			wantOK: false,
		},
	}

	for _, tc := range testcases {
		d := Domain{
			Status: tc.ds,
		}
		d.UnSetOKStatusIfNeeded()
		require.Equal(t, tc.wantOK, d.Status.OK)
	}
}

func TestDomainStatus_NewDomainStatus(t *testing.T) {
	ds := NewDomainStatus()
	require.False(t, ds.OK)
	require.True(t, ds.Inactive)
}
func TestDomainStatus_HasHold(t *testing.T) {
	testcases := []struct {
		name string
		ds   DomainStatus
		want bool
	}{
		{
			name: "no hold",
			ds:   DomainStatus{},
			want: false,
		},
		{
			name: "ClientHold",
			ds: DomainStatus{
				ClientHold: true,
			},
			want: true,
		},
		{
			name: "ServerHold",
			ds: DomainStatus{
				ServerHold: true,
			},
			want: true,
		},
		{
			name: "both holds",
			ds: DomainStatus{
				ClientHold: true,
				ServerHold: true,
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		d := Domain{
			Status: tc.ds,
		}
		require.Equal(t, tc.want, d.Status.HasHold())
	}
}
func TestDomainStatus_StringSlice(t *testing.T) {
	testcases := []struct {
		name string
		ds   DomainStatus
		want []string
	}{
		{
			name: "all false",
			ds:   DomainStatus{},
			want: []string{},
		},
		{
			name: "OK status",
			ds: DomainStatus{
				OK: true,
			},
			want: []string{DomainStatusOK},
		},
		{
			name: "Inactive status",
			ds: DomainStatus{
				Inactive: true,
			},
			want: []string{DomainStatusInactive},
		},
		{
			name: "ClientTransferProhibited status",
			ds: DomainStatus{
				ClientTransferProhibited: true,
			},
			want: []string{DomainStatusClientTransferProhibited},
		},
		{
			name: "ServerHold status",
			ds: DomainStatus{
				ServerHold: true,
			},
			want: []string{DomainStatusServerHold},
		},
		{
			name: "Multiple statuses",
			ds: DomainStatus{
				OK:                       true,
				ClientTransferProhibited: true,
				PendingDelete:            true,
			},
			want: []string{DomainStatusOK, DomainStatusClientTransferProhibited, DomainStatusPendingDelete},
		},
		{
			name: "All statuses",
			ds: DomainStatus{
				OK:                       true,
				Inactive:                 true,
				ClientTransferProhibited: true,
				ServerTransferProhibited: true,
				ClientDeleteProhibited:   true,
				ServerDeleteProhibited:   true,
				ClientUpdateProhibited:   true,
				ServerUpdateProhibited:   true,
				ClientRenewProhibited:    true,
				ServerRenewProhibited:    true,
				PendingCreate:            true,
				PendingDelete:            true,
				PendingTransfer:          true,
				PendingUpdate:            true,
				PendingRenew:             true,
				PendingRestore:           true,
				ClientHold:               true,
				ServerHold:               true,
			},
			want: []string{
				DomainStatusOK,
				DomainStatusInactive,
				DomainStatusClientTransferProhibited,
				DomainStatusServerTransferProhibited,
				DomainStatusClientDeleteProhibited,
				DomainStatusServerDeleteProhibited,
				DomainStatusClientUpdateProhibited,
				DomainStatusServerUpdateProhibited,
				DomainStatusClientRenewProhibited,
				DomainStatusServerRenewProhibited,
				DomainStatusPendingCreate,
				DomainStatusPendingDelete,
				DomainStatusPendingTransfer,
				DomainStatusPendingUpdate,
				DomainStatusPendingRenew,
				DomainStatusPendingRestore,
				DomainStatusClientHold,
				DomainStatusServerHold,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.want, tc.ds.StringSlice())
		})
	}
}
