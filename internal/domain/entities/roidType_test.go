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
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, CONTACT_ROID_ID, SYSTEM_ROID_ID)

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.NoError(t, err)
		require.Equal(t, expectedRoidType, string(roidType))
	})

	t.Run("RoidTypeHost", func(t *testing.T) {
		objectIdentifier := RoidTypeHost
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, HOST_ROID_ID, SYSTEM_ROID_ID)

		roidType, err := NewRoidType(snowflakeID, objectIdentifier)

		require.NoError(t, err)
		require.Equal(t, expectedRoidType, string(roidType))
	})

	t.Run("RoidTypeDomain", func(t *testing.T) {
		objectIdentifier := RoidTypeDomain
		expectedRoidType := fmt.Sprintf("%d_%s-%s", snowflakeID, DOMAIN_ROID_ID, SYSTEM_ROID_ID)

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
	missingUnderscoreRoid := RoidType("invalid-roid")

	t.Run("ValidRoid", func(t *testing.T) {
		err := validRoid.Validate()
		require.NoError(t, err)
	})

	t.Run("Missing Dash", func(t *testing.T) {
		err := missingDashRoid.Validate()
		require.EqualError(t, err, ErrInvalidRoid.Error())
	})

	t.Run("Missing Underscore", func(t *testing.T) {
		err := missingUnderscoreRoid.Validate()
		require.EqualError(t, err, ErrInvalidRoid.Error())
	})
}
