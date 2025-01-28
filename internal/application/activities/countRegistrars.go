package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

func CountRegistrars(correlationID string) (*response.CountResult, error) {
	ENDPOINT := fmt.Sprintf("%s/registrars/count", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get registrar count: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	var countResult *response.CountResult

	err = json.Unmarshal(body, countResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return countResult, nil
}
