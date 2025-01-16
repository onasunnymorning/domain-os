package entities

import (
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
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
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidLabelDash,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     ".com",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrTLDAsDomain,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "Example.com",
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
			wantErr:  ErrInvalidLabelIDN,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.xn--1",
			authInfo: "abc123ABC*",
			clid:     "GoMamma",
			wantErr:  ErrInvalidLabelIDN,
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
				require.Equal(t, DomainName(strings.ToLower(tc.name)), d.Name)
				require.Equal(t, AuthInfoType(tc.authInfo), d.AuthInfo)
				require.Equal(t, ClIDType(tc.clid), d.ClID)
				require.False(t, d.DropCatch)
				if strings.Contains(tc.name, "xn--") {
					// For IDNs we expect the UName field to be set
					require.NotNil(t, d.UName)
				} else {
					// for Non-IDNs we expect the UName field to be nil
					require.Equal(t, "", d.UName.String())
				}
			}
		})
	}
}

func TestDomain_InvalidStatus(t *testing.T) {
	domain, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	require.NoError(t, err)
	domain.Status.OK = true
	domain.Status.PendingDelete = true

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

	domain.Status.ClientRenewProhibited = false
	domain.Status.PendingDelete = true
	require.False(t, domain.CanBeRenewed())

	domain.Status.PendingDelete = false
	domain.Status.PendingCreate = true
	require.False(t, domain.CanBeRenewed())

	domain.Status.PendingCreate = false
	domain.Status.PendingTransfer = true
	require.False(t, domain.CanBeRenewed())

	domain.Status.PendingTransfer = false
	domain.Status.PendingRestore = true
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

func TestDomain_Validate(t *testing.T) {
	testcases := []struct {
		name   string
		domain *Domain
		want   error
	}{
		{
			name: "valid domain",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: nil,
		},
		{
			name: "invalid roid",
			domain: &Domain{
				RoID:     "12345_CONT-APEX",
				Name:     "de.domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: ErrInvalidDomainRoID,
		},
		{
			name: "invalid name",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.-domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: ErrInvalidLabelDash,
		},
		{
			name: "invalid clid",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.domaintesttld",
				ClID:     "g",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: ErrInvalidClIDType,
		},
		{
			name: "invalid authinfo",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "S",
				Status:   DomainStatus{OK: true},
			},
			want: ErrInvalidAuthInfo,
		},
		{
			name: "invalid status",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{},
			},
			want: ErrInvalidDomainStatusCombination,
		},
		{
			name: "invalid use of originalName",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "de.domaintesttld",
				OriginalName: "de.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: ErrOriginalNameFieldReservedForIDN,
		},
		{
			name: "invalid use of UName",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "de.domaintesttld",
				UName:    "de.domaintesttld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: ErrUNameFieldReservedForIDNDomains,
		},
		{
			name: "valid use of UName and OriginalName",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "xn--cario-rta.domaintesttld",
				UName:        "cariño.domaintesttld",
				OriginalName: "xn--carioo-zwa.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: nil,
		},
		{
			name: "OriginalName should be an A-label",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "xn--cario-rta.domaintesttld",
				UName:        "cariño.domaintesttld",
				OriginalName: "cariño.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: ErrOriginalNameShouldBeAlabel,
		},
		{
			name: "UName empty for IDN domain",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "xn--cario-rta.domaintesttld",
				OriginalName: "xn--cario-rta.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: ErrNoUNameProvidedForIDNDomain,
		},
		{
			name: "UName is not the unicode version of the domain name",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "xn--cario-rta.domaintesttld",
				UName:        "fûkûp.domaintesttld",
				OriginalName: "xn--cario-rta.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: ErrUNameDoesNotMatchDomain,
		},
		{
			name: "Domain Name and OriginalName are the same",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "xn--cario-rta.domaintesttld",
				UName:        "cariño.domaintesttld",
				OriginalName: "xn--cario-rta.domaintesttld",
				ClID:         "GoMamma",
				AuthInfo:     "STr0mgP@ZZ",
				Status:       DomainStatus{OK: true},
			},
			want: ErrOriginalNameEqualToDomain,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.domain.Validate())
		})
	}

}

func TestDomain_SetStatus(t *testing.T) {
	testcases := []struct {
		name        string
		ds          DomainStatus
		StatusToSet string
		wantErr     error
	}{
		{
			name: "invalid satus value",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: "invalid",
			wantErr:     ErrInvalidDomainStatus,
		},
		{
			name: "idempotent prohibition",
			ds: DomainStatus{
				ServerUpdateProhibited: true,
			},
			StatusToSet: DomainStatusServerUpdateProhibited,
			wantErr:     nil,
		},
		{
			name: "set inactive with pre-existing prohibitions",
			ds: DomainStatus{
				ServerDeleteProhibited: true,
			},
			StatusToSet: DomainStatusInactive,
			wantErr:     ErrInvalidDomainStatus,
		},
		{
			name: "set OK with pre-existing prohibitions",
			ds: DomainStatus{
				ServerDeleteProhibited: true,
			},
			StatusToSet: DomainStatusOK,
			wantErr:     ErrInvalidDomainStatus,
		},
		{
			name: "set OK with only inactive",
			ds: DomainStatus{
				Inactive: true,
			},
			StatusToSet: DomainStatusOK,
			wantErr:     ErrInvalidDomainStatus,
		},
		{
			name: "set Client Transfer prohibited with only inactive",
			ds: DomainStatus{
				Inactive: true,
			},
			StatusToSet: DomainStatusClientTransferProhibited,
			wantErr:     nil,
		},
		{
			name: "set Client Transfer prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusClientTransferProhibited,
			wantErr:     nil,
		},
		{
			name: "set Client Update prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusClientUpdateProhibited,
			wantErr:     nil,
		},
		{
			name: "set Client Delete prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusClientDeleteProhibited,
			wantErr:     nil,
		},
		{
			name: "set Client Renew prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusClientRenewProhibited,
			wantErr:     nil,
		},
		{
			name: "set Client Hold with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusClientHold,
			wantErr:     nil,
		},
		{
			name: "set Server Transfer prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusServerTransferProhibited,
			wantErr:     nil,
		},
		{
			name: "set Server Update prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusServerUpdateProhibited,
			wantErr:     nil,
		},
		{
			name: "set Server Delete prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusServerDeleteProhibited,
			wantErr:     nil,
		},
		{
			name: "set Server Renew prohibited with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusServerRenewProhibited,
			wantErr:     nil,
		},
		{
			name: "set Server Hold with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusServerHold,
			wantErr:     nil,
		},
		{
			name: "set Pending Create with only inactive",
			ds: DomainStatus{
				Inactive: true,
			},
			StatusToSet: DomainStatusPendingCreate,
			wantErr:     nil,
		},
		{
			name: "set Pending Renew with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusPendingRenew,
			wantErr:     nil,
		},
		{
			name: "set Pending Transfer with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusPendingTransfer,
			wantErr:     nil,
		},
		{
			name: "set Pending Update with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusPendingUpdate,
			wantErr:     nil,
		},
		{
			name: "set Pending Restore with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusPendingRestore,
			wantErr:     nil,
		},
		{
			name: "set Pending Delete with only OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToSet: DomainStatusPendingDelete,
			wantErr:     nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
			require.NoError(t, err)
			require.NotNil(t, d)
			d.Status = tc.ds

			err = d.SetStatus(tc.StatusToSet)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				r := reflect.ValueOf(d.Status)
				require.True(t, reflect.Indirect(r).FieldByName(strings.ToUpper(string(tc.StatusToSet[0]))+tc.StatusToSet[1:]).Bool())
				require.False(t, r.FieldByName("OK").Bool())
			}
		})
	}

}

