package activities

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func UpdateDomain(ctx context.Context, correlationID string, domain entities.Domain) (*entities.Domain, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/", BASEURL, domain.Name.String())

	// set the correlation ID
	qParams := make(map[string]string)
	qParams["correlationID"] = correlationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// Marshall the domain
	jsonData, err := json.Marshal(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal domain: %w", err)
	}

	// Set up an API client
	client := http.Client{}

	// Update the domain
	req, err := prepareRequest(ctx, "PUT", URL.String(), bytes.NewBuffer(jsonData), correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update domain: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
	}

	// Parse the result
	updatedDomain := &entities.Domain{}
	err = json.Unmarshal(body, &updatedDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return updatedDomain, nil
}
