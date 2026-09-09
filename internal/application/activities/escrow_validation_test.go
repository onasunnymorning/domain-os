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
		Scope: f.scope.String(), TLD: f.tld, RydeObjectKey: rydeKey, SigObjectKey: sigKey,
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
		RydeKey: b.RydeKey, SigKey: b.SigKey, RydeSHA256: b.RydeSHA256, SigSHA256: b.SigSHA256,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&res))
	return res
}

func (f *evFixture) emit(t *testing.T, b BindDepositOutput, res rdevalidate.Result) (EmitReportOutput, error) {
	t.Helper()
	var out EmitReportOutput
	val, err := f.env.ExecuteActivity(f.acts.EmitReportAndNotification, EmitReportInput{
		Scope: f.scope.String(), TLD: f.tld, DepositID: b.DepositID, ValidationRunID: b.ValidationRunID, WorkflowID: "wf-1",
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
		ReportKey: emitted.ReportKey, NotificationKey: emitted.NotificationKey, NotificationStatus: emitted.NotificationStatus,
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
	require.NoError(t, os.WriteFile(p, doc, 0o644))
	out, err := exec.Command(xmllint, "--noout", "--schema", filepath.Join("..", "rdereport", "xsd", "eve-schemas.xsd"), p).CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func TestEscrowValidationActivities_PassEndToEnd(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Domains: 4, Contacts: 2, Hosts: 2, Registrars: 1}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "example_2026-09-08_full_S1_R0", pair)

	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	assert.False(t, b.Replay)
	assert.Equal(t, "20260908001", b.Hints.DepositID)
	assert.True(t, strings.HasPrefix(b.RydeKey, "escrow-validation/ryop1/example/"+b.DepositID.String()+"/"))
	archived, ok := f.store.get(b.RydeKey)
	require.True(t, ok)
	assert.Equal(t, pair.Ryde, archived, "artifact archived byte-for-byte")
	run, err := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowValidationRunning, run.Outcome)

	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.True(t, res.Verified())
	assert.Equal(t, 2, f.store.reads[b.RydeKey], "ciphertext streamed exactly twice: verify, then decrypt")

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

	_, err = f.emit(t, b, res)
	require.Error(t, err, "no notification for ERROR")
	assert.True(t, isNonRetryable(err))
	assert.Empty(t, f.reports.keys())

	require.NoError(t, f.finalize(t, b, res, EmitReportOutput{}, ""))
	final, _ := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	assert.Equal(t, entities.EscrowValidationError, final.Outcome)
	assert.Equal(t, entities.EscrowNotificationNone, final.NotificationStatus)
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
	assert.NotContains(t, dep.RydeObjectKey, canary, "archived key is deterministic, not the upload name")
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
