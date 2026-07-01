package activities

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

// BulkCreateHosts calls the admin API to create multiple hosts in one request.
func BulkCreateHosts(ctx context.Context, correlationID string, cmds []commands.CreateHostCommand) error {
	ENDPOINT := fmt.Sprintf("%s/hosts/bulk", BASEURL)

	client := http.Client{}

	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	jsonBody, err := json.Marshal(cmds)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	req, err := prepareRequest(ctx, "POST", URL.String(), bytes.NewBuffer(jsonBody), correlationID)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to bulk create hosts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read body of failed api request: %w", err)
		}
		return fmt.Errorf("error bulk creating hosts: %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
