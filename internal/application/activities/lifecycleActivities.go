package activities

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// LifecycleActivities holds dependencies for domain lifecycle activities that
// bypass HTTP and call the service layer directly. This eliminates per-domain
// HTTP overhead in scheduled workflows.
type LifecycleActivities struct {
	DomainService *services.DomainService
}

// NewLifecycleActivities creates a new LifecycleActivities with a fully wired
// DomainService. Follows the same DB initialization pattern as TLDCleanupActivities
// but wires up the full service layer to preserve DDD boundaries.
func NewLifecycleActivities() (*LifecycleActivities, error) {
	// Initialize gorm DB
	var gormDB *gorm.DB
	var err error
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		gormDB, err = postgres.NewConnectionFromURL(dbURL, false)
	} else {
		dbCfg := postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		}
		gormDB, err = postgres.NewConnection(dbCfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DB for lifecycle activities: %w", err)
	}

	// Wire up repositories
	domainRepo := postgres.NewDomainRepository(gormDB)
	hostRepo := postgres.NewGormHostRepository(gormDB)
	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	tldRepo := postgres.NewGormTLDRepo(gormDB)
	phaseRepo := postgres.NewGormPhaseRepository(gormDB)
	premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(gormDB)
	fxRepo := postgres.NewFXRepository(gormDB)
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)

	// ID generator for roid service
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create ID generator for lifecycle activities: %w", err)
	}
	roidService := services.NewRoidService(idGenerator)

	// Event publisher — use a simple logger for worker-side events
	logger, _ := zap.NewProduction()
	eventPublisher := postgres.NewPostgresEventPublisher(gormDB, logger, true)

	// Assemble the DomainService
	domainService := services.NewDomainService(
		domainRepo, hostRepo, *roidService, nndnRepo,
		tldRepo, phaseRepo, premiumLabelRepo, fxRepo,
		registrarRepo, eventPublisher,
	)

	log.Printf("[lifecycle-activities] Initialized with direct service access (snowflake node: %d)", roidService.ListNode())

	return &LifecycleActivities{
		DomainService: domainService,
	}, nil
}

// --- Batch result types ---

// BatchFailure records a single domain failure within a batch operation.
type BatchFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

const (
	// defaultBatchChunkSize controls how many domains are processed between
	// heartbeat signals within a single activity execution.
	defaultBatchChunkSize = 200
)

// toBatchFailures converts service-layer BatchFailure to activity-layer BatchFailure.
func toBatchFailures(svcFailures []services.BatchFailure) []BatchFailure {
	out := make([]BatchFailure, len(svcFailures))
	for i, f := range svcFailures {
		out[i] = BatchFailure{
			DomainName: f.DomainName,
			Error:      f.Error,
		}
	}
	return out
}

// --- Batch write activities ---

// BatchAutoRenewDomains auto-renews domains in batch via the service layer.
// It processes domains in chunks, sending heartbeats between chunks so Temporal
// knows the activity is alive.
func (a *LifecycleActivities) BatchAutoRenewDomains(ctx context.Context, correlationID string, domainNames []string, years int) (services.BatchResult, error) {
	result := services.BatchResult{
		Succeeded: []string{},
		Failed:    []services.BatchFailure{},
	}

	if len(domainNames) == 0 {
		return result, nil
	}

	if years <= 0 {
		years = 1
	}

	// Process in chunks with heartbeats
	for i := 0; i < len(domainNames); i += defaultBatchChunkSize {
		end := i + defaultBatchChunkSize
		if end > len(domainNames) {
			end = len(domainNames)
		}
		chunk := domainNames[i:end]

		batchResult := a.DomainService.BatchAutoRenewDomains(ctx, chunk, years)
		result.Succeeded = append(result.Succeeded, batchResult.Succeeded...)
		result.Failed = append(result.Failed, batchResult.Failed...)

		// Heartbeat between chunks so Temporal knows we're alive
		activity.RecordHeartbeat(ctx, fmt.Sprintf("auto-renewed %d/%d domains", end, len(domainNames)))

		// Check if context was cancelled (e.g., workflow cancelled)
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, nil
}

// BatchExpireDomains expires domains in batch via the service layer.
func (a *LifecycleActivities) BatchExpireDomains(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
	result := services.BatchResult{
		Succeeded: []string{},
		Failed:    []services.BatchFailure{},
	}

	if len(domainNames) == 0 {
		return result, nil
	}

	for i := 0; i < len(domainNames); i += defaultBatchChunkSize {
		end := i + defaultBatchChunkSize
		if end > len(domainNames) {
			end = len(domainNames)
		}
		chunk := domainNames[i:end]

		batchResult := a.DomainService.BatchExpireDomains(ctx, chunk)
		result.Succeeded = append(result.Succeeded, batchResult.Succeeded...)
		result.Failed = append(result.Failed, batchResult.Failed...)

		activity.RecordHeartbeat(ctx, fmt.Sprintf("expired %d/%d domains", end, len(domainNames)))

		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, nil
}

// BatchPurgeDomains purges domains in batch via the service layer.
func (a *LifecycleActivities) BatchPurgeDomains(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
	result := services.BatchResult{
		Succeeded: []string{},
		Failed:    []services.BatchFailure{},
	}

	if len(domainNames) == 0 {
		return result, nil
	}

	for i := 0; i < len(domainNames); i += defaultBatchChunkSize {
		end := i + defaultBatchChunkSize
		if end > len(domainNames) {
			end = len(domainNames)
		}
		chunk := domainNames[i:end]

		batchResult := a.DomainService.BatchPurgeDomains(ctx, chunk)
		result.Succeeded = append(result.Succeeded, batchResult.Succeeded...)
		result.Failed = append(result.Failed, batchResult.Failed...)

		activity.RecordHeartbeat(ctx, fmt.Sprintf("purged %d/%d domains", end, len(domainNames)))

		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, nil
}

// BatchRestoreDomains restores domains in batch via the service layer.
func (a *LifecycleActivities) BatchRestoreDomains(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
	result := services.BatchResult{
		Succeeded: []string{},
		Failed:    []services.BatchFailure{},
	}

	if len(domainNames) == 0 {
		return result, nil
	}

	for i := 0; i < len(domainNames); i += defaultBatchChunkSize {
		end := i + defaultBatchChunkSize
		if end > len(domainNames) {
			end = len(domainNames)
		}
		chunk := domainNames[i:end]

		batchResult := a.DomainService.BatchRestoreDomains(ctx, chunk)
		result.Succeeded = append(result.Succeeded, batchResult.Succeeded...)
		result.Failed = append(result.Failed, batchResult.Failed...)

		activity.RecordHeartbeat(ctx, fmt.Sprintf("restored %d/%d domains", end, len(domainNames)))

		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, nil
}


