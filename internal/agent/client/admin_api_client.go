package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AdminAPIClient handles communication with the Admin API
type AdminAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewAdminAPIClient creates a new Admin API client
func NewAdminAPIClient(baseURL, token string) *AdminAPIClient {
	return &AdminAPIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest executes an HTTP request to the Admin API
func (c *AdminAPIClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GET request helper
func (c *AdminAPIClient) Get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil)
}

// POST request helper
func (c *AdminAPIClient) Post(path string, body interface{}) ([]byte, error) {
	return c.doRequest("POST", path, body)
}

// PUT request helper
func (c *AdminAPIClient) Put(path string, body interface{}) ([]byte, error) {
	return c.doRequest("PUT", path, body)
}

// DELETE request helper
func (c *AdminAPIClient) Delete(path string) ([]byte, error) {
	return c.doRequest("DELETE", path, nil)
}

// PATCH request helper
func (c *AdminAPIClient) Patch(path string, body interface{}) ([]byte, error) {
	return c.doRequest("PATCH", path, body)
}
