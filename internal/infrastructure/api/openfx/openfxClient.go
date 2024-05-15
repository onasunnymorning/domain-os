package openfx

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

const (
	BASE_URL    = "https://openexchangerates.org/api"
	LATESTRATES = "/latest.json"
)

// OpenFXClient is a client for the Fixer API
type OpenFXClient struct {
	HTTPClient *http.Client
	AppID      string
}

// NewFxClient creates a new FixerClient
func NewFxClient() *OpenFXClient {
	return &OpenFXClient{
		HTTPClient: GetHTTPClient(),
		AppID:      os.Getenv("OPENEXCHANGERATES_APP_ID"),
	}
}

// BaseURL returns the base URL for the Fixer API
func (c *OpenFXClient) BaseURL() string {
	return BASE_URL
}

// LatestRatesURL returns the URL for the 'Latest Rates' endpoint
func (c *OpenFXClient) LatestRatesURL() string {
	return c.BaseURL() + LATESTRATES
}

// GetLatestRates fetches the latest exchange rates from the OpenExchangerates API
func (c *OpenFXClient) GetLatestRates(baseCurrency string, targetCurrencies []string) (*LatestRatesResponse, error) {
	req, err := http.NewRequest("GET", c.LatestRatesURL(), nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("app_id", c.AppID)
	q.Add("base", baseCurrency)
	q.Add("symbols", strings.Join(targetCurrencies, ","))
	req.URL.RawQuery = q.Encode()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rates LatestRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, err
	}
	return &rates, nil
}
