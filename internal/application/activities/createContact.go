package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

func CreateContact(correlationID string, cmd commands.CreateContactCommand) error {
	ENDPOINT := fmt.Sprintf("%s/contacts", BASEURL)
	client := http.Client{}
	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}
	jsonBody, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}
	req, err := http.NewRequest("POST", URL.String(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", GetBearerToken())
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create contact: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create contact failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}
