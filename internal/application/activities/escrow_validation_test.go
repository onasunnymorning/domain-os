package activities

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/internal/application/rdereport"
	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

// evFixture is one wired-up EVE test world: keys, stores, repos, activities.
type evFixture struct {
	scope    entities.OperatorID
	tld      string
	service  rdetest.KeyPair
	registry rdetest.KeyPair
	store    *fakeStore
	reports  *fakeStore
	deposits *fakeDepositRepo
	runs     *fakeRunRepo
	keys     *fakeKeyRepo
	keyRepo  repositories.EscrowTrustedKeyRepository
	prov     *fakeKeyProvider
	trusted  bool
	acts     *EscrowValidationActivities
	env      *testsuite.TestActivityEnvironment
}

func newEVFixture(t *testing.T, tldRepo repositories.TLDRepository, depRepo repositories.EscrowDepositRepository, runRepo repositories.EscrowValidationRunRepository, keyRepo repositories.EscrowTrustedKeyRepository) *evFixture {
	t.Helper()
	scope, err := entities.NewOperatorID("ryop1")
	require.NoError(t, err)
	f := &evFixture{
		scope:    scope,
		tld:      "example",
		service:  rdetest.NewKeyPair(t, "eve-service"),
		registry: rdetest.NewKeyPair(t, "registry-signer"),
		store:    newFakeStore(),
		reports:  newFakeStore(),
	}
	if depRepo == nil {
		f.deposits = newFakeDepositRepo()
		depRepo = f.deposits
	}
	if runRepo == nil {
		f.runs = newFakeRunRepo()
		runRepo = f.runs
	}
	if keyRepo == nil {
		f.keys = &fakeKeyRepo{}
		keyRepo = f.keys
	}
	if tldRepo == nil {
		tldRepo = &fakeTLDRepo{owned: map[string]string{"example": "ryop1"}}
	}
	f.prov = &fakeKeyProvider{ring: openpgp.EntityList{f.service.Entity}}
	f.keyRepo = keyRepo
	f.acts = NewEscrowValidationActivitiesWithDeps(tldRepo, depRepo, runRepo, keyRepo, f.prov, f.store, f.reports, rdevalidate.DefaultLimits(), "domain-os EVE (test)")
	f.acts.now = func() time.Time { return time.Now().UTC().Add(time.Hour) } // after key creation
	var ts testsuite.WorkflowTestSuite
	f.env = ts.NewTestActivityEnvironment()
	f.env.RegisterActivity(f.acts)
	return f
}

func (f *evFixture) upload(t *testing.T, name string, pair rdetest.Pair) (rydeKey, sigKey string) {
	t.Helper()
	rydeKey, sigKey = "uploads/"+name+".ryde", "uploads/"+name+".sig"
	f.store.put(rydeKey, pair.Ryde)
	f.store.put(sigKey, pair.Sig)
	return
}

// trust registers the registry signing key for the fixture's scope/TLD (idempotent).
func (f *evFixture) trust(t *testing.T) {
	t.Helper()
	if f.trusted {
		return
	}
	trusted, err := entities.NewEscrowTrustedKey(f.scope, f.tld, f.registry.Fingerprint, f.registry.ArmoredPublic, "registry", time.Now().UTC().Add(-time.Hour), nil)
	require.NoError(t, err)
	require.NoError(t, f.keyRepo.Create(t.Context(), trusted))
	f.trusted = true
}

func (f *evFixture) bind(t *testing.T, rydeKey, sigKey string) (BindDepositOutput, error) {
	t.Helper()
	f.trust(t)
	var out BindDepositOutput
	val, err := f.env.ExecuteActivity(f.acts.BindDeposit, BindDepositInput{
		Scope: f.scope.String(), TLD: f.tld, ArtifactObjectKey: rydeKey, SignatureObjectKey: sigKey,
		SubmittedBy: "tester", IntakeRef: "ref-1", ReceivedAt: time.Now().UTC(), WorkflowID: "wf-1", RunID: "run-1",
	})
	if err != nil {
		return out, err
	}
	require.NoError(t, val.Get(&out))
	return out, nil
}

