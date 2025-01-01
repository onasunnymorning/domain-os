package activities

import (
	"fmt"
	"io"
	"net/http"
)

// MarkDomainForDeletion takes a domain name and sends a DELETE request to the admin API to mark the domain for deletion. This starts the end-of-life process for the domain. It does NOT delete the domain immediately.
func MarkDomainForDeletion(domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/markdelete", BASEURL, domainName)

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
		return fmt.Errorf("failed to mark domain for deletion: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to mark domain for deletion (%d): %s", resp.StatusCode, body)
	}

	return nil
}
