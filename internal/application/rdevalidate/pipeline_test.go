package rdevalidate

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pipelineFixture struct {
	service, oldService, registry, stranger rdetest.KeyPair
}

func newPipelineFixture(t *testing.T) pipelineFixture {
	t.Helper()
	return pipelineFixture{
		service:    rdetest.NewKeyPair(t, "eve-service"),
		oldService: rdetest.NewKeyPair(t, "eve-service-previous"),
		registry:   rdetest.NewKeyPair(t, "registry-signer"),
		stranger:   rdetest.NewKeyPair(t, "stranger"),
	}
}

func openBytes(b []byte) func(context.Context) (io.ReadCloser, error) {
	return func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }
}

func (f pipelineFixture) input(ryde, sig []byte) Input {
	return Input{
		OpenRyde:    openBytes(ryde),
		Sig:         sig,
		TrustedKeys: []string{f.registry.ArmoredPublic},
		ServiceKeys: openpgp.EntityList{f.service.Entity, f.oldService.Entity},
		BoundTLD:    "example",
		Limits:      DefaultLimits(),
		Now:         func() time.Time { return testNow },
	}
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func TestRun_PassVerified(t *testing.T) {
	f := newPipelineFixture(t)
	opts := rdetest.DepositOpts{TLD: "example", Domains: 5, Contacts: 3, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 2}
	pair := rdetest.BuildPair(t, opts, f.service, f.registry)
	in := f.input(pair.Ryde, pair.Sig)
	in.ExpectedRydeSHA256 = sha(pair.Ryde)
	in.ExpectedSigSHA256 = sha(pair.Sig)
	var beats []Stage
	in.Heartbeat = func(s Stage, _ string) { beats = append(beats, s) }

	res := Run(context.Background(), in)

	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.True(t, res.Verified())
	assert.Empty(t, res.Findings)
	assert.Equal(t, StageRDE, res.StageReached)
	assert.Equal(t, LayoutTar, res.Layout)
	assert.Equal(t, f.registry.Fingerprint, res.Signature.KeyFingerprint)
	assert.Equal(t, f.service.Fingerprint, res.Decryption.KeyFingerprint)
	assert.Equal(t, sha(pair.Ryde), res.Digests.RydeSHA256)
	assert.Equal(t, sha(pair.Sig), res.Digests.SigSHA256)
	assert.Equal(t, sha(pair.Plaintext), res.Digests.PlaintextSHA256)
	assert.Equal(t, "20260908001", res.Deposit.ID)
	assert.Equal(t, "FULL", res.Deposit.Kind)
	assert.Equal(t, 5, res.Deposit.Observed[entities.DOMAIN_URI])
	assert.Equal(t, 5, res.Deposit.Header.DomainCount())
	assert.Equal(t, []Stage{StageSignature, StageDecrypt, StageUnpack, StageXML}, beats)
	assert.Equal(t, entities.EscrowNotificationDVPN, NotificationStatus(res.Outcome))

	// The result round-trips through JSON (it rides in Temporal payloads and the run row).
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	var back Result
	require.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, res.Outcome, back.Outcome)
	assert.Equal(t, res.Deposit.Observed, back.Deposit.Observed)
}

func TestRun_RolloverDecryptsWithPreviousKey(t *testing.T) {
	f := newPipelineFixture(t)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.oldService, f.registry)
	res := Run(context.Background(), f.input(pair.Ryde, pair.Sig))
	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.Equal(t, f.oldService.Fingerprint, res.Decryption.KeyFingerprint)
}