func (f *evFixture) validate(t *testing.T, b BindDepositOutput) rdevalidate.Result {
	t.Helper()
	var res rdevalidate.Result
	val, err := f.env.ExecuteActivity(f.acts.ValidateArtifacts, ValidateArtifactsInput{
		Scope: f.scope.String(), TLD: f.tld, DepositID: b.DepositID, ValidationRunID: b.ValidationRunID, WorkflowID: "wf-1",
		Profile: b.Profile, ArtifactKey: b.ArtifactKey, SignatureKey: b.SignatureKey, ArtifactSHA256: b.ArtifactSHA256, SignatureSHA256: b.SignatureSHA256,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&res))
	return res
}

func (f *evFixture) emit(t *testing.T, b BindDepositOutput, res rdevalidate.Result) (EmitReportOutput, error) {
	t.Helper()
	var out EmitReportOutput
	val, err := f.env.ExecuteActivity(f.acts.EmitReportAndNotification, EmitReportInput{
		Scope: f.scope.String(), TLD: f.tld, Profile: b.Profile,
		DepositID: b.DepositID, ValidationRunID: b.ValidationRunID, WorkflowID: "wf-1", TemporalRunID: "temporal-run-1",
		Result: res, ReceivedAt: time.Now().UTC().Add(-time.Minute), ValidatedAt: time.Now().UTC(), Hints: b.Hints,
	})
	if err != nil {
		return out, err
	}
	require.NoError(t, val.Get(&out))
	return out, nil
}

func (f *evFixture) finalize(t *testing.T, b BindDepositOutput, res rdevalidate.Result, emitted EmitReportOutput, failure string) error {
	t.Helper()
	_, err := f.env.ExecuteActivity(f.acts.FinalizeValidationRun, FinalizeRunInput{
		Scope: f.scope.String(), ValidationRunID: b.ValidationRunID, WorkflowID: "wf-1", Result: res,
		SummaryKey: emitted.SummaryKey, ReportKey: emitted.ReportKey,
		NotificationKey: emitted.NotificationKey, NotificationStatus: emitted.NotificationStatus,
		CompletedAt: time.Now().UTC().Add(2 * time.Hour), Failure: failure,
	})
	return err
}

func isNonRetryable(err error) bool {
	var appErr *temporal.ApplicationError
	return errors.As(err, &appErr) && appErr.NonRetryable()
}

func xmllintOK(t *testing.T, doc []byte) {
	t.Helper()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI")
		}
		t.Skip("xmllint not installed")
	}
	p := filepath.Join(t.TempDir(), "doc.xml")
	require.NoError(t, os.WriteFile(p, doc, 0o600))
	out, err := exec.Command(xmllint, "--noout", "--schema", rdeschema.Path(rdeschema.ReportSchemas), p).CombinedOutput() //nolint:gosec // binary path comes from exec.LookPath, arguments are fixed
	require.NoError(t, err, "%s", out)
}

// bindPlaintext binds an unsigned .xml / .xml.gz deposit: no signature key,
// and the trusted-key registration the signed path needs is never consulted.
func (f *evFixture) bindPlaintext(t *testing.T, artifactKey string) (BindDepositOutput, error) {
	t.Helper()
	var out BindDepositOutput
	val, err := f.env.ExecuteActivity(f.acts.BindDeposit, BindDepositInput{
		Scope: f.scope.String(), TLD: f.tld, Profile: entities.EscrowProfilePlaintextXML,
		ArtifactObjectKey: artifactKey, SubmittedBy: "tester", IntakeRef: "ref-plain",
		ReceivedAt: time.Now().UTC(), WorkflowID: "wf-1", RunID: "run-1",
	})
	if err != nil {
		return out, err
	}
	require.NoError(t, val.Get(&out))
	return out, nil
}

