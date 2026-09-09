package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testFP  = "0123456789ABCDEF0123456789ABCDEF01234567"
	testKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQ==\n-----END PGP PUBLIC KEY BLOCK-----"
)

func testScope(t *testing.T) OperatorID {
	t.Helper()
	s, err := NewOperatorID("ryop1")
	require.NoError(t, err)
	return s
}

func TestNewEscrowDeposit(t *testing.T) {
	scope := testScope(t)
	now := time.Now()

	d, err := NewEscrowDeposit(scope, "Example.", EscrowProfileRydeSig, now, "user-1", " ref ", "k/ryde", testSHA, 10, "k/sig", testSHA, 5)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.Equal(t, "example", d.TLD, "TLD is normalised")
	assert.Equal(t, "ref", d.IntakeRef)
	assert.Equal(t, EscrowProfileRydeSig, d.Profile)
	assert.Equal(t, time.UTC, d.ReceivedAt.Location())

	t.Run("unsigned profile needs no signature", func(t *testing.T) {
		p, err := NewEscrowDeposit(scope, "example", EscrowProfilePlaintextXML, now, "u", "", "k/deposit.xml.gz", testSHA, 10, "", "", 0)
		require.NoError(t, err)
		assert.Empty(t, p.SignatureObjectKey)
		assert.Empty(t, p.SignatureSHA256)
		assert.Zero(t, p.SignatureBytes)
	})

	tests := []struct {
		name string
		fn   func() (*EscrowDeposit, error)
		want error
	}{
		{"bad scope", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(OperatorID(""), "example", EscrowProfileRydeSig, now, "u", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"bad tld", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "bad tld!", EscrowProfileRydeSig, now, "u", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"unknown profile", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", "ryde", now, "u", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrUnknownEscrowProfile},
		{"zero receivedAt", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, time.Time{}, "u", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"no submitter", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, now, " ", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"missing artifact key", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, now, "u", "", "", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"signed profile without signature", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, now, "u", "", "a", testSHA, 1, "", "", 0)
		}, ErrInvalidEscrowDeposit},
		{"unsigned profile with signature", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfilePlaintextXML, now, "u", "", "a", testSHA, 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
		{"bad digest", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, now, "u", "", "a", "ABC", 1, "b", testSHA, 1)
		}, ErrInvalidEscrowDigest},
		{"zero size", func() (*EscrowDeposit, error) {
			return NewEscrowDeposit(scope, "example", EscrowProfileRydeSig, now, "u", "", "a", testSHA, 0, "b", testSHA, 1)
		}, ErrInvalidEscrowDeposit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn()
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v", err)
		})
	}
}

func TestIsSHA256Hex(t *testing.T) {
	assert.True(t, IsSHA256Hex(testSHA))
	assert.False(t, IsSHA256Hex(testSHA[:63]))
	assert.False(t, IsSHA256Hex("0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"), "upper case rejected")
}

