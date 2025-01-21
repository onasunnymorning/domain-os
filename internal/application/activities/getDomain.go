package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// GetDomain retrieves a domain entity based on the provided domain name.
// It constructs an API endpoint URL, sets the necessary query parameters (adds correlation-id), and makes an HTTP GET request
// to fetch the domain details. The response is then unmarshaled into an entities.Domain object.
func GetDomain(correlationID, domainName string) (*entities.Domain, error) {
	if domainName == "" {
		return nil, fmt.Errorf("domain name cannot be empty")
	}
	ENDPOINT := fmt.Sprintf("%s/domains/%s", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	// Set query parameters
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to set query parameters: %w", err)
	}

	// Delete the domain
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domain: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	var domain entities.Domain
	if err := json.Unmarshal(body, &domain); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return &domain, nil

}