func TestEscrowValidationActivities_PlaintextProfileEndToEnd(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	// No trusted key is registered and the key provider is emptied: an unsigned
	// deposit must not need either.
	f.prov.ring = nil
	opts := rdetest.DepositOpts{TLD: "example", Layout: rdetest.LayoutGzip, Domains: 3, Contacts: 2, Hosts: 1, Registrars: 1}
	artifact := rdetest.BuildPayload(t, opts, rdetest.BuildXML(opts))
	f.store.put("uploads/example.xml.gz", artifact)

	b, err := f.bindPlaintext(t, "uploads/example.xml.gz")
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowProfilePlaintextXML, b.Profile)
	assert.Empty(t, b.SignatureKey)
	assert.Empty(t, b.SignatureSHA256)
	assert.True(t, strings.HasSuffix(b.ArtifactKey, "/deposit.xml.gz"), "archived name reflects the artifact: %s", b.ArtifactKey)
	archived, ok := f.store.get(b.ArtifactKey)
	require.True(t, ok)
	assert.Equal(t, artifact, archived)

	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.False(t, res.Verified(), "an unsigned deposit is never a verified pass")

	// An unsigned deposit still gets both of the artifacts that are ours: the
	// summary, always, and the rdeReport, because the run reached a decision.
	// What it does not get is a notification — that is the ICANN claim only a
	// signature can support.
	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	assert.NotEmpty(t, emitted.SummaryKey)
	assert.NotEmpty(t, emitted.ReportKey)
	assert.Empty(t, emitted.NotificationKey, "an unsigned deposit claims nothing to ICANN")
	assert.Empty(t, emitted.NotificationStatus)
	assert.ElementsMatch(t, []string{emitted.SummaryKey, emitted.ReportKey}, f.reports.keys())

	raw, ok := f.reports.get(emitted.SummaryKey)
	require.True(t, ok)
	var summary rdereport.Summary
	require.NoError(t, json.Unmarshal(raw, &summary))
	assert.Equal(t, rdereport.SummarySchemaVersion, summary.SchemaVersion)
	assert.Equal(t, "PASS", summary.Outcome)
	assert.False(t, summary.Verified, "an unsigned deposit is never a verified pass")
	assert.Equal(t, entities.EscrowProfilePlaintextXML, summary.Profile)
	assert.Equal(t, b.ValidationRunID.String(), summary.ValidationRunID)
	assert.Equal(t, "wf-1", summary.CorrelationID)
	assert.Equal(t, "temporal-run-1", summary.TraceID)
	assert.Equal(t, emitted.ReportKey, summary.Artifacts["report"])
	assert.NotContains(t, summary.Artifacts, "notification")
	// The deposit's declared counts are reconciled against what was observed.
	require.NotEmpty(t, summary.Deposit.Counts)
	for _, c := range summary.Deposit.Counts {
		assert.True(t, c.Matches, "declared and observed disagree for %s", c.URI)
	}

	// PASS on an unsigned profile finalises with no notification at all.
	require.NoError(t, f.finalize(t, b, res, emitted, ""))
	run, err := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowValidationPass, run.Outcome)
	assert.Equal(t, entities.EscrowNotificationNone, run.NotificationStatus)
	assert.False(t, run.Verified())
	assert.Equal(t, emitted.SummaryKey, run.SummaryObjectKey)
	assert.Equal(t, emitted.ReportKey, run.ReportObjectKey)
}

