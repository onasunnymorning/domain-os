package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestToDBTld(t *testing.T) {
	tld, err := entities.NewTLD("com", "apex")
	tld.RyID = "ry-123"
	if err != nil {
		t.Fatal(err)
	}
	// add two phases
	phase1, err := entities.NewPhase("sunrise", "Launch", time.Now().UTC())
	require.NoError(t, err)
	phase2, err := entities.NewPhase("GA1", "GA", time.Now().UTC())
	require.NoError(t, err)
	err = tld.AddPhase(phase1)
	require.NoError(t, err)
	err = tld.AddPhase(phase2)
	require.NoError(t, err)
	require.Len(t, tld.Phases, 2)

	dbtld := ToDBTLD(tld)

	require.Equal(t, tld.Name.String(), dbtld.Name, "TLD Name mismatch")
	require.Equal(t, tld.Type.String(), dbtld.Type, "TLD Type mismatch")
	require.Equal(t, tld.UName.String(), dbtld.UName, "TLD UName mismatch")
	require.Equal(t, tld.CreatedAt, dbtld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, tld.UpdatedAt, dbtld.UpdatedAt, "TLD UpdatedAt mismatch")
	require.Equal(t, tld.RyID.String(), dbtld.RyID, "TLD RyID mismatch")
	require.Len(t, dbtld.Phases, 2, "TLD Phases length mismatch")
}

func TestFromDBTld(t *testing.T) {
	dbtld := &TLD{
		Name:      "com",
		Type:      "generic",
		UName:     "com",
		RyID:      "ry-123",
		CreatedAt: entities.RoundTime(time.Now().UTC()),
		UpdatedAt: entities.RoundTime(time.Now().UTC()),
	}

	dbtld.Phases = []Phase{
		{
			Name:   "sunrise",
			Type:   "Launch",
			Starts: time.Now().UTC(),
			Ends:   nil,
		},
		{
			Name:   "GA",
			Type:   "GA",
			Starts: time.Now().UTC(),
			Ends:   nil,
		},
	}

	tld := FromDBTLD(dbtld)

	require.Equal(t, dbtld.Name, tld.Name.String(), "TLD Name mismatch")
	require.Equal(t, dbtld.Type, tld.Type.String(), "TLD Type mismatch")
	require.Equal(t, dbtld.UName, tld.UName.String(), "TLD UName mismatch")
	require.Equal(t, dbtld.CreatedAt, tld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, dbtld.UpdatedAt, tld.UpdatedAt, "TLD UpdatedAt mismatch")
	require.Equal(t, dbtld.RyID, tld.RyID.String(), "TLD RyID mismatch")
	require.Len(t, tld.Phases, 2, "TLD Phases length mismatch")
}
func TestSetAllowEscrowImport(t *testing.T) {
	tld, err := entities.NewTLD("com", "apex")
	require.NoError(t, err)

	// Test setting AllowEscrowImport to true with no active phases
	err = tld.SetAllowEscrowImport(true)
	require.NoError(t, err)
	require.True(t, tld.AllowEscrowImport, "AllowEscrowImport should be true")

	// Test setting AllowEscrowImport to false
	err = tld.SetAllowEscrowImport(false)
	require.NoError(t, err)
	require.False(t, tld.AllowEscrowImport, "AllowEscrowImport should be false")

	// Add an active phase
	phase, err := entities.NewPhase("GA1", "GA", time.Now().UTC())
	require.NoError(t, err)
	err = tld.AddPhase(phase)
	require.NoError(t, err)

	// Test setting AllowEscrowImport to true with active phases
	err = tld.SetAllowEscrowImport(true)
	require.Error(t, err)
	require.Equal(t, entities.ErrCannotSetEscrowImportWithActivePhases, err, "Expected ErrCannotSetEscrowImportWithActivePhases")
	require.False(t, tld.AllowEscrowImport, "AllowEscrowImport should be false after error")
}

func TestSetEnableDNS(t *testing.T) {
	tld, err := entities.NewTLD("com", "apex")
	require.NoError(t, err)

	// Test setting EnableDNS to true
	tld.SetEnableDNS(true)
	require.True(t, tld.EnableDNS, "EnableDNS should be true")

	// Test setting EnableDNS to false
	tld.SetEnableDNS(false)
	require.False(t, tld.EnableDNS, "EnableDNS should be false")
}
