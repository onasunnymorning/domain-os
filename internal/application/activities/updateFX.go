package activities

import (
	"fmt"
	"io"
	"net/http"
)

// UpdateFX updates the FX rate for a given currency.
func UpdateFX(correlationID, cur string) error {
	ENDPOINT := fmt.Sprintf("%s/sync/fx/%s", BASEURL, cur)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	// Get the FX rate
	req, err := http.NewRequest("PUT", URL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get FX rate: %w", err)
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
