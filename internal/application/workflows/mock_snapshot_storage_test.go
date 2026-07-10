package workflows

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
)

// mockSnapshotStorage implements activities.SnapshotStorageAPI for testing.
// It extends the basic upload/download mock with ListObjectKeys support.
type mockSnapshotStorage struct {
	files map[string][]byte
}

func newMockSnapshotStorage() *mockSnapshotStorage {
	return &mockSnapshotStorage{files: make(map[string][]byte)}
}

func (m *mockSnapshotStorage) UploadStream(_ context.Context, key string, reader io.Reader, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.files[key] = data
	return nil
}

func (m *mockSnapshotStorage) DownloadStream(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.files[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockSnapshotStorage) ListObjectKeys(_ context.Context, prefix string, _ bool, maxKeys int) ([]string, error) {
	var keys []string
	for key := range m.files {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if maxKeys > 0 && len(keys) > maxKeys {
		keys = keys[:maxKeys]
	}
	return keys, nil
}
