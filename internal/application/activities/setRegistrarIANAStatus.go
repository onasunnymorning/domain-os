package activities

import (
	"fmt"
	"net/http"
)

// SetRegistrarIANAStatus updates the IANA status of a registrar via REST endpoint.
// If the registrar is not found (404), this is treated as a no-op and not an error.
func SetRegistrarIANAStatus(correlationID, clid, ianaStatus string) error {
	ENDPOINT := fmt.Sprintf("%s/registrars/%s/iana_status/%s", BASEURL, clid, ianaStatus)

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
		return fmt.Errorf("failed to set registrar %s IANA status to %s: %w", clid, ianaStatus, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Treat missing registrar as no-op
		return nil
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to set registrar IANA status through API: %s", resp.Status)
	}

	return nil
}
