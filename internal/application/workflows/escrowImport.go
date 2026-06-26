package workflows

import (
	"fmt"
	"path/filepath"
	"strings"
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

// EscrowImportState tracks the real-time progress and QA status of the escrow import
type EscrowImportState struct {
	Phase                  string                          `json:"phase"` // "validating", "parsing", "staging_db", "resolving", "pending_registrar_overrides", "applying_mappings", "qa_check", "pending_confirmation", "ingesting", "verifying", "completed", "aborted", "qa_failed", "failed"
	TLD                    string                          `json:"tld"`
	ObjectKey              string                          `json:"objectKey"`
	RunPrefix              string                          `json:"runPrefix"`
	BaseDBKey              string                          `json:"baseDbKey,omitempty"`
	StagedDBKey            string                          `json:"stagedDbKey"`
	QAPassed               bool                            `json:"qaPassed"`
	QAReportKey            string                          `json:"qaReportKey,omitempty"`
	Error                  string                          `json:"error,omitempty"`
	Ingested               map[string]int64                `json:"ingested,omitempty"`
	TotalRegistrars        int                             `json:"totalRegistrars,omitempty"`
	MappedRegistrars       int                             `json:"mappedRegistrars,omitempty"`
	UnmappedRegistrars     []activities.UnmappedRegistrar   `json:"unmappedRegistrars,omitempty"`
	OverridesProvided      map[string]string               `json:"overridesProvided,omitempty"`
	VerificationPassed     *bool                           `json:"verificationPassed,omitempty"`
	VerificationReportKey  string                          `json:"verificationReportKey,omitempty"`
}

// EscrowImportResult is the final output of the unified EscrowImportWorkflow
type EscrowImportResult struct {
	TLD                    string                          `json:"tld"`
	ObjectKey              string                          `json:"objectKey"`
	RunPrefix              string                          `json:"runPrefix"`
	DBKey                  string                          `json:"dbKey,omitempty"`
	StagedDBKey            string                          `json:"stagedDbKey"`
	QAPassed               bool                            `json:"qaPassed"`
	QAReportKey            string                          `json:"qaReportKey"`
	Confirmed              bool                            `json:"confirmed"`
	IngestedCounts         map[string]int64                `json:"ingestedCounts,omitempty"`
	TotalRegistrars        int                             `json:"totalRegistrars,omitempty"`
	MappedRegistrars       int                             `json:"mappedRegistrars,omitempty"`
	UnmappedRegistrars     []activities.UnmappedRegistrar   `json:"unmappedRegistrars,omitempty"`
	OverridesProvided      map[string]string               `json:"overridesProvided,omitempty"`
	VerificationPassed     bool                            `json:"verificationPassed"`
	VerificationReportKey  string                          `json:"verificationReportKey,omitempty"`
}

// EscrowImportWorkflow manages the entire escrow import lifecycle.
// 1. Staging: Validates, parses, builds db, maps registrars, runs QA checks.
// 2. Human-In-The-Loop: Pauses for review and confirmation.
// 3. Ingestion: Accredit registrars, ingest contacts, hosts, domains, NNDNs, links.
func EscrowImportWorkflow(ctx workflow.Context, params EscrowImportParams) (EscrowImportResult, error) {
	state := EscrowImportState{
		Phase:     "validating",
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
	}

	err := workflow.SetQueryHandler(ctx, "state", func() (EscrowImportState, error) {
		return state, nil
	})
	if err != nil {
		return EscrowImportResult{}, fmt.Errorf("failed to set state query handler: %w", err)
	}

	aoStaging := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 2,
		ScheduleToCloseTimeout: time.Hour * 4,
		HeartbeatTimeout:       time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    5,
		},
	}
	ctxStaging := workflow.WithActivityOptions(ctx, aoStaging)

	var acts *activities.EscrowImportActivities

	// 0. Validate Escrow Source
	var validateOut activities.ValidateEscrowSourceResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.ValidateEscrowSource, activities.ValidateEscrowSourceArgs{
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
	}).Get(ctxStaging, &validateOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("ValidateEscrowSource(tld=%s, key=%s) activity failed: %w. Check that the escrow file exists in S3/MinIO and that the TLD is correctly configured.", params.TLD, params.ObjectKey, err)
	}
	if !validateOut.Exists {
		state.Phase = "failed"
		state.Error = fmt.Sprintf("object %s does not exist", params.ObjectKey)
		return EscrowImportResult{}, fmt.Errorf("escrow object key %s does not exist on S3. Please verify the file exists before launching.", params.ObjectKey)
	}

	// 1. Parse & Extract Assets
	state.Phase = "parsing"
	wID := workflow.GetInfo(ctx).WorkflowExecution.ID
	// Flat bucket layout: the workflow ID IS the run folder name.
	// e.g. "escrow-import-best-20260625-001231" — already contains TLD + date.
	runPrefix := wID
	state.RunPrefix = runPrefix

	var assetsOut activities.ParseAndExtractAssetsResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.ParseAndExtractAssets, activities.ParseAndExtractAssetsArgs{
		TLD:       params.TLD,
		ObjectKey: params.ObjectKey,
		RunPrefix: runPrefix,
	}).Get(ctxStaging, &assetsOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("ParseAndExtractAssets(tld=%s, key=%s) activity failed: %w. Verify that the file layout/encoding is valid.", params.TLD, params.ObjectKey, err)
	}

	if assetsOut.HasIssues {
		state.Phase = "failed"
		state.Error = fmt.Sprintf("ParseAndExtractAssets completed with issues: %v", assetsOut.AnalysisErrors)
		return EscrowImportResult{
			TLD:       params.TLD,
			ObjectKey: params.ObjectKey,
			RunPrefix: runPrefix,
		}, fmt.Errorf("ParseAndExtractAssets found structural problems with the escrow deposit: %v. Ingestion blocked.", assetsOut.AnalysisErrors)
	}

	// 1b. Copy source file into the run folder (server-side S3 copy, no download)
	_ = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}), acts.CopySourceToRunFolder, activities.CopySourceToRunFolderArgs{
		SourceKey: params.ObjectKey,
		RunPrefix: runPrefix,
	}).Get(ctx, nil) // best-effort: don't fail the workflow if copy fails

	// 2. Build Staging Database (Ryde.db)
	state.Phase = "staging_db"
	base := filepath.Base(params.ObjectKey)
	base = strings.TrimSuffix(base, ".gz")
	base = strings.TrimSuffix(base, ".xml")

	var collateOut activities.BuildStagingDatabaseResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.BuildStagingDatabase, activities.BuildStagingDatabaseArgs{
		TLD:          params.TLD,
		RunPrefix:    assetsOut.RunPrefix,
		AssetKeys:    assetsOut.AssetKeys,
		BaseFilename: base,
	}).Get(ctxStaging, &collateOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("BuildStagingDatabase(tld=%s, runPrefix=%s) activity failed: %w. Check space/permissions in SQLite builder.", params.TLD, runPrefix, err)
	}

	// 3. Resolve Registrars
	state.Phase = "resolving"
	state.BaseDBKey = collateOut.DBKey
	overrides := make(map[string]string)
	if val, ok := params.Options["registrarOverrides"]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			for k, v := range mapVal {
				overrides[k] = fmt.Sprint(v)
			}
		}
	}

	var mapOut activities.ResolveRegistrarsResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.ResolveRegistrars, activities.ResolveRegistrarsArgs{
		TLD:       params.TLD,
		DBKey:     collateOut.DBKey,
		RunPrefix: assetsOut.RunPrefix,
		Overrides: overrides,
	}).Get(ctxStaging, &mapOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("ResolveRegistrars(tld=%s, db=%s) activity failed: %w. Ensure registrar overrides match existing registrar IDs.", params.TLD, collateOut.DBKey, err)
	}

	// Propagate mapping summary to state so the UI can surface it
	state.TotalRegistrars = mapOut.TotalRegistrars
	state.MappedRegistrars = mapOut.MappedCount
	if mapOut.HasIssues {
		state.UnmappedRegistrars = mapOut.UnmappedRegistrars
	}

	// 3b. Registrar Override Gate — pause if unmapped registrars have domains
	// Check if any unmapped registrar has domains (the critical case that blocks import)
	hasUnmappedWithDomains := false
	for _, u := range mapOut.UnmappedRegistrars {
		if u.DomainCount > 0 {
			hasUnmappedWithDomains = true
			break
		}
	}

	if mapOut.HasIssues && hasUnmappedWithDomains {
		state.Phase = "pending_registrar_overrides"
		workflow.GetLogger(ctx).Info("Unmapped registrars with domains found. Waiting for ProvideRegistrarOverrides or SkipRegistrarOverrides signal.",
			"unmappedCount", len(mapOut.UnmappedRegistrars))

		overrideChan := workflow.GetSignalChannel(ctx, "ProvideRegistrarOverrides")
		skipChan := workflow.GetSignalChannel(ctx, "SkipRegistrarOverrides")

		overrideSelector := workflow.NewSelector(ctx)

		// Option A: User provides overrides
		overrideSelector.AddReceive(overrideChan, func(c workflow.ReceiveChannel, more bool) {
			var signalOverrides map[string]interface{}
			c.Receive(ctx, &signalOverrides)

			// Merge signal overrides into existing overrides
			for k, v := range signalOverrides {
				overrides[k] = fmt.Sprint(v)
			}
			state.OverridesProvided = overrides

			workflow.GetLogger(ctx).Info("Received ProvideRegistrarOverrides signal",
				"overrideCount", len(signalOverrides))

			// Re-run ResolveRegistrars with merged overrides
			state.Phase = "resolving"
			var reMapOut activities.ResolveRegistrarsResult
			if err := workflow.ExecuteActivity(ctxStaging, acts.ResolveRegistrars, activities.ResolveRegistrarsArgs{
				TLD:       params.TLD,
				DBKey:     collateOut.DBKey,
				RunPrefix: assetsOut.RunPrefix,
				Overrides: overrides,
			}).Get(ctxStaging, &reMapOut); err != nil {
				state.Phase = "failed"
				state.Error = err.Error()
				// Can't return from closure, so we store the error and let the main flow handle it
				mapOut = reMapOut
				return
			}

			// Update state with new mapping results
			mapOut = reMapOut
			state.TotalRegistrars = reMapOut.TotalRegistrars
			state.MappedRegistrars = reMapOut.MappedCount
			if reMapOut.HasIssues {
				state.UnmappedRegistrars = reMapOut.UnmappedRegistrars
			} else {
				state.UnmappedRegistrars = nil
			}
		})

		// Option B: User skips overrides
		overrideSelector.AddReceive(skipChan, func(c workflow.ReceiveChannel, more bool) {
			var skip bool
			c.Receive(ctx, &skip)
			workflow.GetLogger(ctx).Info("Received SkipRegistrarOverrides signal, continuing with gaps")
		})

		overrideSelector.Select(ctx)

		// Check if re-resolution failed
		if state.Phase == "failed" {
			return EscrowImportResult{}, fmt.Errorf("ResolveRegistrars(tld=%s, db=%s) re-resolution with overrides failed: %s", params.TLD, collateOut.DBKey, state.Error)
		}
	}

	// 4. Apply Registrar Mappings
	state.Phase = "applying_mappings"
	var stageOut activities.ApplyRegistrarMappingsResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.ApplyRegistrarMappings, activities.ApplyRegistrarMappingsArgs{
		TLD:   params.TLD,
		DBKey: mapOut.DBKey,
	}).Get(ctxStaging, &stageOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("ApplyRegistrarMappings(tld=%s, db=%s) activity failed: %w", params.TLD, mapOut.DBKey, err)
	}
	state.StagedDBKey = stageOut.StagedDBKey

	// 5. QA Staged Database
	state.Phase = "qa_check"
	var qaOut activities.QAStagedDatabaseResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.QAStagedDatabase, activities.QAStagedDatabaseArgs{
		TLD:         params.TLD,
		StagedDBKey: stageOut.StagedDBKey,
		RunPrefix:   assetsOut.RunPrefix,
	}).Get(ctxStaging, &qaOut); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("QAStagedDatabase(tld=%s, stagedDb=%s) activity failed: %w", params.TLD, stageOut.StagedDBKey, err)
	}
	state.QAPassed = qaOut.Passed
	state.QAReportKey = qaOut.QAReportKey

	// If QA failed, return result with QA info but do NOT progress to ingestion.
	// This is an application-level outcome, not an infrastructure error — the workflow
	// ran correctly and caught data quality issues. Returning nil lets Temporal mark
	// the workflow COMPLETED so the result (with artifacts) is accessible in the UI.
	if !qaOut.Passed {
		state.Phase = "qa_failed"
		return EscrowImportResult{
			TLD:                params.TLD,
			ObjectKey:          params.ObjectKey,
			RunPrefix:          assetsOut.RunPrefix,
			DBKey:              collateOut.DBKey,
			StagedDBKey:        stageOut.StagedDBKey,
			QAPassed:           false,
			QAReportKey:        qaOut.QAReportKey,
			Confirmed:          false,
			TotalRegistrars:    mapOut.TotalRegistrars,
			MappedRegistrars:   mapOut.MappedCount,
			UnmappedRegistrars: mapOut.UnmappedRegistrars,
			OverridesProvided:  state.OverridesProvided,
		}, nil
	}

	// 6. Pause and wait for user confirmation via the ConfirmEscrowImport signal
	state.Phase = "pending_confirmation"
	confirmationSignalChan := workflow.GetSignalChannel(ctx, "ConfirmEscrowImport")
	var confirmed bool

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(confirmationSignalChan, func(c workflow.ReceiveChannel, more bool) {
		var signalVal bool
		c.Receive(ctx, &signalVal)
		confirmed = signalVal
	})

	workflow.GetLogger(ctx).Info("Staging and QA complete. Waiting for ConfirmEscrowImport signal.")
	selector.Select(ctx)

	if !confirmed {
		state.Phase = "aborted"
		// Rejection is an application-level outcome, not an infrastructure error.
		// Returning nil lets Temporal mark the workflow COMPLETED so the result
		// (with artifacts) is accessible in the UI — same pattern as QA failure.
		return EscrowImportResult{
			TLD:                params.TLD,
			ObjectKey:          params.ObjectKey,
			RunPrefix:          assetsOut.RunPrefix,
			DBKey:              collateOut.DBKey,
			StagedDBKey:        stageOut.StagedDBKey,
			QAPassed:           true,
			QAReportKey:        qaOut.QAReportKey,
			Confirmed:          false,
			TotalRegistrars:    mapOut.TotalRegistrars,
			MappedRegistrars:   mapOut.MappedCount,
			UnmappedRegistrars: mapOut.UnmappedRegistrars,
			OverridesProvided:  state.OverridesProvided,
		}, nil
	}

	// 7. Ingestion Phase
	state.Phase = "ingesting"
	aoIngest := workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 10,
		ScheduleToCloseTimeout: time.Hour * 20,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
		HeartbeatTimeout: time.Minute * 10,
	}
	ctxIngest := workflow.WithActivityOptions(ctx, aoIngest)
	counts := make(map[string]int64)

	// Ingest Contacts
	var cRes activities.IngestContactsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.IngestContacts, activities.IngestContactsArgs{
		StagedDBKey: stageOut.StagedDBKey,
	}).Get(ctxIngest, &cRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("IngestContacts failed: %w", err)
	}
	counts["contacts_total"] = cRes.Total
	counts["contacts_inserted"] = cRes.Inserted
	counts["contacts_updated"] = cRes.Updated
	state.Ingested = counts

	// Ingest Hosts
	var hRes activities.IngestHostsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.IngestHosts, activities.IngestHostsArgs{
		StagedDBKey: stageOut.StagedDBKey,
	}).Get(ctxIngest, &hRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("IngestHosts failed: %w", err)
	}
	counts["hosts_total"] = hRes.Total
	counts["hosts_inserted"] = hRes.Inserted
	counts["hosts_updated"] = hRes.Updated
	state.Ingested = counts

	// Ingest Domains
	var dRes activities.IngestDomainsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.IngestDomains, activities.IngestDomainsArgs{
		StagedDBKey: stageOut.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctxIngest, &dRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("IngestDomains failed: %w", err)
	}
	counts["domains_total"] = dRes.Total
	counts["domains_inserted"] = dRes.Inserted
	counts["domains_updated"] = dRes.Updated
	state.Ingested = counts

	// Ingest NNDNs
	var nRes activities.IngestNNDNsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.IngestNNDNs, activities.IngestNNDNsArgs{
		StagedDBKey: stageOut.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctxIngest, &nRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("IngestNNDNs failed: %w", err)
	}
	counts["nndns_total"] = nRes.Total
	counts["nndns_inserted"] = nRes.Inserted
	counts["nndns_updated"] = nRes.Updated
	state.Ingested = counts

	// Link Domain Hosts
	var lRes activities.LinkDomainHostsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.LinkDomainHosts, activities.LinkDomainHostsArgs{
		StagedDBKey: stageOut.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctxIngest, &lRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("LinkDomainHosts failed: %w", err)
	}
	counts["links_total"] = lRes.Total
	counts["links_inserted"] = lRes.Inserted
	state.Ingested = counts

	// Accredit Registrars
	var aRes activities.AccreditRegistrarsResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.AccreditRegistrars, activities.AccreditRegistrarsArgs{
		StagedDBKey: stageOut.StagedDBKey,
		TLD:         params.TLD,
	}).Get(ctxIngest, &aRes); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return EscrowImportResult{}, fmt.Errorf("AccreditRegistrars failed: %w", err)
	}
	counts["accreditations_total"] = aRes.Total
	state.Ingested = counts

	// 8. Persist Import Summary to S3
	var persistOut activities.PersistImportSummaryResult
	if err := workflow.ExecuteActivity(ctxIngest, acts.PersistImportSummary, activities.PersistImportSummaryArgs{
		TLD:            params.TLD,
		RunPrefix:      assetsOut.RunPrefix,
		WorkflowID:     wID,
		QAPassed:       true,
		QAReportKey:    qaOut.QAReportKey,
		IngestedCounts: counts,
	}).Get(ctxIngest, &persistOut); err != nil {
		workflow.GetLogger(ctx).Warn("Failed to persist import summary to S3", "error", err)
	}

	// 9. Post-Ingestion Verification
	// Compares staged data against live system via admin API.
	// Verification failures are warnings — they don't fail the workflow.
	state.Phase = "verifying"
	var verifyOut activities.VerifyIngestionResult
	if err := workflow.ExecuteActivity(ctxStaging, acts.VerifyIngestion, activities.VerifyIngestionArgs{
		TLD:         params.TLD,
		StagedDBKey: stageOut.StagedDBKey,
		RunPrefix:   assetsOut.RunPrefix,
	}).Get(ctxStaging, &verifyOut); err != nil {
		workflow.GetLogger(ctx).Warn("Post-ingestion verification failed to run", "error", err)
	}
	vPassed := verifyOut.Passed
	state.VerificationPassed = &vPassed
	state.VerificationReportKey = verifyOut.ReportKey

	state.Phase = "completed"

	return EscrowImportResult{
		TLD:                    params.TLD,
		ObjectKey:              params.ObjectKey,
		RunPrefix:              assetsOut.RunPrefix,
		DBKey:                  collateOut.DBKey,
		StagedDBKey:            stageOut.StagedDBKey,
		QAPassed:               true,
		QAReportKey:            qaOut.QAReportKey,
		Confirmed:              true,
		IngestedCounts:         counts,
		TotalRegistrars:        mapOut.TotalRegistrars,
		MappedRegistrars:       mapOut.MappedCount,
		UnmappedRegistrars:     mapOut.UnmappedRegistrars,
		OverridesProvided:      state.OverridesProvided,
		VerificationPassed:     verifyOut.Passed,
		VerificationReportKey:  verifyOut.ReportKey,
	}, nil
}
