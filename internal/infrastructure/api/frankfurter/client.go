// Package frankfurter provides a client for the Frankfurter exchange-rate API
// (https://frankfurter.dev/). Frankfurter aggregates reference rates from
// central banks, requires no API key, and imposes no usage quotas.
package frankfurter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public Frankfurter v2 API endpoint.
	DefaultBaseURL = "https://api.frankfurter.dev/v2"

	// defaultTimeout bounds each HTTP request.
	defaultTimeout = 30 * time.Second
)

// Rate is a single exchange rate as returned by the /rates endpoint.
type Rate struct {
	// Date is the publication date of the rate in YYYY-MM-DD format.
	Date string `json:"date"`
	// Base is the base currency (ISO 4217 code).
	Base string `json:"base"`
	// Quote is the target currency (ISO 4217 code).
	Quote string `json:"quote"`
	// Rate is the amount of Quote currency per 1 unit of Base currency.
	Rate float64 `json:"rate"`
}

// ParsedDate returns the rate's publication date as a time.Time (UTC midnight).
func (r Rate) ParsedDate() (time.Time, error) {
	return time.Parse(time.DateOnly, r.Date)
}

// Client is an HTTP client for the Frankfurter API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (used in tests and for self-hosted
// Frankfurter instances).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates a Frankfurter API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    DefaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetLatestRates fetches the latest exchange rates for the given base
// currency. If quotes is non-empty, results are limited to those target
// currencies; otherwise all available quotes are returned.
func (c *Client) GetLatestRates(ctx context.Context, base string, quotes []string) ([]Rate, error) {
	if base == "" {
		return nil, fmt.Errorf("frankfurter: base currency is required")
	}

	q := url.Values{}
	q.Set("base", strings.ToUpper(base))
	if len(quotes) > 0 {
		q.Set("quotes", strings.ToUpper(strings.Join(quotes, ",")))
	}
	endpoint := fmt.Sprintf("%s/rates?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("frankfurter: failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("frankfurter: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // rates payloads are small; 4 MiB is generous
	if err != nil {
		return nil, fmt.Errorf("frankfurter: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frankfurter: unexpected status %d for base %s: %s", resp.StatusCode, base, truncate(string(body), 200))
	}

	var rates []Rate
	if err := json.Unmarshal(body, &rates); err != nil {
		return nil, fmt.Errorf("frankfurter: failed to parse response: %w", err)
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("frankfurter: no rates returned for base %s", base)
	}

	return rates, nil
}

// truncate shortens s to at most n characters for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
