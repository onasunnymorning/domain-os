package entities

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
