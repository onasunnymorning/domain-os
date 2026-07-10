package activities

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func SetDomainStatus(ctx context.Context, cmd commands.ToggleDomainStatusCommand) (*entities.Domain, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/status/%s", BASEURL, cmd.DomainName, cmd.Status)

	// marshall the request body
	jsonData, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Set up an API client
	client := http.Client{}

	qParams := make(map[string]string)
	qParams["correlation_id"] = cmd.CorrelationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create URL: %w", err)
	}

	req, err := prepareRequest(ctx, "POST", URL.String(), bytes.NewBuffer(jsonData), cmd.CorrelationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
	}

	domain := &entities.Domain{}
	err = json.Unmarshal(body, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return domain, nil
}
