package main

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/bootstrap"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create a shared Temporal client
	cfg := temporal.NewClientConfigFromEnv("")

	client, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer client.Close()

	// Self-healing infrastructure: ensure all schedules exist.
	// Idempotent — safe to run on every startup/deploy.
	bootstrap.EnsureTemporalInfrastructure(client)

	// --- Object Lifecycle Worker (queue: object-lifecycle) ---
	lifecycleWorker := worker.New(client, temporal.QueueLifecycle, worker.Options{})

	lifecycleWorker.RegisterWorkflow(workflows.ExpiryLoop)
	lifecycleWorker.RegisterWorkflow(workflows.PurgeLoop)
	lifecycleWorker.RegisterWorkflow(workflows.RestoreWorkflow)
	lifecycleWorker.RegisterWorkflow(workflows.SyncRegistrarsWorkflow)

	lifecycleWorker.RegisterActivity(activities.CheckDomainCanAutoRenew)
	lifecycleWorker.RegisterActivity(activities.CheckDomainsCanAutoRenew)
	lifecycleWorker.RegisterActivity(activities.GetExpiredDomainCount)
	lifecycleWorker.RegisterActivity(activities.ListExpiringDomains)
	lifecycleWorker.RegisterActivity(activities.AutoRenewDomain)
	lifecycleWorker.RegisterActivity(activities.ExpireDomain)
	lifecycleWorker.RegisterActivity(activities.PurgeDomain)
	lifecycleWorker.RegisterActivity(activities.GetPurgeableDomainCount)
	lifecycleWorker.RegisterActivity(activities.ListPurgeableDomains)
	lifecycleWorker.RegisterActivity(activities.ListRestoredDomains)
	lifecycleWorker.RegisterActivity(activities.GetDomain)
	lifecycleWorker.RegisterActivity(activities.RenewDomain)
	lifecycleWorker.RegisterActivity(activities.SetDomainStatus)
	lifecycleWorker.RegisterActivity(activities.UnSetDomainStatus)
	lifecycleWorker.RegisterActivity(activities.SyncIanaRegistrars)
	lifecycleWorker.RegisterActivity(activities.GetICANNRegistrars)
	lifecycleWorker.RegisterActivity(activities.CountRegistrars)
	lifecycleWorker.RegisterActivity(activities.GetIANARegistrars)
	lifecycleWorker.RegisterActivity(activities.DiffAndPlanRegistrars)
	lifecycleWorker.RegisterActivity(activities.MakeCreateRegistrarCommands)
	lifecycleWorker.RegisterActivity(activities.SetRegistrarStatus)
	lifecycleWorker.RegisterActivity(activities.SetRegistrarIANAStatus)
	lifecycleWorker.RegisterActivity(activities.BulkUpdateRegistrarStatuses)
	lifecycleWorker.RegisterActivity(activities.GetRegistrarListItems)
	lifecycleWorker.RegisterActivity(activities.CreateRegistrar)
	lifecycleWorker.RegisterActivity(activities.BulkCreateRegistrars)

	// --- Data Pipeline Worker (queue: data-pipeline) ---
	// Handles escrow staging/ingestion, TLD cleanup, and FX rate updates.
	dataWorker := worker.New(client, temporal.QueueData, worker.Options{})

	dataWorker.RegisterWorkflow(workflows.EscrowImportWorkflow)
	dataWorker.RegisterWorkflow(workflows.TLDCleanupWorkflow)
	dataWorker.RegisterWorkflow(workflows.UpdateFX)
	dataWorker.RegisterWorkflow(workflows.SyncSpec5Workflow)

	dataWorker.RegisterActivity(&activities.EscrowImportActivities{})
	dataWorker.RegisterActivity(activities.UpdateFX)
	dataWorker.RegisterActivity(activities.SyncSpec5)

	// TLD Cleanup activities require DB + S3; gracefully skip if unavailable
	tldActs, err := activities.NewTLDCleanupActivities()
	if err != nil {
		log.Printf("WARNING: TLD cleanup activities not available (DB/S3 not configured): %v", err)
	} else {
		dataWorker.RegisterActivity(tldActs.CheckTLDCanBeDeleted)
		dataWorker.RegisterActivity(tldActs.PlanTLDCleanup)
		dataWorker.RegisterActivity(tldActs.BackupTLDAssets)
		dataWorker.RegisterActivity(tldActs.DeleteTLDAssets)
	}

	// Snapshot workflows (Take Snapshot + Seed from Snapshot)
	dataWorker.RegisterWorkflow(workflows.TakeSnapshotWorkflow)
	dataWorker.RegisterWorkflow(workflows.SeedFromSnapshotWorkflow)

	// Snapshot activities require DB + S3; gracefully skip if unavailable
	snapActs, err := activities.NewSnapshotActivities()
	if err != nil {
		log.Printf("WARNING: Snapshot activities not available (DB/S3 not configured): %v", err)
	} else {
		dataWorker.RegisterActivity(snapActs.TakeSnapshot)
		dataWorker.RegisterActivity(snapActs.ValidateSnapshot)
		dataWorker.RegisterActivity(snapActs.SeedFromSnapshot)
		dataWorker.RegisterActivity(snapActs.ListSnapshots)
	}

	// Spec5 Sweep (workflow + activity)
	dataWorker.RegisterWorkflow(workflows.Spec5SweepWorkflow)
	spec5SweepActs, err := activities.NewSpec5SweepActivities()
	if err != nil {
		log.Printf("WARNING: Spec5 sweep activities not available (DB/S3 not configured): %v", err)
	} else {
		dataWorker.RegisterActivity(spec5SweepActs.SweepSpec5Labels)
	}

	// Start all workers concurrently. worker.InterruptCh() returns a channel
	// that is closed on SIGINT/SIGTERM, which gracefully stops all workers.
	interruptCh := worker.InterruptCh()

	errCh := make(chan error, 2)
	go func() { errCh <- lifecycleWorker.Run(interruptCh) }()
	go func() { errCh <- dataWorker.Run(interruptCh) }()

	// Wait for any worker to exit (error or interrupt)
	if err := <-errCh; err != nil {
		log.Fatalln("worker exited with error:", err)
	}
}
