package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// CheckDomainCanAutoRenew checks if a domain can be auto-renewed based on the current GA Phase and owning Registrar settings
func CheckDomainCanAutoRenew(ctx context.Context, correlationID string, domainName string) (bool, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/canautorenew", BASEURL, domainName)

	client := http.Client{}

	// Set up query parameters
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return false, fmt.Errorf("failed to create URL: %w", err)
	}

	req, err := prepareRequest(ctx, "GET", URL.String(), nil, correlationID)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, httpResponseError(resp, body)
	}

	canAutoRenewResponse := &response.CanAutoRenewResponse{}
	if err := json.Unmarshal(body, canAutoRenewResponse); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	return canAutoRenewResponse.CanAutoRenew, nil
}
