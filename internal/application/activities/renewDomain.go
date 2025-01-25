package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

// RenewDomain takes a domain name and sends a POST request to the admin API to renew the domain.
// If force is true, it will call the /domains/{name}/renew/force endpoint instead of /domains/{name}/renew.
func RenewDomain(correlationID string, cmd commands.RenewDomainCommand, force bool) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/renew", BASEURL, cmd.Name)
	if force {
		ENDPOINT = fmt.Sprintf("%s/domains/%s/renew/force", BASEURL, cmd.Name)
	}

	// marshall the request body
	jsonData, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Set up an API client
	client := http.Client{}

	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to create URL: %w", err)
	}

	req, err := http.NewRequest("POST", URL.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}
