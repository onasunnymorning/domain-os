package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// AdminClient is a client for the Domain OS Admin API
type AdminClient struct {
	BaseURL    string
	Token      string
	HttpClient *http.Client
}

// NewAdminClient creates a new AdminClient
func NewAdminClient(baseURL, token string) *AdminClient {
	return &AdminClient{
		BaseURL: baseURL,
		Token:   token,
		HttpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with authentication
func (c *AdminClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// checkResponse checks the response status code and returns an error if it's not successful
func (c *AdminClient) checkResponse(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
}

// BulkCreateRegistrars imports registrars in bulk
func (c *AdminClient) BulkCreateRegistrars(cmds []commands.CreateRegistrarCommand) error {
	if len(cmds) == 0 {
		return nil
	}
	// Chunking logic could be here, but assuming caller handles sensible batch sizes or API handles it.
	// However, to be safe and robust, let's chunk here if the list is huge,
	// but strictly speaking the controller implementation should handle it or we relying on a reasonable payload size.
	// For simplicity, we send one request per call, assuming the service layer chunks if needed.

	resp, err := c.doRequest("POST", "/registrars/bulk", cmds) // Note: controller registers at /registrars-bulk AND /registrars/bulk (POST) inside group?
	// Checking registrar_controller.go:
	// rarGroup := e.Group("/registrars", handler) ... rarGroup.POST("/bulk", controller.BulkCreate) -> /registrars/bulk
	// AND e.POST("/registrars-bulk", handler, controller.BulkCreate)
	// We'll use /registrars/bulk
	if err != nil {
		return err
	}
	return c.checkResponse(resp)
}

// BulkCreateContacts imports contacts in bulk
func (c *AdminClient) BulkCreateContacts(cmds []commands.CreateContactCommand) error {
	if len(cmds) == 0 {
		return nil
	}
	resp, err := c.doRequest("POST", "/contacts/bulk", cmds)
	if err != nil {
		return err
	}
	return c.checkResponse(resp)
}

// BulkCreateHosts imports hosts in bulk
func (c *AdminClient) BulkCreateHosts(cmds []commands.CreateHostCommand) error {
	if len(cmds) == 0 {
		return nil
	}
	resp, err := c.doRequest("POST", "/hosts/bulk", cmds)
	if err != nil {
		return err
	}
	return c.checkResponse(resp)
}

// BulkCreateDomains imports domains in bulk
func (c *AdminClient) BulkCreateDomains(cmds []commands.CreateDomainCommand) error {
	if len(cmds) == 0 {
		return nil
	}
	resp, err := c.doRequest("POST", "/domains/bulk", cmds)
	if err != nil {
		return err
	}
	return c.checkResponse(resp)
}

// AddHostToDomainByHostName links a host to a domain
func (c *AdminClient) AddHostToDomainByHostName(domainName, hostName string) error {
	// Endpoint: /domains/{name}/hostname/{hostName}?force=true
	// Note: The controller defines: domainGroup.POST(":name/hostname/:hostName", controller.AddHostToDomainByHostName)
	path := fmt.Sprintf("/domains/%s/hostname/%s?force=true", domainName, hostName)
	resp, err := c.doRequest("POST", path, nil)
	if err != nil {
		return err
	}
	// 204 No Content is expected
	return c.checkResponse(resp)
}

// GetTLD gets a TLD by name
func (c *AdminClient) GetTLD(name string) (*entities.TLD, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/tlds/%s", name), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var tld entities.TLD
	if err := json.NewDecoder(resp.Body).Decode(&tld); err != nil {
		return nil, fmt.Errorf("failed to decode TLD: %w", err)
	}
	return &tld, nil
}

// ListRegistrars lists registrars with optional filtering
func (c *AdminClient) ListRegistrars() ([]entities.RegistrarListItem, error) {
	// fetching all for mapping - pagination might be needed if many, but starting simple
	// Assuming default page size is enough or we loop?
	// The controller defaults pagesize to 100 usually. We might want to loop.
	// For now, let's request a large page size to keep it simple, or implement full iteration.
	// Let's implement full iteration.

	var allRars []entities.RegistrarListItem
	cursor := ""

	for {
		var lr struct {
			Data []entities.RegistrarListItem `json:"Data"` // Uppercase match
			Meta struct {
				PageCursor string `json:"PageCursor"` // Uppercase match
			} `json:"Meta"` // Uppercase match
		}

		err := func() error {
			path := fmt.Sprintf("/registrars?pagesize=1000&cursor=%s", cursor)
			resp, err := c.doRequest("GET", path, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
			}

			if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
				return fmt.Errorf("failed to decode registrars list: %w", err)
			}
			return nil
		}()

		if err != nil {
			return nil, err
		}

		allRars = append(allRars, lr.Data...)

		if lr.Meta.PageCursor == "" {
			break
		}
		cursor = string(lr.Meta.PageCursor) // It might be base64? SetMeta encodes it.
		// Wait, the client sends cursor param. Controller decodes it.
		// Response writes encoded cursor.
		// So we pass implicit encoded cursor back?
		// AdminClient uses cursor string in URL.
		// Yes, we just pass the string we got.
		cursor = lr.Meta.PageCursor
	}

	return allRars, nil
}

// ListHosts lists hosts with optional filtering (retrieving all by default)
func (c *AdminClient) ListHosts() ([]entities.Host, error) {
	var allHosts []entities.Host
	cursor := ""

	for {
		var lr struct {
			Data []entities.Host `json:"Data"` // Uppercase match
			Meta struct {
				PageCursor string `json:"PageCursor"` // Uppercase match
			} `json:"Meta"` // Uppercase match
		}

		err := func() error {
			path := fmt.Sprintf("/hosts?pagesize=1000&cursor=%s", cursor)
			resp, err := c.doRequest("GET", path, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
			}

			if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
				return fmt.Errorf("failed to decode hosts list: %w", err)
			}
			return nil
		}()

		if err != nil {
			return nil, err
		}

		allHosts = append(allHosts, lr.Data...)

		if lr.Meta.PageCursor == "" {
			break
		}
		cursor = lr.Meta.PageCursor
	}

	return allHosts, nil
}

// ListContacts lists contacts with optional filtering (retrieving all by default)
func (c *AdminClient) ListContacts() ([]entities.Contact, error) {
	var allContacts []entities.Contact
	cursor := ""

	for {
		var lr struct {
			Data []entities.Contact `json:"Data"` // Uppercase match
			Meta struct {
				PageCursor string `json:"PageCursor"` // Uppercase match
			} `json:"Meta"` // Uppercase match
		}

		err := func() error {
			path := fmt.Sprintf("/contacts?pagesize=1000&cursor=%s", cursor)
			resp, err := c.doRequest("GET", path, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
			}

			if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
				return fmt.Errorf("failed to decode contacts list: %w", err)
			}
			return nil
		}()

		if err != nil {
			return nil, err
		}

		allContacts = append(allContacts, lr.Data...)

		if lr.Meta.PageCursor == "" {
			break
		}
		cursor = lr.Meta.PageCursor
	}

	return allContacts, nil
}
