package activities

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
)

// ingestCursor is where an ingest activity resumes: the cursor its previous
// attempt heartbeated, or the zero cursor to start at the top.
//
// The importers used to heartbeat the cursor as preformatted JSON text, and a
// workflow in flight across the deploy can still hand one of those back, so a
// string is decoded as that JSON. Anything else cannot be trusted as a key:
// the attempt starts over, which is safe because every ingest phase upserts.
func ingestCursor(ctx context.Context, phase string) services.ImportCursor {
	if !activity.HasHeartbeatDetails(ctx) {
		return services.ImportCursor{}
	}
	logger := activity.GetLogger(ctx)

	var cursor services.ImportCursor
	if err := activity.GetHeartbeatDetails(ctx, &cursor); err == nil {
		logger.Info("Resuming ingest from heartbeat", "phase", phase, "lastKey", cursor.LastKey, "lastClID", cursor.LastClID, "processed", cursor.Processed)
		return cursor
	}

	var legacy string
	if err := activity.GetHeartbeatDetails(ctx, &legacy); err != nil {
		logger.Warn("Heartbeat details are not an import cursor; starting the ingest over", "phase", phase, "error", err)
		return services.ImportCursor{}
	}
	cursor, err := services.ParseImportCursor(legacy)
	if err != nil {
		logger.Warn("Heartbeat details are not an import cursor; starting the ingest over", "phase", phase, "details", legacy, "error", err)
		return services.ImportCursor{}
	}
	logger.Info("Resuming ingest from legacy heartbeat", "phase", phase, "lastKey", cursor.LastKey, "processed", cursor.Processed)
	return cursor
}

// directImportCursor is where an *Direct activity resumes: the cursor it keeps
// in S3 at cursorKey, or the zero cursor when there is none or it cannot be
// read. It returns the heartbeat that records the cursor there and on the
// activity.
func directImportCursor(ctx context.Context, s3c *storage.S3Client, cursorKey string) (services.ImportCursor, func(services.ImportCursor)) {
	logger := activity.GetLogger(ctx)
	var cursor services.ImportCursor
	if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
		if cursor, err = services.ParseImportCursor(strings.TrimSpace(content)); err != nil {
			logger.Warn("Ignoring unreadable import cursor; starting over", "key", cursorKey, "error", err)
		}
	}

	heartbeat := func(c services.ImportCursor) {
		activity.RecordHeartbeat(ctx, c)
		data, err := json.Marshal(c)
		if err != nil {
			return
		}
		_ = s3c.UploadString(ctx, cursorKey, string(data))
	}
	return cursor, heartbeat
}
