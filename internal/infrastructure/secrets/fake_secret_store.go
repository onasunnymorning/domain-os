package secrets

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// MemorySecretStore is an in-memory interfaces.EscrowSecretStore for tests and
// local composition. It versions values the way a real backend does, so a
// test can prove a reference pins a version.
type MemorySecretStore struct {
	mu       sync.Mutex
	values   map[string]map[string][]byte // name -> backend version -> value
	current  map[string]string
	deleted  map[string]bool
	seq      int
	FailWith error // when set, every call fails with it (store outage)
}

var _ interfaces.EscrowSecretStore = (*MemorySecretStore)(nil)

// NewMemorySecretStore creates an empty store.
func NewMemorySecretStore() *MemorySecretStore {
	return &MemorySecretStore{values: map[string]map[string][]byte{}, current: map[string]string{}, deleted: map[string]bool{}}
}

// Put stores a value under a new name.
func (m *MemorySecretStore) Put(_ context.Context, name string, _ map[string]string, value []byte) (entities.EscrowSecretRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWith != nil {
		return entities.EscrowSecretRef{}, m.FailWith
	}
	name = "memory/" + strings.TrimLeft(name, "/")
	if _, ok := m.values[name]; ok {
		return entities.EscrowSecretRef{}, entities.ErrEscrowSecretAlreadyExists
	}
	m.seq++
	version := fmt.Sprintf("v%d", m.seq)
	m.values[name] = map[string][]byte{version: append([]byte(nil), value...)}
	m.current[name] = version
	return entities.EscrowSecretRef{Name: name, BackendVersionID: version}, nil
}

// Replace writes a new backend version of an existing secret, as a rewrap or
// passphrase change would. Test-only: the key registry never overwrites.
func (m *MemorySecretStore) Replace(ref entities.EscrowSecretRef, value []byte) entities.EscrowSecretRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	version := fmt.Sprintf("v%d", m.seq)
	m.values[ref.Name][version] = append([]byte(nil), value...)
	m.current[ref.Name] = version
	return entities.EscrowSecretRef{Name: ref.Name, BackendVersionID: version}
}

// Get returns a value.
func (m *MemorySecretStore) Get(_ context.Context, ref entities.EscrowSecretRef) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWith != nil {
		return nil, m.FailWith
	}
	versions, ok := m.values[ref.Name]
	if !ok || m.deleted[ref.Name] {
		return nil, entities.ErrEscrowSecretNotFound
	}
	version := ref.BackendVersionID
	if version == "" {
		version = m.current[ref.Name]
	}
	v, ok := versions[version]
	if !ok {
		return nil, entities.ErrEscrowSecretNotFound
	}
	return append([]byte(nil), v...), nil
}

// ScheduleDelete marks a secret deleted.
func (m *MemorySecretStore) ScheduleDelete(_ context.Context, ref entities.EscrowSecretRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWith != nil {
		return m.FailWith
	}
	m.deleted[ref.Name] = true
	return nil
}
