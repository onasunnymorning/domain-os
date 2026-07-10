package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/serialdrift"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CheckSerialDriftWorkflow queries all nameservers for a zone's SOA serial,
// evaluates drift between master and slave serials, persists observations,
// and raises alerts for critical drift.
func CheckSerialDriftWorkflow(ctx workflow.Context, params serialdrift.Params) (serialdrift.Result, error) {
	started := workflow.Now(ctx)
	runID := getWorkflowID(ctx)

	result := serialdrift.Result{
		RunID:     runID,
		StartedAt: started.Format(time.RFC3339),
	}

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (serialdrift.Result, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Step 1: Get slaving configuration
	// When SlavingID is set, load from the DB record. When empty (ad-hoc run
	// launched from the UI), use the inline config from params.
	var config serialdrift.Config
	var history []serialdrift.ObservationHistoryEntry
	adHoc := params.SlavingID == ""

	if adHoc {
		// Build config from inline params with sensible defaults
		config = serialdrift.Config{
			MasterNS:        params.MasterNS,
			SlaveNS:         params.SlaveNS,
			StalledAfterN:   params.StalledAfterN,
			ConfidenceN:     params.ConfidenceN,
			GraceMultiplier: params.GraceMultiplier,
		}
		if config.StalledAfterN == 0 {
			config.StalledAfterN = 3
		}
		if config.ConfidenceN == 0 {
			config.ConfidenceN = 5
		}
		if config.GraceMultiplier == 0 {
			config.GraceMultiplier = 2.5
		}
		if len(config.MasterNS) == 0 || len(config.SlaveNS) == 0 {
			result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
			result.Notes = append(result.Notes, "Ad-hoc run requires masterNS and slaveNS")
			return result, fmt.Errorf("CheckSerialDriftWorkflow: masterNS and slaveNS are required for ad-hoc runs")
		}
		result.Notes = append(result.Notes, "Ad-hoc run (no slaving monitor record)")
		// No history for ad-hoc — stall detection starts fresh
	} else {
		// Load config from existing ZoneSlaving record
		if err := workflow.ExecuteActivity(ctx, "GetSlavingConfig", params.TenantID, params.SlavingID).Get(ctx, &config); err != nil {
			result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
			result.Notes = append(result.Notes, "Failed to get slaving config: "+err.Error())
			return result, err
		}

		// Step 2: Get recent observation history for stall detection
		historyLimit := config.StalledAfterN + 1
		if err := workflow.ExecuteActivity(ctx, "GetRecentHistory", params.TenantID, params.SlavingID, historyLimit).Get(ctx, &history); err != nil {
			result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
			result.Notes = append(result.Notes, "Failed to get recent history: "+err.Error())
			return result, err
		}
	}


	// Step 3: Fan-out DNS queries to all nameservers
	allNS := make([]string, 0, len(config.MasterNS)+len(config.SlaveNS))
	allNS = append(allNS, config.MasterNS...)
	allNS = append(allNS, config.SlaveNS...)

	resultCh := workflow.NewChannel(ctx)
	mu := workflow.NewMutex(ctx)
	var soaResults []serialdrift.SOAQueryResult

	for _, ns := range allNS {
		ns := ns // capture
		workflow.Go(ctx, func(gCtx workflow.Context) {
			var qResult serialdrift.SOAQueryResult
			_ = workflow.ExecuteActivity(gCtx, "QuerySOASerial", params.Zone, ns).Get(gCtx, &qResult)
			_ = mu.Lock(gCtx)
			soaResults = append(soaResults, qResult)
			mu.Unlock()
			resultCh.Send(gCtx, true)
		})
	}

	// Wait for all goroutines to complete
	for range allNS {
		var done bool
		resultCh.Receive(ctx, &done)
	}

	// Split results into master and slave
	masterNSSet := make(map[string]bool, len(config.MasterNS))
	for _, ns := range config.MasterNS {
		masterNSSet[ns] = true
	}

	var masterResult serialdrift.SOAQueryResult
	var slaveResults []serialdrift.SOAQueryResult
	masterFound := false

	for _, r := range soaResults {
		if masterNSSet[r.Nameserver] {
			if !masterFound || r.Error == "" {
				masterResult = r
				masterFound = true
			}
		} else {
			slaveResults = append(slaveResults, r)
		}
	}

	if !masterFound || masterResult.Error != "" {
		result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
		errMsg := "no reachable master nameserver"
		if masterFound && masterResult.Error != "" {
			errMsg = "master nameserver unreachable: " + masterResult.Error
		}
		result.Notes = append(result.Notes, errMsg)
		result.DriftStatus = "unknown"
		return result, fmt.Errorf("CheckSerialDriftWorkflow: %s", errMsg)
	}

	result.MasterSerial = masterResult.Serial
	result.SOARefresh = masterResult.Refresh
	result.SOARetry = masterResult.Retry
	result.SOAExpire = masterResult.Expire

	// Step 4: Evaluate drift (pure function, NOT an activity)
	observations, overallTier := EvaluateDrift(masterResult, slaveResults, config, history)

	result.DriftStatus = overallTier

	// Step 5: Persist observations (skip for ad-hoc runs — no slaving record)
	if !adHoc {
		persistInput := serialdrift.PersistObservationsInput{
			TenantID:     params.TenantID,
			SlavingID:    params.SlavingID,
			RunID:        runID,
			Zone:         params.Zone,
			MasterSerial: masterResult.Serial,
			SOARefresh:   masterResult.Refresh,
			SOARetry:     masterResult.Retry,
			SOAExpire:    masterResult.Expire,
			DriftStatus:  overallTier,
			Observations: observations,
		}
		if err := workflow.ExecuteActivity(ctx, "PersistObservations", persistInput).Get(ctx, nil); err != nil {
			result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
			result.Notes = append(result.Notes, "Failed to persist observations: "+err.Error())
			return result, err
		}
	}

	// Build observation refs for result
	for _, obs := range observations {
		result.Observations = append(result.Observations, serialdrift.ObservationRef{
			Nameserver: obs.Nameserver,
			Serial:     obs.Serial,
			Status:     obs.Status,
		})
	}

	// Step 6: If any observation is critical, raise alert
	hasCritical := false
	for _, obs := range observations {
		if obs.DriftTier == "critical" {
			hasCritical = true
			break
		}
	}
	if hasCritical {
		alertInput := serialdrift.RaiseAlertInput{
			TenantID:  params.TenantID,
			SlavingID: params.SlavingID,
			RunID:     runID,
			Details:   fmt.Sprintf("Critical serial drift detected for zone %s (master serial %d)", params.Zone, masterResult.Serial),
		}
		if err := workflow.ExecuteActivity(ctx, "RaiseAlert", alertInput).Get(ctx, nil); err != nil {
			// Don't fail the workflow for alert failures, just note it
			result.Notes = append(result.Notes, "Failed to raise alert: "+err.Error())
		}
	}

	// Step 7: Return result
	result.CompletedAt = workflow.Now(ctx).Format(time.RFC3339)
	return result, nil
}

// ---------------------------------------------------------------------------
// Pure evaluation function (NOT an activity — deterministic & side-effect free)
// ---------------------------------------------------------------------------

// EvaluateDrift evaluates the drift between master and slave SOA serials.
// It returns individual observation results and an overall drift tier.
//
// Tier hierarchy: expected < warning < critical
func EvaluateDrift(
	master serialdrift.SOAQueryResult,
	slaves []serialdrift.SOAQueryResult,
	config serialdrift.Config,
	history []serialdrift.ObservationHistoryEntry,
) ([]serialdrift.ObservationResult, string) {
	overallTier := "expected"
	var results []serialdrift.ObservationResult

	for _, slave := range slaves {
		obs := serialdrift.ObservationResult{
			Nameserver: slave.Nameserver,
			Serial:     slave.Serial,
			IsMaster:   false,
		}

		switch {
		case slave.Error != "":
			obs.Status = "unreachable"
			obs.DriftTier = "warning"
			obs.Error = slave.Error

		case entities.SerialEqual(slave.Serial, master.Serial):
			obs.Status = "converged"
			obs.DriftTier = "expected"

		case entities.SerialLessThan(slave.Serial, master.Serial):
			// Slave is behind master — check history for stall
			stallCount := countConsecutiveStalls(slave.Nameserver, slave.Serial, history)
			if stallCount >= config.StalledAfterN {
				obs.Status = "stalled"
				obs.DriftTier = "critical"
			} else {
				obs.Status = "lagging"
				obs.DriftTier = "expected"
			}

		default:
			// Slave ahead of master per RFC 1982 — unusual
			obs.Status = "lagging"
			obs.DriftTier = "warning"
			obs.Error = fmt.Sprintf("slave serial %d ahead of master serial %d per RFC 1982", slave.Serial, master.Serial)
		}

		// Promote overall tier
		overallTier = worstTier(overallTier, obs.DriftTier)
		results = append(results, obs)
	}

	return results, overallTier
}

// countConsecutiveStalls counts how many consecutive recent history entries
// show the given nameserver stuck at the same serial while master advanced.
func countConsecutiveStalls(nameserver string, currentSerial uint32, history []serialdrift.ObservationHistoryEntry) int {
	count := 0
	for _, h := range history {
		if h.Nameserver != nameserver {
			continue
		}
		if entities.SerialEqual(h.Serial, currentSerial) && !entities.SerialEqual(h.MasterSerial, currentSerial) {
			count++
		} else {
			break // non-stall entry breaks the streak
		}
	}
	return count
}

// worstTier returns the more severe tier of the two.
func worstTier(a, b string) string {
	tierRank := map[string]int{
		"expected": 0,
		"warning":  1,
		"critical": 2,
	}
	if tierRank[b] > tierRank[a] {
		return b
	}
	return a
}
