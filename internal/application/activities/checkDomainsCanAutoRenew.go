package activities

import (
	"sync"
)

// CheckFailure records an eligibility check failure for a single domain.
type CheckFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

// CheckDomainsCanAutoRenewResult represents the results of the batch checking process.
type CheckDomainsCanAutoRenewResult struct {
	EligibleForAutoRenew []string       `json:"eligibleForAutoRenew"`
	EligibleForExpiry    []string       `json:"eligibleForExpiry"`
	CheckFailures        []CheckFailure `json:"checkFailures"`
}

// CheckDomainsCanAutoRenew checks the auto-renew eligibility for multiple domains concurrently.
func CheckDomainsCanAutoRenew(correlationID string, domainNames []string) (CheckDomainsCanAutoRenewResult, error) {
	var result CheckDomainsCanAutoRenewResult
	result.EligibleForAutoRenew = []string{}
	result.EligibleForExpiry = []string{}
	result.CheckFailures = []CheckFailure{}

	if len(domainNames) == 0 {
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to 20 concurrent HTTP requests
	sem := make(chan struct{}, 20)

	for _, name := range domainNames {
		wg.Add(1)
		sem <- struct{}{}

		go func(domain string) {
			defer wg.Done()
			defer func() { <-sem }()

			canAutoRenew, err := CheckDomainCanAutoRenew(correlationID, domain)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.CheckFailures = append(result.CheckFailures, CheckFailure{
					DomainName: domain,
					Error:      err.Error(),
				})
			} else if canAutoRenew {
				result.EligibleForAutoRenew = append(result.EligibleForAutoRenew, domain)
			} else {
				result.EligibleForExpiry = append(result.EligibleForExpiry, domain)
			}
		}(name)
	}

	wg.Wait()
	return result, nil
}
