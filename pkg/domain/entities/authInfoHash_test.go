package entities

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCode = "Tr4nsf3r-C0de-Xq9#Lm2!"

func TestHashAuthInfo(t *testing.T) {
	h, err := HashAuthInfo(testCode)
	require.NoError(t, err)
	assert.True(t, h.IsSet())
	assert.NoError(t, h.Validate())
	assert.True(t, strings.HasPrefix(string(h), "$sha256$"))
	assert.NotContains(t, string(h), testCode, "the code itself is never stored")

	t.Run("each hash has its own salt", func(t *testing.T) {
		again, err := HashAuthInfo(testCode)
		require.NoError(t, err)
		assert.NotEqual(t, h, again)
		assert.True(t, again.Matches(testCode))
	})
	t.Run("only the code matches (RFC 9154 §4.4)", func(t *testing.T) {
		assert.True(t, h.Matches(testCode))
		assert.False(t, h.Matches(testCode+"x"))
		assert.False(t, h.Matches(""), "an empty code matches nothing")
		assert.False(t, AuthInfoHash("").Matches(testCode), "nothing matches an unset code")
		assert.False(t, AuthInfoHash("").Matches(""))
	})
	t.Run("an empty code is no code", func(t *testing.T) {
		none, err := HashAuthInfo("")
		require.NoError(t, err)
		assert.False(t, none.IsSet())
		assert.NoError(t, none.Validate())
	})
	t.Run("a stored value not in the hash's form is refused", func(t *testing.T) {
		for _, bad := range []string{testCode, "$sha256$", "$sha256$abc$def", "$md5$c2FsdHNhbHRzYWx0c2FsdA$MTIz"} {
			assert.ErrorIs(t, AuthInfoHash(bad).Validate(), ErrInvalidAuthInfoHash, bad)
			assert.False(t, AuthInfoHash(bad).Matches(bad))
			assert.False(t, IsAuthInfoHash(bad))
		}
		assert.True(t, IsAuthInfoHash(string(h)))
	})
}

func TestValidateSecureAuthInfo(t *testing.T) {
	for _, ok := range []string{
		testCode,
		"9f86d081884c7d659a2feaa0c55ad015", // 32 lower-case hex digits: 165 bits by the estimate
		"k3j9x2m7p1q8z4w6v5n0r3t8y",        // 25 lower-case letters and digits
		"Aa1!Bb2@Cc3#Dd4$Ee5%",
	} {
		assert.NoError(t, ValidateSecureAuthInfo(ok), ok)
	}
	for _, weak := range []string{
		"",
		"Secret-Auth-1!",               // the registry's earlier rule: 14 characters, 92 bits
		"abcdefghijklmnopqrstuvwxy",    // 25 lower-case letters: 117 bits
		"aaaaaaaaaaaaaaaaaaaaaaaaaab",  // 27 lower-case: 127 bits, and 2 distinct
		"Aa1!Aa1!Aa1!Aa1!Aa1!Aa1!",     // four characters repeated
		"Tr4nsf3r-C0de-Xq9#Lm2!é",      // not ASCII
		strings.Repeat("Ab1!xY9#", 17), // longer than 128
	} {
		assert.ErrorIs(t, ValidateSecureAuthInfo(weak), ErrAuthInfoNotSecure, weak)
	}
}

func TestValidateAdminAuthInfo(t *testing.T) {
	assert.NoError(t, ValidateAdminAuthInfo(""), "no code")
	assert.NoError(t, ValidateAdminAuthInfo(testCode), "an RFC 9154 code")
	assert.NoError(t, ValidateAdminAuthInfo("Secret-Auth-1!"), "the registry's earlier rule")
	assert.Error(t, ValidateAdminAuthInfo("weak"))
}

func TestDomainJSONCarriesNoAuthInfo(t *testing.T) {
	d, err := NewDomain("1_DOM-TEST", "example.test", "registrar1", testCode)
	require.NoError(t, err)
	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "sha256", "the hash stays in the registry")
	assert.NotContains(t, string(b), `"AuthInfo"`)
	assert.Contains(t, string(b), `"AuthInfoSet":true`)

	var back Domain
	require.NoError(t, json.Unmarshal(b, &back))
	assert.False(t, back.AuthInfo.IsSet(), "a domain read from JSON has no hash to write back")

	none, err := NewDomain("2_DOM-TEST", "none.test", "registrar1", "")
	require.NoError(t, err)
	b, err = json.Marshal(none)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"AuthInfoSet":false`)
}
