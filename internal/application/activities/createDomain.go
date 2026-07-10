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

func CreateDomain(ctx context.Context, correlationID string, cmd commands.CreateDomainCommand) error {
	ENDPOINT := fmt.Sprintf("%s/domains", BASEURL)
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
	req, err := prepareRequest(ctx, "POST", URL.String(), bytes.NewBuffer(jsonBody), correlationID)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create domain failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}
