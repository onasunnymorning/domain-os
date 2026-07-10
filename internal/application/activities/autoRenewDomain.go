package activities

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// AutoRenewDomain takes a domain name and sends a POST request to the admin API to auto-renew the domain.
func AutoRenewDomain(ctx context.Context, correlationID, domainName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/autorenew", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to create URL: %w", err)
	}

	req, err := prepareRequest(ctx, "POST", URL.String(), nil, correlationID)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

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
		return httpResponseError(resp, body)
	}

	return nil
}
