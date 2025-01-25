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

	// Register the workflows
	w.RegisterWorkflow(workflows.ExpiryLoop)
	w.RegisterWorkflow(workflows.PurgeLoop)
	w.RegisterWorkflow(workflows.RestoreWorkflow)

	// Register the activities (ExpiryLoop)
	w.RegisterActivity(activities.CheckDomainCanAutoRenew)
	w.RegisterActivity(activities.GetExpiredDomainCount)
	w.RegisterActivity(activities.ListExpiringDomains)
	w.RegisterActivity(activities.AutoRenewDomain)
	w.RegisterActivity(activities.ExpireDomain)

	// Register the activities (PurgeLoop)
	w.RegisterActivity(activities.PurgeDomain)
	w.RegisterActivity(activities.GetPurgeableDomainCount)
	w.RegisterActivity(activities.ListPurgeableDomains)

	// Register the activities (RestoreWorkflow)
	w.RegisterActivity(activities.ListRestoredDomains)
	w.RegisterActivity(activities.GetDomain)
	w.RegisterActivity(activities.RenewDomain)
	w.RegisterActivity(activities.UnSetDomainStatus)

	// Start listening to the Task Queue.
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("unable to start Worker", err)
	}

}
