package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetPurgeableDomainCount takes a PurgeableDomainsQuery and returns the number of domains that have expired and are past the grace period (ExpiryDate is in the past or before the supplied date). It gets these through the admin API.
func GetPurgeableDomainCount(correlationID string, query queries.PurgeableDomainsQuery) (*response.CountResult, error) {
	// COUNT_ENDPOINT := fmt.Sprintf("http://%s:%s/domains/expiring/count", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	COUNT_ENDPOINT := fmt.Sprintf("%s/domains/purgeable/count", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := make(map[string]string)
	qParams["correlationID"] = correlationID
	URL, err := getURLAndSetQueryParams(COUNT_ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// check the total amount of domains to renew
	req, err := http.NewRequest("GET", URL.String(), nil)
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
	countResponse := &response.CountResult{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return countResponse, nil
}
