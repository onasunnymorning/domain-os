package main

import (
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create a shared Temporal client
	cfg := temporal.TemporalClientconfig{
		HostPort:   os.Getenv("TMPIO_HOST_PORT"),
		Namespace:  os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:  os.Getenv("TMPIO_KEY"),
		ClientCert: os.Getenv("TMPIO_CERT"),
		APIKey:     os.Getenv("TMPIO_API_KEY"),
	}

	client, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer client.Close()

	// --- Domain Lifecycle Worker (queue: TMPIO_QUEUE) ---
	lifecycleQueue := os.Getenv("TMPIO_QUEUE")
	if lifecycleQueue == "" {
		lifecycleQueue = "domain-lifecycle"
	}
	lifecycleWorker := worker.New(client, lifecycleQueue, worker.Options{})

	lifecycleWorker.RegisterWorkflow(workflows.ExpiryLoop)
	lifecycleWorker.RegisterWorkflow(workflows.PurgeLoop)
	lifecycleWorker.RegisterWorkflow(workflows.RestoreWorkflow)
	lifecycleWorker.RegisterWorkflow(workflows.SyncRegistrarsWorkflow)

	lifecycleWorker.RegisterActivity(activities.CheckDomainCanAutoRenew)
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
	lifecycleWorker.RegisterActivity(activities.UnSetDomainStatus)
	lifecycleWorker.RegisterActivity(activities.SyncIanaRegistrars)
	lifecycleWorker.RegisterActivity(activities.GetICANNRegistrars)
	lifecycleWorker.RegisterActivity(activities.CountRegistrars)
	lifecycleWorker.RegisterActivity(activities.GetIANARegistrars)
	lifecycleWorker.RegisterActivity(activities.DiffAndPlanRegistrars)
	lifecycleWorker.RegisterActivity(activities.MakeCreateRegistrarCommands)
	lifecycleWorker.RegisterActivity(activities.SetRegistrarStatus)
	lifecycleWorker.RegisterActivity(activities.GetRegistrarListItems)
	lifecycleWorker.RegisterActivity(activities.CreateRegistrar)

	// --- Escrow Import Worker (queue: ESCROW_QUEUE) ---
	escrowQueue := os.Getenv("ESCROW_QUEUE")
	if escrowQueue == "" {
		escrowQueue = "escrow-import"
	}
	escrowWorker := worker.New(client, escrowQueue, worker.Options{})

	escrowWorker.RegisterWorkflow(workflows.EscrowStagingWorkflow)
	escrowWorker.RegisterWorkflow(workflows.EscrowIngestionWorkflow)
	escrowWorker.RegisterWorkflow(workflows.TLDCleanupWorkflow)

	escrowWorker.RegisterActivity(&activities.EscrowImportActivities{})

	// TLD Cleanup activities require DB + S3; gracefully skip if unavailable
	tldActs, err := activities.NewTLDCleanupActivities()
	if err != nil {
		log.Printf("WARNING: TLD cleanup activities not available (DB/S3 not configured): %v", err)
	} else {
		escrowWorker.RegisterActivity(tldActs.CheckTLDCanBeDeleted)
		escrowWorker.RegisterActivity(tldActs.PlanTLDCleanup)
		escrowWorker.RegisterActivity(tldActs.BackupTLDAssets)
		escrowWorker.RegisterActivity(tldActs.DeleteTLDAssets)
	}

	// --- Sync Worker (queue: TMPIO_SYNC_QUEUE) ---
	syncQueue := os.Getenv("TMPIO_SYNC_QUEUE")
	if syncQueue == "" {
		syncQueue = "sync"
	}
	syncWorker := worker.New(client, syncQueue, worker.Options{})

	syncWorker.RegisterWorkflow(workflows.UpdateFX)
	syncWorker.RegisterActivity(activities.UpdateFX)

	// Start all workers concurrently. worker.InterruptCh() returns a channel
	// that is closed on SIGINT/SIGTERM, which gracefully stops all workers.
	interruptCh := worker.InterruptCh()

	errCh := make(chan error, 3)
	go func() { errCh <- lifecycleWorker.Run(interruptCh) }()
	go func() { errCh <- escrowWorker.Run(interruptCh) }()
	go func() { errCh <- syncWorker.Run(interruptCh) }()

	// Wait for any worker to exit (error or interrupt)
	if err := <-errCh; err != nil {
		log.Fatalln("worker exited with error:", err)
	}
}
