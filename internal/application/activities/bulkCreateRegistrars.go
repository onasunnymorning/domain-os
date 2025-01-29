package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

func BulkCreateRegistrars(correlationID string, cmds []commands.CreateRegistrarCommand) error {
	ENDPOINT := fmt.Sprintf("%s/registrars/bulk", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	// Marshall the body
	jsonBody, err := json.Marshal(cmds)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("POST", URL.String(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to bulk create registrars: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		// read the body for error message
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read body of failed api request: %w", err)
		}

		return fmt.Errorf("error bulk creating registrars: %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
