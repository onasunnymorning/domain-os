package workflows

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowImportParams defines the input for the EscrowImportWorkflow
type EscrowImportParams struct {
	TLD       string                 `json:"tld"`
	ObjectKey string                 `json:"objectKey"`
	Options   map[string]interface{} `json:"options"`
}

// EscrowStagingResult (formerly EscrowImportResult)
type EscrowStagingResult struct {
	TLD         string            `json:"tld"`
	ObjectKey   string            `json:"objectKey"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt time.Time         `json:"completedAt"`
	RunPrefix   string            `json:"runPrefix"`
	DBKey       string            `json:"dbKey"`
	StagedDBKey string            `json:"stagedDbKey"`
	Artifacts   map[string]string `json:"artifacts"`
	Notes       []string          `json:"notes"`
	// Ingestion trigger result
	IngestionRunID string `json:"ingestionRunId,omitempty"`
}

// EscrowStagingWorkflow handles the preparation of data up to the Staged DB
// It parses, validates, collates, and stages the data.
// It can optionally trigger the Ingestion Workflow.
func EscrowStagingWorkflow(ctx workflow.Context, params EscrowImportParams) (EscrowStagingResult, error) {
	started := workflow.Now(ctx)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 2,
		ScheduleToCloseTimeout: time.Hour * 4,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var acts *activities.EscrowImportActivities

	// 0. Base Validation
	var validateOut activities.ValidateInputResult
	if err := workflow.ExecuteActivity(ctx, acts.ValidateInput, activities.ValidateInputArgs{
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
	}).Get(ctx, &validateOut); err != nil {
		return EscrowStagingResult{}, err
	}
	if !validateOut.Exists {
		return EscrowStagingResult{}, fmt.Errorf("object %s does not exist", params.ObjectKey)
	}

	// 1. Validate & Generate Assets (ParseAndAssetize)
	// Construct canonical RunPrefix: escrow/<tld>/<date>/<workflowID>
	// This matches what ListImports expects for UI linking.
	tld := params.TLD
	wID := workflow.GetInfo(ctx).WorkflowExecution.ID
	date := workflow.GetInfo(ctx).WorkflowStartTime.Format("20060102")
	runPrefix := fmt.Sprintf("escrow/%s/%s/%s", tld, date, wID)

	var assetsOut activities.ParseAndAssetizeResult
	if err := workflow.ExecuteActivity(ctx, acts.ParseAndAssetize, activities.ParseAndAssetizeArgs{
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
		RunPrefix: runPrefix,
	}).Get(ctx, &assetsOut); err != nil {
		return EscrowStagingResult{}, err
	}

	if assetsOut.HasIssues {
		return EscrowStagingResult{
			TLD:         params.TLD,
			ObjectKey:   params.ObjectKey,
			Notes:       append(assetsOut.AnalysisErrors, "ParseAndAssetize completed with issues"),
			CompletedAt: workflow.Now(ctx),
		}, nil
	}

	// 2. Collate Assets to Ryde.db
	// Check Parse output to get accurate base, or derive it.
	// ParseAndAssetize creates assets named like <Base>-domains.csv.
	// We should pass this base to Collate logic.
	base := strings.TrimSuffix(filepath.Base(params.ObjectKey), filepath.Ext(params.ObjectKey))

	var collateOut activities.CollateAssetsResult
	if err := workflow.ExecuteActivity(ctx, acts.CollateAssets, activities.CollateAssetsArgs{
		TLD:          params.TLD,
		RunPrefix:    assetsOut.RunPrefix,
		AssetKeys:    assetsOut.AssetKeys,
		BaseFilename: base,
	}).Get(ctx, &collateOut); err != nil {
		return EscrowStagingResult{}, err
	}

	// 3. Registrar Mapping
	overrides := make(map[string]string)
	if val, ok := params.Options["registrarOverrides"]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			for k, v := range mapVal {
				overrides[k] = fmt.Sprint(v)
			}
		}
	}

	var mapOut activities.RegistrarMapResult
	if err := workflow.ExecuteActivity(ctx, acts.RegistrarMap, activities.RegistrarMapArgs{
		TLD:       params.TLD,
		DBKey:     collateOut.DBKey,
		RunPrefix: assetsOut.RunPrefix,
		Overrides: overrides,
	}).Get(ctx, &mapOut); err != nil {
		return EscrowStagingResult{}, err
	}

	// 4. Stage Import
	var stageOut activities.StageImportResult
	if err := workflow.ExecuteActivity(ctx, acts.StageImport, activities.StageImportArgs{
		TLD:   params.TLD,
		DBKey: mapOut.DBKey,
	}).Get(ctx, &stageOut); err != nil {
		return EscrowStagingResult{}, err
	}

	res := EscrowStagingResult{
		TLD:         params.TLD,
		ObjectKey:   params.ObjectKey,
		StartedAt:   started,
		CompletedAt: workflow.Now(ctx),
		RunPrefix:   assetsOut.RunPrefix,
		DBKey:       collateOut.DBKey,
		StagedDBKey: stageOut.StagedDBKey,
		Artifacts:   assetsOut.AssetKeys,
		Notes:       []string{"Staging Completed"},
	}

	// Trigger Ingestion if requested (autoIngest: true)
	// Default to TRUE for now if not specified to maintain backward comaptibility feel?
	// User said "Have Stage trigger Import when requested".
	// Let's assume we look for "autoIngest" boolean in options.
	autoIngest := false
	if val, ok := params.Options["autoIngest"]; ok {
		if v, ok2 := val.(bool); ok2 {
			autoIngest = v
		}
	}

	if autoIngest {
		// Launch as Child Workflow with Abandon policy (Fire-and-Forget)
		ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID:        "ingest-" + workflow.GetInfo(ctx).WorkflowExecution.ID,
			ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
		})

		future := workflow.ExecuteChildWorkflow(ctx, EscrowIngestionWorkflow, EscrowIngestionParams{
			TLD:         params.TLD,
			StagedDBKey: stageOut.StagedDBKey,
		})

		// Wait for start to get ID
		var we workflow.Execution
		if err := future.GetChildWorkflowExecution().Get(ctx, &we); err != nil {
			return res, err
		}
		res.IngestionRunID = we.ID
		res.Notes = append(res.Notes, "Ingestion Triggered")
	}

	return res, nil
}

// EscrowIngestionParams input
type EscrowIngestionParams struct {
	TLD         string `json:"tld"`
	StagedDBKey string `json:"stagedDbKey"`
}

// EscrowIngestionResult output
type EscrowIngestionResult struct {
	Success bool
	Counts  map[string]int64
}

// EscrowIngestionWorkflow handles the granular ingestion of staged data
func EscrowIngestionWorkflow(ctx workflow.Context, params EscrowIngestionParams) (EscrowIngestionResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 10, // Long timeout for bulk ingestion
		ScheduleToCloseTimeout: time.Hour * 20,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3, // Reduced as per user request
		},
		HeartbeatTimeout: time.Minute * 10, // Ensure activities heartbeat
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var acts *activities.EscrowImportActivities
	counts := make(map[string]int64)

	// 1. Ingest Contacts
	var cRes activities.IngestContactsResult
	if err := workflow.ExecuteActivity(ctx, acts.IngestContacts, activities.IngestContactsArgs{
		StagedDBKey: params.StagedDBKey,
	}).Get(ctx, &cRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["contacts_total"] = cRes.Total
	counts["contacts_skipped"] = cRes.Skipped

	// 2. Ingest Hosts
	var hRes activities.IngestHostsResult
	if err := workflow.ExecuteActivity(ctx, acts.IngestHosts, activities.IngestHostsArgs{
		StagedDBKey: params.StagedDBKey,
	}).Get(ctx, &hRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["hosts_total"] = hRes.Total
	counts["hosts_skipped"] = hRes.Skipped

	// 3. Ingest Domains
	var dRes activities.IngestDomainsResult
	if err := workflow.ExecuteActivity(ctx, acts.IngestDomains, activities.IngestDomainsArgs{
		StagedDBKey: params.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctx, &dRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["domains_total"] = dRes.Total
	counts["domains_skipped"] = dRes.Skipped

	// 4. Ingest NNDNs
	var nRes activities.IngestNNDNsResult
	if err := workflow.ExecuteActivity(ctx, acts.IngestNNDNs, activities.IngestNNDNsArgs{
		StagedDBKey: params.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctx, &nRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["nndns_total"] = nRes.Total
	counts["nndns_skipped"] = nRes.Skipped

	// 5. Link Domain Hosts
	var lRes activities.LinkDomainHostsResult
	if err := workflow.ExecuteActivity(ctx, acts.LinkDomainHosts, activities.LinkDomainHostsArgs{
		StagedDBKey: params.StagedDBKey,
	}).Get(ctx, &lRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["links_total"] = lRes.Total

	// 6. Accredit Registrars
	var aRes activities.AccreditRegistrarsResult
	if err := workflow.ExecuteActivity(ctx, acts.AccreditRegistrars, activities.AccreditRegistrarsArgs{
		StagedDBKey: params.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctx, &aRes); err != nil {
		return EscrowIngestionResult{}, err
	}
	counts["accreditations_total"] = aRes.Total

	return EscrowIngestionResult{
		Success: true,
		Counts:  counts,
	}, nil
}