func TestRun_FailurePaths(t *testing.T) {
	f := newPipelineFixture(t)
	good := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)

	cases := []struct {
		name  string
		build func() Input
		code  Code
		stage Stage
	}{
		{"tampered deposit: signature invalid", func() Input {
			return f.input(rdetest.Tamper(good.Ryde, 64), good.Sig)
		}, CodeSigInvalid, StageSignature},
		{"mismatched signature (signed other bytes)", func() Input {
			other := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", ID: "other"}, f.service, f.registry)
			return f.input(good.Ryde, other.Sig)
		}, CodeSigInvalid, StageSignature},
		{"untrusted signer", func() Input {
			return f.input(good.Ryde, rdetest.Sign(t, good.Ryde, f.stranger, false))
		}, CodeSigKeyUntrusted, StageSignature},
		{"no trusted key registered", func() Input {
			in := f.input(good.Ryde, good.Sig)
			in.TrustedKeys = nil
			return in
		}, CodeSigKeyUntrusted, StageSignature},
		{"malformed signature", func() Input {
			return f.input(good.Ryde, []byte("garbage"))
		}, CodeSigMalformed, StageSignature},
		{"encrypted to a stranger", func() Input {
			plain := rdetest.BuildPayload(t, rdetest.DepositOpts{TLD: "example"}, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"}))
			ryde := rdetest.Encrypt(t, plain, f.stranger)
			return f.input(ryde, rdetest.Sign(t, ryde, f.registry, false))
		}, CodeDecryptNotForServiceKey, StageDecrypt},
		{"signed but not encrypted (plaintext passed off as a deposit)", func() Input {
			plain := rdetest.BuildPayload(t, rdetest.DepositOpts{TLD: "example"}, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"}))
			return f.input(plain, rdetest.Sign(t, plain, f.registry, false))
		}, CodeDecryptFailed, StageDecrypt},
		{"unsafe archive entry", func() Input {
			opts := rdetest.DepositOpts{TLD: "example", Before: []rdetest.TarEntry{{Name: "../../evil", Type: tar.TypeSymlink, Link: "/etc/passwd"}}}
			p := rdetest.BuildPair(t, opts, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeArchiveUnsafeEntry, StageUnpack},
		{"unpacked size limit", func() Input {
			opts := rdetest.DepositOpts{TLD: "example", Domains: 40, Contacts: 40, Hosts: 40, Registrars: 1}
			p := rdetest.BuildPair(t, opts, f.service, f.registry)
			in := f.input(p.Ryde, p.Sig)
			in.Limits.MaxUnpackedBytes = 4096
			return in
		}, CodeArchiveLimitUnpackedSize, StageUnpack},
		{"compressed size limit", func() Input {
			in := f.input(good.Ryde, good.Sig)
			in.Limits.MaxCompressedBytes = int64(len(good.Ryde) - 1)
			return in
		}, CodeIntakeLimitCompressedSize, StageIntake},
		{"nesting limit", func() Input {
			opts := rdetest.DepositOpts{TLD: "example", EntryGzip: true, GzipWraps: 2}
			p := rdetest.BuildPair(t, opts, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeArchiveLimitNesting, StageUnpack},
		{"two payloads (split deposit)", func() Input {
			xml := rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"})
			opts := rdetest.DepositOpts{TLD: "example", After: []rdetest.TarEntry{{Name: "part2.xml", Type: tar.TypeReg, Body: xml}}}
			p := rdetest.BuildPair(t, opts, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeArchiveUnsupportedLayout, StageUnpack},
		{"malformed XML", func() Input {
			p := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Malformed: true}, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeXMLMalformed, StageXML},
		{"invalid required RDE object", func() Input {
			p := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", BreakDomain: true}, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeRDEObjectInvalid, StageRDE},
		{"header count mismatch", func() Input {
			opts := rdetest.DepositOpts{TLD: "example", Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1, HeaderCounts: map[string]int{entities.DOMAIN_URI: 9, entities.CONTACT_URI: 1, entities.HOST_URI: 1, entities.REGISTRAR_URI: 1}}
			p := rdetest.BuildPair(t, opts, f.service, f.registry)
			return f.input(p.Ryde, p.Sig)
		}, CodeRDECountMismatch, StageRDE},
		{"header tld differs from bound tld", func() Input {
			p := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "other"}, f.service, f.registry)
			return f.input(p.Ryde, p.Sig) // bound to "example"
		}, CodeRDEHeaderTLDMismatch, StageRDE},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := Run(context.Background(), c.build())
			require.Equal(t, OutcomeFail, res.Outcome, "findings: %v", res.Findings)
			assert.False(t, res.Verified())
			assert.Contains(t, res.Codes(), c.code)
			assert.Equal(t, entities.EscrowNotificationDVFN, NotificationStatus(res.Outcome))
			// Fail closed: a signature failure never reaches decryption.
			if c.stage == StageSignature {
				assert.Equal(t, StageSignature, res.StageReached)
				assert.Empty(t, res.Decryption.KeyFingerprint)
				assert.Empty(t, res.Digests.PlaintextSHA256)
			}
			for _, fd := range res.Findings {
				assert.NotContains(t, fd.Message, "evil")
				assert.NotContains(t, fd.Message, "passwd")
				assert.NotContains(t, fd.Locator, "evil")
			}
		})
	}
}

func TestRun_ErrorClass(t *testing.T) {
	f := newPipelineFixture(t)
	good := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)

	t.Run("service key unavailable is ERROR, not DVFN", func(t *testing.T) {
		in := f.input(good.Ryde, good.Sig)
		in.ServiceKeys = nil
		res := Run(context.Background(), in)
		assert.Equal(t, OutcomeError, res.Outcome)
		assert.Contains(t, res.Codes(), CodeDecryptKeyUnavailable)
		assert.Equal(t, entities.EscrowNotificationNone, NotificationStatus(res.Outcome))
	})
	t.Run("artifact swapped after intake is an internal error", func(t *testing.T) {
		in := f.input(good.Ryde, good.Sig)
		in.ExpectedRydeSHA256 = strings.Repeat("0", 64)
		res := Run(context.Background(), in)
		assert.Equal(t, OutcomeError, res.Outcome)
		assert.Contains(t, res.Codes(), CodeInternal)
	})
	t.Run("artifact cannot be opened", func(t *testing.T) {
		in := f.input(good.Ryde, good.Sig)
		in.OpenRyde = func(context.Context) (io.ReadCloser, error) { return nil, errors.New("s3 down") }
		res := Run(context.Background(), in)
		assert.Equal(t, OutcomeError, res.Outcome)
		assert.Contains(t, res.Codes(), CodeInternal)
	})
	t.Run("cancelled context is a timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		res := Run(ctx, f.input(good.Ryde, good.Sig))
		assert.Equal(t, OutcomeError, res.Outcome)
		assert.Contains(t, res.Codes(), CodeValidationTimeout)
	})
	t.Run("misconfigured limits", func(t *testing.T) {
		in := f.input(good.Ryde, good.Sig)
		in.Limits.MaxFiles = 0
		res := Run(context.Background(), in)
		assert.Equal(t, OutcomeError, res.Outcome)
	})
	t.Run("panic is contained", func(t *testing.T) {
		in := f.input(good.Ryde, good.Sig)
		in.Heartbeat = func(Stage, string) { panic("boom") }
		res := Run(context.Background(), in)
		assert.Equal(t, OutcomeError, res.Outcome)
		assert.Contains(t, res.Codes(), CodeInternal)
	})
}

func TestRun_NeverLeaksPayloadContent(t *testing.T) {
	// Canaries are planted in a tar entry name and a contact org; the run is
	// forced through unpack and rde findings, and nothing it emits may carry them.
	f := newPipelineFixture(t)
	const canary = "CANARYzq7"
	opts := rdetest.DepositOpts{TLD: "example", Canary: canary, EntryName: canary + ".xml", BreakDomain: true,
		After: []rdetest.TarEntry{{Name: canary + "-notes", Type: tar.TypeReg, Body: []byte(canary)}}}
	p := rdetest.BuildPair(t, opts, f.service, f.registry)
	res := Run(context.Background(), f.input(p.Ryde, p.Sig))
	require.Equal(t, OutcomeFail, res.Outcome)
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), canary)
	assert.NotContains(t, res.String(), canary)
}

func TestRun_ReplayIsDeterministic(t *testing.T) {
	f := newPipelineFixture(t)
	p := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	a := Run(context.Background(), f.input(p.Ryde, p.Sig))
	b := Run(context.Background(), f.input(p.Ryde, p.Sig))
	assert.Equal(t, a.Outcome, b.Outcome)
	assert.Equal(t, a.Digests, b.Digests)
	assert.Equal(t, a.Codes(), b.Codes())
}
