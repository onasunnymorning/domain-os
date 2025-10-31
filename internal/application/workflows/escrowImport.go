package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowImportParams defines the input for the EscrowImportWorkflow
type EscrowImportParams struct {
	TLD       string                 `json:"tld"`
	ObjectKey string                 `json:"objectKey"`
	Options   map[string]interface{} `json:"options"`
}

// EscrowImportResult is a placeholder summary of the import
type EscrowImportResult struct {
	TLD         string            `json:"tld"`
	ObjectKey   string            `json:"objectKey"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt time.Time         `json:"completedAt"`
	Counts      map[string]int64  `json:"counts"`
	Notes       []string          `json:"notes"`
	RunPrefix   string            `json:"runPrefix"`
	DBKey       string            `json:"dbKey"`
	SummaryKey  string            `json:"summaryKey"`
	Artifacts   map[string]string `json:"artifacts"`
}

// EscrowImportWorkflow is a stub workflow to be expanded with activities
func EscrowImportWorkflow(ctx workflow.Context, params EscrowImportParams) (EscrowImportResult, error) {
	started := workflow.Now(ctx)
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Activity options for all steps
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Minute * 2,
		ScheduleToCloseTimeout: time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 1) Validate input
	var validateOut activities.ValidateInputResult
	var acts *activities.EscrowImportActivities
	if err := workflow.ExecuteActivity(ctx, acts.ValidateInput, activities.ValidateInputArgs{
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
	}).Get(ctx, &validateOut); err != nil {
		return EscrowImportResult{}, err
	}
	if !validateOut.Exists {
		return EscrowImportResult{}, fmt.Errorf("object %s does not exist in escrow bucket", params.ObjectKey)
	}

	// 2) Streaming analysis: produce CSV artifacts and store in S3 under approved path
	var analysisOut activities.StreamingAnalysisResult
	if err := workflow.ExecuteActivity(ctx, acts.StreamingAnalysis, activities.StreamingAnalysisArgs{
		TLD:           params.TLD,
		ObjectKey:     params.ObjectKey,
		MapRegistrars: true,
	}).Get(ctx, &analysisOut); err != nil {
		return EscrowImportResult{}, err
	}

	// If analysis found issues, gracefully short-circuit: skip DB convert/import, finalize with analysis findings.
	if analysisOut.HasIssues {
		// persist a run-report capturing analysis-only findings
		_ = workflow.ExecuteActivity(ctx, acts.PersistRunReport, activities.PersistRunReportArgs{
			TLD:             params.TLD,
			RunPrefix:       analysisOut.RunPrefix,
			WorkflowID:      wfID,
			AnalysisErrors:  analysisOut.AnalysisErrors,
			MissingContacts: analysisOut.MissingContacts,
			Events:          nil,
			Tallies:         map[string]int64{},
			Extra:           map[string]any{"phase": "analysis"},
		}).Get(ctx, nil)
		var finalizeOut activities.FinalizeAndQAResult
		if err := workflow.ExecuteActivity(ctx, acts.FinalizeAndQA, activities.FinalizeAndQAArgs{
			TLD:             params.TLD,
			RunPrefix:       analysisOut.RunPrefix,
			AnalysisCounts:  analysisOut.Counts,
			SqliteCounts:    map[string]int64{},
			AnalysisErrors:  analysisOut.AnalysisErrors,
			MissingContacts: analysisOut.MissingContacts,
		}).Get(ctx, &finalizeOut); err != nil {
			return EscrowImportResult{}, err
		}

		res := EscrowImportResult{
			TLD:         params.TLD,
			ObjectKey:   params.ObjectKey,
			StartedAt:   started,
			CompletedAt: workflow.Now(ctx),
			Counts:      map[string]int64{},
			Notes: []string{
				"Validated input",
				"Streaming analysis complete with issues; skipped SQLite/import",
				"Finalize summary written",
			},
			RunPrefix:  analysisOut.RunPrefix,
			DBKey:      "",
			SummaryKey: finalizeOut.SummaryKey,
			Artifacts:  analysisOut.ArtifactKeys,
		}
		return res, nil
	}

	// 3) Convert CSV artifacts to SQLite DB and upload it
	var sqliteOut activities.ConvertToSQLiteResult
	if err := workflow.ExecuteActivity(ctx, acts.ConvertToSQLite, activities.ConvertToSQLiteArgs{
		TLD:          params.TLD,
		RunPrefix:    analysisOut.RunPrefix,
		BaseFilename: analysisOut.BaseFilename,
		ArtifactKeys: analysisOut.ArtifactKeys,
	}).Get(ctx, &sqliteOut); err != nil {
		return EscrowImportResult{}, err
	}

	// 4) Import from SQLite (counts and linking)
	longAO := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour,
		ScheduleToCloseTimeout: time.Hour * 2,
		HeartbeatTimeout:       time.Minute * 2,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	importCtx := workflow.WithActivityOptions(ctx, longAO)

	var importOut activities.ImportFromSQLiteResult
	if err := workflow.ExecuteActivity(importCtx, acts.ImportFromSQLite, activities.ImportFromSQLiteArgs{
		TLD:   params.TLD,
		DBKey: sqliteOut.DBKey,
	}).Get(ctx, &importOut); err != nil {
		return EscrowImportResult{}, err
	}

	// Persist run-report with detailed import events/tallies
	_ = workflow.ExecuteActivity(ctx, acts.PersistRunReport, activities.PersistRunReportArgs{
		TLD:        params.TLD,
		RunPrefix:  analysisOut.RunPrefix,
		WorkflowID: wfID,
		Events:     importOut.Events,
		Tallies:    importOut.Tallies,
		Extra: map[string]any{
			"phase":                    "import",
			"registrarMappingRowCount": sqliteOut.RegistrarMappingRowCount,
		},
	}).Get(ctx, nil)

	// 5) Finalize and QA: produce a summary.json comparing counts
	var finalizeOut activities.FinalizeAndQAResult
	if err := workflow.ExecuteActivity(ctx, acts.FinalizeAndQA, activities.FinalizeAndQAArgs{
		TLD:            params.TLD,
		RunPrefix:      analysisOut.RunPrefix,
		AnalysisCounts: analysisOut.Counts,
		SqliteCounts:   importOut.Counts,
	}).Get(ctx, &finalizeOut); err != nil {
		return EscrowImportResult{}, err
	}

	res := EscrowImportResult{
		TLD:         params.TLD,
		ObjectKey:   params.ObjectKey,
		StartedAt:   started,
		CompletedAt: workflow.Now(ctx),
		Counts:      importOut.Counts,
		Notes:       []string{"Validated input", "Streaming analysis complete", "SQLite created", "Finalize summary written"},
		RunPrefix:   analysisOut.RunPrefix,
		DBKey:       sqliteOut.DBKey,
		SummaryKey:  finalizeOut.SummaryKey,
		Artifacts:   analysisOut.ArtifactKeys,
	}
	return res, nil
}
