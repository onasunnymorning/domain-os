package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

var (
	BATCHSIZE = 1000
)

// ListExpiringDomains takes an ExpiringDomainsQuery and returns a list of domains that are expiring before the given date. It gets these through the admin API.
func ListExpiringDomains(query queries.ExpiringDomainsQuery) ([]response.DomainExpiryItem, error) {
	LIST_ENDPOINT := "http://api.dos.dev.geoff.it:8080/domains/expiring"
	BEARER := "Bearer " + "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"
	// Set up an API client
	client := http.Client{}

	// get a list of domains that have expired
	req, err := http.NewRequest("GET", LIST_ENDPOINT, nil)
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
	listResponse := &ListItemResult{}
	err = json.Unmarshal(body, &listResponse)
	if err != nil {
		return nil, errors.Join(errors.New("failed to unmarshal response"), err)
	}

	return listResponse.Data, nil
}

type ListItemResult struct {
	Meta response.PaginationMetaData `json:"meta"`
	Data []response.DomainExpiryItem `json:"data"`
}