func TestDomain_HasHosts(t *testing.T) {
	testcases := []struct {
		name  string
		hosts []*Host
		want  bool
	}{
		{
			name:  "no hosts",
			hosts: nil,
			want:  false,
		},
		{
			name: "has one host",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
			},
			want: true,
		},
		{
			name: "has two hosts",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
				{
					Name: "ns2.example.com",
				},
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain("12345_DOM-APEX", "deli.cusco", "GoMamma", "STr0mgP@ZZ")
			require.NoError(t, err)
			require.NotNil(t, d)
			d.Hosts = tc.hosts

			require.Equal(t, tc.want, d.HasHosts())
		})
	}
}

func TestDomain_SetUnsetInactiveStatus(t *testing.T) {
	testcases := []struct {
		name  string
		hosts []*Host
		want  bool
	}{
		{
			name:  "no hosts",
			hosts: nil,
			want:  true,
		},
		{
			name: "has one host",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
			},
			want: false,
		},
		{
			name: "has two hosts",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
				{
					Name: "ns2.example.com",
				},
			},
			want: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain("12345_DOM-APEX", "deli.cusco", "GoMamma", "STr0mgP@ZZ")
			require.NoError(t, err)
			require.NotNil(t, d)
			d.Hosts = tc.hosts

			d.SetUnsetInactiveStatus()

			require.Equal(t, tc.want, d.Status.Inactive)
		})
	}
}

func TestDomain_UnSetStatus(t *testing.T) {
	testcases := []struct {
		name          string
		ds            DomainStatus
		StatusToUnSet string
		wantErr       error
	}{
		{
			name: "invalid satus value",
			ds: DomainStatus{
				OK: true,
			},
			StatusToUnSet: "invalid",
			wantErr:       ErrInvalidDomainStatus,
		},
		{
			name: "Try and set OK",
			ds: DomainStatus{
				OK: true,
			},
			StatusToUnSet: DomainStatusOK,
			wantErr:       ErrInvalidDomainStatus,
		},
		{
			name: "Try and set inactive",
			ds: DomainStatus{
				OK: true,
			},
			StatusToUnSet: DomainStatusInactive,
			wantErr:       ErrInvalidDomainStatus,
		},
		{
			name: "unset Client Transfer prohibited with only inactive",
			ds: DomainStatus{
				ClientTransferProhibited: true,
				Inactive:                 true,
			},
			StatusToUnSet: DomainStatusClientTransferProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Client Transfer prohibited with only OK",
			ds: DomainStatus{
				ClientTransferProhibited: true,
			},
			StatusToUnSet: DomainStatusClientTransferProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Client Update prohibited with only OK",
			ds: DomainStatus{
				ClientUpdateProhibited: true,
			},
			StatusToUnSet: DomainStatusClientUpdateProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Client Delete prohibited with only OK",
			ds: DomainStatus{
				ClientDeleteProhibited: true,
			},
			StatusToUnSet: DomainStatusClientDeleteProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Client Renew prohibited with only OK",
			ds: DomainStatus{
				ClientRenewProhibited: true,
			},
			StatusToUnSet: DomainStatusClientRenewProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Client Hold with only OK",
			ds: DomainStatus{
				ClientHold: true,
			},
			StatusToUnSet: DomainStatusClientHold,
			wantErr:       nil,
		},
		{
			name: "unset Server Transfer prohibited with only OK",
			ds: DomainStatus{
				ServerTransferProhibited: true,
			},
			StatusToUnSet: DomainStatusServerTransferProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Server Update prohibited with only OK",
			ds: DomainStatus{
				ServerUpdateProhibited: true,
			},
			StatusToUnSet: DomainStatusServerUpdateProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Server Delete prohibited with only OK",
			ds: DomainStatus{
				ServerDeleteProhibited: true,
			},
			StatusToUnSet: DomainStatusServerDeleteProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Server Renew prohibited with only OK",
			ds: DomainStatus{
				ServerRenewProhibited: true,
			},
			StatusToUnSet: DomainStatusServerRenewProhibited,
			wantErr:       nil,
		},
		{
			name: "unset Server Hold with only OK",
			ds: DomainStatus{
				ServerHold: true,
			},
			StatusToUnSet: DomainStatusServerHold,
			wantErr:       nil,
		},
		{
			name: "unset Pending Create with only inactive",
			ds: DomainStatus{
				PendingCreate: true,
			},
			StatusToUnSet: DomainStatusPendingCreate,
			wantErr:       nil,
		},
		{
			name: "unset Pending Renew with only OK",
			ds: DomainStatus{
				PendingRenew: true,
			},
			StatusToUnSet: DomainStatusPendingRenew,
			wantErr:       nil,
		},
		{
			name: "unset Pending Transfer with only OK",
			ds: DomainStatus{
				PendingTransfer: true,
			},
			StatusToUnSet: DomainStatusPendingTransfer,
			wantErr:       nil,
		},
		{
			name: "unset Pending Update with only OK",
			ds: DomainStatus{
				PendingUpdate: true,
			},
			StatusToUnSet: DomainStatusPendingUpdate,
			wantErr:       nil,
		},
		{
			name: "unset Pending Restore with only OK",
			ds: DomainStatus{
				PendingRestore: true,
			},
			StatusToUnSet: DomainStatusPendingRestore,
			wantErr:       nil,
		},
		{
			name: "unset Pending Delete with only OK",
			ds: DomainStatus{
				PendingDelete: true,
			},
			StatusToUnSet: DomainStatusPendingDelete,
			wantErr:       nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
			require.NoError(t, err)
			require.NotNil(t, d)
			d.Status = tc.ds
			// Make sure the domain has no host so we always expect inactive to be set

			err = d.UnSetStatus(tc.StatusToUnSet)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				r := reflect.ValueOf(d.Status)
				require.False(t, reflect.Indirect(r).FieldByName(strings.ToUpper(string(tc.StatusToUnSet[0]))+tc.StatusToUnSet[1:]).Bool())
				require.True(t, d.Status.OK)
				require.True(t, d.Status.Inactive)
			}
		})
	}

}
func TestDomain_containsHost(t *testing.T) {
	d := &Domain{
		Hosts: []*Host{
			{Name: "host1"},
			{Name: "host2"},
			{Name: "host3"},
		},
	}

	t.Run("existing host", func(t *testing.T) {
		host := &Host{Name: "host2"}
		index, found := d.containsHost(host)
		require.True(t, found)
		require.Equal(t, 1, index)
	})

	t.Run("non-existing host", func(t *testing.T) {
		host := &Host{Name: "host4"}
		_, found := d.containsHost(host)
		require.False(t, found)
	})
}

