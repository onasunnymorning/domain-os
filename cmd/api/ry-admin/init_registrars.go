package main

import (
	"context"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	sharedconfig "github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/client"
	"go.uber.org/zap"
)

func runInitRegistrars(cfg *config.AdminApiConfig, logger *zap.Logger) {
	logger.Info("Starting init-registrars check")

	// Set up the GORM DB connection. No migration here: the app has run or
	// will run it.
	dbCfg, err := sharedconfig.LoadDatabase()
	if err != nil {
		logger.Fatal("Database configuration is invalid", zap.Error(err))
	}
	gormDB, err := postgres.NewConnection(postgres.Config(dbCfg))
	if err != nil {
		logger.Fatal("Failed to connect to the database", zap.Error(err))
	}

	// Count Registrars
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	eventPublisher := postgres.NewPostgresEventPublisher(gormDB, logger, cfg.LogEvents)
	registrarService := services.NewRegistrarService(registrarRepo, eventPublisher)

	count, err := registrarService.Count(context.Background())
	if err != nil {
		logger.Fatal("Failed to count registrars", zap.Error(err))
	}

	logger.Info("Current registrar count", zap.Int64("count", count))

	if count > 0 {
		logger.Info("Registrars already exist. Skipping init.")
		return
	}

	logger.Info("No registrars found. Triggering SyncRegistrars workflow...")

	// Connect to Temporal
	tCfg := temporal.NewClientConfigFromEnv(temporal.QueueScheduled)

	cli, err := temporal.GetTemporalClient(tCfg)
	if err != nil {
		logger.Fatal("Failed to connect to Temporal", zap.Error(err))
	}
	defer cli.Close()

	workflowOptions := client.StartWorkflowOptions{
		ID:        "init-sync-registrars",
		TaskQueue: tCfg.WorkerQueue,
	}

	we, err := cli.ExecuteWorkflow(context.Background(), workflowOptions, workflows.SyncRegistrarsTypeName, workflows.SyncRegistrarsParams{})
	if err != nil {
		logger.Fatal("Failed to start workflow", zap.Error(err))
	}

	logger.Info("Started SyncRegistrars workflow", zap.String("WorkflowID", we.GetID()), zap.String("RunID", we.GetRunID()))
}
