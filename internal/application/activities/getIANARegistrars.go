package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// GetIANARegistrars queries an API for all IANA registrars, following pagination links until there are no more.
func GetIANARegistrars(ctx context.Context, correlationID string, batchsize int) ([]entities.IANARegistrar, error) {
	// Example: create a dedicated HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Build the initial URL with query parameters (correlationID)
	ENDPOINT := fmt.Sprintf("%s/ianaregistrars", BASEURL)
	initialURL, err := getURLAndSetQueryParams(ENDPOINT, map[string]string{
		"correlationID": correlationID,
		"pagesize":      strconv.Itoa(batchsize),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build initial URL: %w", err)
	}

	var allRegistrars []entities.IANARegistrar
	currentURL := initialURL.String()

	token, err := GetBearerToken()
	if err != nil {
		return nil, err
	}

	// Loop until no NextLink is returned
	for currentURL != "" {
		// Fetch the current page
		apiResponse, err := fetchIANARegistrarsPage(ctx, client, currentURL, token, correlationID)
		if err != nil {
			return nil, err
		}

		// Extract the data
		pageRegistrars, ok := apiResponse.Data.(*[]entities.IANARegistrar)
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

// fetchIANARegistrarsPage fetches a single page of IANA registrars from the provided URL.
// It handles sending the request, reading the response, checking the status code, and unmarshaling JSON.
func fetchIANARegistrarsPage(ctx context.Context, client *http.Client, urlStr, bearerToken string, correlationID string) (*response.ListItemResult, error) {
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

	// Unmarshal robustly: decode Meta directly and defer decoding Data to a typed slice.
	// Some endpoints may return Data: null or omit Data entirely when empty; using a RawMessage
	// prevents the interface{} field from being set to nil unexpectedly.
	// Unmarshal robustly: decode Meta directly and defer decoding Data to a typed slice.
	// Some endpoints may return Data: null or omit Data entirely when empty; using a RawMessage
	// prevents the interface{} field from being set to nil unexpectedly.
	// Also use localMeta to avoid interface unmarshal errors on Filter.
	type localMeta struct {
		PageSize   int         `json:"PageSize"`
		PageCursor string      `json:"PageCursor"`
		NextLink   string      `json:"NextLink"`
		Filter     interface{} `json:"Filter"`
	}
	type pageEnvelope struct {
		Meta localMeta       `json:"Meta"`
		Data json.RawMessage `json:"Data"`
	}

	var env pageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON envelope: %w", err)
	}

	// Decode Data into a typed slice, treating null/missing as empty slice
	registrars := []entities.IANARegistrar{}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &registrars); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Data field: %w", err)
		}
	}

	// Reconstruct a ListItemResult compatible with callers expecting a pointer to slice
	result := &response.ListItemResult{
		Meta: response.PaginationMetaData{
			PageSize:   env.Meta.PageSize,
			PageCursor: env.Meta.PageCursor,
			NextLink:   env.Meta.NextLink,
			// Filter ignored
		},
	}
	result.Data = &registrars

	return result, nil
}
