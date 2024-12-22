package workflows

import (
	"fmt"
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

	domainCount := &response.CountResult{}
	GetExpiredDomainCountError := workflow.ExecuteActivity(ctx, activities.GetExpiredDomainCount).Get(ctx, domainCount)
	if GetExpiredDomainCountError != nil {
		return GetExpiredDomainCountError
	}

	fmt.Println("Total domains to expiring: ", domainCount.Count)

	return nil
}
