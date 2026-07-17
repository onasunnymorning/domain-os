package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetPurgeableDomainCount takes a PurgeableDomainsQuery and returns the number of domains that have PendingDelete set and whose purge date falls on or before the query cutoff (Before). It gets these through the admin API.
func GetPurgeableDomainCount(ctx context.Context, correlationID string, query queries.PurgeableDomainsQuery) (*response.CountResult, error) {
	COUNT_ENDPOINT := fmt.Sprintf("%s/domains/purgeable/count", BASEURL)

	// Set up an API client
	client := http.Client{}

	// Serialize the full query so count and list evaluate the same cutoff.
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	if !query.Before.IsZero() {
		qParams["before"] = query.Before.Format(time.RFC3339)
	}
	if query.ClID.String() != "" {
		qParams["clid"] = query.ClID.String()
	}
	if query.TLD.String() != "" {
		qParams["tld"] = query.TLD.String()
	}
	URL, err := getURLAndSetQueryParams(COUNT_ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// check the total amount of domains to purge
	req, err := prepareRequest(ctx, "GET", URL.String(), nil, correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domain count: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
	}

	// Parse the result
	countResponse := &response.CountResult{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return countResponse, nil
}
