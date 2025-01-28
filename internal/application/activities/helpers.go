package activities

import (
	"fmt"
	"net/url"
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
