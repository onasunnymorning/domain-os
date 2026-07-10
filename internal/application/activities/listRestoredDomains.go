package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

func ListRestoredDomains(ctx context.Context, correlationID string, q *queries.RestoredDomainsQuery) ([]response.DomainRestoredItem, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/restored", BASEURL)

	// set the correlation ID and pagesize
	qParams := make(map[string]string)
	qParams["correlationID"] = correlationID
	// This is probably a good place to start if you are looking to optimize
	qParams["pagesize"] = fmt.Sprintf("%d", BATCHSIZE)

	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// Set up an API client
	client := http.Client{}

	// Retrieve the list of domains
	req, err := prepareRequest(ctx, "GET", URL.String(), nil, correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch restored domains: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
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
		Meta localMeta                     `json:"meta"`
		Data []response.DomainRestoredItem `json:"data"`
	}

	listResponse := &localResult{}
	err = json.Unmarshal(body, &listResponse)
	if err != nil {
		return nil, errors.Join(errors.New("failed to unmarshal response"), err)
	}

	return listResponse.Data, nil
}