func TestDomain_AddHost(t *testing.T) {
	ip, _ := netip.ParseAddr("195.238.2.21")
	testcases := []struct {
		name          string
		domain        *Domain
		host          *Host
		wantErr       error
		wantHostCount int
		wantInactive  bool
	}{
		{
			name: "domain can't be updated",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status: DomainStatus{
					Inactive:               true,
					ClientUpdateProhibited: true,
				},
			},
			host: &Host{
				Name: "ns1.inti.raymi",
			},
			wantErr:       ErrDomainUpdateNotAllowed,
			wantHostCount: 0,
			wantInactive:  true,
		},
		{
			name: "max hosts exceeded",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Hosts: []*Host{
					{
						Name:   "ns1.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns2.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns3.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns4.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns5.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns6.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns7.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns8.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns9.inti.raymi",
						Status: HostStatus{Linked: true},
					},
					{
						Name:   "ns10.inti.raymi",
						Status: HostStatus{Linked: true},
					},
				},
				Status: DomainStatus{
					Inactive: false,
				},
			},
			host: &Host{
				Name: "ns11.inti.raymi",
			},
			wantErr:       ErrMaxHostsPerDomainExceeded,
			wantHostCount: 10,
			wantInactive:  false,
		},
		{
			name: "duplicate host",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Hosts: []*Host{
					{
						Name:   "ns1.inti.raymi",
						Status: HostStatus{Linked: true},
					},
				},
				Status: DomainStatus{
					Inactive: false,
				},
			},
			host: &Host{
				Name: "ns1.inti.raymi",
			},
			wantErr:       ErrDuplicateHost,
			wantHostCount: 1,
			wantInactive:  false,
		},
		{
			name: "sponsorship mismatch",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Hosts: []*Host{
					{
						Name:   "ns1.inti.raymi",
						Status: HostStatus{Linked: true},
					},
				},
				Status: DomainStatus{
					Inactive: false,
				},
			},
			host:          &Host{Name: "ns2.cusco.raymi"},
			wantErr:       ErrHostSponsorMismatch,
			wantHostCount: 1,
			wantInactive:  false,
		},
		{
			name: "in-bailiwick without address",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Hosts: []*Host{
					{
						Name:   "ns1.inti.raymi",
						Status: HostStatus{Linked: true},
					},
				},
				Status: DomainStatus{
					Inactive: false,
				},
			},
			host: &Host{
				Name: "ns2.inti.raymi",
				ClID: "GoMamma",
			},
			wantErr:       ErrInBailiwickHostsMustHaveAddress,
			wantHostCount: 1,
			wantInactive:  false,
		},
		{
			name: "in-bailiwick with address",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Hosts: []*Host{
					{
						Name:   "ns1.inti.raymi",
						Status: HostStatus{Linked: true},
					},
				},
				Status: DomainStatus{
					Inactive: false,
				},
			},
			host: &Host{
				Name:      "ns2.inti.raymi",
				Addresses: []netip.Addr{ip},
				ClID:      "GoMamma",
				Status: HostStatus{
					OK: true,
				},
			},
			wantErr:       nil,
			wantHostCount: 2,
			wantInactive:  false,
		},
		{
			name: "firsthost",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status: DomainStatus{
					Inactive: true,
				},
			},
			host: &Host{
				Name: "ns2.cloud.raymi",
				ClID: "GoMamma",
				Status: HostStatus{
					OK: true,
				},
			},
			wantErr:       nil,
			wantHostCount: 1,
			wantInactive:  false,
		},
		{
			name: "host with conflicting status",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status: DomainStatus{
					Inactive: true,
				},
			},
			host: &Host{
				Name: "ns2.cloud.raymi",
				ClID: "GoMamma",
				Status: HostStatus{
					PendingCreate: true,
					PendingUpdate: true,
				},
			},
			wantErr:       ErrHostStatusIncompatible,
			wantHostCount: 0,
			wantInactive:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			i, err := tc.domain.AddHost(tc.host, false)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				require.Equal(t, tc.wantHostCount-1, i)
			}
			require.Equal(t, tc.wantHostCount, len(tc.domain.Hosts))
			require.Equal(t, tc.wantInactive, tc.domain.Status.Inactive)
			for _, h := range tc.domain.Hosts {
				require.True(t, h.Status.Linked)
			}
		})
	}

}
func TestDomain_AddHost_set_in_bailiwick(t *testing.T) {
	ip, _ := netip.ParseAddr("195.238.2.21")
	testcases := []struct {
		name            string
		domain          *Domain
		host            *Host
		wantErr         error
		wantInBailiwick bool
	}{
		{
			name: "in-bailiwick without address",
			domain: &Domain{
				RoID:     "12345_DOM-APEX",
				Name:     "inti.raymi",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{Inactive: false},
			},
			host: &Host{
				Name:      "ns2.inti.raymi",
				ClID:      "GoMamma",
				Addresses: []netip.Addr{ip},
				Status: HostStatus{
					OK: true,
				},
			},
			wantErr:         nil,
			wantInBailiwick: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.domain.AddHost(tc.host, false)
			require.ErrorIs(t, err, tc.wantErr)
			require.Equal(t, tc.wantInBailiwick, tc.domain.Hosts[0].InBailiwick)
		})
	}
}

