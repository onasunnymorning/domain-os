package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// ListPurgeableDomains takes a PurgeableDomainsQuery and returns a list of domains that have PendingDelete set and whose purge date falls on or before the query cutoff (Before). It gets these through the admin API.
func ListPurgeableDomains(ctx context.Context, correlationID string, query queries.PurgeableDomainsQuery) ([]response.DomainExpiryItem, error) {
	ENDPOINT := fmt.Sprintf("%s/domains/purgeable", BASEURL)

	// Serialize the full query so the server evaluates the same cutoff the
	// workflow locked, instead of defaulting to its own time.Now().
	qParams := make(map[string]string)
	qParams["correlation_id"] = correlationID
	qParams["pagesize"] = fmt.Sprintf("%d", pageSizeOrDefault(query.PageSize))
	if !query.Before.IsZero() {
		qParams["before"] = query.Before.Format(time.RFC3339)
	}
	if query.ClID.String() != "" {
		qParams["clid"] = query.ClID.String()
	}
	if query.TLD.String() != "" {
		qParams["tld"] = query.TLD.String()
	}
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
		return nil, fmt.Errorf("failed to fetch purgeable domains: %w", err)
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
