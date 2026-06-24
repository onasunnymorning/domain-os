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
	Key   string `json:"key"`
	Label string `json:"label"`
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
			Key:         "escrow-staging",
			Name:        "Escrow Staging",
			Description: "Parses, validates, and stages escrow deposit data for a TLD",
			Queue:       temporal.QueueData,
			Category:    "data-pipeline",
			Tags:        []string{"data", "escrow", "import"},
			Steps: []WorkflowStep{
				{Key: "validate-escrow-source", Label: "Validate Escrow Source"},
				{Key: "parse-extract-assets", Label: "Parse & Extract Assets"},
				{Key: "build-staging-database", Label: "Build Staging Database"},
				{Key: "resolve-registrars", Label: "Resolve Registrars"},
				{Key: "finalize-staging", Label: "Finalize Staging"},
				{Key: "qa-staged-database", Label: "QA Staged Database"},
			},
			docFile: "escrowImport.doc.md",
		},
		{
			Key:         "escrow-ingestion",
			Name:        "Escrow Ingestion",
			Description: "Ingests staged escrow data into the live registry database",
			Queue:       temporal.QueueData,
			Category:    "data-pipeline",
			Tags:        []string{"data", "escrow", "ingest"},
			Steps: []WorkflowStep{
				{Key: "ingest-contacts", Label: "Ingest Contacts"},
				{Key: "ingest-hosts", Label: "Ingest Hosts"},
				{Key: "ingest-domains", Label: "Ingest Domains"},
				{Key: "ingest-nndns", Label: "Ingest NNDNs"},
				{Key: "link-domain-hosts", Label: "Link Domain Hosts"},
				{Key: "accredit-registrars", Label: "Accredit Registrars"},
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
				{Key: "check-eligibility", Label: "Check Eligibility"},
				{Key: "plan-cleanup", Label: "Plan Cleanup"},
				{Key: "await-confirmation", Label: "Await Confirmation"},
				{Key: "backup-assets", Label: "Backup Assets"},
				{Key: "delete-assets", Label: "Delete Assets"},
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
				{Key: "sync-iana", Label: "Sync IANA"},
				{Key: "count-registrars", Label: "Count Registrars"},
				{Key: "diff-plan", Label: "Diff & Plan"},
				{Key: "apply-creates", Label: "Apply Creates"},
				{Key: "apply-updates", Label: "Apply Updates"},
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
				{Key: "update-exchange-rates", Label: "Update Exchange Rates"},
			},
			docFile: "updateFX.doc.md",
		},
		{
			Key:          "expiry-loop",
			Name:         "Expiry Loop",
			Description:  "Processes expired and expiring domains for auto-renew or expiration",
			Queue:        temporal.QueueLifecycle,
			Category:     "lifecycle",
			Tags:         []string{"lifecycle", "domains", "expiry"},
			Scheduled:    true,
			ScheduleInfo: "Every hour",
			Steps: []WorkflowStep{
				{Key: "count-expired", Label: "Count Expired"},
				{Key: "list-expiring", Label: "List Expiring"},
				{Key: "process-auto-renew-expire", Label: "Process Auto-Renew/Expire"},
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
				{Key: "list-purgeable", Label: "List Purgeable"},
				{Key: "purge-domains", Label: "Purge Domains"},
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
				{Key: "list-restored", Label: "List Restored"},
				{Key: "unset-status", Label: "Unset Status"},
				{Key: "force-renew", Label: "Force Renew"},
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
