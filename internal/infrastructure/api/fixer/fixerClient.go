package fixer

import (
	"net/http"
	"os"
)

const (
	BASE_URL    = "http://data.fixer.io/api"
	LATESTRATES = "/latest"
)

// FixerClient is a client for the Fixer API
type FixerClient struct {
	HTTPClient *http.Client
	ApiKey     string
}

// NewFixerClient creates a new FixerClient
func NewFixerClient() *FixerClient {
	return &FixerClient{
		HTTPClient: GetHTTPClient(),
		ApiKey:     os.Getenv("FIXER_API_KEY"),
	}
}

// BaseURL returns the base URL for the Fixer API
func (c *FixerClient) BaseURL() string {
	return BASE_URL
}

// LatestRatesURL returns the URL for the 'Latest Rates' endpoint
func (c *FixerClient) LatestRatesURL() string {
	return c.BaseURL() + LATESTRATES
}
