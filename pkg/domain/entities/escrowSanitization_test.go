package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSanitizationRun(t *testing.T, started time.Time) *EscrowSanitizationRun {
	t.Helper()
	r, err := NewEscrowSanitizationRun(testScope(t), "Example.", uuid.New(), uuid.New(), testSHA,
		"rde-baseline-v1", "escrow-sanitize/1", "Artful-Dodger.", "wf-1", "run-1", started)
	require.NoError(t, err)
	return r
}

func TestNewEscrowSanitizationRun(t *testing.T) {
	started := time.Now()
	r := newSanitizationRun(t, started)
	assert.Equal(t, "example", r.TLD, "TLD is normalised")
	assert.Equal(t, "artful-dodger", r.SyntheticSuffix, "synthetic suffix is normalised like any other name")
	assert.Equal(t, EscrowSanitizationRunning, r.Outcome)
	assert.Equal(t, time.UTC, r.StartedAt.Location())

	scope := testScope(t)
	bad := []struct {
		name string
		fn   func() (*EscrowSanitizationRun, error)
	}{
		{"bad scope", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(OperatorID(""), "example", uuid.New(), uuid.New(), testSHA, "p", "w", "synthetic", "wf", "run", started)
		}},
		{"no source run", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(scope, "example", uuid.Nil, uuid.New(), testSHA, "p", "w", "synthetic", "wf", "run", started)
		}},
		{"bad source digest", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(scope, "example", uuid.New(), uuid.New(), "nope", "p", "w", "synthetic", "wf", "run", started)
		}},
		{"no policy version", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(scope, "example", uuid.New(), uuid.New(), testSHA, " ", "w", "synthetic", "wf", "run", started)
		}},
		{"suffix equals source TLD", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(scope, "example", uuid.New(), uuid.New(), testSHA, "p", "w", "example", "wf", "run", started)
		}},
		{"suffix is not a name", func() (*EscrowSanitizationRun, error) {
			return NewEscrowSanitizationRun(scope, "example", uuid.New(), uuid.New(), testSHA, "p", "w", "not a suffix!", "wf", "run", started)
		}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn()
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidEscrowSanitizationRun), "got %v", err)
		})
	}
}

func TestEscrowSanitizationRun_FinalizeOnlyPassCarriesADerivative(t *testing.T) {
	started := time.Now()
	done := started.Add(time.Minute)
	pass := EscrowSanitizationFinalization{
		Outcome: EscrowSanitizationPass, StageReached: "verify",
		DerivativeObjectKey: "k/deposit.xml.gz", DerivativeSHA256: testSHA, DerivativeBytes: 42,
		ManifestObjectKey: "k/manifest.json", CompletedAt: done,
	}

	t.Run("a pass without a derivative is rejected", func(t *testing.T) {
		r := newSanitizationRun(t, started)
		f := pass
		f.DerivativeObjectKey = ""
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowSanitizationRun))
		f = pass
		f.ManifestObjectKey = ""
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowSanitizationRun))
		f = pass
		f.DerivativeBytes = 0
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowSanitizationRun))
		f = pass
		f.DerivativeSHA256 = "short"
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowDigest))
		assert.False(t, r.IsFinal(), "rejected finalisations leave the run RUNNING")
	})

	t.Run("a quarantined run must point at nothing", func(t *testing.T) {
		r := newSanitizationRun(t, started)
		f := pass
		f.Outcome = EscrowSanitizationQuarantined
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowSanitizationRun),
			"nothing was published, so nothing may be referenced")

		require.NoError(t, r.Finalize(EscrowSanitizationFinalization{
			Outcome: EscrowSanitizationQuarantined, StageReached: "rewrite",
			Findings:    []EscrowFinding{{Code: "POLICY_UNKNOWN_ELEMENT", Severity: "ERROR", Stage: "rewrite", Message: "m"}},
			CompletedAt: done,
		}))
		assert.True(t, r.IsFinal())
		assert.Equal(t, []string{"POLICY_UNKNOWN_ELEMENT"}, r.FindingCodes())
		assert.Empty(t, r.DerivativeObjectKey)
	})

	t.Run("a pass finalises once", func(t *testing.T) {
		r := newSanitizationRun(t, started)
		require.NoError(t, r.Finalize(pass))
		assert.Equal(t, EscrowSanitizationPass, r.Outcome)
		require.NotNil(t, r.CompletedAt)

		err := r.Finalize(EscrowSanitizationFinalization{Outcome: EscrowSanitizationError, StageReached: "x", CompletedAt: done})
		assert.True(t, errors.Is(err, ErrEscrowSanitizationRunAlreadyFinal))
		assert.Equal(t, EscrowSanitizationPass, r.Outcome, "the first outcome stands")
	})

	t.Run("completion cannot precede the start", func(t *testing.T) {
		r := newSanitizationRun(t, started)
		f := pass
		f.CompletedAt = started.Add(-time.Second)
		assert.True(t, errors.Is(r.Finalize(f), ErrInvalidEscrowSanitizationRun))
	})
}

func TestEscrowValidationRunIsSanitizableSource(t *testing.T) {
	scope := testScope(t)
	started := time.Now()
	mk := func(profile string, outcome EscrowValidationOutcome) *EscrowValidationRun {
		r, err := NewEscrowValidationRun(uuid.New(), scope, "example", "wf", "run", profile, started)
		require.NoError(t, err)
		r.Outcome = outcome
		return r
	}
	assert.True(t, EscrowValidationRunIsSanitizableSource(mk(EscrowProfileRydeSig, EscrowValidationPass)))
	assert.True(t, EscrowValidationRunIsSanitizableSource(mk(EscrowProfilePlaintextXML, EscrowValidationPass)),
		"an unsigned deposit that passed validation is still an accepted source")
	assert.False(t, EscrowValidationRunIsSanitizableSource(mk(EscrowProfileRydeSig, EscrowValidationFail)))
	assert.False(t, EscrowValidationRunIsSanitizableSource(mk(EscrowProfileRydeSig, EscrowValidationRunning)))
	assert.False(t, EscrowValidationRunIsSanitizableSource(nil))
}
