package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func GetDomain(domainName string) (*entities.Domain, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/%s", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	// Delete the domain
	req, err := http.NewRequest("GET", ENDPOINT, nil)
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
