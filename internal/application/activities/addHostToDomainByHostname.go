package activities

import (
	"fmt"
	"net/http"
)

// AddHostToDomainByHostname calls the admin API to link a host (by name) to a domain.
func AddHostToDomainByHostname(correlationID, domainName, hostName string) error {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/hostname/%s", BASEURL, domainName, hostName)

	client := http.Client{}

	qParams := map[string]string{"correlationID": correlationID}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return fmt.Errorf("failed to add query params: %w", err)
	}

	req, err := http.NewRequest("POST", URL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to link host to domain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("link host to domain failed: status=%d", resp.StatusCode)
	}
	return nil
}
