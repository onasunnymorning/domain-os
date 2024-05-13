package entities

import (
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
				Name:     "de.domain--testtld",
				ClID:     "GoMamma",
				AuthInfo: "STr0mgP@ZZ",
				Status:   DomainStatus{OK: true},
			},
			want: ErrInvalidLabelDoubleDash,
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
			i, err := tc.domain.AddHost(tc.host)
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
}
