package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func CreateRegistrar(correlationID string, cmd commands.CreateRegistrarCommand) (*entities.Registrar, error) {
	ENDPOINT := fmt.Sprintf("%s/registrars", BASEURL)

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// Marshall the body
	jsonBody, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("POST", URL.String(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", GetBearerToken())

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create registrar with iana id %d: %w", cmd.GurID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, httpResponseError(resp, body)
	}

	var rar entities.Registrar

	err = json.Unmarshal(body, &rar)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return &rar, nil
}
