package entities

import (
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
