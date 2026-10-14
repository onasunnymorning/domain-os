package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
)

// fakeReportStore is an in-memory importerReportStore.
type fakeReportStore struct {
	objects   map[string][]byte
	uploadErr error
	existsErr error
}

func newFakeReportStore() *fakeReportStore {
	return &fakeReportStore{objects: map[string][]byte{}}
}

func (f *fakeReportStore) Exists(_ context.Context, key string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.objects[key]
	return ok, nil
}

func (f *fakeReportStore) DownloadStream(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("no such key %s", key)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *fakeReportStore) UploadStream(_ context.Context, key string, r io.Reader, _ string) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.objects[key] = b
	return nil
}

func (f *fakeReportStore) report(t *testing.T, key string) services.RunReport {
	t.Helper()
	b, ok := f.objects[key]
	require.True(t, ok, "no report saved at %s", key)
	var r services.RunReport
	require.NoError(t, json.Unmarshal(b, &r))
	return r
}

type warnings []string

func (w *warnings) warn(msg string, _ ...interface{}) { *w = append(*w, msg) }

const reportKey = "escrow/run-1/run-report-domains.json"

func TestImporterReportKey(t *testing.T) {
	require.Equal(t, "escrow/run-1/run-report-domains.json", importerReportKey("escrow/run-1", "escrow/run-1/staged-x.db", "domains"))
	// An activity scheduled before RunPrefix was passed: the staged DB's folder is the run's.
	require.Equal(t, "escrow/run-1/run-report-hosts.json", importerReportKey("", "escrow/run-1/staged-x.db", "hosts"))
}

// The first attempt saves what it reported, replacing anything at the key.
func TestRunWithImporterReport_FirstAttemptSaves(t *testing.T) {
	store := newFakeReportStore()
	store.objects[reportKey] = []byte(`{"skipped_ds_records":[{"name":"stale.example","reason":"from another run"}]}`)
	report := &services.RunReport{}
	var w warnings

	key, err := runWithImporterReport(context.Background(), store, reportKey, false, report, w.warn, func() error {
		report.Merge(services.RunReport{SkippedDSRecords: []services.SkippedItem{{Name: "a.example", Reason: "bad digest"}}})
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, reportKey, key)
	require.Equal(t, []services.SkippedItem{{Name: "a.example", Reason: "bad digest"}}, store.report(t, reportKey).SkippedDSRecords)
	require.Empty(t, w)
}

// A resumed attempt does not rerun the batches before its cursor, so their
// items come from the report the earlier attempt saved, ahead of its own.
func TestRunWithImporterReport_ResumeKeepsEarlierAttempts(t *testing.T) {
	store := newFakeReportStore()
	started := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	earlier := services.RunReport{
		Timestamp:       started,
		ModifiedDomains: []services.ModifiedItem{{Name: "a.example", Field: "ExpiryDate"}},
	}
	require.NoError(t, saveImporterReport(context.Background(), store, reportKey, &earlier))

	report := &services.RunReport{Timestamp: started.Add(time.Hour)}
	var w warnings
	_, err := runWithImporterReport(context.Background(), store, reportKey, true, report, w.warn, func() error {
		report.Merge(services.RunReport{ModifiedDomains: []services.ModifiedItem{{Name: "z.example", Field: "ExpiryDate"}}})
		return nil
	})
	require.NoError(t, err)

	got := store.report(t, reportKey)
	require.Equal(t, []services.ModifiedItem{{Name: "a.example", Field: "ExpiryDate"}, {Name: "z.example", Field: "ExpiryDate"}}, got.ModifiedDomains)
	require.True(t, started.Equal(got.Timestamp), "the report dates from the first attempt, got %s", got.Timestamp)
	require.Empty(t, w)
}

// An earlier attempt that died before saving leaves nothing to resume from:
// say so, and carry on.
func TestRunWithImporterReport_ResumeWithoutSavedReportWarns(t *testing.T) {
	store := newFakeReportStore()
	var w warnings
	key, err := runWithImporterReport(context.Background(), store, reportKey, true, &services.RunReport{}, w.warn, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, reportKey, key)
	require.Len(t, w, 1)
}

// A saved report that cannot be read fails the attempt before it ingests
// anything, rather than replace that report with a partial one.
func TestRunWithImporterReport_UnreadableReportFailsBeforeRunning(t *testing.T) {
	for name, store := range map[string]*fakeReportStore{
		"corrupt": {objects: map[string][]byte{reportKey: []byte("{not json")}},
		"store":   {objects: map[string][]byte{}, existsErr: errors.New("s3 unavailable")},
	} {
		t.Run(name, func(t *testing.T) {
			before := string(store.objects[reportKey])
			ran := false
			var w warnings
			_, err := runWithImporterReport(context.Background(), store, reportKey, true, &services.RunReport{}, w.warn, func() error { ran = true; return nil })
			require.Error(t, err)
			require.False(t, ran)
			require.Equal(t, before, string(store.objects[reportKey]), "the saved report must be left as it was")
		})
	}
}

// What a failed attempt committed stays committed, so its report is saved for
// the retry to resume from, and the failure is still returned.
func TestRunWithImporterReport_SavesWhenRunFails(t *testing.T) {
	store := newFakeReportStore()
	report := &services.RunReport{}
	runErr := errors.New("batch failed")
	var w warnings

	key, err := runWithImporterReport(context.Background(), store, reportKey, false, report, w.warn, func() error {
		report.Merge(services.RunReport{SkippedHosts: []services.SkippedItem{{Name: "ns1.example", Reason: "Invalid name"}}})
		return runErr
	})
	require.ErrorIs(t, err, runErr)
	require.Equal(t, reportKey, key)
	require.Len(t, store.report(t, reportKey).SkippedHosts, 1)
}

// A report that cannot be saved does not fail an ingest that succeeded.
func TestRunWithImporterReport_SaveFailureWarns(t *testing.T) {
	store := newFakeReportStore()
	store.uploadErr = errors.New("s3 unavailable")
	var w warnings

	key, err := runWithImporterReport(context.Background(), store, reportKey, false, &services.RunReport{}, w.warn, func() error { return nil })
	require.NoError(t, err)
	require.Empty(t, key)
	require.Len(t, w, 1)
}