// TestEscrowValidationActivities_SummaryCountsSuppressedFindings is the case
// that sent us here: a deposit whose objects are rejected in their tens of
// thousands. The run record keeps MaxFindings of them, so the summary's tally
// is the only exact account of what the deposit actually contained.
func TestEscrowValidationActivities_SummaryCountsSuppressedFindings(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	f.prov.ring = nil

	var res rdevalidate.Result
	res.Profile = rdevalidate.ProfilePlaintextXML
	res.StageReached = rdevalidate.StageRDE
	res.Deposit = rdevalidate.DepositSummary{ID: "20260908001", Kind: "FULL", Watermark: time.Now().UTC().Add(-time.Hour), HeaderFound: true}
	const rejected = rdevalidate.MaxFindings + 4242
	for i := 0; i < rejected; i++ {
		res.Add(rdevalidate.Finding{Code: rdevalidate.CodeRDEObjectEntityRejected, Severity: rdevalidate.SeverityWarning, Stage: rdevalidate.StageRDE})
	}
	res.Outcome = rdevalidate.OutcomePass
	res.Tally = []rdevalidate.FindingTally{{
		Code: rdevalidate.CodeRDEObjectEntityRejected, Severity: rdevalidate.SeverityWarning,
		Stage: rdevalidate.StageRDE, Count: rejected,
	}}

	opts := rdetest.DepositOpts{TLD: "example", Layout: rdetest.LayoutGzip}
	f.store.put("uploads/example.xml.gz", rdetest.BuildPayload(t, opts, rdetest.BuildXML(opts)))
	b, err := f.bindPlaintext(t, "uploads/example.xml.gz")
	require.NoError(t, err)

	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	raw, ok := f.reports.get(emitted.SummaryKey)
	require.True(t, ok)
	var summary rdereport.Summary
	require.NoError(t, json.Unmarshal(raw, &summary))

	assert.Equal(t, rejected, summary.Findings.Total, "the tally counts every finding, not the retained ones")
	assert.Equal(t, rdevalidate.MaxFindings, summary.Findings.Retained)
	assert.Equal(t, rejected-rdevalidate.MaxFindings, summary.Findings.Suppressed)
	require.Len(t, summary.Findings.ByCode, 1)
	assert.Equal(t, string(rdevalidate.CodeRDEObjectEntityRejected), summary.Findings.ByCode[0].Code)
	assert.Equal(t, rejected, summary.Findings.ByCode[0].Count)
	assert.False(t, summary.Findings.ByCode[0].ErrorClass)
	assert.Equal(t, rejected, summary.Findings.BySeverity["WARNING"])
	assert.LessOrEqual(t, len(summary.Findings.Sample), rdereport.MaxSummarySampleFindings)
}

