package rdesanitize

import (
	"bytes"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizer_DeterministicPerTenantAndPurpose(t *testing.T) {
	a, err := NewTokenizer(testKey, "ryop1", PolicyVersion)
	require.NoError(t, err)
	again, err := NewTokenizer(testKey, "ryop1", PolicyVersion)
	require.NoError(t, err)
	b, err := NewTokenizer(testKey, "ryop2", PolicyVersion)
	require.NoError(t, err)
	nextPolicy, err := NewTokenizer(testKey, "ryop1", "rde-baseline-v2")
	require.NoError(t, err)

	// Deterministic: the same value in the same tenant always tokenises the
	// same way, which is what keeps a shared contact one contact.
	assert.Equal(t, a.Value(ValContactID, "CONT1"), again.Value(ValContactID, "CONT1"))
	// Distinct across tenants: a token cannot be used to join two operators.
	assert.NotEqual(t, a.Value(ValContactID, "CONT1"), b.Value(ValContactID, "CONT1"))
	// Distinct across policy versions: a re-run under a new policy is a
	// separate derivative and is not silently joinable to the old one.
	assert.NotEqual(t, a.Value(ValContactID, "CONT1"), nextPolicy.Value(ValContactID, "CONT1"))
	// Distinct across kinds: an id and a ROID with the same text do not collide.
	assert.NotEqual(t, a.Value(ValContactID, "X"), a.Value(ValContactROID, "X"))
	// Different inputs give different tokens.
	assert.NotEqual(t, a.Value(ValContactID, "CONT1"), a.Value(ValContactID, "CONT2"))
}

func TestTokenizer_ValuesStayInsideTheirSchemaTypes(t *testing.T) {
	tok, err := NewTokenizer(testKey, "ryop1", PolicyVersion)
	require.NoError(t, err)

	clID := regexp.MustCompile(`^[A-Z0-9]{3,16}$`) // eppcom:clIDType, 3-16 chars
	roid := regexp.MustCompile(`^(\w|_){1,80}-\w{1,8}$`)
	for _, in := range []string{"CONT1", "", "a very long contact handle that exceeds every limit"} {
		id := tok.Value(ValContactID, in)
		assert.Len(t, id, 16)
		assert.Regexp(t, clID, id)
		assert.Regexp(t, roid, tok.Value(ValContactROID, in))
		assert.Regexp(t, `^[a-z0-9]{16}@example\.invalid$`, tok.Value(ValEmail, in))
		assert.Regexp(t, `^Contact [A-Z0-9]{12}$`, tok.Value(ValPersonName, in))
	}
}

func TestTokenizer_RefusesAWeakKeyAndNeverEchoesIt(t *testing.T) {
	_, err := NewTokenizer([]byte("too short"), "ryop1", PolicyVersion)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoTokenKey))
	assert.NotContains(t, err.Error(), "too short", "an error must never echo key material")

	_, err = NewTokenizer(testKey, "", PolicyVersion)
	assert.Error(t, err, "a token key is scoped to a tenant")
}

func TestTokenizer_FingerprintIdentifiesTheKeyWithoutRevealingIt(t *testing.T) {
	a, err := NewTokenizer(testKey, "ryop1", PolicyVersion)
	require.NoError(t, err)
	sameKeyOtherTenant, err := NewTokenizer(testKey, "ryop2", PolicyVersion)
	require.NoError(t, err)
	rotated, err := NewTokenizer(bytes.Repeat([]byte("a-different-master-key-value!!!!"), 2), "ryop1", PolicyVersion)
	require.NoError(t, err)

	// The fingerprint tracks the master key, not the tenant, so a rotation is
	// visible as a break in joinability wherever it is recorded.
	assert.Equal(t, a.KeyFingerprint(), sameKeyOtherTenant.KeyFingerprint())
	assert.NotEqual(t, a.KeyFingerprint(), rotated.KeyFingerprint())
	assert.NotContains(t, string(testKey), a.KeyFingerprint())
}
