package activities

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"go.temporal.io/sdk/temporal"
)

// getURLAndSetQueryParams takes a URI string and a map of query parameters,
// parses the URI, adds or overwrites the query parameters from the map,
// and returns the resulting URL with the updated query parameters.
func getURLAndSetQueryParams(uri string, queryParamsMap map[string]string) (*url.URL, error) {
	endpointURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Get the current query parameters
	q := endpointURL.Query()

	// Add (or overwrite) the query parameters from the map
	for key, value := range queryParamsMap {
		q.Set(key, value)
	}

	// Encode and set the final query
	endpointURL.RawQuery = q.Encode()

	return endpointURL, nil
}

// httpResponseError returns an appropriate error for a non-OK HTTP response.
// 4xx responses are wrapped as non-retryable (business logic errors that will
// never succeed on retry). 5xx responses are returned as plain errors so
// Temporal retries them according to the activity's retry policy.
func httpResponseError(resp *http.Response, body []byte) error {
	msg := fmt.Sprintf("(%d) %s", resp.StatusCode, string(body))
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return temporal.NewNonRetryableApplicationError(msg, "HTTP_CLIENT_ERROR", nil)
	}
	return errors.New(msg)
}