func TestEscrowValidationActivities_PlaintextProfileRejectsSignatureKey(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	f.store.put("uploads/a.xml", []byte("<x/>"))
	f.store.put("uploads/a.sig", []byte("sig"))

	_, err := f.env.ExecuteActivity(f.acts.BindDeposit, BindDepositInput{
		Scope: f.scope.String(), TLD: f.tld, Profile: entities.EscrowProfilePlaintextXML,
		ArtifactObjectKey: "uploads/a.xml", SignatureObjectKey: "uploads/a.sig",
		SubmittedBy: "tester", ReceivedAt: time.Now().UTC(), WorkflowID: "wf-1", RunID: "run-1",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(err), "a mismatched artifact set is a caller error, not a retryable one")
}

func TestEscrowValidationActivities_PassEndToEnd(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Domains: 4, Contacts: 2, Hosts: 2, Registrars: 1}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "example_2026-09-08_full_S1_R0", pair)

	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	assert.False(t, b.Replay)
	assert.Equal(t, "20260908001", b.Hints.DepositID)
	assert.True(t, strings.HasPrefix(b.ArtifactKey, "escrow-validation/ryop1/example/"+b.DepositID.String()+"/"))
	archived, ok := f.store.get(b.ArtifactKey)
	require.True(t, ok)
	assert.Equal(t, pair.Ryde, archived, "artifact archived byte-for-byte")
	run, err := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowValidationRunning, run.Outcome)

	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.True(t, res.Verified())
	assert.Equal(t, 2, f.store.reads[b.ArtifactKey], "ciphertext streamed exactly twice: verify, then decrypt")

	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	assert.Equal(t, "DVPN", emitted.NotificationStatus)
	notif, ok := f.reports.get(emitted.NotificationKey)
	require.True(t, ok)
	assert.Contains(t, string(notif), "<rdeNotification:status>DVPN</rdeNotification:status>")
	xmllintOK(t, notif)
	report, ok := f.reports.get(emitted.ReportKey)
	require.True(t, ok)
	xmllintOK(t, report)

	require.NoError(t, f.finalize(t, b, res, emitted, ""))
	final, err := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowValidationPass, final.Outcome)
	assert.True(t, final.Verified())
	assert.Equal(t, entities.EscrowNotificationDVPN, final.NotificationStatus)
	assert.Equal(t, f.registry.Fingerprint, final.SigningKeyFingerprint)
	assert.Equal(t, f.service.Fingerprint, final.DecryptionKeyFingerprint)
	assert.Equal(t, "20260908001", final.RDEDepositID)
	assert.Equal(t, emitted.ReportKey, final.ReportObjectKey)
	require.NotNil(t, final.RDEWatermark)

	// Finalising again never overwrites the first outcome.
	err = f.finalize(t, b, rdevalidate.Result{Outcome: rdevalidate.OutcomeFail}, EmitReportOutput{NotificationStatus: "DVFN"}, "")
	require.Error(t, err)
	assert.True(t, isNonRetryable(err))
	again, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	assert.Equal(t, entities.EscrowValidationPass, again.Outcome)

	// Replay: same pair binds to the same deposit with a new run; artifacts untouched.
	b2, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	assert.True(t, b2.Replay)
	assert.Equal(t, b.DepositID, b2.DepositID)
	assert.NotEqual(t, b.ValidationRunID, b2.ValidationRunID)
	runs, _ := f.runs.ListByDeposit(t.Context(), f.scope, b.DepositID)
	assert.Len(t, runs, 2)
}

func TestEscrowValidationActivities_BadSignatureIsDVFN(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	pair.Sig = rdetest.Sign(t, []byte("not the deposit"), f.registry, false)
	rydeKey, sigKey := f.upload(t, "bad", pair)

	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomeFail, res.Outcome)
	assert.Contains(t, res.Codes(), rdevalidate.CodeSigInvalid)
	assert.Empty(t, res.Digests.PlaintextSHA256, "never decrypted")

	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	assert.Equal(t, "DVFN", emitted.NotificationStatus)
	notif, _ := f.reports.get(emitted.NotificationKey)
	assert.Contains(t, string(notif), `code="4103"`)
	xmllintOK(t, notif)
	require.NoError(t, f.finalize(t, b, res, emitted, ""))
	final, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	assert.Equal(t, entities.EscrowValidationFail, final.Outcome)
	assert.Equal(t, []string{string(rdevalidate.CodeSigInvalid)}, final.FindingCodes())
}

func TestEscrowValidationActivities_TenantCannotBindForeignTLD(t *testing.T) {
	f := newEVFixture(t, &fakeTLDRepo{owned: map[string]string{"example": "someoneelse"}}, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "foreign", pair)
	_, err := f.bind(t, rydeKey, sigKey)
	require.Error(t, err)
	assert.True(t, isNonRetryable(err))
	assert.Empty(t, f.deposits.rows, "no record is created")
	assert.Empty(t, f.runs.rows)
}

func TestEscrowValidationActivities_MissingArtifact(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	f.store.put("uploads/only.ryde", pair.Ryde)
	_, err := f.bind(t, "uploads/only.ryde", "uploads/missing.sig")
	require.Error(t, err)
	assert.True(t, isNonRetryable(err))
	assert.Contains(t, err.Error(), string(rdevalidate.CodeIntakeArtifactMissing))
}

