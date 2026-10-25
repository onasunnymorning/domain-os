package activities

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
)

// ingestCursor is where an ingest activity resumes: the cursor its previous
// attempt heartbeated, or the zero cursor to start at the top.
//
// The importers used to heartbeat the cursor as preformatted JSON text, and a
// workflow in flight across the deploy can still hand one of those back, so a
// string is decoded as that JSON, then held to services.ResumeCursor like any
// legacy cursor. Anything else cannot be trusted as a key: the attempt starts
// over, which is safe because every ingest phase upserts. An attempt that
// starts over returns the zero cursor, so the caller does not seed its report
// from the earlier attempts it is about to repeat.
func ingestCursor(ctx context.Context, phase services.ImportPhase) services.ImportCursor {
	if !activity.HasHeartbeatDetails(ctx) {
		return services.ImportCursor{}
	}
	logger := activity.GetLogger(ctx)

	var cursor services.ImportCursor
	if err := activity.GetHeartbeatDetails(ctx, &cursor); err == nil {
		return resumableCursor(logger, phase, cursor)
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
	return resumableCursor(logger, phase, cursor)
}

// resumableCursor holds a recorded cursor to services.ResumeCursor and logs
// where the phase resumes.
func resumableCursor(logger log.Logger, phase services.ImportPhase, recorded services.ImportCursor) services.ImportCursor {
	cursor, restarted := services.ResumeCursor(phase, recorded)
	if restarted {
		logger.Warn("Legacy import cursor cannot be resumed from exactly; starting the ingest over", "phase", phase, "lastKey", recorded.LastKey)
		return cursor
	}
	logger.Info("Resuming ingest", "phase", phase, "version", cursor.Version, "lastKey", cursor.LastKey, "lastClID", cursor.LastClID, "processed", cursor.Processed)
	return cursor
}
