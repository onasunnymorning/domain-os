package activities

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// PurgeDomain purges (deletes) a domain from the system.
func PurgeDomain(ctx context.Context, correlationID, domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/purge", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID and drophosts = true
	qParams := make(map[string]string)
	qParams["correlationID"] = correlationID
	qParams["drophosts"] = "true"
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	// Delete the domain
	req, err := prepareRequest(ctx, "DELETE", URL.String(), nil, correlationID)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

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
		return httpResponseError(resp, body)
	}

	return nil
}
