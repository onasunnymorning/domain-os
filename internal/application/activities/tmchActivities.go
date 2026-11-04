package activities

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/tmch"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// TMCHActivities refresh the platform's TMCH data (ADR 0024 §4, roadmap
// 9a) on the worker's shared database pool.
type TMCHActivities struct {
	svc interfaces.TMCHService
}

// NewTMCHActivities wires the activities to the worker's database and the
// Clearinghouse feed described by cfg (read at the composition root,
// INV-17). With an unconfigured feed the refresh reports that and does
// nothing, so the schedule can exist on every installation.
func NewTMCHActivities(gormDB *gorm.DB, cfg tmch.Config) *TMCHActivities {
	return &TMCHActivities{svc: services.NewTMCHService(postgres.NewTMCHRepository(gormDB), tmch.NewFeed(cfg, nil), "domain-os/worker")}
}

// NewTMCHActivitiesWithService builds the activities on a service (tests).
func NewTMCHActivitiesWithService(svc interfaces.TMCHService) *TMCHActivities {
	return &TMCHActivities{svc: svc}
}

// RefreshTMCHData fetches the three sets and stores each newer generation.
// It returns the result and, when any set failed, an error so Temporal
// retries: a retry is safe, a set already stored is left unchanged.
func (a *TMCHActivities) RefreshTMCHData(ctx context.Context, correlationID string) (queries.TMCHRefreshResult, error) {
	if !a.svc.Configured() {
		logger := activity.GetLogger(ctx)
		logger.Info("TMCH feed not configured; nothing to refresh", "correlationID", correlationID)
		return queries.TMCHRefreshResult{}, nil
	}
	// The downloads and the DNL replace can take minutes; heartbeat while
	// they run so a slow attempt is not timed out and retried beside itself.
	if activity.IsActivity(ctx) {
		stop := heartbeat(ctx, tmchHeartbeatEvery, "refreshing TMCH data")
		defer stop()
	}
	result, err := a.svc.Refresh(ctx)
	if result == nil {
		result = &queries.TMCHRefreshResult{}
	}
	if err != nil {
		return *result, fmt.Errorf("refresh TMCH data: %w", err)
	}
	return *result, nil
}

// tmchHeartbeatEvery is how often the refresh activity heartbeats; the
// workflow's heartbeat timeout is a multiple of it.
const tmchHeartbeatEvery = 30 * time.Second

// heartbeat records a heartbeat every interval until stop is called.
func heartbeat(ctx context.Context, every time.Duration, details string) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				activity.RecordHeartbeat(ctx, details)
			}
		}
	}()
	return func() { close(done) }
}
