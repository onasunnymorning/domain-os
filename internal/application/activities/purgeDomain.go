package activities

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// PurgeDomain purges (deletes) a domain from the system.
func PurgeDomain(domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	endpointURL, err := url.Parse(ENDPOINT)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Add the query parameters
	q := endpointURL.Query()
	q.Set("drophosts", "true")
	endpointURL.RawQuery = q.Encode()

	// Delete the domain
	req, err := http.NewRequest("DELETE", endpointURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to purge domain: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	return nil
}
