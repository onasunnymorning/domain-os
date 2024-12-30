package workflows

import (
	"log"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func PurgeLoop(ctx workflow.Context) error {
	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3, // 0 is unlimited retries
		NonRetryableErrorTypes: []string{"none"},
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: time.Minute,
		// Optionally provide a customized RetryPolicy.
		// Temporal retries failed Activities by default.
		RetryPolicy: retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Check if there are any domains that are purgeable
	domainCount := &response.CountResult{}
	purgeableDomainCountErr := workflow.ExecuteActivity(ctx, activities.GetPurgeableDomainCount).Get(ctx, domainCount)
	if purgeableDomainCountErr != nil {
		log.Println("Error getting purgeable domain count: ", purgeableDomainCountErr)
		return purgeableDomainCountErr
	}

	// If there are no domains to purge, exit
	if domainCount.Count == 0 {
		return nil
	}

	// Get the list of domains that are purgeable
	domains := []response.DomainExpiryItem{}
	purgeableDomainsError := workflow.ExecuteActivity(ctx, activities.ListPurgeableDomains).Get(ctx, &domains)
	if purgeableDomainsError != nil {
		log.Println("Error getting purgeable domains: ", purgeableDomainsError)
		return purgeableDomainsError
	}

	// Process the list of purgeable domains
	for _, domain := range domains {
		// Purge the domain
		purgeActivityErr := workflow.ExecuteActivity(ctx, activities.PurgeDomain, domain.Name).Get(ctx, nil)
		if purgeActivityErr != nil {
			log.Println("Error purging domain: ", domain.Name)
			return purgeActivityErr
		}
	}

	return nil

}
