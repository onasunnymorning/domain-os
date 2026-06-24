package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

var (
	BATCHSIZE = 1000
)

// ListExpiringDomains takes an ExpiringDomainsQuery and returns a list of domains that are expiring before the given date. It gets these through the admin API.
func ListExpiringDomains(correlationID string, query queries.ExpiringDomainsQuery) ([]response.DomainExpiryItem, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/expiring", BASEURL)

	// Set up an API client
	client := http.Client{}

	// Set up query parameters
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	qParams["pagesize"] = fmt.Sprintf("%d", BATCHSIZE)
	if !query.Before.IsZero() {
		qParams["before"] = query.Before.Format(time.RFC3339)
	}
	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create URL: %w", err)
	}

	// get a list of domains that have expired
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", GetBearerToken())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domain count: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch domain count (%d): %s", resp.StatusCode, body)
	}

	// Parse the result
	// robust unmarshal to avoid interface errors
	type localMeta struct {
		PageSize   int         `json:"PageSize"`
		PageCursor string      `json:"PageCursor"`
		NextLink   string      `json:"NextLink"`
		Filter     interface{} `json:"Filter"`
	}
	type localResult struct {
		Meta localMeta                   `json:"meta"`
		Data []response.DomainExpiryItem `json:"data"`
	}
	listResponse := &localResult{}
	err = json.Unmarshal(body, &listResponse)
	if err != nil {
		return nil, errors.Join(errors.New("failed to unmarshal response"), err)
	}

	return listResponse.Data, nil
}
