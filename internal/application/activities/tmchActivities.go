package activities

import (
	"context"
	"fmt"

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
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, "fetching TMCH data")
	}
	if !a.svc.Configured() {
		logger := activity.GetLogger(ctx)
		logger.Info("TMCH feed not configured; nothing to refresh", "correlationID", correlationID)
		return queries.TMCHRefreshResult{}, nil
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
