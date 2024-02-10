package entities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestDomain_NewDomain(t *testing.T) {
	testcases := []struct {
		roid     string
		name     string
		authInfo string
		wantErr  error
	}{
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "abc123",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "-example.com",
			authInfo: "abc123",
			wantErr:  ErrInvalidLabelDash,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     ".com",
			authInfo: "abc123",
			wantErr:  ErrInvalidLabelLength,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.com",
			authInfo: "abc123ABC*",
			wantErr:  nil,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "xn--c1yn36f.com",
			authInfo: "abc123ABC*",
			wantErr:  nil,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "xn--1.com",
			authInfo: "abc123ABC*",
			wantErr:  ErrInvalidDomainName,
		},
		{
			roid:     "123456_DOM-APEX",
			name:     "example.xn--1",
			authInfo: "abc123ABC*",
			wantErr:  ErrInvalidDomainName,
		},
		{
			roid:     "123456_DOM-",
			name:     "example.com",
			authInfo: "abc123ABC*",
			wantErr:  ErrInvalidRoid,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain(tc.roid, tc.name, tc.authInfo)
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
