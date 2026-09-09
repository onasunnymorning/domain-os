package activities

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// ---- in-memory object store ----

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	reads   map[string]int
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, reads: map[string]int{}}
}

func (f *fakeStore) put(key string, b []byte) { f.mu.Lock(); defer f.mu.Unlock(); f.objects[key] = b }
func (f *fakeStore) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	return b, ok
}
func (f *fakeStore) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func (f *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := f.get(key)
	return ok, nil
}
func (f *fakeStore) GetObjectStream(_ context.Context, key string) (io.ReadCloser, int64, error) {
	b, ok := f.get(key)
	if !ok {
		return nil, 0, errors.New("fakeStore: no such key")
	}
	f.mu.Lock()
	f.reads[key]++
	f.mu.Unlock()
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}
func (f *fakeStore) CopyObject(_ context.Context, src, dst string) error {
	b, ok := f.get(src)
	if !ok {
		return errors.New("fakeStore: no such source")
	}
	f.put(dst, append([]byte(nil), b...))
	return nil
}
func (f *fakeStore) UploadStream(_ context.Context, key string, r io.Reader, _ string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.put(key, b)
	return nil
}

// ---- in-memory repositories ----

type fakeTLDRepo struct{ owned map[string]string } // tld -> ryid

func (f *fakeTLDRepo) GetByNameForOperator(_ context.Context, scope entities.OperatorID, name string) (*entities.TLD, error) {
	if f.owned[name] != scope.String() {
		return nil, entities.ErrTLDNotFound
	}
	t, _ := entities.NewTLD(name, scope.String())
	return t, nil
}
func (f *fakeTLDRepo) Create(context.Context, *entities.TLD) error {
	return errors.New("not implemented")
}
func (f *fakeTLDRepo) GetByName(context.Context, string, bool) (*entities.TLD, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTLDRepo) List(context.Context, queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	return nil, "", errors.New("not implemented")
}
func (f *fakeTLDRepo) Update(context.Context, *entities.TLD) error {
	return errors.New("not implemented")
}
func (f *fakeTLDRepo) DeleteByName(context.Context, string) error {
	return errors.New("not implemented")
}
func (f *fakeTLDRepo) Count(context.Context, queries.ListTldsFilter) (int64, error) {
	return 0, errors.New("not implemented")
}

type fakeDepositRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*entities.EscrowDeposit
}

func newFakeDepositRepo() *fakeDepositRepo {
	return &fakeDepositRepo{rows: map[uuid.UUID]*entities.EscrowDeposit{}}
}
func (f *fakeDepositRepo) Create(_ context.Context, d *entities.EscrowDeposit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.TenantID == d.TenantID && r.TLD == d.TLD && r.Profile == d.Profile &&
			r.ArtifactSHA256 == d.ArtifactSHA256 && r.SignatureSHA256 == d.SignatureSHA256 {
			return errors.New("unique violation")
		}
	}
	cp := *d
	f.rows[d.ID] = &cp
	return nil
}
func (f *fakeDepositRepo) GetByID(_ context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowDeposit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[id]; ok && r.TenantID == scope {
		cp := *r
		return &cp, nil
	}
	return nil, entities.ErrEscrowDepositNotFound
}
func (f *fakeDepositRepo) FindByDigests(_ context.Context, scope entities.OperatorID, tld, profile, artifact, sig string) (*entities.EscrowDeposit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.TenantID == scope && r.TLD == tld && r.Profile == profile && r.ArtifactSHA256 == artifact && r.SignatureSHA256 == sig {
			cp := *r
			return &cp, nil
		}
	}
	return nil, entities.ErrEscrowDepositNotFound
}
func (f *fakeDepositRepo) List(context.Context, entities.OperatorID, queries.ListItemsQuery) ([]*entities.EscrowDeposit, string, error) {
	return nil, "", nil
}

type fakeRunRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*entities.EscrowValidationRun
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{rows: map[uuid.UUID]*entities.EscrowValidationRun{}}
}
func (f *fakeRunRepo) Create(_ context.Context, r *entities.EscrowValidationRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *r
	f.rows[r.ID] = &cp
	return nil
}
func (f *fakeRunRepo) Finalize(_ context.Context, scope entities.OperatorID, r *entities.EscrowValidationRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.rows[r.ID]
	if !ok || cur.TenantID != scope || cur.Outcome != entities.EscrowValidationRunning {
		return entities.ErrEscrowValidationRunAlreadyFinal
	}
	cp := *r
	f.rows[r.ID] = &cp
	return nil
}
func (f *fakeRunRepo) GetByID(_ context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowValidationRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[id]; ok && r.TenantID == scope {
		cp := *r
		return &cp, nil
	}
	return nil, entities.ErrEscrowValidationRunNotFound
}
func (f *fakeRunRepo) ListByDeposit(_ context.Context, scope entities.OperatorID, depositID uuid.UUID) ([]*entities.EscrowValidationRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*entities.EscrowValidationRun
	for _, r := range f.rows {
		if r.TenantID == scope && r.DepositID == depositID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeRunRepo) List(context.Context, entities.OperatorID, queries.ListItemsQuery) ([]*entities.EscrowValidationRun, string, error) {
	return nil, "", nil
}

type fakeKeyRepo struct{ keys []*entities.EscrowTrustedKey }

func (f *fakeKeyRepo) Create(_ context.Context, k *entities.EscrowTrustedKey) error {
	f.keys = append(f.keys, k)
	return nil
}
func (f *fakeKeyRepo) Retire(context.Context, entities.OperatorID, uuid.UUID, time.Time) error {
	return nil
}
func (f *fakeKeyRepo) GetByID(context.Context, entities.OperatorID, uuid.UUID) (*entities.EscrowTrustedKey, error) {
	return nil, entities.ErrEscrowTrustedKeyNotFound
}
func (f *fakeKeyRepo) ListActive(_ context.Context, scope entities.OperatorID, tld string, at time.Time) ([]*entities.EscrowTrustedKey, error) {
	var out []*entities.EscrowTrustedKey
	for _, k := range f.keys {
		if k.TenantID == scope && k.TLD == tld && k.Active(at) {
			out = append(out, k)
		}
	}
	return out, nil
}
func (f *fakeKeyRepo) List(context.Context, entities.OperatorID, string) ([]*entities.EscrowTrustedKey, error) {
	return f.keys, nil
}

type fakeKeyProvider struct {
	ring openpgp.EntityList
	err  error
}

func (f *fakeKeyProvider) DecryptionKeyring(context.Context) (openpgp.EntityList, error) {
	return f.ring, f.err
}