func TestDomain_RemoveHost(t *testing.T) {
	d := &Domain{
		Hosts: []*Host{
			{Name: DomainName("host1")},
			{Name: DomainName("host2")},
			{Name: DomainName("host3")},
		},
		Status: DomainStatus{
			Inactive: false,
		},
	}

	t.Run("Remove existing host", func(t *testing.T) {
		host := &Host{Name: "host2"}
		err := d.RemoveHost(host)
		require.NoError(t, err)
		require.Len(t, d.Hosts, 2)
		require.Equal(t, "host1", d.Hosts[0].Name.String())
		require.Equal(t, "host3", d.Hosts[1].Name.String())
		require.False(t, d.Status.Inactive)
	})

	t.Run("Remove non-existing host", func(t *testing.T) {
		host := &Host{Name: "host4"}
		err := d.RemoveHost(host)
		require.Equal(t, ErrHostNotFound, err)
		require.Len(t, d.Hosts, 2)
		require.Equal(t, "host1", d.Hosts[0].Name.String())
		require.Equal(t, "host3", d.Hosts[1].Name.String())
		require.False(t, d.Status.Inactive)
	})

	t.Run("Remove all hosts", func(t *testing.T) {
		host := &Host{Name: "host1"}
		err := d.RemoveHost(host)
		require.NoError(t, err)
		require.Len(t, d.Hosts, 1)
		require.False(t, d.Status.Inactive)

		host = &Host{Name: "host3"}
		err = d.RemoveHost(host)
		require.NoError(t, err)
		require.Len(t, d.Hosts, 0)
		require.True(t, d.Status.Inactive)
	})

	t.Run("Remove host from empty list", func(t *testing.T) {
		d := &Domain{}
		host := &Host{Name: "host1"}
		err := d.RemoveHost(host)
		require.Equal(t, ErrHostNotFound, err)
		require.Empty(t, d.Hosts)
	})
}
func TestRegisterDomain(t *testing.T) {
	// Test case 1: Valid domain registration
	roid := "123456_DOM-APEX"
	name := "example.com"
	clid := "client123"
	authInfo := "STr0mgP@ZZ"
	registrantID := "registrant123"
	adminID := "admin123"
	techID := "tech123"
	billingID := "billing123"
	phase := &Phase{
		Name: "registration",
		Policy: PhasePolicy{
			RegistrationGP:     30,
			TransferLockPeriod: 60,
			AutoRenewalGP:      7,
		},
	}
	years := 2

	domain, err := RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, techID, billingID, phase, years)

	assert.NoError(t, err)
	assert.NotNil(t, domain)
	assert.Equal(t, roid, string(domain.RoID))
	assert.Equal(t, name, string(domain.Name))
	assert.Equal(t, clid, string(domain.ClID))
	assert.Equal(t, clid, string(domain.CrRr))
	assert.Equal(t, years-1, domain.RenewedYears)
	assert.Equal(t, authInfo, string(domain.AuthInfo))
	assert.Equal(t, registrantID, string(domain.RegistrantID))
	assert.Equal(t, adminID, string(domain.AdminID))
	assert.Equal(t, techID, string(domain.TechID))
	assert.Equal(t, billingID, string(domain.BillingID))
	expectedExpiryDate := time.Now().UTC().AddDate(years, 0, 0)
	assert.Equal(t, expectedExpiryDate.Year(), domain.ExpiryDate.Year())
	assert.Equal(t, expectedExpiryDate.Month(), domain.ExpiryDate.Month())
	assert.Equal(t, expectedExpiryDate.Day(), domain.ExpiryDate.Day())
	expectedRegistrationGPEnd := time.Now().UTC().AddDate(0, 0, phase.Policy.RegistrationGP)
	assert.Equal(t, expectedRegistrationGPEnd.Year(), domain.RGPStatus.AddPeriodEnd.Year())
	assert.Equal(t, expectedRegistrationGPEnd.Month(), domain.RGPStatus.AddPeriodEnd.Month())
	assert.Equal(t, expectedRegistrationGPEnd.Day(), domain.RGPStatus.AddPeriodEnd.Day())
	expectedTransferLockPeriodEnd := time.Now().UTC().AddDate(0, 0, phase.Policy.TransferLockPeriod)
	assert.Equal(t, expectedTransferLockPeriodEnd.Year(), domain.RGPStatus.TransferLockPeriodEnd.Year())
	assert.Equal(t, expectedTransferLockPeriodEnd.Month(), domain.RGPStatus.TransferLockPeriodEnd.Month())
	assert.Equal(t, expectedTransferLockPeriodEnd.Day(), domain.RGPStatus.TransferLockPeriodEnd.Day())

	// Test case 2: Missing phase
	_, err = RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, techID, billingID, nil, years)
	assert.Error(t, err)
	assert.Equal(t, ErrPhaseNotProvided, err)

	// Test case 3: Invalid domain name
	_, err = RegisterDomain(roid, "example..com", clid, authInfo, registrantID, adminID, techID, billingID, phase, years)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidLabelLength, err)

	// Test case 4: Requires Validation in Phase
	validationTrue := true
	phase.Policy.RequiresValidation = &validationTrue
	dom, err := RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, techID, billingID, phase, years)
	assert.NoError(t, err)
	assert.NotNil(t, dom)
	assert.True(t, dom.Status.PendingCreate)

	// Test case 5: Violate ContactDataPolicy in Phase
	phase.Policy.ContactDataPolicy.AdminContactDataPolicy = ContactDataPolicyTypeProhibited // This should clear the admin contact
	phase.Policy.ContactDataPolicy.TechContactDataPolicy = ContactDataPolicyTypeMandatory   // This should fire an error for empty tech contact
	_, err = RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, "", billingID, phase, years)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ErrTechIDRequiredButNotSet.Error())
	dom, err = RegisterDomain(roid, name, clid, authInfo, registrantID, techID, techID, billingID, phase, years)
	assert.NoError(t, err)
	assert.Empty(t, dom.AdminID)

	// TEst case 6: 0 years
	_, err = RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, techID, billingID, phase, 0)
	assert.Error(t, err)
	assert.Equal(t, ErrZeroRenewalPeriod, err)

}

