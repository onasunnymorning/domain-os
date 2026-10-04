package activities

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

// ---- in-memory intake store ----

type fakeIntakeStore struct {
	mu      sync.Mutex
	objects map[string]interfaces.ObjectInfo
	data    map[string]string
	// failRemove makes the next RemoveObject of this key fail once, to model a
	// crash between the copy and the delete of a move.
	failRemove map[string]bool
	listErr    error
}

func newFakeIntakeStore() *fakeIntakeStore {
	return &fakeIntakeStore{objects: map[string]interfaces.ObjectInfo{}, data: map[string]string{}, failRemove: map[string]bool{}}
}

func (f *fakeIntakeStore) put(key, body string, modified time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = interfaces.ObjectInfo{Key: key, Size: int64(len(body)), ETag: "etag-" + body, LastModified: modified}
	f.data[key] = body
}

func (f *fakeIntakeStore) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeIntakeStore) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data[key]
}

func (f *fakeIntakeStore) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *fakeIntakeStore) ListObjectsInfo(_ context.Context, prefix string, limit int) ([]interfaces.ObjectInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []interfaces.ObjectInfo
	for _, k := range f.keys() {
		if strings.HasPrefix(k, prefix) {
			f.mu.Lock()
			out = append(out, f.objects[k])
			f.mu.Unlock()
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeIntakeStore) Exists(_ context.Context, key string) (bool, error) { return f.has(key), nil }

func (f *fakeIntakeStore) CopyObject(_ context.Context, src, dst string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[src]
	if !ok {
		return errors.New("fakeIntakeStore: no such source")
	}
	o.Key = dst
	f.objects[dst] = o
	f.data[dst] = f.data[src]
	return nil
}

func (f *fakeIntakeStore) RemoveObject(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRemove[key] {
		delete(f.failRemove, key)
		return errors.New("fakeIntakeStore: injected remove failure")
	}
	delete(f.objects, key)
	delete(f.data, key)
	return nil
}

// ---- fixture ----

type intakeFixture struct {
	store *fakeIntakeStore
	runs  *fakeRunRepo
	acts  *EscrowIntakeActivities
	env   *testsuite.TestActivityEnvironment
	now   time.Time
}

func newIntakeFixture(t *testing.T, enabled bool) *intakeFixture {
	t.Helper()
	return newIntakeFixtureWith(t, EscrowIntakeConfig{Enabled: enabled, Prefix: defaultEscrowIntakePrefix})
}

func newIntakeFixtureWith(t *testing.T, cfg EscrowIntakeConfig) *intakeFixture {
	t.Helper()
	f := &intakeFixture{store: newFakeIntakeStore(), runs: newFakeRunRepo(), now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}
	acts, err := NewEscrowIntakeActivities(f.store, f.runs, cfg)
	require.NoError(t, err)
	f.acts = acts
	f.acts.now = func() time.Time { return f.now }
	var ts testsuite.WorkflowTestSuite
	f.env = ts.NewTestActivityEnvironment()
	f.env.RegisterActivity(f.acts)
	return f
}

func (f *intakeFixture) list(t *testing.T, maxPairs int) ListIntakePairsOutput {
	t.Helper()
	val, err := f.env.ExecuteActivity(f.acts.ListIntakePairs, ListIntakePairsInput{MaxPairs: maxPairs})
	require.NoError(t, err)
	var out ListIntakePairsOutput
	require.NoError(t, val.Get(&out))
	return out
}

func (f *intakeFixture) claim(pair IntakePair) (ClaimIntakeOutput, error) {
	val, err := f.env.ExecuteActivity(f.acts.ClaimIntakePair, ClaimIntakeInput{
		Scope: pair.Scope, TLD: pair.TLD, Profile: pair.Profile, IntakeID: pair.IntakeID,
		ArtifactKey: pair.ArtifactKey, SignatureKey: pair.SignatureKey,
	})
	if err != nil {
		return ClaimIntakeOutput{}, err
	}
	var out ClaimIntakeOutput
	err = val.Get(&out)
	return out, err
}

func (f *intakeFixture) settle(pair IntakePair, claimed ClaimIntakeOutput, validationWorkflowID string) (SettleIntakeOutput, error) {
	val, err := f.env.ExecuteActivity(f.acts.SettleIntakePair, SettleIntakeInput{
		Scope: pair.Scope, TLD: pair.TLD, Profile: pair.Profile, IntakeID: pair.IntakeID, ValidationWorkflowID: validationWorkflowID,
		ArtifactKey: claimed.ArtifactKey, SignatureKey: claimed.SignatureKey,
	})
	if err != nil {
		return SettleIntakeOutput{}, err
	}
	var out SettleIntakeOutput
	err = val.Get(&out)
	return out, err
}

const intakeInbox = "sftp/inbox/"

// ---- configuration ----

func TestNormalizeEscrowIntakePrefix(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		wantErr   bool
	}{
		{raw: "", want: "sftp/"},
		{raw: "  ", want: "sftp/"},
		{raw: "sftp", want: "sftp/"},
		{raw: "/sftp/", want: "sftp/"},
		{raw: "intake/sftp", want: "intake/sftp/"},
		{raw: "/", wantErr: true},
		{raw: "escrow-validation", wantErr: true},
		{raw: "escrow-validation/sftp", wantErr: true},
		{raw: "uploads/", wantErr: true},
		{raw: "uploads/sftp", wantErr: true},
		{raw: "escrow-validation-sftp", want: "escrow-validation-sftp/"}, // a sibling, not an overlap
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := normalizeEscrowIntakePrefix(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewEscrowIntakeActivities_Prefix(t *testing.T) {
	acts, err := NewEscrowIntakeActivities(newFakeIntakeStore(), newFakeRunRepo(), EscrowIntakeConfig{})
	require.NoError(t, err)
	assert.Equal(t, "sftp/", acts.cfg.Prefix, "an unset prefix is the default")

	acts, err = NewEscrowIntakeActivities(newFakeIntakeStore(), newFakeRunRepo(), EscrowIntakeConfig{Prefix: "/intake"})
	require.NoError(t, err)
	assert.Equal(t, "intake/", acts.cfg.Prefix)

	_, err = NewEscrowIntakeActivities(newFakeIntakeStore(), newFakeRunRepo(), EscrowIntakeConfig{Prefix: "uploads"})
	require.Error(t, err, "a prefix overlapping another area is refused at construction")
}

// ---- parsing and pairing ----

func TestParseEscrowIntakeKey(t *testing.T) {
	for _, tc := range []struct {
		key  string
		ok   bool
		want escrowIntakeKey
	}{
		{key: intakeInbox + "ryop1/example/example_2026-10-03_full_S1_R0.ryde", ok: true,
			want: escrowIntakeKey{ryID: "ryop1", tld: "example", base: "example_2026-10-03_full_S1_R0", ext: ".ryde"}},
		{key: intakeInbox + "ryop1/example/a.sig", ok: true, want: escrowIntakeKey{ryID: "ryop1", tld: "example", base: "a", ext: ".sig"}},
		{key: intakeInbox + "ryop1/example/", ok: false}, // directory marker
		{key: intakeInbox + "ryop1/example/a.xml", ok: true, want: escrowIntakeKey{ryID: "ryop1", tld: "example", base: "a", ext: ".xml"}},
		{key: intakeInbox + "ryop1/example/a.xml.gz", ok: true, want: escrowIntakeKey{ryID: "ryop1", tld: "example", base: "a", ext: ".xml.gz"}},
		{key: intakeInbox + "ryop1/example/a.gz", ok: false},       // a bare .gz is not a deposit
		{key: intakeInbox + "ryop1/example/a.txt", ok: false},      // other extension
		{key: intakeInbox + "ryop1/example/.ryde", ok: false},      // no base name
		{key: intakeInbox + "ryop1/example/sub/a.ryde", ok: false}, // deeper path
		{key: intakeInbox + "ryop1/a.ryde", ok: false},             // shallower path
		{key: intakeInbox + "ry/example/a.ryde", ok: false},        // RyID too short
		{key: intakeInbox + "ry_op1/example/a.ryde", ok: false},    // RyID outside the tenant charset
		{key: intakeInbox + "ryop1/exa mple/a.ryde", ok: false},    // not a TLD
		{key: "sftp/claimed/ryop1/example/x/a.ryde", ok: false},    // another area
		{key: "uploads/ryop1/example/a.ryde", ok: false},           // not intake at all
		{key: intakeInbox + "ryop1/EXAMPLE/a.ryde", ok: true, want: escrowIntakeKey{ryID: "ryop1", tld: "example", base: "a", ext: ".ryde"}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			got, ok := parseEscrowIntakeKey(intakeInbox, tc.key)
			require.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestPairEscrowIntakeObjects(t *testing.T) {
	t0 := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	obj := func(key, etag string, at time.Time) interfaces.ObjectInfo {
		return interfaces.ObjectInfo{Key: intakeInbox + key, ETag: etag, LastModified: at}
	}
	listing := pairEscrowIntakeObjects(intakeInbox, []interfaces.ObjectInfo{
		obj("ryop1/example/", "", t0),                            // marker
		obj("ryop1/example/b.ryde", "e1", t0.Add(2*time.Minute)), // pair b, newer
		obj("ryop1/example/b.sig", "e2", t0.Add(3*time.Minute)),
		obj("ryop1/example/a.sig", "e3", t0.Add(time.Minute)), // pair a, older
		obj("ryop1/example/a.ryde", "e4", t0),
		obj("ryop1/example/c.ryde", "e5", t0.Add(-time.Hour)), // no signature yet
		obj("ryop2/other/c.sig", "e6", t0),                    // same base, other tenant: no pair
		obj("ryop1/example/notes.txt", "e7", t0),              // ignored
	}, false)
	require.Len(t, listing.pairs, 2)
	assert.Equal(t, 1, listing.markers)
	assert.Equal(t, 1, listing.ignored)
	assert.Equal(t, 2, listing.unpaired)
	assert.Equal(t, t0.Add(-time.Hour), listing.oldestUnpaired)

	a, b := listing.pairs[0], listing.pairs[1]
	assert.Equal(t, intakeInbox+"ryop1/example/a.ryde", a.ArtifactKey, "oldest pair first")
	assert.Equal(t, intakeInbox+"ryop1/example/a.sig", a.SignatureKey)
	assert.Equal(t, t0.Add(time.Minute), a.ReceivedAt, "received when the later half arrived")
	assert.Equal(t, "ryop1", a.Scope)
	assert.Equal(t, "example", a.TLD)
	assert.Len(t, a.IntakeID, 16)
	assert.Equal(t, "escrow-intake-ryop1-example-"+a.IntakeID, a.WorkflowID)
	assert.Equal(t, intakeInbox+"ryop1/example/b.ryde", b.ArtifactKey)
	assert.NotEqual(t, a.IntakeID, b.IntakeID)
}

func TestEscrowIntakeID_IdentifiesTheUpload(t *testing.T) {
	t0 := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	art := interfaces.ObjectInfo{Key: intakeInbox + "ryop1/example/a.ryde", ETag: "e1", LastModified: t0}
	sig := interfaces.ObjectInfo{Key: intakeInbox + "ryop1/example/a.sig", ETag: "e2", LastModified: t0}
	id := escrowIntakeID(art, sig)
	assert.Equal(t, id, escrowIntakeID(art, sig), "the same upload must always get the same ID")

	reupload := art
	reupload.LastModified = t0.Add(time.Hour)
	assert.NotEqual(t, id, escrowIntakeID(reupload, sig), "an identical file uploaded again is a new intake")

	changed := art
	changed.ETag = "e9"
	assert.NotEqual(t, id, escrowIntakeID(changed, sig))
}

// ---- ListIntakePairs ----

func TestListIntakePairs_DisabledDoesNothing(t *testing.T) {
	f := newIntakeFixture(t, false)
	f.store.listErr = errors.New("must not be called")
	f.store.put(intakeInbox+"ryop1/example/a.ryde", "R", f.now)
	f.store.put(intakeInbox+"ryop1/example/a.sig", "S", f.now)
	out := f.list(t, 0)
	assert.False(t, out.Enabled)
	assert.Empty(t, out.Pairs)
}

func TestListIntakePairs_CapsAndCounts(t *testing.T) {
	f := newIntakeFixture(t, true)
	for i, base := range []string{"a", "b", "c"} {
		at := f.now.Add(time.Duration(i-10) * time.Minute)
		f.store.put(intakeInbox+"ryop1/example/"+base+".ryde", "R"+base, at)
		f.store.put(intakeInbox+"ryop1/example/"+base+".sig", "S"+base, at)
	}
	f.store.put(intakeInbox+"ryop1/example/lonely.ryde", "L", f.now.Add(-2*time.Hour))
	f.store.put(intakeInbox+"ryop1/example/", "", f.now)
	f.store.put("sftp/claimed/ryop1/example/x/old.ryde", "O", f.now) // other areas are not listed

	out := f.list(t, 2)
	assert.True(t, out.Enabled)
	require.Len(t, out.Pairs, 2)
	assert.Equal(t, intakeInbox+"ryop1/example/a.ryde", out.Pairs[0].ArtifactKey)
	assert.Equal(t, 1, out.Deferred)
	assert.Equal(t, 1, out.Unpaired)
	assert.Equal(t, 2*time.Hour, out.OldestUnpairedWaited)
	assert.Equal(t, 0, out.Ignored)
	assert.False(t, out.Truncated)
}

func TestListIntakePairs_ListingErrorIsRetried(t *testing.T) {
	f := newIntakeFixture(t, true)
	f.store.listErr = errors.New("bucket unreachable")
	_, err := f.env.ExecuteActivity(f.acts.ListIntakePairs, ListIntakePairsInput{})
	require.Error(t, err)
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		assert.False(t, appErr.NonRetryable(), "a storage outage must be retried")
	}
}

// ---- ClaimIntakePair ----

func (f *intakeFixture) onePair(t *testing.T, scope, tld string) IntakePair {
	t.Helper()
	f.store.put(intakeInbox+scope+"/"+tld+"/dep.ryde", "RYDE", f.now.Add(-time.Minute))
	f.store.put(intakeInbox+scope+"/"+tld+"/dep.sig", "SIG", f.now.Add(-time.Minute))
	out := f.list(t, 0)
	require.Len(t, out.Pairs, 1)
	return out.Pairs[0]
}

func TestClaimIntakePair_MovesThePairOutOfTheInbox(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair := f.onePair(t, "ryop1", "example")
	claimed, err := f.claim(pair)
	require.NoError(t, err)

	prefix := "sftp/claimed/ryop1/example/" + pair.IntakeID + "/"
	assert.Equal(t, prefix+"dep.ryde", claimed.ArtifactKey, "the base name is kept for BindDeposit's hints")
	assert.Equal(t, prefix+"dep.sig", claimed.SignatureKey)
	assert.Equal(t, "RYDE", f.store.body(claimed.ArtifactKey))
	assert.Equal(t, "SIG", f.store.body(claimed.SignatureKey))
	assert.False(t, f.store.has(pair.ArtifactKey))
	assert.False(t, f.store.has(pair.SignatureKey))
	assert.Empty(t, f.list(t, 0).Pairs, "a claimed pair is no longer in the inbox")
}

func TestClaimIntakePair_ResumesAfterAPartialMove(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair := f.onePair(t, "ryop1", "example")

	// The artifact was copied but not deleted, then the activity died.
	f.store.failRemove[pair.ArtifactKey] = true
	_, err := f.claim(pair)
	require.Error(t, err)
	require.True(t, f.store.has(pair.ArtifactKey))

	claimed, err := f.claim(pair)
	require.NoError(t, err)
	assert.False(t, f.store.has(pair.ArtifactKey))
	assert.False(t, f.store.has(pair.SignatureKey))
	assert.True(t, f.store.has(claimed.ArtifactKey))
	assert.True(t, f.store.has(claimed.SignatureKey))

	// And once the move is complete, claiming again is a no-op.
	again, err := f.claim(pair)
	require.NoError(t, err)
	assert.Equal(t, claimed, again)
}

func TestClaimIntakePair_VanishedObjectIsNotRetried(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair := f.onePair(t, "ryop1", "example")
	require.NoError(t, f.store.RemoveObject(context.Background(), pair.SignatureKey)) // renamed by the operator
	_, err := f.claim(pair)
	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable())
}

func TestClaimIntakePair_RefusesKeysThatAreNotThePair(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair := f.onePair(t, "ryop1", "example")
	for name, mutate := range map[string]func(*IntakePair){
		"other tenant":      func(p *IntakePair) { p.Scope = "ryop2" },
		"other tld":         func(p *IntakePair) { p.TLD = "other" },
		"swapped":           func(p *IntakePair) { p.ArtifactKey, p.SignatureKey = p.SignatureKey, p.ArtifactKey },
		"outside the inbox": func(p *IntakePair) { p.ArtifactKey = "escrow-validation/ryop1/example/x/deposit.ryde" },
	} {
		t.Run(name, func(t *testing.T) {
			p := pair
			mutate(&p)
			_, err := f.claim(p)
			require.Error(t, err)
			assert.True(t, f.store.has(pair.ArtifactKey), "nothing may move")
		})
	}
}

// ---- SettleIntakePair ----

func (f *intakeFixture) claimedPair(t *testing.T) (IntakePair, ClaimIntakeOutput) {
	t.Helper()
	pair := f.onePair(t, "ryop1", "example")
	claimed, err := f.claim(pair)
	require.NoError(t, err)
	return pair, claimed
}

func (f *intakeFixture) recordRun(t *testing.T, scope, workflowID string) *entities.EscrowValidationRun {
	t.Helper()
	op, err := entities.NewOperatorID(scope)
	require.NoError(t, err)
	run, err := entities.NewEscrowValidationRun(uuid.New(), op, "example", workflowID, "temporal-run", entities.EscrowProfileRydeSig, f.now)
	require.NoError(t, err)
	require.NoError(t, f.runs.Create(context.Background(), run))
	return run
}

func TestSettleIntakePair_BoundDepositDeletesTheClaimedCopy(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair, claimed := f.claimedPair(t)
	validationID := pair.WorkflowID + "-validation"
	run := f.recordRun(t, "ryop1", validationID)

	out, err := f.settle(pair, claimed, validationID)
	require.NoError(t, err)
	assert.Equal(t, EscrowIntakeArchived, out.Disposition)
	assert.Equal(t, run.ID.String(), out.ValidationRunID)
	assert.Empty(t, f.store.keys(), "the archive in escrow-validation/ is the record; nothing of the intake remains")

	// Settling twice (a retry after the delete) is harmless.
	out, err = f.settle(pair, claimed, validationID)
	require.NoError(t, err)
	assert.Equal(t, EscrowIntakeArchived, out.Disposition)
}

func TestSettleIntakePair_UnboundPairIsRejected(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair, claimed := f.claimedPair(t)
	// A run of another tenant under the same workflow ID must not count.
	f.recordRun(t, "ryop2", pair.WorkflowID+"-validation")

	out, err := f.settle(pair, claimed, pair.WorkflowID+"-validation")
	require.NoError(t, err)
	assert.Equal(t, EscrowIntakeRejected, out.Disposition)
	rejected := "sftp/rejected/ryop1/example/" + pair.IntakeID + "/"
	assert.Equal(t, []string{rejected + "dep.ryde", rejected + "dep.sig"}, f.store.keys())
	assert.Equal(t, "RYDE", f.store.body(rejected+"dep.ryde"))

	// A retry after a completed move finds the pair already rejected.
	out, err = f.settle(pair, claimed, pair.WorkflowID+"-validation")
	require.NoError(t, err)
	assert.Equal(t, EscrowIntakeRejected, out.Disposition)
}

type failingRunRepo struct{ *fakeRunRepo }

func (failingRunRepo) GetByWorkflowID(context.Context, entities.OperatorID, string) (*entities.EscrowValidationRun, error) {
	return nil, errors.New("database unavailable")
}

func TestSettleIntakePair_LookupFailureKeepsThePair(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair, claimed := f.claimedPair(t)
	f.acts.runs = failingRunRepo{f.runs}

	_, err := f.settle(pair, claimed, pair.WorkflowID+"-validation")
	require.Error(t, err)
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		assert.False(t, appErr.NonRetryable(), "an unknown answer must be retried, never guessed")
	}
	assert.True(t, f.store.has(claimed.ArtifactKey))
	assert.True(t, f.store.has(claimed.SignatureKey))
}

func TestSettleIntakePair_RefusesKeysOutsideItsClaim(t *testing.T) {
	f := newIntakeFixture(t, true)
	pair, claimed := f.claimedPair(t)
	f.store.put("escrow-validation/ryop1/example/d/deposit.ryde", "ARCHIVE", f.now)
	bad := claimed
	bad.ArtifactKey = "escrow-validation/ryop1/example/d/deposit.ryde"
	_, err := f.settle(pair, bad, pair.WorkflowID+"-validation")
	require.Error(t, err)
	assert.True(t, f.store.has("escrow-validation/ryop1/example/d/deposit.ryde"), "settle must never touch the archive")

	other := pair
	other.IntakeID = "0000000000000000"
	_, err = f.settle(other, claimed, pair.WorkflowID+"-validation")
	require.Error(t, err, "a claim belongs to one intake")
}

// ---- plaintext deposits (ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT) ----

func TestListIntakePairs_PlaintextRefusedByDefault(t *testing.T) {
	f := newIntakeFixture(t, true)
	f.store.put(intakeInbox+"ryop1/example/dep.xml", "<xml/>", f.now)
	f.store.put(intakeInbox+"ryop1/example/dep.xml.gz", "GZ", f.now)
	out := f.list(t, 0)
	assert.Empty(t, out.Pairs)
	assert.Equal(t, 2, out.PlaintextRefused)
	assert.Zero(t, out.Ignored, "a refused plaintext deposit is counted on its own, not as noise")
	assert.True(t, f.store.has(intakeInbox+"ryop1/example/dep.xml"), "a refused deposit is left where it is")
}

func TestListIntakePairs_PlaintextIsADepositOnItsOwn(t *testing.T) {
	f := newIntakeFixtureWith(t, EscrowIntakeConfig{Enabled: true, AllowPlaintext: true, Prefix: defaultEscrowIntakePrefix})
	f.store.put(intakeInbox+"ryop1/example/dep.xml.gz", "GZ", f.now.Add(-2*time.Minute))
	f.store.put(intakeInbox+"ryop1/example/dep.ryde", "RYDE", f.now.Add(-time.Minute)) // same base, different deposit
	f.store.put(intakeInbox+"ryop1/example/dep.sig", "SIG", f.now.Add(-time.Minute))
	out := f.list(t, 0)
	require.Len(t, out.Pairs, 2)
	xml, signed := out.Pairs[0], out.Pairs[1]
	assert.Equal(t, entities.EscrowProfilePlaintextXML, xml.Profile)
	assert.Equal(t, intakeInbox+"ryop1/example/dep.xml.gz", xml.ArtifactKey)
	assert.Empty(t, xml.SignatureKey)
	assert.Equal(t, f.now.Add(-2*time.Minute), xml.ReceivedAt)
	assert.Equal(t, entities.EscrowProfileRydeSig, signed.Profile)
	assert.NotEqual(t, xml.IntakeID, signed.IntakeID)
	assert.Zero(t, out.Unpaired, "a plaintext file never waits for a partner")
}

func TestPlaintextIntake_ClaimAndSettle(t *testing.T) {
	f := newIntakeFixtureWith(t, EscrowIntakeConfig{Enabled: true, AllowPlaintext: true, Prefix: defaultEscrowIntakePrefix})
	f.store.put(intakeInbox+"ryop1/example/dep.xml.gz", "GZ", f.now)
	out := f.list(t, 0)
	require.Len(t, out.Pairs, 1)
	pair := out.Pairs[0]

	claimed, err := f.claim(pair)
	require.NoError(t, err)
	assert.Equal(t, "sftp/claimed/ryop1/example/"+pair.IntakeID+"/dep.xml.gz", claimed.ArtifactKey, "the .gz suffix survives, so BindDeposit archives deposit.xml.gz")
	assert.Empty(t, claimed.SignatureKey)
	assert.False(t, f.store.has(pair.ArtifactKey))

	// Not bound: rejected, one object.
	settled, err := f.settle(pair, claimed, pair.WorkflowID+"-validation")
	require.NoError(t, err)
	assert.Equal(t, EscrowIntakeRejected, settled.Disposition)
	assert.Equal(t, []string{"sftp/rejected/ryop1/example/" + pair.IntakeID + "/dep.xml.gz"}, f.store.keys())
}

func TestPlaintextIntake_ClaimRefusedWhenNotAllowed(t *testing.T) {
	f := newIntakeFixtureWith(t, EscrowIntakeConfig{Enabled: true, AllowPlaintext: true, Prefix: defaultEscrowIntakePrefix})
	f.store.put(intakeInbox+"ryop1/example/dep.xml", "<xml/>", f.now)
	pair := f.list(t, 0).Pairs[0]

	// The setting was turned off between the sweep and the claim.
	f.acts.cfg.AllowPlaintext = false
	_, err := f.claim(pair)
	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable())
	assert.True(t, f.store.has(pair.ArtifactKey), "nothing moves")
}

func TestIntakeProfileAndObjectsMustAgree(t *testing.T) {
	f := newIntakeFixtureWith(t, EscrowIntakeConfig{Enabled: true, AllowPlaintext: true, Prefix: defaultEscrowIntakePrefix})
	signed := f.onePair(t, "ryop1", "example")
	f.store.put(intakeInbox+"ryop1/example/plain.xml", "<xml/>", f.now)

	for name, p := range map[string]IntakePair{
		"signed pair claimed as plaintext":   func() IntakePair { p := signed; p.Profile = entities.EscrowProfilePlaintextXML; return p }(),
		"signed profile without a signature": func() IntakePair { p := signed; p.SignatureKey = ""; return p }(),
		"plaintext file claimed as signed": {Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfileRydeSig, IntakeID: "0000000000000000",
			ArtifactKey: intakeInbox + "ryop1/example/plain.xml", SignatureKey: signed.SignatureKey},
		".ryde claimed as plaintext": {Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML, IntakeID: "0000000000000000",
			ArtifactKey: signed.ArtifactKey},
		"unknown profile": func() IntakePair { p := signed; p.Profile = "zip"; return p }(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.claim(p)
			require.Error(t, err)
		})
	}
	assert.True(t, f.store.has(signed.ArtifactKey))
	assert.True(t, f.store.has(intakeInbox+"ryop1/example/plain.xml"))
}
