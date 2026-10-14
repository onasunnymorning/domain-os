package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// importerReportStore is the part of the S3 client that keeps the direct
// importer's run report: what it skipped or changed while ingesting.
type importerReportStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	DownloadStream(ctx context.Context, key string) (io.ReadCloser, error)
	UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error
}

// importerReportKey is where an ingest phase keeps its report: beside the
// run's other artifacts. A workflow that scheduled the activity before it
// passed RunPrefix leaves it empty; the staged database lives in the run
// folder, so its folder is the run prefix.
func importerReportKey(runPrefix, stagedDBKey, phase string) string {
	if runPrefix == "" {
		runPrefix = path.Dir(stagedDBKey)
	}
	return runPrefix + "/run-report-" + phase + ".json"
}

// resumeImporterReport seeds report with the one earlier attempts saved. An
// attempt that resumes from a heartbeat does not run the batches before it
// again, so their items exist only there. None saved (an attempt that died
// without saving) is not an error; the caller logs it.
func resumeImporterReport(ctx context.Context, store importerReportStore, key string, report *services.RunReport) (bool, error) {
	exists, err := store.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check for importer report %s: %w", key, err)
	}
	if !exists {
		return false, nil
	}
	body, err := store.DownloadStream(ctx, key)
	if err != nil {
		return false, fmt.Errorf("download importer report %s: %w", key, err)
	}
	defer body.Close()

	var saved services.RunReport
	if err := json.NewDecoder(body).Decode(&saved); err != nil {
		return false, fmt.Errorf("decode importer report %s: %w", key, err)
	}
	// Earlier attempts' items come first, as their batches did.
	saved.Merge(*report)
	*report = saved
	return true, nil
}

// saveImporterReport writes report to key, replacing what was there.
func saveImporterReport(ctx context.Context, store importerReportStore, key string, report *services.RunReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode importer report: %w", err)
	}
	if err := store.UploadStream(ctx, key, bytes.NewReader(data), "application/json"); err != nil {
		return fmt.Errorf("upload importer report %s: %w", key, err)
	}
	return nil
}

// runWithImporterReport runs an ingest phase with its report kept at key:
// seeded from earlier attempts when this one resumes, and saved afterwards
// whether the phase succeeded or not, since what it committed stays committed.
// It returns key once the report is saved, "" if saving failed. A report that
// cannot be read on resume fails the attempt before anything is ingested, so a
// retry does not overwrite it with a partial one; a report that cannot be
// saved is a warning, like the import summary.
func runWithImporterReport(ctx context.Context, store importerReportStore, key string, resuming bool, report *services.RunReport, warn func(msg string, keyvals ...interface{}), run func() error) (string, error) {
	if resuming {
		found, err := resumeImporterReport(ctx, store, key, report)
		if err != nil {
			return "", err
		}
		if !found {
			warn("Resuming without an importer report from earlier attempts; items they reported are not in this one", "key", key)
		}
	}

	runErr := run()
	if err := saveImporterReport(ctx, store, key, report); err != nil {
		warn("Failed to save importer report", "key", key, "error", err)
		return "", runErr
	}
	return key, runErr
}