func TestDomain_Renew(t *testing.T) {
	dom := &Domain{
		RoID:     "12345_DOM-APEX",
		Name:     "a.pex.domains",
		ClID:     "GoMamma",
		AuthInfo: "STr0mgP@ZZ",
	}
	testcases := []struct {
		name      string
		domStatus *DomainStatus
		expDate   time.Time
		years     int
		auto      bool
		phase     *Phase
		wantErr   error
	}{
		{
			name:      "phase is nil",
			domStatus: &DomainStatus{OK: true},
			expDate:   time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC),
			years:     0,
			phase:     nil,
			wantErr:   ErrPhaseNotProvided,
		},
		{
			name:      "zero years",
			domStatus: &DomainStatus{OK: true},
			expDate:   time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC),
			years:     0,
			phase:     &Phase{Policy: PhasePolicy{RegistrationGP: 5}},
			wantErr:   ErrZeroRenewalPeriod,
		},
		{
			name:      "can't be renewed",
			domStatus: &DomainStatus{PendingCreate: true},
			expDate:   time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC),
			years:     1,
			phase:     &Phase{Policy: PhasePolicy{RegistrationGP: 5}},
			wantErr:   ErrDomainRenewNotAllowed,
		},
		{
			name:      "exceed max horizon",
			domStatus: &DomainStatus{OK: true},
			expDate:   time.Now().UTC(),
			years:     11,
			phase:     &Phase{Policy: PhasePolicy{RegistrationGP: 5, MaxHorizon: 10}},
			wantErr:   ErrDomainRenewExceedsMaxHorizon,
		},
		{
			name:      "valid explicit renew",
			domStatus: &DomainStatus{OK: true},
			expDate:   time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			years:     4,
			phase:     &Phase{Policy: PhasePolicy{RenewalGP: 5, MaxHorizon: 10}},
			wantErr:   nil,
		},
		{
			name:      "valid auto renew",
			domStatus: &DomainStatus{OK: true},
			expDate:   time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			years:     1,
			auto:      true,
			phase:     &Phase{Policy: PhasePolicy{AutoRenewalGP: 45, MaxHorizon: 10}},
			wantErr:   nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			dom.RenewedYears = 0
			dom.Status = *tc.domStatus
			dom.ExpiryDate = tc.expDate
			err := dom.Renew(tc.years, tc.auto, tc.phase)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				assert.Equal(t, tc.expDate.AddDate(tc.years, 0, 0), dom.ExpiryDate)
				assert.Equal(t, tc.years, dom.RenewedYears)
				assert.Equal(t, dom.ClID, dom.UpRr)
				if tc.auto {
					assert.Equal(t, time.Now().UTC().AddDate(0, 0, tc.phase.Policy.AutoRenewalGP).Truncate(time.Hour), dom.RGPStatus.AutoRenewPeriodEnd.Truncate(time.Hour))
				} else {
					assert.Equal(t, time.Now().UTC().AddDate(0, 0, tc.phase.Policy.RenewalGP).Truncate(time.Hour), dom.RGPStatus.RenewPeriodEnd.Truncate(time.Hour))
				}
			}
		})
	}
}

func TestDomain_CanBeRestored(t *testing.T) {
	testcases := []struct {
		name            string
		DomainStatus    DomainStatus
		DomainRGPStatus DomainRGPStatus
		want            bool
	}{
		{
			name:         "domain can be restored",
			DomainStatus: DomainStatus{PendingDelete: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			want: true,
		},
		{
			name:         "domain not in pendingDelete",
			DomainStatus: DomainStatus{OK: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			want: false,
		},
		{
			name:         "domain not in redemption period",
			DomainStatus: DomainStatus{PendingDelete: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, -1),
			},
			want: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Domain{
				Status:    tc.DomainStatus,
				RGPStatus: tc.DomainRGPStatus,
			}

			assert.Equal(t, tc.want, d.CanBeRestored())
		})
	}
}

func TestDomain_Restore(t *testing.T) {
	testcases := []struct {
		name            string
		DomainStatus    DomainStatus
		DomainRGPStatus DomainRGPStatus
		wantErr         error
	}{
		{
			name:         "domain can be restored",
			DomainStatus: DomainStatus{PendingDelete: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			wantErr: nil,
		},
		{
			name:         "domain not in pendingDelete",
			DomainStatus: DomainStatus{OK: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			wantErr: ErrDomainRestoreNotAllowed,
		},
		{
			name:         "domain not in redemption period",
			DomainStatus: DomainStatus{PendingDelete: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, -1),
			},
			wantErr: ErrDomainRestoreNotAllowed,
		},
		{
			name:         "status prevents updates",
			DomainStatus: DomainStatus{PendingDelete: true, ClientUpdateProhibited: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			wantErr: ErrDomainUpdateNotAllowed,
		},
		{
			name:         "conflicting statusses",
			DomainStatus: DomainStatus{PendingDelete: true, PendingTransfer: true},
			DomainRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: time.Now().UTC().AddDate(0, 0, 1),
			},
			wantErr: ErrInvalidDomainStatusCombination,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Domain{
				Status:    tc.DomainStatus,
				RGPStatus: tc.DomainRGPStatus,
			}

			err := d.Restore()
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				assert.False(t, d.Status.PendingDelete)
				assert.True(t, d.Status.PendingRestore)
				assert.False(t, d.Status.OK)
			}
		})
	}

}

