package activities

import (
	"fmt"
	"io"
	"net/http"
)

// AutoRenewDomain takes a domain name and sends a POST request to the admin API to auto-renew the domain.
func AutoRenewDomain(domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/autorenew", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	req, err := http.NewRequest("POST", ENDPOINT, nil)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}
