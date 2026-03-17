package main

import (
	"context"
	"os"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/client"
	"go.uber.org/zap"
)

func runInitRegistrars(cfg *config.AdminApiConfig, logger *zap.Logger) {
	logger.Info("Starting init-registrars check")

	// Set up the GORM DB connection
	gormDB, err := postgres.NewConnection(
		postgres.Config{
			User:        os.Getenv("DB_USER"),
			Pass:        os.Getenv("DB_PASS"),
			Host:        os.Getenv("DB_HOST"),
			Port:        os.Getenv("DB_PORT"),
			DBName:      os.Getenv("DB_NAME"),
			SSLmode:     os.Getenv("DB_SSLMODE"),
			AutoMigrate: false, // Don't migrate here, assume app has run or will run
		},
	)
	if err != nil {
		logger.Fatal("Failed to connect to the database", zap.Error(err))
	}

	// Count Registrars
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	registrarService := services.NewRegistrarService(registrarRepo)

	count, err := registrarService.Count(context.Background())
	if err != nil {
		logger.Fatal("Failed to count registrars", zap.Error(err))
	}

	logger.Info("Current registrar count", zap.Int64("count", count))

	if count > 0 {
		logger.Info("Registrars already exist. Skipping init.")
		return
	}

	logger.Info("No registrars found. Triggering SyncRegistrarsWorkflow...")

	// Connect to Temporal
	tCfg := temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: os.Getenv("TMPIO_QUEUE"),
	}

	cli, err := temporal.GetTemporalClient(tCfg)
	if err != nil {
		logger.Fatal("Failed to connect to Temporal", zap.Error(err))
	}
	defer cli.Close()

	workflowOptions := client.StartWorkflowOptions{
		ID:        "init-sync-registrars",
		TaskQueue: tCfg.WorkerQueue,
	}

	we, err := cli.ExecuteWorkflow(context.Background(), workflowOptions, workflows.SyncRegistrarsWorkflow, 100)
	if err != nil {
		logger.Fatal("Failed to start workflow", zap.Error(err))
	}

	logger.Info("Started SyncRegistrarsWorkflow", zap.String("WorkflowID", we.GetID()), zap.String("RunID", we.GetRunID()))
}
