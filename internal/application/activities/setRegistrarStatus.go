package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func SetRegistrarStatus(correlationID, clid, status string) error {
	ENDPOINT := fmt.Sprintf("%s/registrars/%s/status/%s", BASEURL, clid, status)

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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	var rar *entities.Registrar

	err = json.Unmarshal(body, rar)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}
