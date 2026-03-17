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
	cfg := temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: getQueue(),
	}

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}

	w := worker.New(cli, cfg.WorkerQueue, worker.Options{})

	tldActs, err := activities.NewTLDCleanupActivities()
	if err != nil {
		log.Fatalln("unable to initialize TLD cleanup activities", err)
	}

	w.RegisterWorkflow(workflows.EscrowStagingWorkflow)
	w.RegisterWorkflow(workflows.EscrowIngestionWorkflow)
	w.RegisterWorkflow(workflows.TLDCleanupWorkflow)

	w.RegisterActivity(&activities.EscrowImportActivities{})
	w.RegisterActivity(tldActs.CheckTLDCanBeDeleted)
	w.RegisterActivity(tldActs.PlanTLDCleanup)
	w.RegisterActivity(tldActs.BackupTLDAssets)
	w.RegisterActivity(tldActs.DeleteTLDAssets)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("unable to start Worker", err)
	}
}

func getQueue() string {
	q := os.Getenv("ESCROW_QUEUE")
	if q == "" {
		return "escrow-import"
	}
	return q
}