func TestEscrowValidationRun_Lifecycle(t *testing.T) {
	scope := testScope(t)
	started := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	_, err := NewEscrowValidationRun(uuid.Nil, scope, "example", "wf", "run", EscrowProfileRydeSig, started)
	assert.True(t, errors.Is(err, ErrInvalidEscrowValidationRun))
	_, err = NewEscrowValidationRun(uuid.New(), scope, "example", "", "run", EscrowProfileRydeSig, started)
	assert.True(t, errors.Is(err, ErrInvalidEscrowValidationRun))

	r, err := NewEscrowValidationRun(uuid.New(), scope, "EXAMPLE", "wf", "run", EscrowProfileRydeSig, started)
	require.NoError(t, err)
	assert.Equal(t, EscrowValidationRunning, r.Outcome)
	assert.False(t, r.IsFinal())
	assert.False(t, r.Verified())
	assert.Equal(t, "example", r.TLD)

	// Outcome / notification pairing is enforced.
	assert.True(t, errors.Is(r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationPass, NotificationStatus: EscrowNotificationDVFN, CompletedAt: started}), ErrInvalidEscrowValidationRun))
	assert.True(t, errors.Is(r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationFail, NotificationStatus: EscrowNotificationNone, CompletedAt: started}), ErrInvalidEscrowValidationRun))
	assert.True(t, errors.Is(r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationError, NotificationStatus: EscrowNotificationDVFN, CompletedAt: started}), ErrInvalidEscrowValidationRun))
	assert.True(t, errors.Is(r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationRunning, CompletedAt: started}), ErrInvalidEscrowValidationRun))
	assert.True(t, errors.Is(r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationPass, NotificationStatus: EscrowNotificationDVPN, CompletedAt: started.Add(-time.Second)}), ErrInvalidEscrowValidationRun))
	assert.False(t, r.IsFinal(), "rejected finalisations leave the run RUNNING")

	wm := started.Add(-time.Hour)
	require.NoError(t, r.Finalize(EscrowValidationFinalization{
		Outcome:            EscrowValidationPass,
		StageReached:       "rde",
		Findings:           []EscrowFinding{{Code: "X", Severity: "INFO", Stage: "rde", Message: "m", At: started}, {Code: "X", Severity: "INFO", Stage: "rde", Message: "m", At: started}, {Code: "Y", Severity: "INFO", Stage: "rde", Message: "m", At: started}},
		NotificationStatus: EscrowNotificationDVPN,
		RDEWatermark:       &wm,
		CompletedAt:        started.Add(time.Minute),
	}))
	assert.True(t, r.IsFinal())
	assert.True(t, r.Verified())
	assert.Equal(t, []string{"X", "Y"}, r.FindingCodes())
	require.NotNil(t, r.CompletedAt)

	// Second finalisation is rejected and changes nothing.
	err = r.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationFail, NotificationStatus: EscrowNotificationDVFN, CompletedAt: started.Add(time.Hour)})
	assert.True(t, errors.Is(err, ErrEscrowValidationRunAlreadyFinal))
	assert.Equal(t, EscrowValidationPass, r.Outcome)

	// Only the signed+encrypted profile is a verified pass, and only it may
	// carry an ICANN notification.
	other, err := NewEscrowValidationRun(uuid.New(), scope, "example", "wf", "run", EscrowProfilePlaintextXML, started)
	require.NoError(t, err)
	assert.True(t, errors.Is(
		other.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationPass, NotificationStatus: EscrowNotificationDVPN, CompletedAt: started}),
		ErrInvalidEscrowValidationRun), "an unsigned profile must not emit a DVPN")
	require.NoError(t, other.Finalize(EscrowValidationFinalization{Outcome: EscrowValidationPass, NotificationStatus: EscrowNotificationNone, CompletedAt: started}))
	assert.False(t, other.Verified())

	// An unknown profile is not a run at all.
	_, err = NewEscrowValidationRun(uuid.New(), scope, "example", "wf", "run", "ryde", started)
	assert.True(t, errors.Is(err, ErrUnknownEscrowProfile))
}

func TestEscrowTrustedKey(t *testing.T) {
	scope := testScope(t)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)

	k, err := NewEscrowTrustedKey(scope, "example", "0123 4567 89ab cdef 0123 4567 89ab cdef 0123 4567", testKey, " reg key ", from, &to)
	require.NoError(t, err)
	assert.Equal(t, testFP, k.Fingerprint, "fingerprint normalised to upper-case, no spaces")
	assert.Equal(t, "reg key", k.Label)

	assert.False(t, k.Active(from.Add(-time.Second)))
	assert.True(t, k.Active(from))
	assert.True(t, k.Active(to.Add(-time.Second)))
	assert.False(t, k.Active(to), "validTo is exclusive")

	openEnded, err := NewEscrowTrustedKey(scope, "example", testFP, testKey, "", from, nil)
	require.NoError(t, err)
	assert.True(t, openEnded.Active(from.AddDate(10, 0, 0)))

	retireAt := from.AddDate(0, 6, 0)
	require.NoError(t, openEnded.Retire(retireAt))
	assert.True(t, openEnded.Active(retireAt.Add(-time.Second)))
	assert.False(t, openEnded.Active(retireAt))
	assert.True(t, errors.Is(openEnded.Retire(retireAt), ErrEscrowTrustedKeyAlreadyRetired))

	bad := []struct {
		name string
		fn   func() error
		want error
	}{
		{"fingerprint", func() error { _, e := NewEscrowTrustedKey(scope, "example", "abc", testKey, "", from, nil); return e }, ErrEscrowTrustedKeyInvalidFingerprint},
		{"not armored", func() error { _, e := NewEscrowTrustedKey(scope, "example", testFP, "nope", "", from, nil); return e }, ErrInvalidEscrowTrustedKey},
		{"window", func() error {
			_, e := NewEscrowTrustedKey(scope, "example", testFP, testKey, "", from, &from)
			return e
		}, ErrEscrowTrustedKeyInvalidWindow},
		{"scope", func() error {
			_, e := NewEscrowTrustedKey(OperatorID(""), "example", testFP, testKey, "", from, nil)
			return e
		}, ErrInvalidEscrowTrustedKey},
		{"zero from", func() error {
			_, e := NewEscrowTrustedKey(scope, "example", testFP, testKey, "", time.Time{}, nil)
			return e
		}, ErrInvalidEscrowTrustedKey},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v", err)
		})
	}
}
