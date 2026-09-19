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

func TestEscrowSanitizationRun_ReopenOnlyFromError(t *testing.T) {
	started := time.Now()
	done := started.Add(time.Minute)
	retryAt := started.Add(time.Hour)
	reopening := EscrowSanitizationReopening{
		WorkflowVersion: "escrow-sanitize/2", SyntheticSuffix: "Sandbox-Zone.", WorkflowID: "wf-2", RunID: "run-2", StartedAt: retryAt,
	}
	errored := func(t *testing.T) *EscrowSanitizationRun {
		t.Helper()
		r := newSanitizationRun(t, started)
		require.NoError(t, r.Finalize(EscrowSanitizationFinalization{
			Outcome: EscrowSanitizationError, StageReached: "source",
			Findings:            []EscrowFinding{{Code: "TOKEN_KEY_UNAVAILABLE", Severity: "ERROR", Stage: "source", Message: "m"}},
			TokenKeyFingerprint: "fp", CompletedAt: done,
		}))
		return r
	}

	t.Run("an errored run returns to RUNNING as a fresh attempt", func(t *testing.T) {
		r := errored(t)
		id := r.ID
		require.NoError(t, r.Reopen(reopening))

		assert.Equal(t, id, r.ID, "it is the same record")
		assert.Equal(t, EscrowSanitizationRunning, r.Outcome)
		assert.False(t, r.IsFinal())
		assert.Empty(t, r.Findings, "the failed attempt's findings are cleared")
		assert.Empty(t, r.StageReached)
		assert.Empty(t, r.TokenKeyFingerprint)
		assert.Nil(t, r.CompletedAt)
		assert.Equal(t, "wf-2", r.WorkflowID)
		assert.Equal(t, "run-2", r.RunID)
		assert.Equal(t, "escrow-sanitize/2", r.WorkflowVersion)
		assert.Equal(t, "sandbox-zone", r.SyntheticSuffix, "the suffix is normalised like any other name")
		assert.True(t, r.StartedAt.Equal(retryAt))

		// And it can finish again, once.
		require.NoError(t, r.Finalize(EscrowSanitizationFinalization{
			Outcome: EscrowSanitizationQuarantined, StageReached: "rewrite", CompletedAt: retryAt.Add(time.Minute),
		}))
		assert.True(t, errors.Is(r.Reopen(reopening), ErrEscrowSanitizationRunNotReopenable), "a decision is final")
	})

	t.Run("a decision is never reopened", func(t *testing.T) {
		for _, f := range []EscrowSanitizationFinalization{
			{Outcome: EscrowSanitizationPass, StageReached: "verify", DerivativeObjectKey: "k", DerivativeSHA256: testSHA,
				DerivativeBytes: 1, ManifestObjectKey: "m", CompletedAt: done},
			{Outcome: EscrowSanitizationQuarantined, StageReached: "rewrite", CompletedAt: done},
		} {
			r := newSanitizationRun(t, started)
			require.NoError(t, r.Finalize(f))
			err := r.Reopen(reopening)
			assert.True(t, errors.Is(err, ErrEscrowSanitizationRunNotReopenable), "%s: %v", f.Outcome, err)
			assert.Equal(t, f.Outcome, r.Outcome)
			assert.Equal(t, "wf-1", r.WorkflowID, "a refused reopen changes nothing")
		}
	})

	t.Run("a run that is still running has nothing to reopen", func(t *testing.T) {
		r := newSanitizationRun(t, started)
		assert.True(t, errors.Is(r.Reopen(reopening), ErrEscrowSanitizationRunNotReopenable))
	})

	t.Run("an invalid attempt is refused and leaves the error in place", func(t *testing.T) {
		for name, mutate := range map[string]func(*EscrowSanitizationReopening){
			"no workflow version":       func(o *EscrowSanitizationReopening) { o.WorkflowVersion = "" },
			"no workflow id":            func(o *EscrowSanitizationReopening) { o.WorkflowID = " " },
			"no start time":             func(o *EscrowSanitizationReopening) { o.StartedAt = time.Time{} },
			"an unusable suffix":        func(o *EscrowSanitizationReopening) { o.SyntheticSuffix = "not a name!" },
			"a suffix equal to the TLD": func(o *EscrowSanitizationReopening) { o.SyntheticSuffix = "Example." },
		} {
			r := errored(t)
			o := reopening
			mutate(&o)
			assert.True(t, errors.Is(r.Reopen(o), ErrInvalidEscrowSanitizationRun), name)
			assert.Equal(t, EscrowSanitizationError, r.Outcome, name)
			assert.Equal(t, "wf-1", r.WorkflowID, name)
		}
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
