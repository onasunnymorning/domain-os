package workflows

import (
	"embed"
	"io/fs"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
)

//go:embed *.doc.md
var docFS embed.FS

// WorkflowStep represents a step in a workflow for UI progress visualization
type WorkflowStep struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	ActivityName string `json:"activityName,omitempty"` // Temporal activity name for progress tracking
}

// WorkflowMeta defines UI-facing metadata for a workflow type
type WorkflowMeta struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Queue        string         `json:"queue"`
	Category     string         `json:"category"`               // "data-pipeline" | "lifecycle" | "operations"
	Tags         []string       `json:"tags"`
	HasSignal    bool           `json:"hasSignal"`
	SignalName   string         `json:"signalName,omitempty"`
	Scheduled    bool           `json:"scheduled"`
	ScheduleInfo string         `json:"scheduleInfo,omitempty"` // e.g., "Every hour"
	Steps        []WorkflowStep `json:"steps"`
	DocMarkdown  string         `json:"docMarkdown,omitempty"`
	docFile      string
}

// GetWorkflowRegistry returns metadata for all available workflow types.
// This is the single source of truth used by the Launchpad UI.
func GetWorkflowRegistry() []WorkflowMeta {
	registry := []WorkflowMeta{
		{
			Key:         "escrow-import",
			Name:        "TLD Import",
			Description: "Unified workflow to parse, stage, QA, and ingest escrow deposit data after confirmation",
			Queue:       temporal.QueueData,
			Category:    "data-pipeline",
			Tags:        []string{"data", "escrow", "import"},
			HasSignal:   true,
			SignalName:  "ConfirmEscrowImport",
			Steps: []WorkflowStep{
				{Key: "validate-escrow-source", Label: "Validate Escrow Source", ActivityName: "ValidateEscrowSource"},
				{Key: "parse-extract-assets", Label: "Parse & Extract Assets", ActivityName: "ParseAndExtractAssets"},
				{Key: "build-staging-database", Label: "Build Staging Database", ActivityName: "BuildStagingDatabase"},
				{Key: "resolve-registrars", Label: "Resolve Registrars", ActivityName: "ResolveRegistrars"},
				{Key: "finalize-staging", Label: "Finalize Staging", ActivityName: "FinalizeStaging"},
				{Key: "qa-staged-database", Label: "QA Staged Database", ActivityName: "QAStagedDatabase"},
				{Key: "await-confirmation", Label: "Await Ingest Confirmation"},
				{Key: "ingest-contacts", Label: "Ingest Contacts", ActivityName: "IngestContacts"},
				{Key: "ingest-hosts", Label: "Ingest Hosts", ActivityName: "IngestHosts"},
				{Key: "ingest-domains", Label: "Ingest Domains", ActivityName: "IngestDomains"},
				{Key: "ingest-nndns", Label: "Ingest NNDNs", ActivityName: "IngestNNDNs"},
				{Key: "link-domain-hosts", Label: "Link Domain Hosts", ActivityName: "LinkDomainHosts"},
				{Key: "accredit-registrars", Label: "Accredit Registrars", ActivityName: "AccreditRegistrars"},
			},
			docFile: "escrowImport.doc.md",
		},
		{
			Key:         "tld-cleanup",
			Name:        "TLD Cleanup",
			Description: "Backs up and removes all assets associated with a TLD after confirmation",
			Queue:       temporal.QueueData,
			Category:    "operations",
			Tags:        []string{"operations", "tld", "cleanup"},
			HasSignal:   true,
			SignalName:  "ConfirmTLDCleanup",
			Steps: []WorkflowStep{
				{Key: "check-eligibility", Label: "Check Eligibility", ActivityName: "CheckTLDCanBeDeleted"},
				{Key: "plan-cleanup", Label: "Plan Cleanup", ActivityName: "PlanTLDCleanup"},
				{Key: "await-confirmation", Label: "Await Confirmation"}, // Signal wait, no activity
				{Key: "backup-assets", Label: "Backup Assets", ActivityName: "BackupTLDAssets"},
				{Key: "delete-assets", Label: "Delete Assets", ActivityName: "DeleteTLDAssets"},
			},
			docFile: "tldCleanupWorkflow.doc.md",
		},
		{
			Key:          "sync-registrars",
			Name:         "Sync Registrars",
			Description:  "Synchronizes the registrar list with the IANA registry",
			Queue:        temporal.QueueLifecycle,
			Category:     "operations",
			Tags:         []string{"operations", "registrars", "sync"},
			Scheduled:    true,
			ScheduleInfo: "Daily",
			Steps: []WorkflowStep{
				{Key: "sync-iana", Label: "Sync IANA", ActivityName: "SyncIanaRegistrars"},
				{Key: "count-registrars", Label: "Count Registrars", ActivityName: "CountRegistrars"},
				{Key: "diff-plan", Label: "Diff & Plan", ActivityName: "DiffAndPlanRegistrars"},
				{Key: "apply-creates", Label: "Apply Creates", ActivityName: "BulkCreateRegistrars"},
				{Key: "apply-updates", Label: "Apply Updates", ActivityName: "SetRegistrarStatus"},
			},
			docFile: "syncRegistrarsWorkflow.doc.md",
		},
		{
			Key:          "update-fx",
			Name:         "Update FX Rates",
			Description:  "Fetches and updates foreign exchange rates",
			Queue:        temporal.QueueData,
			Category:     "data-pipeline",
			Tags:         []string{"data", "finance", "fx"},
			Scheduled:    true,
			ScheduleInfo: "Every hour",
			Steps: []WorkflowStep{
				{Key: "update-exchange-rates", Label: "Update Exchange Rates", ActivityName: "UpdateFX"},
			},
			docFile: "updateFX.doc.md",
		},
		{
			Key:          "expiry-loop",
			Name:         "Expiry Loop",
			Description:  "Processes expired domains for auto-renew or expiration. Returns structured result with counts and failure details.",
			Queue:        temporal.QueueLifecycle,
			Category:     "lifecycle",
			Tags:         []string{"lifecycle", "domains", "expiry"},
			Scheduled:    true,
			ScheduleInfo: "Every hour",
			Steps: []WorkflowStep{
				{Key: "lock-reference-time", Label: "Lock Reference Time", ActivityName: ""},
				{Key: "count-expired", Label: "Count Expired", ActivityName: "GetExpiredDomainCount"},
				{Key: "list-expiring", Label: "List Expiring", ActivityName: "ListExpiringDomains"},
				{Key: "batch-check-autorenew", Label: "Batch Check Auto-Renew", ActivityName: "CheckDomainsCanAutoRenew"},
				{Key: "parallel-writes", Label: "Process Auto-Renew/Expire Writes", ActivityName: "AutoRenewDomain"},
			},
			docFile: "expiryLoop.doc.md",
		},
		{
			Key:          "purge-loop",
			Name:         "Purge Loop",
			Description:  "Purges domains that have completed their redemption grace period",
			Queue:        temporal.QueueLifecycle,
			Category:     "lifecycle",
			Tags:         []string{"lifecycle", "domains", "purge"},
			Scheduled:    true,
			ScheduleInfo: "Every hour",
			Steps: []WorkflowStep{
				{Key: "count-purgeable", Label: "Count Purgeable", ActivityName: "GetPurgeableDomainCount"},
				{Key: "list-purgeable", Label: "List Purgeable", ActivityName: "ListPurgeableDomains"},
				{Key: "purge-domains", Label: "Purge Domains", ActivityName: "PurgeDomain"},
			},
			docFile: "purgeLoop.doc.md",
		},
		{
			Key:          "restore-workflow",
			Name:         "Restore Workflow",
			Description:  "Processes restored domains by unsetting status and forcing renewal",
			Queue:        temporal.QueueLifecycle,
			Category:     "lifecycle",
			Tags:         []string{"lifecycle", "domains", "restore"},
			Scheduled:    true,
			ScheduleInfo: "Every hour",
			Steps: []WorkflowStep{
				{Key: "list-restored", Label: "List Restored", ActivityName: "ListRestoredDomains"},
				{Key: "unset-status", Label: "Unset Status", ActivityName: "UnSetDomainStatus"},
				{Key: "force-renew", Label: "Force Renew", ActivityName: "RenewDomain"},
			},
			docFile: "restoreWorkflow.doc.md",
		},
	}

	for i := range registry {
		if registry[i].docFile != "" {
			data, err := fs.ReadFile(docFS, registry[i].docFile)
			if err == nil {
				registry[i].DocMarkdown = string(data)
			}
		}
	}

	return registry
}

// GetWorkflowMeta looks up a workflow type by its key.
// Returns the metadata and true if found, or a zero value and false otherwise.
func GetWorkflowMeta(key string) (WorkflowMeta, bool) {
	for _, wf := range GetWorkflowRegistry() {
		if wf.Key == key {
			return wf, true
		}
	}
	return WorkflowMeta{}, false
}

// BuildActivityStepMap creates a mapping from Temporal activity name → step key
// for a given workflow type key. Used when parsing workflow execution history
// to determine which step is active.
func BuildActivityStepMap(workflowTypeKey string) map[string]string {
	meta, ok := GetWorkflowMeta(workflowTypeKey)
	if !ok {
		return nil
	}
	m := make(map[string]string, len(meta.Steps))
	for _, step := range meta.Steps {
		if step.ActivityName != "" {
			m[step.ActivityName] = step.Key
		}
	}
	return m
}
