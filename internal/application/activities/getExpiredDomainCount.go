package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetExpiredDomainCount takes a ExpiringDomainsQuery and returns the number of domains that have expired and are past the grace period (ExpiryDate is in the past or before the supplied date). It gets these through the admin API.
func GetExpiredDomainCount(queries.ExpiringDomainsQuery) (*response.CountResult, error) {
	// COUNT_ENDPOINT := fmt.Sprintf("http://%s:%s/domains/expiring/count", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	COUNT_ENDPOINT := "http://api.dos.dev.geoff.it:8080/domains/expiring/count"
	BEARER := "Bearer " + "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"

	// Set up an API client
	client := http.Client{}

	// check the total amount of domains to renew
	req, err := http.NewRequest("GET", COUNT_ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", BEARER)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch domain count (%d): %s", resp.StatusCode, body)
	}

	// Parse the result
	countResponse := &response.CountResult{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		return nil, err
	}

	return countResponse, nil
}
