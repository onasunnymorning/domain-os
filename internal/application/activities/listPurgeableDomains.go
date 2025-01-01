package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// ListPurgeableDomains takes an PurgeableDomainsQuery and returns a list of domains that have PendingDelete set and are past the grace period (PurgeDate is in the past or before the supplied date). It gets these through the admin API.
func ListPurgeableDomains(query queries.PurgeableDomainsQuery) ([]response.DomainExpiryItem, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/purgeable", BASEURL)

	endpointURL, err := url.Parse(ENDPOINT)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Add the query parameters
	q := endpointURL.Query()
	q.Set("pagesize", fmt.Sprintf("%d", BATCHSIZE))
	endpointURL.RawQuery = q.Encode()

	// Set up an API client
	client := http.Client{}

	// Retrieve the list of domains
	req, err := http.NewRequest("GET", endpointURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

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
		return nil, fmt.Errorf("failed to fetch domain count (%d): %s", resp.StatusCode, body)
	}

	// Parse the result
	listResponse := &ListItemResult{}
	err = json.Unmarshal(body, &listResponse)
	if err != nil {
		return nil, errors.Join(errors.New("failed to unmarshal response"), err)
	}

	return listResponse.Data, nil
}
