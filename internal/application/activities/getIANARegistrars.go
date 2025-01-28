package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetIANARegistrars queries an API for all IANA registrars, following pagination links until there are no more.
func GetIANARegistrars(correlationID, baseURL, bearerToken string) ([]entities.IANARegistrar, error) {
	// Example: create a dedicated HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Build the initial URL with query parameters (correlationID)
	ENDPOINT := fmt.Sprintf("%s/ianaregistrars", baseURL)
	initialURL, err := getURLAndSetQueryParams(ENDPOINT, map[string]string{
		"correlationID": correlationID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build initial URL: %w", err)
	}

	var allRegistrars []entities.IANARegistrar
	currentURL := initialURL.String()

	// Loop until no NextLink is returned
	for currentURL != "" {
		// Fetch the current page
		apiResponse, err := fetchRegistrarsPage(context.Background(), client, currentURL, bearerToken)
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

// fetchRegistrarsPage fetches a single page of IANA registrars from the provided URL.
// It handles sending the request, reading the response, checking the status code, and unmarshaling JSON.
func fetchRegistrarsPage(ctx context.Context, client *http.Client, urlStr, bearerToken string) (*response.ListItemResult, error) {
	// Create the request with context for cancellation/timeouts
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Attach bearer token (e.g., "Bearer abc123")
	req.Header.Add("Authorization", bearerToken)

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
		return nil, fmt.Errorf("request failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// Unmarshal into ListItemResult
	var apiResponse response.ListItemResult
	// Make sure apiResponse.Data is set to a pointer of the correct type so that
	// JSON unmarshal knows where to put the data.
	apiResponse.Data = &[]entities.IANARegistrar{}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &apiResponse, nil
}
