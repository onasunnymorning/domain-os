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

// GetExpiredDomainCount takes a ExpiringDomainsQuery and returns the number of domains that have expired and are past the grace period (ExpiryDate is in the past or before the supplied date). It gets these through the admin API.
func GetExpiredDomainCount(ctx context.Context, correlationID string, query queries.ExpiringDomainsQuery) (*response.CountResult, error) {
	COUNT_ENDPOINT := fmt.Sprintf("%s/domains/expiring/count", BASEURL)

	client := http.Client{}

	// Set up query parameters
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	if !query.Before.IsZero() {
		qParams["before"] = query.Before.Format(time.RFC3339)
	}
	URL, err := getURLAndSetQueryParams(COUNT_ENDPOINT, qParams)

	req, err := prepareRequest(ctx, "GET", URL.String(), nil, correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
	}

	countResponse := &response.CountResult{}
	if err := json.Unmarshal(body, countResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return countResponse, nil
}
