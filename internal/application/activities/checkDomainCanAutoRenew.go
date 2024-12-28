package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// CheckDomainCanAutoRenew
func CheckDomainCanAutoRenew(domainName string) (bool, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/%s/canautorenew", BASEURL, domainName)

	// Set up an API client
	client := http.Client{}

	// check the total amount of domains to renew
	req, err := http.NewRequest("GET", ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s", body)
	}

	canAutoRenewResponse := &response.CanAutoRenewResponse{}
	if err := json.Unmarshal(body, canAutoRenewResponse); err != nil {
		return false, err
	}

	return canAutoRenewResponse.CanAutoRenew, nil
}
