package activities

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func SyncIanaRegistrars(ctx context.Context, correlationID string) error {
	ENDPOINT := fmt.Sprintf("%s/sync/iana-registrars", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	// Create the request
	req, err := prepareRequest(ctx, "PUT", URL.String(), nil, correlationID)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync IANA registrars: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return httpResponseError(resp, body)
	}

	return nil
}
