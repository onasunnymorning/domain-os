package entities

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestDomain_NewDomain(t *testing.T) {
	testcases := []struct {
		roid     string
		name     string
		authInfo string
		clid     string
		wantErr  error
	}{
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "abc123",
			clid:     "GoMamma",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "",
			clid:     "GoMamma",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "-example.com",
			authInfo: "abc123",
			clid:     "GoMamma",
			wantErr:  ErrInvalidLabelDash,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     ".com",
			authInfo: "abc123",
			clid:     "GoMamma",
			wantErr:  ErrInvalidLabelLength,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  nil,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "xn--c1yn36f.com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  nil,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "xn--1.com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidDomainName,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.xn--1",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidDomainName,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.xn--1",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidDomainName,
		},
		{
			roid:     "123456_DOM-",
			name:     "example.com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidRoid,
		},
		{
			roid:     "123456_HOST-APEX",
			name:     "example.com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidDomainRoID,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "xn--c1yn36f.com",
			authInfo: "abc123ABC*",
			clid:     "g",
			wantErr:  ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain(tc.roid, tc.name, tc.clid, tc.authInfo)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.Equal(t, RoidType(tc.roid), d.RoID)
				require.Equal(t, DomainName(tc.name), d.Name)
				require.Equal(t, AuthInfoType(tc.authInfo), d.AuthInfo)
				if !strings.Contains(tc.name, "xn--") {
					require.Equal(t, tc.name, d.UName)
				} else {
					expected, _ := idna.ToUnicode(tc.name)
					require.Equal(t, expected, d.UName)
				}
			}
		})
	}
}

func TestDomain_NewDomain_InvalidStatus(t *testing.T) {
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
			Name: "inactive but missing ok",
			ds: DomainStatus{
				Inactive: true,
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
	require.True(t, ds.OK)
	require.True(t, ds.Inactive)
}

func TestDomain_InvalidStatus(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)
	domain.Status.ClientHold = true

	require.ErrorIs(t, domain.Validate(), ErrInvalidDomainStatusCombination)

}

func TestDomain_CanBeDeleted(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)

	require.True(t, domain.CanBeDeleted())

	domain.Status.PendingDelete = true
	require.False(t, domain.CanBeDeleted())

	domain.Status.PendingDelete = false
	domain.Status.ClientDeleteProhibited = true
	require.False(t, domain.CanBeDeleted())
}

func TestDomain_CanBeUpdated(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)

	require.True(t, domain.CanBeUpdated())

	domain.Status.PendingUpdate = true
	require.False(t, domain.CanBeUpdated())

	domain.Status.PendingUpdate = false
	domain.Status.ClientUpdateProhibited = true
	require.False(t, domain.CanBeUpdated())
}

func TestDomain_CanBeRenewed(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)

	require.True(t, domain.CanBeRenewed())

	domain.Status.PendingRenew = true
	require.False(t, domain.CanBeRenewed())

	domain.Status.PendingRenew = false
	domain.Status.ClientRenewProhibited = true
	require.False(t, domain.CanBeRenewed())
}

func TestDomain_CanBeTransferred(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)

	require.True(t, domain.CanBeTransferred())

	domain.Status.PendingTransfer = true
	require.False(t, domain.CanBeTransferred())

	domain.Status.PendingTransfer = false
	domain.Status.ClientTransferProhibited = true
	require.False(t, domain.CanBeTransferred())
}

func TestDomainRGPStatus_IsNil(t *testing.T) {
	rgp := DomainRGPStatus{}
	require.True(t, rgp.IsNil())

	rgp = DomainRGPStatus{
		AddPeriodEnd: time.Now().UTC(),
	}
	require.False(t, rgp.IsNil())
}
