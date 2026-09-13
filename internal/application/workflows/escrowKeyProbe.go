package workflows

import (
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowKeyProbeParams launches a probe of one key version (issue #429,
// ADR-0009). It is started by the key registry API after authorizing the
// caller for the owner, and deliberately not listed in the Launchpad registry:
// a probe reads secret material and must not be launchable around that check.
type EscrowKeyProbeParams struct {
	Owner       activities.EscrowKeyOwnerRef `json:"owner"`
	VersionID   string                       `json:"versionId"` // uuid; parsed and validated by the activity
	RequestedBy string                       `json:"requestedBy"`
}

// EscrowKeyProbeResult is the recorded verdict.
type EscrowKeyProbeResult struct {
	OK          bool   `json:"ok"`
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// EscrowKeyProbeWorkflow probes a version on a worker and records the verdict.
// A key store that stays unreachable through the probe's retries is recorded
// as a failed probe (KEY_STORE_UNAVAILABLE), so an operator sees why a version
// cannot be activated instead of a workflow that silently failed. A version
// that no longer exists or can no longer be probed, or a database failure,
// ends the workflow without a record.
func EscrowKeyProbeWorkflow(ctx workflow.Context, params EscrowKeyProbeParams) (EscrowKeyProbeResult, error) {
	logger := workflow.GetLogger(ctx)
	info := workflow.GetInfo(ctx)
	wfID, runID := info.WorkflowExecution.ID, info.WorkflowExecution.RunID

	var acts *activities.EscrowKeyActivities
	ctxProbe := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 3},
	})
	ctxRecord := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 2 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 5},
	})

	var probed activities.ProbeEscrowKeyVersionOutput
	err := workflow.ExecuteActivity(ctxProbe, acts.ProbeEscrowKeyVersion, activities.ProbeEscrowKeyVersionInput{
		Owner: params.Owner, VersionID: params.VersionID, WorkflowID: wfID,
	}).Get(ctxProbe, &probed)
	if err != nil {
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) || appErr.Type() != activities.EscrowKeyStoreUnavailableErrorType {
			// Rejected versions and our own infrastructure failures (the
			// database) are not verdicts about the key; record nothing.
			return EscrowKeyProbeResult{}, fmt.Errorf("ProbeEscrowKeyVersion(version=%s): %w", params.VersionID, err)
		}
		probed = activities.ProbeEscrowKeyVersionOutput{OK: false, Code: activities.EscrowKeyProbeStoreUnavailable}
	}

	if err := workflow.ExecuteActivity(ctxRecord, acts.RecordEscrowKeyProbe, activities.RecordEscrowKeyProbeInput{
		Owner: params.Owner, VersionID: params.VersionID, OK: probed.OK, Code: probed.Code,
		Actor: params.RequestedBy, WorkflowID: wfID, TemporalRunID: runID, At: workflow.Now(ctx),
	}).Get(ctxRecord, nil); err != nil {
		return EscrowKeyProbeResult{}, fmt.Errorf("RecordEscrowKeyProbe(version=%s): %w", params.VersionID, err)
	}
	logger.Info("escrow key probe: recorded", "correlation_id", wfID, "key_version_id", params.VersionID, "code", probed.Code)
	return EscrowKeyProbeResult(probed), nil
}
