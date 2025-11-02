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

	// Register escrow import workflow and activities
	w.RegisterWorkflow(workflows.EscrowImportWorkflow)
	w.RegisterActivity(&activities.EscrowImportActivities{})

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
