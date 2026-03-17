package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
)

type MapRegistrarsArgs struct {
	TLD             string            `json:"tld"`
	RunPrefix       string            `json:"runPrefix"`
	BaseFilename    string            `json:"baseFilename"`
	AnalysisErrors  []string          `json:"analysisErrors"`
	MissingContacts []string          `json:"missingContacts"`
	Overrides       map[string]string `json:"overrides"`
}

type MapRegistrarsResult struct {
	AnalysisErrors  []string `json:"analysisErrors"`
	MissingContacts []string `json:"missingContacts"`
	HasIssues       bool     `json:"hasIssues"`
	AnalysisKey     string   `json:"analysisKey"`
	MappingCsvKey   string   `json:"mappingCsvKey"`
}

func (a *EscrowImportActivities) MapRegistrars(ctx context.Context, args MapRegistrarsArgs) (MapRegistrarsResult, error) {
	// Heartbeat
	activity.RecordHeartbeat(ctx, "Starting MapRegistrars")

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// 1. Download analysis.json
	analysisKey := args.RunPrefix + "/" + args.BaseFilename + "-analysis.json"

	// Use DownloadToFile which manages temp file creation
	downloadedPath, err := s3c.DownloadToFile(context.Background(), analysisKey)
	if err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to download analysis file: %w", err)
	}
	defer os.Remove(downloadedPath)

	// 2. Hydrate service
	svc := &services.XMLEscrowService{
		Deposit: entities.RDEDeposit{FileName: "dummy.xml"},
	}

	jsonBytes, err := os.ReadFile(downloadedPath)
	if err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to read downloaded analysis file: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, svc); err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to unmarshal analysis JSON: %w", err)
	}

	// 3. Prepare for MapRegistrars (ensure it writes to a safe location)
	originalFileName := svc.Deposit.FileName
	safeBaseName := filepath.Base(originalFileName)
	safeDir := os.TempDir()
	// Override Filename so GetDepositFileNameWoExtension points to temp dir
	svc.Deposit.FileName = filepath.Join(safeDir, safeBaseName)

	// 4. Perform MapRegistrars with Overrides
	token := GetBearerToken()

	if err := svc.MapRegistrars(token, args.Overrides); err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to map registrars: %w", err)
	}

	// 5. Save updated analysis.json
	updatedBytes, err := json.MarshalIndent(svc, "", "	")
	if err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to marshal updated analysis: %w", err)
	}

	// Write back to the downloaded file location to overwrite it
	if err := os.WriteFile(downloadedPath, updatedBytes, 0644); err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to write updated analysis to temp file: %w", err)
	}

	// 6. Upload updated analysis.json
	// Re-upload using the same temporary file path
	if err := s3c.UploadFile(context.Background(), analysisKey, downloadedPath, "application/json"); err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to upload updated analysis file: %w", err)
	}

	// 7. Upload registrarMapping.csv
	// The file should have been created at safeDir/BaseName-registrarMapping.csv
	// Note: GetDepositFileNameWoExtension typically returns the path without extension.
	// Since we set FileName to /tmp/basename.xml (or similar), it returns /tmp/basename.
	localMappingFile := svc.GetDepositFileNameWoExtension() + "-registrarMapping.csv"

	// Check if it exists
	if _, err := os.Stat(localMappingFile); os.IsNotExist(err) {
		return MapRegistrarsResult{}, fmt.Errorf("expected mapping file not found at %s", localMappingFile)
	}

	mappingKey := args.RunPrefix + "/registrarMapping.csv"
	if err := s3c.UploadFile(context.Background(), mappingKey, localMappingFile, "text/csv"); err != nil {
		return MapRegistrarsResult{}, fmt.Errorf("failed to upload mapping file: %w", err)
	}

	// Clean up local CSV
	defer os.Remove(localMappingFile)

	return MapRegistrarsResult{
		AnalysisErrors:  svc.Analysis.Errors,
		MissingContacts: svc.Analysis.MissingContacts,
		HasIssues:       len(svc.Analysis.Errors) > 0,
		AnalysisKey:     analysisKey,
		MappingCsvKey:   mappingKey,
	}, nil
}
