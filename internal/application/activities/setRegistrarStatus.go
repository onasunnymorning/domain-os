package activities

import (
	"fmt"
	"net/http"
	"strings"
)

func SetRegistrarStatus(correlationID, clid, status string) error {
	ENDPOINT := fmt.Sprintf("%s/registrars/%s/status/%s", BASEURL, clid, strings.ToLower(status))

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
	req.Header.Add("Authorization", BEARER_TOKEN)

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set registrar %s status to %s: %w", clid, status, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to set registrar status through API: %s", resp.Status)
	}

	return nil
}