func TestDomain_MarkForDeletion(t *testing.T) {
	now := time.Now().UTC()
	testcases := []struct {
		name            string
		DomainStatus    DomainStatus
		DomainRGPStatus DomainRGPStatus
		Phase           *Phase
		wantErr         error
	}{
		{
			name:            "domain can be marked for deletion",
			DomainStatus:    DomainStatus{OK: true},
			DomainRGPStatus: DomainRGPStatus{},
			Phase: &Phase{
				Policy: PhasePolicy{
					RedemptionGP:    30,
					PendingDeleteGP: 5,
				},
			},
			wantErr: nil,
		},
		{
			name:         "domain within AddGracePeriod",
			DomainStatus: DomainStatus{OK: true},
			DomainRGPStatus: DomainRGPStatus{
				AddPeriodEnd: now.AddDate(0, 0, 1),
			},
			Phase: &Phase{
				Policy: PhasePolicy{
					RedemptionGP:    30,
					PendingDeleteGP: 5,
				},
			},
			wantErr: nil,
		},
		{
			name:            "domain not deletable",
			DomainStatus:    DomainStatus{PendingDelete: true},
			DomainRGPStatus: DomainRGPStatus{},
			Phase: &Phase{
				Policy: PhasePolicy{
					RedemptionGP:    30,
					PendingDeleteGP: 5,
				},
			},
			wantErr: ErrDomainDeleteNotAllowed,
		},
		{
			name:            "status prevents updates",
			DomainStatus:    DomainStatus{ClientUpdateProhibited: true},
			DomainRGPStatus: DomainRGPStatus{},
			Phase: &Phase{
				Policy: PhasePolicy{
					RedemptionGP:    30,
					PendingDeleteGP: 5,
				},
			},
			wantErr: ErrDomainUpdateNotAllowed,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Domain{
				Status:    tc.DomainStatus,
				RGPStatus: tc.DomainRGPStatus,
			}

			err := d.MarkForDeletion(tc.Phase)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				assert.True(t, d.Status.PendingDelete)
				assert.False(t, d.Status.OK)
				if tc.DomainRGPStatus.AddPeriodEnd.Equal(time.Time{}) {
					assert.Equal(t, d.RGPStatus.RedemptionPeriodEnd.Year(), now.AddDate(0, 0, 30).Year())
					assert.Equal(t, d.RGPStatus.RedemptionPeriodEnd.Month(), now.AddDate(0, 0, 30).Month())
					assert.Equal(t, d.RGPStatus.RedemptionPeriodEnd.Day(), now.AddDate(0, 0, 30).Day())
					assert.Equal(t, d.RGPStatus.PurgeDate.Year(), now.AddDate(0, 0, 35).Year())
					assert.Equal(t, d.RGPStatus.PurgeDate.Month(), now.AddDate(0, 0, 35).Month())
					assert.Equal(t, d.RGPStatus.PurgeDate.Day(), now.AddDate(0, 0, 35).Day())
					assert.True(t, d.RGPStatus.PurgeDate.After(d.RGPStatus.RedemptionPeriodEnd))
				} else {
					// sleep 1 second to make sure the time is different
					time.Sleep(1 * time.Second)
					assert.True(t, d.RGPStatus.RenewPeriodEnd.Before(time.Now().UTC()))
					assert.True(t, d.RGPStatus.AutoRenewPeriodEnd.Before(time.Now().UTC()))
				}
			}
		})
	}

}

