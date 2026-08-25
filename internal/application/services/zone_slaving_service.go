package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	appinterfaces "github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/serialdrift"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// checkSerialDriftWorkflowName is the registered name of the workflow.
// Using a string reference avoids importing the workflows package.
const checkSerialDriftWorkflowName = "CheckSerialDriftWorkflow"

// ZoneSlavingService provides application-level operations for zone slaving monitors.
type ZoneSlavingService struct {
	repo repositories.SerialDriftRepository
}

// Ensure ZoneSlavingService satisfies the interface.
var _ appinterfaces.ZoneSlavingService = (*ZoneSlavingService)(nil)

// NewZoneSlavingService returns a new ZoneSlavingService instance.
func NewZoneSlavingService(repo repositories.SerialDriftRepository) *ZoneSlavingService {
	return &ZoneSlavingService{repo: repo}
}

// CreateSlaving creates a new ZoneSlaving monitor and starts its Temporal schedule.
func (s *ZoneSlavingService) CreateSlaving(ctx context.Context, scope entities.OperatorID, req appinterfaces.CreateSlavingRequest) (*entities.ZoneSlaving, error) {
	zs, err := entities.NewZoneSlaving(scope.String(), req.Zone, req.MasterNS, req.SlaveNS)
	if err != nil {
		return nil, fmt.Errorf("CreateSlaving(operator=%s, zone=%s): %w", scope, req.Zone, err)
	}

	// Apply optional overrides
	if req.CheckIntervalS > 0 {
		zs.CheckInterval = time.Duration(req.CheckIntervalS) * time.Second
	}
	if req.StalledAfterN > 0 {
		zs.StalledAfterN = req.StalledAfterN
	}
	if req.ConfidenceN > 0 {
		zs.ConfidenceN = req.ConfidenceN
	}
	if req.GraceMultiplier > 0 {
		zs.GraceMultiplier = req.GraceMultiplier
	}

	// Persist the entity
	if err := s.repo.CreateSlaving(ctx, zs); err != nil {
		return nil, fmt.Errorf("CreateSlaving(operator=%s, zone=%s): persist: %w", scope, req.Zone, err)
	}

	// Create the Temporal schedule
	if err := s.createSchedule(ctx, scope, zs); err != nil {
		// Best-effort cleanup: we logged the orphan in pending test TestPending_OrphanedScheduleCleanup
		return zs, fmt.Errorf("CreateSlaving(operator=%s, zone=%s): schedule: %w — ZoneSlaving created but schedule failed, create manually or retry", scope, req.Zone, err)
	}

	return zs, nil
}

// GetSlaving retrieves a ZoneSlaving monitor by ID.
func (s *ZoneSlavingService) GetSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.ZoneSlaving, error) {
	zs, err := s.repo.GetSlaving(ctx, scope, id)
	if err != nil {
		return nil, fmt.Errorf("GetSlaving(operator=%s, id=%s): %w", scope, id, err)
	}
	return zs, nil
}

// CompleteSlaving marks a slaving monitor as completed and deletes its schedule.
func (s *ZoneSlavingService) CompleteSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) error {
	if err := s.repo.UpdateSlavingStatus(ctx, scope, id, entities.ZoneSlavingStatusCompleted); err != nil {
		return fmt.Errorf("CompleteSlaving(operator=%s, id=%s): update status: %w", scope, id, err)
	}
	if err := s.deleteSchedule(ctx, id); err != nil {
		return fmt.Errorf("CompleteSlaving(operator=%s, id=%s): delete schedule: %w — status updated but schedule may be orphaned", scope, id, err)
	}
	return nil
}

// AbandonSlaving marks a slaving monitor as abandoned and deletes its schedule.
func (s *ZoneSlavingService) AbandonSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) error {
	if err := s.repo.UpdateSlavingStatus(ctx, scope, id, entities.ZoneSlavingStatusAbandoned); err != nil {
		return fmt.Errorf("AbandonSlaving(operator=%s, id=%s): update status: %w", scope, id, err)
	}
	if err := s.deleteSchedule(ctx, id); err != nil {
		return fmt.Errorf("AbandonSlaving(operator=%s, id=%s): delete schedule: %w — status updated but schedule may be orphaned", scope, id, err)
	}
	return nil
}

// ListActiveSlavings lists all active slaving monitors for an operator.
func (s *ZoneSlavingService) ListActiveSlavings(ctx context.Context, scope entities.OperatorID) ([]*entities.ZoneSlaving, error) {
	return s.repo.ListActiveSlavings(ctx, scope)
}

// GetConfidenceRollup returns the current confidence state for a slaving monitor.
func (s *ZoneSlavingService) GetConfidenceRollup(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.SlavingConfidenceRollup, error) {
	return s.repo.GetConfidenceRollup(ctx, scope, id)
}

// ListObservationHistory returns cursor-paginated observation history.
func (s *ZoneSlavingService) ListObservationHistory(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error) {
	if pageSize <= 0 {
		pageSize = 25
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return s.repo.ListObservations(ctx, scope, slavingID, pageSize, cursor)
}

// scheduleID returns the deterministic Temporal schedule ID for a slaving monitor.
func scheduleID(slavingID uuid.UUID) string {
	return "zone-slaving-" + slavingID.String()
}

// createSchedule creates a Temporal schedule for the given ZoneSlaving.
func (s *ZoneSlavingService) createSchedule(ctx context.Context, scope entities.OperatorID, zs *entities.ZoneSlaving) error {
	cfg := temporal.NewClientConfigFromEnv(temporal.QueueFastOps)
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to Temporal: %w", err)
	}
	defer cli.Close()

	sid := scheduleID(zs.ID)
	params := serialdrift.Params{
		TenantID:  scope,
		SlavingID: zs.ID.String(),
		Zone:      zs.Zone,
	}

	_, err = cli.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: sid,
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{Every: zs.CheckInterval},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        sid + "-run",
			Workflow:  checkSerialDriftWorkflowName,
			Args:      []interface{}{params},
			TaskQueue: temporal.QueueFastOps,
		},
		Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		CatchupWindow: zs.CheckInterval,
		Note:          fmt.Sprintf("Zone slaving monitor for %s — managed by ZoneSlavingService", zs.Zone),
	})
	if err != nil {
		// If schedule already exists (e.g., retry after partial failure), that's OK.
		var alreadyExists *serviceerror.AlreadyExists
		if errors.As(err, &alreadyExists) {
			return nil
		}
		return fmt.Errorf("create schedule %s: %w", sid, err)
	}

	return nil
}

// deleteSchedule deletes the Temporal schedule for the given slaving ID.
func (s *ZoneSlavingService) deleteSchedule(ctx context.Context, slavingID uuid.UUID) error {
	cfg := temporal.NewClientConfigFromEnv(temporal.QueueFastOps)
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to Temporal: %w", err)
	}
	defer cli.Close()

	sid := scheduleID(slavingID)
	handle := cli.ScheduleClient().GetHandle(ctx, sid)
	if err := handle.Delete(ctx); err != nil {
		// If schedule already deleted (e.g., manual cleanup), that's OK.
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("delete schedule %s: %w", sid, err)
	}

	return nil
}
