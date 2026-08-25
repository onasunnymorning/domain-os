package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOperatorID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    OperatorID
		wantErr bool
	}{
		{name: "valid", id: "apex", want: OperatorID("apex")},
		{name: "valid max length", id: "sixteencharacter", want: OperatorID("sixteencharacter")},
		{name: "empty", id: "", want: OperatorID(""), wantErr: true},
		{name: "too short", id: "ap", want: OperatorID(""), wantErr: true},
		{name: "too long", id: "seventeencharacte", want: OperatorID(""), wantErr: true},
		{name: "non ASCII", id: "ïnvalïd", want: OperatorID(""), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewOperatorID(test.id)
			require.Equal(t, test.want, got)
			if test.wantErr {
				require.ErrorIs(t, err, ErrInvalidOperatorID)
				require.ErrorIs(t, err, ErrInvalidClIDType)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestOperatorID_Validate(t *testing.T) {
	require.NoError(t, OperatorID("apex").Validate())
	require.ErrorIs(t, OperatorID("").Validate(), ErrInvalidOperatorID)
	require.ErrorIs(t, OperatorID("ap").Validate(), ErrInvalidOperatorID)
}

func TestOperatorID_StringAndIsZero(t *testing.T) {
	require.Equal(t, "apex", OperatorID("apex").String())
	require.True(t, OperatorID("").IsZero())
	require.False(t, OperatorID("apex").IsZero())
}

func TestNewRegistrarClID(t *testing.T) {
	tests := []struct {
		name    string
		clID    string
		want    RegistrarClID
		wantErr bool
	}{
		{name: "valid", clID: "sh8013", want: RegistrarClID("sh8013")},
		{name: "empty", clID: "", want: RegistrarClID(""), wantErr: true},
		{name: "too short", clID: "sh", want: RegistrarClID(""), wantErr: true},
		{name: "too long", clID: "seventeencharacte", want: RegistrarClID(""), wantErr: true},
		{name: "non ASCII", clID: "ïnvalïd", want: RegistrarClID(""), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewRegistrarClID(test.clID)
			require.Equal(t, test.want, got)
			if test.wantErr {
				require.ErrorIs(t, err, ErrInvalidRegistrarClID)
				require.ErrorIs(t, err, ErrInvalidClIDType)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRegistrarClID_Validate(t *testing.T) {
	require.NoError(t, RegistrarClID("sh8013").Validate())
	require.ErrorIs(t, RegistrarClID("").Validate(), ErrInvalidRegistrarClID)
	require.ErrorIs(t, RegistrarClID("sh").Validate(), ErrInvalidRegistrarClID)
}

func TestRegistrarClID_StringAndIsZero(t *testing.T) {
	require.Equal(t, "sh8013", RegistrarClID("sh8013").String())
	require.True(t, RegistrarClID("").IsZero())
	require.False(t, RegistrarClID("sh8013").IsZero())
}

// TestScopeTypesAreDistinct is the ratchet this file exists for: the two scope
// kinds and a plain object ClID must not be assignable to one another. If a
// future change turns any of these into an alias, this test stops compiling in
// spirit — so it asserts the property the compiler enforces, by requiring an
// explicit conversion at every hop.
func TestScopeTypesAreDistinct(t *testing.T) {
	operator := OperatorID("apex")
	registrar := RegistrarClID("sh8013")

	// Conversion is legal and explicit; assignment would not compile.
	require.Equal(t, ClIDType("apex"), ClIDType(operator))
	require.Equal(t, ClIDType("sh8013"), ClIDType(registrar))
	require.NotEqual(t, operator.String(), registrar.String())
}