func TestEscrowValidationActivities_KeyUnavailableIsErrorNotDVFN(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "nokey", pair)
	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	f.prov.err = errors.New("secrets service unreachable")
	res := f.validate(t, b)
	assert.Equal(t, rdevalidate.OutcomeError, res.Outcome)
	assert.Contains(t, res.Codes(), rdevalidate.CodeDecryptKeyUnavailable)

	// An undecided run claims nothing — no rdeReport, no DVFN — but it still
	// gets the summary, which is where the reason code is written down.
	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	assert.NotEmpty(t, emitted.SummaryKey)
	assert.Empty(t, emitted.ReportKey, "an undecided run states no counts")
	assert.Empty(t, emitted.NotificationKey, "an undecided run claims nothing to ICANN")
	assert.Empty(t, emitted.NotificationStatus)
	assert.Equal(t, []string{emitted.SummaryKey}, f.reports.keys())

	require.NoError(t, f.finalize(t, b, res, emitted, ""))
	final, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	assert.Equal(t, entities.EscrowValidationError, final.Outcome)
	assert.Equal(t, entities.EscrowNotificationNone, final.NotificationStatus)
	assert.Equal(t, emitted.SummaryKey, final.SummaryObjectKey)
}

func TestEscrowValidationActivities_FinalizeFailurePath(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "fail", pair)
	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	require.NoError(t, f.finalize(t, b, rdevalidate.Result{}, EmitReportOutput{}, "activity ValidateArtifacts failed"))
	final, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	assert.Equal(t, entities.EscrowValidationError, final.Outcome)
	assert.Equal(t, []string{string(rdevalidate.CodeInternal)}, final.FindingCodes())
}

func TestEscrowValidationActivities_NoSensitiveDataPersisted(t *testing.T) {
	// Canaries in a tar entry name, a contact org and the upload key: none may
	// reach the run row, the result payload or the emitted documents.
	const canary = "CANARYxq9"
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Canary: canary, EntryName: canary + ".xml", BreakDomain: true}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, canary, pair)
	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomeFail, res.Outcome)
	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	require.NoError(t, f.finalize(t, b, res, emitted, ""))

	resJSON, _ := json.Marshal(res)
	assert.NotContains(t, string(resJSON), canary)
	final, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	runJSON, _ := json.Marshal(final)
	assert.NotContains(t, string(runJSON), canary)
	for _, k := range f.reports.keys() {
		doc, _ := f.reports.get(k)
		assert.NotContains(t, string(doc), canary, "document %s", k)
		assert.NotContains(t, k, canary)
	}
	dep, _ := f.deposits.GetByID(t.Context(), f.scope, b.DepositID)
	assert.NotContains(t, dep.ArtifactObjectKey, canary, "archived key is deterministic, not the upload name")
}

func TestLoadEscrowValidationLimits(t *testing.T) {
	t.Setenv("ESCROW_VALIDATION_MAX_FILES", "3")
	t.Setenv("ESCROW_VALIDATION_TIMEOUT", "45m")
	lim, err := loadEscrowValidationLimits()
	require.NoError(t, err)
	assert.Equal(t, 3, lim.MaxFiles)
	assert.Equal(t, 45*time.Minute, lim.Timeout)
	assert.Equal(t, rdevalidate.DefaultLimits().MaxUnpackedBytes, lim.MaxUnpackedBytes)

	t.Setenv("ESCROW_VALIDATION_MAX_UNPACKED_BYTES", "lots")
	_, err = loadEscrowValidationLimits()
	assert.Error(t, err)
	t.Setenv("ESCROW_VALIDATION_MAX_UNPACKED_BYTES", "0")
	_, err = loadEscrowValidationLimits()
	assert.Error(t, err, "zero budget is rejected")
}

func TestEscrowValidationActivities_HoldNoDatabaseHandle(t *testing.T) {
	// Structural half of "validation is not import": the activities struct
	// holds escrow repositories only, never a raw database handle.
	rt := reflect.TypeOf(EscrowValidationActivities{})
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i).Type.String()
		assert.NotContains(t, ft, "gorm.DB", "field %s holds a database handle", rt.Field(i).Name)
		assert.NotContains(t, ft, "sql.DB", "field %s holds a database handle", rt.Field(i).Name)
	}
}
