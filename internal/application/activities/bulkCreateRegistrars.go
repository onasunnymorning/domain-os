package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// BulkCreateRegistrars posts the commands to the API and reports which registrars
// were actually inserted. The API skips rows that collide on a unique constraint
// (e.g. a name that is already taken) and still answers 201, so a nil error does
// not mean every registrar was created: check Skipped.
func BulkCreateRegistrars(ctx context.Context, correlationID string, cmds []commands.CreateRegistrarCommand) (response.BulkCreateRegistrarsResult, error) {
	var result response.BulkCreateRegistrarsResult

	ENDPOINT := fmt.Sprintf("%s/registrars/bulk", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlation_id": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return result, fmt.Errorf("failed to add query params: %w", err)
	}

	// Marshall the body
	jsonBody, err := json.Marshal(cmds)
	if err != nil {
		return result, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Create the request
	req, err := prepareRequest(ctx, "POST", URL.String(), bytes.NewBuffer(jsonBody), correlationID)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to bulk create registrars: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		// read the body for error message
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, fmt.Errorf("failed to read body of failed api request: %w", err)
		}

		return result, httpResponseError(resp, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read body of bulk create registrars response: %w", err)
	}

	// An API that predates the result body answers 201 with "null". It cannot tell
	// us what was skipped, so fall back to treating the batch as created.
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		for _, cmd := range cmds {
			result.Created = append(result.Created, cmd.ClID)
		}
		return result, nil
	}
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal bulk create registrars response: %w", err)
	}

	return result, nil
}