func TestIsGrandFathered(t *testing.T) {
	// Create a domain object with GrandFathering status
	d := &Domain{
		GrandFathering: DomainGrandFathering{
			GFAmount:   100,
			GFCurrency: "USD",
		},
	}

	// Assert that the domain is indeed grand fathered
	if !d.IsGrandFathered() {
		t.Errorf("Expected domain to be grand fathered, but it is not")
	}

	d = &Domain{}

	// Assert that the domain is not grand fathered
	if d.IsGrandFathered() {
		t.Errorf("Expected domain to not be grand fathered, but it is")
	}

}
func TestDomain_GetHostsAsStringSlice(t *testing.T) {
	testcases := []struct {
		name  string
		hosts []*Host
		want  []string
	}{
		{
			name:  "no hosts",
			hosts: nil,
			want:  make([]string, 0),
		},
		{
			name: "one host",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
			},
			want: []string{"ns1.example.com"},
		},
		{
			name: "multiple hosts",
			hosts: []*Host{
				{
					Name: "ns1.example.com",
				},
				{
					Name: "ns2.example.com",
				},
			},
			want: []string{"ns1.example.com", "ns2.example.com"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Domain{
				Hosts: tc.hosts,
			}
			got := d.GetHostsAsStringSlice()
			require.Equal(t, tc.want, got)
		})
	}
}
func TestDomain_Expire(t *testing.T) {
	now := time.Now().UTC()
	phase := &Phase{
		Policy: PhasePolicy{
			RedemptionGP:    30,
			PendingDeleteGP: 5,
		},
	}

	testcases := []struct {
		name          string
		domain        *Domain
		wantErr       error
		wantStatus    DomainStatus
		wantRGPStatus DomainRGPStatus
	}{
		{
			name: "domain not expired yet",
			domain: &Domain{
				ExpiryDate: now.AddDate(0, 0, 1),
			},
			wantErr: ErrDomainExpiryNotAllowed,
		},
		{
			name: "domain pendingTransfer",
			domain: &Domain{
				ExpiryDate: now.AddDate(0, 0, -1),
				Status: DomainStatus{
					PendingTransfer: true,
				},
			},
			wantErr: ErrDomainExpiryFailed,
		},
		{
			name: "domain expired",
			domain: &Domain{
				ExpiryDate: now.AddDate(0, 0, -1),
			},
			wantErr: nil,
			wantStatus: DomainStatus{
				PendingDelete: true,
			},
			wantRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: now.AddDate(0, 0, 29),
				PurgeDate:           now.AddDate(0, 0, 4),
			},
		},
		{
			name: "domain expired with ClientUpdateprohibited",
			domain: &Domain{
				Status: DomainStatus{
					ClientUpdateProhibited: true,
				},
				ExpiryDate: now.AddDate(0, 0, -1),
			},
			wantErr: nil,
			wantStatus: DomainStatus{
				PendingDelete: true,
			},
			wantRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: now.AddDate(0, 0, 29),
				PurgeDate:           now.AddDate(0, 0, 4),
			},
		},
		{
			name: "domain expired with ClientDeleterohibited",
			domain: &Domain{
				Status: DomainStatus{
					ClientDeleteProhibited: true,
				},
				ExpiryDate: now.AddDate(0, 0, -1),
			},
			wantErr: nil,
			wantStatus: DomainStatus{
				PendingDelete: true,
			},
			wantRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: now.AddDate(0, 0, 29),
				PurgeDate:           now.AddDate(0, 0, 4),
			},
		},
		{
			name: "domain expired with ServerUpdateprohibited",
			domain: &Domain{
				Status: DomainStatus{
					ServerUpdateProhibited: true,
				},
				ExpiryDate: now.AddDate(0, 0, -1),
			},
			wantErr: nil,
			wantStatus: DomainStatus{
				PendingDelete: true,
			},
			wantRGPStatus: DomainRGPStatus{
				RedemptionPeriodEnd: now.AddDate(0, 0, 29),
				PurgeDate:           now.AddDate(0, 0, 4),
			},
		},
		{
			name: "domain expired with ServerDeleteprohibited",
			domain: &Domain{
				Status: DomainStatus{
					ServerDeleteProhibited: true,
				},
				ExpiryDate: now.AddDate(0, 0, -1),
			},
			wantErr: ErrDomainExpiryNotAllowed,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.domain.Expire(phase)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				assert.True(t, tc.domain.Status.PendingDelete)
				assert.Equal(t, tc.wantRGPStatus.RedemptionPeriodEnd, tc.domain.ExpiryDate.AddDate(0, 0, phase.Policy.RedemptionGP))
				assert.Equal(t, tc.wantRGPStatus.PurgeDate, tc.domain.ExpiryDate.AddDate(0, 0, phase.Policy.PendingDeleteGP))
				assert.True(t, tc.domain.RGPStatus.PurgeDate.After(tc.domain.RGPStatus.RedemptionPeriodEnd))
			}
		})
	}
}
func TestDomain_applyContactDataPolicy(t *testing.T) {
	tcases := []struct {
		name      string
		domain    *Domain
		policy    ContactDataPolicy
		wantErr   error
		wantEmpty []string
	}{
		{
			name: "all mandatory fields set",
			domain: &Domain{
				RegistrantID: "reg123",
				AdminID:      "adm123",
				TechID:       "tec123",
				BillingID:    "bil123",
			},
			policy: ContactDataPolicy{
				RegistrantContactDataPolicy: ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      ContactDataPolicyTypeMandatory,
				TechContactDataPolicy:       ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy:    ContactDataPolicyTypeMandatory,
			},
			wantErr:   nil,
			wantEmpty: nil,
		},
		{
			name: "missing mandatory registrant",
			domain: &Domain{
				RegistrantID: "",
				AdminID:      "adm123",
				TechID:       "tec123",
				BillingID:    "bil123",
			},
			policy: ContactDataPolicy{
				RegistrantContactDataPolicy: ContactDataPolicyTypeMandatory,
			},
			wantErr:   ErrRegistrantIDRequiredButNotSet,
			wantEmpty: nil,
		},
		{
			name: "missing mandatory admin",
			domain: &Domain{
				RegistrantID: "reg123",
				AdminID:      "",
			},
			policy: ContactDataPolicy{
				AdminContactDataPolicy: ContactDataPolicyTypeMandatory,
			},
			wantErr:   ErrAdminIDRequiredButNotSet,
			wantEmpty: nil,
		},
		{
			name: "missing mandatory billing",
			domain: &Domain{
				RegistrantID: "reg123",
				BillingID:    "",
			},
			policy: ContactDataPolicy{
				BillingContactDataPolicy: ContactDataPolicyTypeMandatory,
			},
			wantErr:   ErrBillingIDRequiredButNotSet,
			wantEmpty: nil,
		},
		{
			name: "tech  mandatory missing",
			domain: &Domain{
				RegistrantID: "reg123",
				TechID:       "",
			},
			policy: ContactDataPolicy{
				TechContactDataPolicy:    ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy: ContactDataPolicyTypeMandatory,
			},
			wantErr:   ErrTechIDRequiredButNotSet, // fails fast on tech first
			wantEmpty: nil,
		},
		{
			name: "prohibited fields must be emptied",
			domain: &Domain{
				RegistrantID: "reg123",
				AdminID:      "adm123",
				TechID:       "tec123",
				BillingID:    "bil123",
			},
			policy: ContactDataPolicy{
				RegistrantContactDataPolicy: ContactDataPolicyTypeProhibited,
				TechContactDataPolicy:       ContactDataPolicyTypeProhibited,
				BillingContactDataPolicy:    ContactDataPolicyTypeProhibited,
				AdminContactDataPolicy:      ContactDataPolicyTypeProhibited,
			},
			wantErr:   nil,
			wantEmpty: []string{"RegistrantID", "TechID", "BillingID", "AdminID"},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.domain.ApplyContactDataPolicy(tc.policy)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			for _, field := range tc.wantEmpty {
				switch field {
				case "RegistrantID":
					assert.Empty(t, tc.domain.RegistrantID, "RegistrantID should be empty")
				case "AdminID":
					assert.Empty(t, tc.domain.AdminID, "AdminID should be empty")
				case "TechID":
					assert.Empty(t, tc.domain.TechID, "TechID should be empty")
				case "BillingID":
					assert.Empty(t, tc.domain.BillingID, "BillingID should be empty")
				}
			}
		})
	}
}
func TestDomain_Clone(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour * 24)

	type testCase struct {
		name        string
		domain      *Domain
		shouldBeNil bool
		// Optionally, you can include expected fields to verify after clone
	}

	testCases := []testCase{
		{
			name:        "Nil domain",
			domain:      nil,
			shouldBeNil: true,
		},
		{
			name: "Minimal domain (no hosts)",
			domain: &Domain{
				RoID:       "12345_DOM-APEX",
				Name:       "example.com",
				ExpiryDate: later,
			},
		},
		{
			name: "Full domain with two hosts",
			domain: &Domain{
				RoID:         "12345_DOM-APEX",
				Name:         "example.com",
				OriginalName: "original-example.com",
				UName:        "unicode-example.com",
				RegistrantID: "registrant123",
				AdminID:      "admin123",
				TechID:       "tech123",
				BillingID:    "billing123",
				ClID:         "client123",
				CrRr:         "createRegistrar",
				UpRr:         "updateRegistrar",
				TLDName:      "com",
				ExpiryDate:   later,
				DropCatch:    true,
				RenewedYears: 2,
				AuthInfo:     "authInfo123",
				CreatedAt:    now,
				UpdatedAt:    now,
				Status: DomainStatus{
					OK: true,
				},
				RGPStatus: DomainRGPStatus{
					AddPeriodEnd: later,
				},
				GrandFathering: DomainGrandFathering{
					GFAmount:   100,
					GFCurrency: "USD",
				},
				Hosts: []*Host{
					{
						RoID:        "12345_HOST-APEX",
						Name:        "ns1.example.com",
						InBailiwick: true,
						Addresses: []netip.Addr{
							netip.MustParseAddr("192.168.0.1"),
						},
					},
					{
						RoID:        "23456_HOST-APEX",
						Name:        "ns2.example.com",
						InBailiwick: false,
						Addresses: []netip.Addr{
							netip.MustParseAddr("192.168.0.2"),
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cloned := tc.domain.Clone()

			if tc.shouldBeNil {
				require.Nil(t, cloned)
				return
			}
			require.NotNil(t, cloned)
			require.NotSame(t, tc.domain, cloned)

			// Verify top-level fields (spot-check or do them all).
			require.Equal(t, tc.domain.RoID, cloned.RoID)
			require.Equal(t, tc.domain.Name, cloned.Name)
			require.Equal(t, tc.domain.OriginalName, cloned.OriginalName)
			require.Equal(t, tc.domain.UName, cloned.UName)
			require.Equal(t, tc.domain.RegistrantID, cloned.RegistrantID)
			require.Equal(t, tc.domain.AdminID, cloned.AdminID)
			require.Equal(t, tc.domain.TechID, cloned.TechID)
			require.Equal(t, tc.domain.BillingID, cloned.BillingID)
			require.Equal(t, tc.domain.ClID, cloned.ClID)
			require.Equal(t, tc.domain.CrRr, cloned.CrRr)
			require.Equal(t, tc.domain.UpRr, cloned.UpRr)
			require.Equal(t, tc.domain.TLDName, cloned.TLDName)
			require.Equal(t, tc.domain.ExpiryDate, cloned.ExpiryDate)
			require.Equal(t, tc.domain.DropCatch, cloned.DropCatch)
			require.Equal(t, tc.domain.RenewedYears, cloned.RenewedYears)
			require.Equal(t, tc.domain.AuthInfo, cloned.AuthInfo)
			require.Equal(t, tc.domain.CreatedAt, cloned.CreatedAt)
			require.Equal(t, tc.domain.UpdatedAt, cloned.UpdatedAt)
			require.Equal(t, tc.domain.Status, cloned.Status)
			require.Equal(t, tc.domain.RGPStatus, cloned.RGPStatus)
			require.Equal(t, tc.domain.GrandFathering, cloned.GrandFathering)

			// Check Hosts slice
			if len(tc.domain.Hosts) == 0 {
				require.Empty(t, cloned.Hosts)
			} else {
				require.Equal(t, len(tc.domain.Hosts), len(cloned.Hosts))

				require.NotEqual(t,
					fmt.Sprintf("%p", tc.domain.Hosts),
					fmt.Sprintf("%p", cloned.Hosts),
				)

				// Check each *Host pointer inside the slice:
				for i := range tc.domain.Hosts {
					require.NotSame(t, tc.domain.Hosts[i], cloned.Hosts[i])
				}

				for i := range tc.domain.Hosts {
					originalHost := tc.domain.Hosts[i]
					clonedHost := cloned.Hosts[i]
					require.NotNil(t, clonedHost)
					require.NotSame(t, originalHost, clonedHost)
					require.Equal(t, originalHost.RoID, clonedHost.RoID)
					require.Equal(t, originalHost.Name, clonedHost.Name)
					require.Equal(t, originalHost.ClID, clonedHost.ClID)
					require.Equal(t, originalHost.CrRr, clonedHost.CrRr)
					require.Equal(t, originalHost.UpRr, clonedHost.UpRr)
					require.Equal(t, originalHost.CreatedAt, clonedHost.CreatedAt)
					require.Equal(t, originalHost.UpdatedAt, clonedHost.UpdatedAt)
					require.Equal(t, originalHost.InBailiwick, clonedHost.InBailiwick)
					require.Equal(t, originalHost.Status, clonedHost.Status)

					// Check Addresses slice
					if len(originalHost.Addresses) == 0 {
						require.Empty(t, clonedHost.Addresses)
					} else {
						require.Len(t, clonedHost.Addresses, len(originalHost.Addresses))
						for j := range originalHost.Addresses {
							require.Equal(t, originalHost.Addresses[j], clonedHost.Addresses[j])
						}
					}
				}
			}
		})
	}
}
