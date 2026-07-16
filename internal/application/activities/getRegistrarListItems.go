package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// GetRegistrarListItems queries an API for all Registrar List Items, following pagination links until there are no more.
func GetRegistrarListItems(ctx context.Context, correlationID string, batchsize int) ([]entities.RegistrarListItem, error) {
	// Example: create a dedicated HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Build the initial URL with query parameters (correlationID)
	ENDPOINT := fmt.Sprintf("%s/registrars", BASEURL)
	initialURL, err := getURLAndSetQueryParams(ENDPOINT, map[string]string{
		"correlationID": correlationID,
		"pagesize":      fmt.Sprintf("%d", batchsize),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build initial URL: %w", err)
	}

	var allRegistrars []entities.RegistrarListItem
	currentURL := initialURL.String()

	token, err := GetBearerToken()
	if err != nil {
		return nil, err
	}

	// Loop until no NextLink is returned
	for currentURL != "" {
		// Fetch the current page
		apiResponse, err := fetchRegistrarsPage(ctx, client, currentURL, token, correlationID)
		if err != nil {
			return nil, err
		}

		// Extract the data
		pageRegistrars, ok := apiResponse.Data.(*[]entities.RegistrarListItem)
		if !ok {
			return nil, fmt.Errorf("unexpected data type in response - maybe null response/sync failed")
		}
		allRegistrars = append(allRegistrars, *pageRegistrars...)

		// Prepare for the next loop iteration (if any)
		nextLink := apiResponse.Meta.NextLink
		if nextLink == "" {
			// No more pages
			break
		}

		// If there's another page, we need to apply the same correlation ID again (if required)
		nextURL, err := getURLAndSetQueryParams(ENDPOINT, map[string]string{
			"correlationID": correlationID,
			"cursor":        apiResponse.Meta.PageCursor,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build next page URL: %w", err)
		}
		currentURL = nextURL.String()
	}

	return allRegistrars, nil
}

// fetchRegistrarsPage fetches a single page of IANA registrars from the provided URL.
// It handles sending the request, reading the response, checking the status code, and unmarshaling JSON.
func fetchRegistrarsPage(ctx context.Context, client *http.Client, urlStr, bearerToken string, correlationID string) (*response.ListItemResult, error) {
	// Create the request with context for cancellation/timeouts
	req, err := prepareRequest(ctx, http.MethodGet, urlStr, nil, correlationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Attach bearer token (e.g., "Bearer abc123")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	// Always close the body promptly
	defer resp.Body.Close()

	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-200 response codes
	if resp.StatusCode != http.StatusOK {
		return nil, httpResponseError(resp, body)
	}

	// Unmarshal into a local struct to avoid interface unmarshalling errors for Filter
	type localMeta struct {
		PageSize   int         `json:"PageSize"`
		PageCursor string      `json:"PageCursor"`
		NextLink   string      `json:"NextLink"`
		Filter     interface{} `json:"Filter"` // Unmarshal filter as generic interface (ignored)
	}
	type localResult struct {
		Meta localMeta       `json:"Meta"`
		Data json.RawMessage `json:"Data"`
	}

	var localRes localResult
	if err := json.Unmarshal(body, &localRes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Prepare the final response object
	apiResponse := response.ListItemResult{
		Meta: response.PaginationMetaData{
			PageSize:   localRes.Meta.PageSize,
			PageCursor: localRes.Meta.PageCursor,
			NextLink:   localRes.Meta.NextLink,
			// Filter is left nil as it's not needed by the worker
		},
	}

	// Unmarshal Data if present
	registrars := []entities.RegistrarListItem{}
	if len(localRes.Data) > 0 && string(localRes.Data) != "null" {
		if err := json.Unmarshal(localRes.Data, &registrars); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Data field: %w", err)
		}
	}
	apiResponse.Data = &registrars

	return &apiResponse, nil
}
