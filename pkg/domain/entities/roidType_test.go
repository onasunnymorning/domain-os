package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRoidType(t *testing.T) {
	snowflakeID := int64(12345)

	t.Run("RoidTypeContact", func(t *testing.T) {
		objectIdentifier := RoidTypeContact
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, CONTACT_ROID_ID, EPP_REPOSITORY_ID)

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.NoError(t, err)
		require.Equal(t, expectedRoidType, string(roidType))
	})

	t.Run("RoidTypeHost", func(t *testing.T) {
		objectIdentifier := RoidTypeHost
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, HOST_ROID_ID, EPP_REPOSITORY_ID)

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.NoError(t, err)
		require.Equal(t, expectedRoidType, string(roidType))
	})

	t.Run("RoidTypeDomain", func(t *testing.T) {
		objectIdentifier := RoidTypeDomain
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, DOMAIN_ROID_ID, EPP_REPOSITORY_ID)

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.NoError(t, err)
		require.Equal(t, expectedRoidType, string(roidType))
	})

	t.Run("InvalidObjectIdentifier", func(t *testing.T) {
		objectIdentifier := "invalid"
		expectedRoidType := ""

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.EqualError(t, err, ErrInvalidObjectIdentifier.Error())
		require.Equal(t, expectedRoidType, string(roidType))
	})
}

func TestRoidType_String(t *testing.T) {
	roidType := RoidType("12345_CONT-APEX")

	require.Equal(t, "12345_CONT-APEX", roidType.String())
}

func TestRoidType_Int64(t *testing.T) {
	roidType := RoidType("12345_CONT-APEX")

	expectedInt64 := int64(12345)
	actualInt64, err := roidType.Int64()

	require.Equal(t, expectedInt64, actualInt64)
	require.Equal(t, err, nil)
}

func TestRoidType_ObjectIdentifier(t *testing.T) {
	roidType := RoidType("12345_CONT-APEX")

	expectedObjectIdentifier := "CONT"
	actualObjectIdentifier := roidType.ObjectIdentifier()

	require.Equal(t, expectedObjectIdentifier, actualObjectIdentifier)
}

func TestRoidType_SystemIdentifier(t *testing.T) {
	roidType := RoidType("12345_CONT-APEX")

	expectedSystemIdentifier := "APEX"
	actualSystemIdentifier := roidType.SystemIdentifier()

	require.Equal(t, expectedSystemIdentifier, actualSystemIdentifier)
}
func TestRoidType_Validate(t *testing.T) {
	validRoid := RoidType("12345_CONT-APEX")
	missingDashRoid := RoidType("invalid_roid")

	t.Run("ValidRoid", func(t *testing.T) {
		err := validRoid.Validate()
		require.NoError(t, err)
	})

	t.Run("Missing Dash", func(t *testing.T) {
		err := missingDashRoid.Validate()
		require.EqualError(t, err, ErrInvalidRoid.Error())
	})

	// RFC 5730 constrains a roid to (\w|_){1,80}-\w{1,8} and says nothing about
	// its structure. The {id}_{object}-{system} shape is this registry's own,
	// enforced by NewRoidType, and a deposit from another registry has no
	// reason to follow it. Requiring the underscore here rejected every object
	// in a real .radio deposit, whose roids read "Dztys40879-RADIO".
	t.Run("a conformant roid from another registry is valid", func(t *testing.T) {
		require.NoError(t, RoidType("Dztys40879-RADIO").Validate())
		require.NoError(t, RoidType("invalid-roid").Validate())
		require.False(t, RoidType("Dztys40879-RADIO").IsIssuedHere())
		require.True(t, RoidType("1_DOM-APEX").IsIssuedHere())
		require.False(t, RoidType("REG_ADHMIIJVVLE2392-RADIO").IsIssuedHere(),
			"the local shape with an object identifier we never issue is still foreign")
	})

	// The accessors split on "_" and "-" and used to index the result blindly,
	// so reading the object type of a foreign roid panicked.
	t.Run("accessors are total over any roid", func(t *testing.T) {
		foreign := RoidType("Dztys40879-RADIO")
		require.Equal(t, "", foreign.ObjectIdentifier())
		require.Equal(t, "RADIO", foreign.SystemIdentifier())
		_, err := foreign.Int64()
		require.Error(t, err, "a foreign roid carries no snowflake id")

		local := RoidType("42_DOM-APEX")
		require.Equal(t, "DOM", local.ObjectIdentifier())
		require.Equal(t, "APEX", local.SystemIdentifier())
		n, err := local.Int64()
		require.NoError(t, err)
		require.Equal(t, int64(42), n)

		bare := RoidType("nodashes")
		require.Equal(t, "", bare.ObjectIdentifier())
		require.Equal(t, "", bare.SystemIdentifier())
	})
}
