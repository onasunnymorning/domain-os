package entities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestDomain_NewDomain(t *testing.T) {
	testcases := []struct {
		name     string
		authInfo string
		wantErr  error
	}{
		{
			name:     "example.com",
			authInfo: "abc123",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			name:     "example.com",
			authInfo: "",
			wantErr:  ErrInvalidAuthInfo,
		},
		{
			name:     "-example.com",
			authInfo: "abc123",
			wantErr:  ErrInvalidLabelDash,
		},
		{
			name:     ".com",
			authInfo: "abc123",
			wantErr:  ErrInvalidLabelLength,
		},
		{
			name:     "example.com",
			authInfo: "abc123ABC*",
			wantErr:  nil,
		},
		{
			name:     "xn--c1yn36f.com",
			authInfo: "abc123ABC*",
			wantErr:  nil,
		},
		{
			name:     "xn--1.com",
			authInfo: "abc123ABC*",
			wantErr:  ErrInvalidDomainName,
		},
		{
			name:     "example.xn--1",
			authInfo: "abc123ABC*",
			wantErr:  ErrInvalidDomainName,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDomain(tc.name, tc.authInfo)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
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
