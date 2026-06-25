package activities

import (
	"fmt"
	"io"
	"net/http"
)

// SyncSpec5 triggers the synchronization of Spec5 labels from ICANN.
func SyncSpec5(correlationID string) error {
	ENDPOINT := fmt.Sprintf("%s/sync/icann-spec5", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("PUT", URL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", GetBearerToken())

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync Spec5 labels: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	return nil
}
