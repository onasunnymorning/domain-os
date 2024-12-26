package workflows

import (
	"log"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExpiryLoop ref: https://www.notion.so/apex-domains/Domain-lifecycle-18200bd9d73849e6abfe2e616f1a3443?pvs=4#2e597291f85a43699422a7ac5f122bc8
func ExpiryLoop(ctx workflow.Context) error {

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

	// See if there are any domains that are expiring
	domainCount := &response.CountResult{}
	GetExpiredDomainCountError := workflow.ExecuteActivity(ctx, activities.GetExpiredDomainCount).Get(ctx, domainCount)
	if GetExpiredDomainCountError != nil {
		return GetExpiredDomainCountError
	}
	log.Println("Total domains to expiring: ", domainCount.Count)
	// If there are no domains to expire, sleep for 5 mins and check again
	if domainCount.Count == 0 {

		return nil

		// This is where we might evolve to... or not Ref: https://www.notion.so/apex-domains/Domain-lifecycle-18200bd9d73849e6abfe2e616f1a3443?pvs=4#1666c0599d538096841df47282b78990
		// workflow.Sleep(ctx, 5*time.Minute)
		// // Continue the loop
		// return ExpiryLoop(ctx)

	}

	// Get the list of domains that are expiring
	domains := []response.DomainExpiryItem{}
	GetExpiredDomainsError := workflow.ExecuteActivity(ctx, activities.ListExpiringDomains).Get(ctx, &domains)
	if GetExpiredDomainsError != nil {
		return GetExpiredDomainsError
	}

	// For each domain that is expiring, either renew or delete
	for _, domain := range domains {
		// Try and auto-renew the domain
		autoRenewErr := workflow.ExecuteActivity(ctx, activities.AutoRenewDomain, domain.Name).Get(ctx, nil)
		if autoRenewErr != nil {
			// If the domain is not eligible for auto-renew, it should be marked for deletion
			if strings.Contains(autoRenewErr.Error(), "auto renew is not enabled") {
				log.Println("Domain", domain.Name, "is not eligible for auto-renew, marking for deletion")
				softDeleteErr := workflow.ExecuteActivity(ctx, activities.MarkDomainForDeletion, domain.Name).Get(ctx, nil)
				if softDeleteErr != nil {
					log.Printf("Failed to mark domain %s for deletion: %s\n", domain.Name, softDeleteErr)
					continue
				}
				log.Println("Domain", domain.Name, "marked for deletion")
				continue
			}
			// If another error occurred, log it and continue
			log.Println("Failed to auto-renew domain", domain.Name, ":", autoRenewErr)
		}
		log.Println("Domain", domain.Name, "auto-renewed")
	}

	return nil
}
