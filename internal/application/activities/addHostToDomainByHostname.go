package activities

import (
	"fmt"
	"net/http"
	"strings"
)

// AddHostToDomainByHostname calls the admin API to link a host (by name) to a domain.
func AddHostToDomainByHostname(correlationID, domainName, hostName string) error {
	// Normalize: trim any trailing dots that may appear in escrow files
	cleanHost := strings.TrimRight(hostName, ".")
	ENDPOINT := fmt.Sprintf("%s/domains/%s/hostname/%s", BASEURL, domainName, cleanHost)

	client := http.Client{}

	// Force is needed during escrow imports because domains may carry update prohibitions
	qParams := map[string]string{"correlationID": correlationID, "force": "true"}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	req, err := http.NewRequest("POST", URL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", GetBearerToken())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to link host to domain: %w", err)
	}
	defer resp.Body.Close()

	// API returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("link host to domain failed: status=%d", resp.StatusCode)
	}
	return nil
}
