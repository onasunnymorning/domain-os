package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetExpiredDomainCount takes a ExpiringDomainsQuery and returns the number of domains that have expired and are past the grace period (ExpiryDate is in the past or before the supplied date). It gets these through the admin API.
func GetExpiredDomainCount(query queries.ExpiringDomainsQuery) (*response.CountResult, error) {
	COUNT_ENDPOINT := fmt.Sprintf("%s/domains/expiring/count", BASEURL)

	client := http.Client{}

	req, err := http.NewRequest("GET", COUNT_ENDPOINT, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

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
		return nil, fmt.Errorf("failed to fetch domain count (%d): %s", resp.StatusCode, string(body))
	}

	countResponse := &response.CountResult{}
	if err := json.Unmarshal(body, countResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return countResponse, nil
}
