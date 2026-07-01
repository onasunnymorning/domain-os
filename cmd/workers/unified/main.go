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

	// --- Workers Setup ---

	// 1. fast-ops worker (queue: fast-ops)
	// Low-latency, sub-minute activities.
	fastOpsWorker := worker.New(client, temporal.QueueFastOps, worker.Options{
		MaxConcurrentWorkflowTaskPollers:   5,
		MaxConcurrentActivityTaskPollers:   5,
		MaxConcurrentActivityExecutionSize: 50,
	})

	// 2. scheduled worker (queue: scheduled)
	// Periodic background work.
	scheduledWorker := worker.New(client, temporal.QueueScheduled, worker.Options{
		MaxConcurrentWorkflowTaskPollers:   4,
		MaxConcurrentActivityTaskPollers:   4,
		MaxConcurrentActivityExecutionSize: 20,
	})

	// 3. heavy-batch worker (queue: heavy-batch)
	// Multi-hour, resource-intensive operations.
	heavyBatchWorker := worker.New(client, temporal.QueueHeavyBatch, worker.Options{
		MaxConcurrentWorkflowTaskPollers:   3,
		MaxConcurrentActivityTaskPollers:   3,
		MaxConcurrentActivityExecutionSize: 5,
	})

	// 4. lifecycle worker (queue: lifecycle)
	// Domain state-machine transitions.
	lifecycleWorker := worker.New(client, temporal.QueueLifecycle, worker.Options{
		MaxConcurrentWorkflowTaskPollers:   4,
		MaxConcurrentActivityTaskPollers:   4,
		MaxConcurrentActivityExecutionSize: 30,
	})

	// 5. drainDataWorker (queue: data-pipeline - DEPRECATED)
	// Temporary worker to drain in-flight workflows on the old data queue.
	drainDataWorker := worker.New(client, temporal.QueueData, worker.Options{
		MaxConcurrentWorkflowTaskPollers: 2,
		MaxConcurrentActivityTaskPollers: 2,
	})

	// 6. drainLifecycleWorker (queue: object-lifecycle - DEPRECATED)
	// Temporary worker to drain in-flight workflows on the old lifecycle queue.
	drainLifecycleWorker := worker.New(client, temporal.QueueLifecycleDeprecated, worker.Options{
		MaxConcurrentWorkflowTaskPollers: 2,
		MaxConcurrentActivityTaskPollers: 2,
	})

	// --- Workflows Registration ---

	// Fast Ops
	fastOpsWorker.RegisterWorkflow(workflows.CheckSerialDriftWorkflow)
	fastOpsWorker.RegisterWorkflow(workflows.UpdateFX)

	// Scheduled
	scheduledWorker.RegisterWorkflow(workflows.SyncSpec5Workflow)
	scheduledWorker.RegisterWorkflow(workflows.EventRelay)
	scheduledWorker.RegisterWorkflow(workflows.EventPrune)
	scheduledWorker.RegisterWorkflow(workflows.Spec5SweepWorkflow)
	scheduledWorker.RegisterWorkflow(workflows.SyncRegistrarsWorkflow)

	// Heavy Batch
	heavyBatchWorker.RegisterWorkflow(workflows.EscrowImportWorkflow)
	heavyBatchWorker.RegisterWorkflow(workflows.TLDCleanupWorkflow)
	heavyBatchWorker.RegisterWorkflow(workflows.TakeSnapshotWorkflow)
	heavyBatchWorker.RegisterWorkflow(workflows.SeedFromSnapshotWorkflow)

	// Lifecycle
	lifecycleWorker.RegisterWorkflow(workflows.ExpiryLoop)
	lifecycleWorker.RegisterWorkflow(workflows.PurgeLoop)
	lifecycleWorker.RegisterWorkflow(workflows.RestoreWorkflow)
	lifecycleWorker.RegisterWorkflow(workflows.TombstoneBackfill)

	// Drain Data (Deprecated data-pipeline)
	drainDataWorker.RegisterWorkflow(workflows.EscrowImportWorkflow)
	drainDataWorker.RegisterWorkflow(workflows.TLDCleanupWorkflow)
	drainDataWorker.RegisterWorkflow(workflows.UpdateFX)
	drainDataWorker.RegisterWorkflow(workflows.SyncSpec5Workflow)
	drainDataWorker.RegisterWorkflow(workflows.TakeSnapshotWorkflow)
	drainDataWorker.RegisterWorkflow(workflows.SeedFromSnapshotWorkflow)
	drainDataWorker.RegisterWorkflow(workflows.Spec5SweepWorkflow)
	drainDataWorker.RegisterWorkflow(workflows.EventRelay)
	drainDataWorker.RegisterWorkflow(workflows.EventPrune)
	drainDataWorker.RegisterWorkflow(workflows.TombstoneBackfill)
	drainDataWorker.RegisterWorkflow(workflows.CheckSerialDriftWorkflow)

	// Drain Lifecycle (Deprecated object-lifecycle)
	drainLifecycleWorker.RegisterWorkflow(workflows.ExpiryLoop)
	drainLifecycleWorker.RegisterWorkflow(workflows.PurgeLoop)
	drainLifecycleWorker.RegisterWorkflow(workflows.RestoreWorkflow)
	drainLifecycleWorker.RegisterWorkflow(workflows.SyncRegistrarsWorkflow)

	// --- Activities Registration ---

	// 1. Basic/Standalone Activities
	
	// UpdateFX (Fast Ops + Drain Data)
	fastOpsWorker.RegisterActivity(activities.UpdateFX)
	drainDataWorker.RegisterActivity(activities.UpdateFX)

	// SyncSpec5 (Scheduled + Drain Data)
	scheduledWorker.RegisterActivity(activities.SyncSpec5)
	drainDataWorker.RegisterActivity(activities.SyncSpec5)

	// Escrow Import (Heavy Batch + Drain Data)
	heavyBatchWorker.RegisterActivity(&activities.EscrowImportActivities{})
	drainDataWorker.RegisterActivity(&activities.EscrowImportActivities{})

	// Registrar & Basic Lifecycle activities
	// These are split between Scheduled/Lifecycle (new) and Drain Lifecycle (deprecated)
	
	// Scheduled Registrar Activities
	registrarActs := []interface{}{
		activities.SyncIanaRegistrars,
		activities.GetICANNRegistrars,
		activities.CountRegistrars,
		activities.GetIANARegistrars,
		activities.DiffAndPlanRegistrars,
		activities.MakeCreateRegistrarCommands,
		activities.SetRegistrarStatus,
		activities.SetRegistrarIANAStatus,
		activities.BulkUpdateRegistrarStatuses,
		activities.GetRegistrarListItems,
		activities.CreateRegistrar,
		activities.BulkCreateRegistrars,
	}
	for _, act := range registrarActs {
		scheduledWorker.RegisterActivity(act)
		drainLifecycleWorker.RegisterActivity(act)
	}

	// Lifecycle Basic Activities
	lifecycleBasicActs := []interface{}{
		activities.CheckDomainCanAutoRenew,
		activities.CheckDomainsCanAutoRenew,
		activities.GetExpiredDomainCount,
		activities.ListExpiringDomains,
		activities.AutoRenewDomain,
		activities.ExpireDomain,
		activities.PurgeDomain,
		activities.GetPurgeableDomainCount,
		activities.ListPurgeableDomains,
		activities.ListRestoredDomains,
		activities.GetDomain,
		activities.RenewDomain,
		activities.SetDomainStatus,
		activities.UnSetDomainStatus,
	}
	for _, act := range lifecycleBasicActs {
		lifecycleWorker.RegisterActivity(act)
		drainLifecycleWorker.RegisterActivity(act)
	}

	// 2. Resource-Dependent Activities (requiring DB and/or S3)

	// Lifecycle Batch Activities (Lifecycle + Drain Lifecycle)
	lifecycleActs, err := activities.NewLifecycleActivities()
	if err != nil {
		log.Printf("WARNING: Lifecycle batch activities not available (DB not configured): %v", err)
	} else {
		lifecycleWorker.RegisterActivity(lifecycleActs)
		drainLifecycleWorker.RegisterActivity(lifecycleActs)
	}

	// TLD Cleanup Activities (Heavy Batch + Drain Data)
	tldActs, err := activities.NewTLDCleanupActivities()
	if err != nil {
		log.Printf("WARNING: TLD cleanup activities not available (DB/S3 not configured): %v", err)
	} else {
		heavyBatchWorker.RegisterActivity(tldActs.CheckTLDCanBeDeleted)
		heavyBatchWorker.RegisterActivity(tldActs.PlanTLDCleanup)
		heavyBatchWorker.RegisterActivity(tldActs.BackupTLDAssets)
		heavyBatchWorker.RegisterActivity(tldActs.DeleteTLDAssets)

		drainDataWorker.RegisterActivity(tldActs.CheckTLDCanBeDeleted)
		drainDataWorker.RegisterActivity(tldActs.PlanTLDCleanup)
		drainDataWorker.RegisterActivity(tldActs.BackupTLDAssets)
		drainDataWorker.RegisterActivity(tldActs.DeleteTLDAssets)
	}

	// Snapshot Activities (Heavy Batch + Drain Data)
	snapActs, err := activities.NewSnapshotActivities()
	if err != nil {
		log.Printf("WARNING: Snapshot activities not available (DB/S3 not configured): %v", err)
	} else {
		heavyBatchWorker.RegisterActivity(snapActs.TakeSnapshot)
		heavyBatchWorker.RegisterActivity(snapActs.ValidateSnapshot)
		heavyBatchWorker.RegisterActivity(snapActs.SeedFromSnapshot)
		heavyBatchWorker.RegisterActivity(snapActs.ListSnapshots)

		drainDataWorker.RegisterActivity(snapActs.TakeSnapshot)
		drainDataWorker.RegisterActivity(snapActs.ValidateSnapshot)
		drainDataWorker.RegisterActivity(snapActs.SeedFromSnapshot)
		drainDataWorker.RegisterActivity(snapActs.ListSnapshots)
	}

	// Spec5 Sweep Activities (Scheduled + Drain Data)
	spec5SweepActs, err := activities.NewSpec5SweepActivities()
	if err != nil {
		log.Printf("WARNING: Spec5 sweep activities not available (DB/S3 not configured): %v", err)
	} else {
		scheduledWorker.RegisterActivity(spec5SweepActs.SweepSpec5Labels)
		drainDataWorker.RegisterActivity(spec5SweepActs.SweepSpec5Labels)
	}

	// Event Relay Activities (Scheduled + Drain Data)
	eventRelayActs, err := activities.NewEventRelayActivities()
	if err != nil {
		log.Printf("WARNING: Event relay activities not available (DB/S3 not configured): %v", err)
	} else {
		scheduledWorker.RegisterActivity(eventRelayActs)
		drainDataWorker.RegisterActivity(eventRelayActs)
	}

	// Tombstone Backfill Activities (Lifecycle + Drain Data)
	backfillActs, err := activities.NewTombstoneBackfillActivities()
	if err != nil {
		log.Printf("WARNING: Tombstone backfill activities not available (DB not configured): %v", err)
	} else {
		lifecycleWorker.RegisterActivity(backfillActs)
		drainDataWorker.RegisterActivity(backfillActs)
	}

	// Serial Drift Activities (Fast Ops + Drain Data)
	driftActs, err := activities.NewSerialDriftActivities()
	if err != nil {
		log.Printf("WARNING: Serial drift activities not available (DB not configured): %v", err)
	} else {
		fastOpsWorker.RegisterActivity(driftActs)
		drainDataWorker.RegisterActivity(driftActs)
	}

	// Start all workers concurrently. worker.InterruptCh() returns a channel
	// that is closed on SIGINT/SIGTERM, which gracefully stops all workers.
	interruptCh := worker.InterruptCh()

	errCh := make(chan error, 6)
	go func() { errCh <- fastOpsWorker.Run(interruptCh) }()
	go func() { errCh <- scheduledWorker.Run(interruptCh) }()
	go func() { errCh <- heavyBatchWorker.Run(interruptCh) }()
	go func() { errCh <- lifecycleWorker.Run(interruptCh) }()
	go func() { errCh <- drainDataWorker.Run(interruptCh) }()
	go func() { errCh <- drainLifecycleWorker.Run(interruptCh) }()

	// Wait for any worker to exit (error or interrupt)
	if err := <-errCh; err != nil {
		log.Fatalln("worker exited with error:", err)
	}
}
