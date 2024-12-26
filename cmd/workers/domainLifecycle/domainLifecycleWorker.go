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
	// Create a Temporal client Config

	cfg := temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: os.Getenv("TMPIO_QUEUE"),
	}

	// Create a Temporal client
	client, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}

	// Create the ExpiryLoop worker
	w := worker.New(client, cfg.WorkerQueue, worker.Options{})

	// Register the ExpiryLoop workflow
	w.RegisterWorkflow(workflows.ExpiryLoop)
	w.RegisterWorkflow(workflows.PurgeLoop)

	// Register the activities
	w.RegisterActivity(activities.GetExpiredDomainCount)
	w.RegisterActivity(activities.ListExpiringDomains)
	w.RegisterActivity(activities.AutoRenewDomain)
	w.RegisterActivity(activities.MarkDomainForDeletion)
	w.RegisterActivity(activities.PurgeDomain)

	// Start listening to the Task Queue.
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("unable to start Worker", err)
	}

}
