package activities

import (
	"fmt"
	"io"
	"net/http"
)

// ExpireDomain takes a domain name and sends a DELETE request to the admin API to expire the domain for deletion. This starts the end-of-life process for the domain. It does NOT delete the domain immediately.
func ExpireDomain(domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/expire", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	// Request the domain be marked for deletion
	req, err := http.NewRequest("DELETE", ENDPOINT, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}
