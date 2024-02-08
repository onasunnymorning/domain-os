package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthInfoType_IsValid(t *testing.T) {
	testCases := []struct {
		password string
		expected error
	}{
		{"ValidP@ssw0rd", nil},
		{"Weak", ErrInvalidAuthInfo},
		{"NoSpecialCharacter", ErrInvalidAuthInfo},
		{"nouppercase1@", ErrInvalidAuthInfo},
		{"NOLOWERCASEP@SSWORD", ErrInvalidAuthInfo},
		{"WayTooLongPasswordWithSpecialChars$!_", ErrInvalidAuthInfo},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("Password: %s", test.password), func(t *testing.T) {
			authInfo := AuthInfoType(test.password)
			if authInfo.Validate() != test.expected {
				t.Errorf("Expected %s to be %v", authInfo.String(), test.expected)
			}
		})
	}
}

func TestAuthInfoType_NewAuthInfo(t *testing.T) {
	testcases := []struct {
		testname string
		authinfo string
		err      error
	}{
		{"Valid password", "Str)NGp@zz", nil},
		{"Invalid password", "weak", ErrInvalidAuthInfo},
	}

	for _, test := range testcases {
		t.Run(test.testname, func(t *testing.T) {
			_, err := NewAuthInfoType(test.authinfo)
			require.Equal(t, test.err, err)
		})
	}
}
