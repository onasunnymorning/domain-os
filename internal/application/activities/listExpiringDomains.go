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

var (
	BATCHSIZE = 1000
)

// ListExpiringDomains takes an ExpiringDomainsQuery and returns a list of domains that are expiring before the given date. It gets these through the admin API.
func ListExpiringDomains(query queries.ExpiringDomainsQuery) ([]response.DomainExpiryItem, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/expiring", BASEURL)

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

	// get a list of domains that have expired
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
	defer resp.Body.Close()

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

type ListItemResult struct {
	Meta response.PaginationMetaData `json:"meta"`
	Data []response.DomainExpiryItem `json:"data"`
}
