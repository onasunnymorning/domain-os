package mosapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

const (
	AuthTypeBasic       = "basic"
	AuthTypeCertificate = "certificate"
	MOSAPI_URL          = "https://mosapi.icann.org"
	V2                  = "v2"
	EntityRegistry      = "ry"
	EntityRegistrar     = "rr"
)

var (
	SupportedAuthTypes = []string{AuthTypeBasic, AuthTypeCertificate}
	SupportedVersions  = []string{V2}
	SupportedEntities  = []string{EntityRegistry} // We only support the registry entity for now

	ErrUnsupportedAuthType = errors.New("unsupported auth type")
)

// MosapiClient is a client for the MOSAPI
type MosapiClient struct {
	HTTPClient *http.Client
	Config     *MosapiConfig
}

// NewMosapiClient creates a new MosapiClient
func NewMosapiClient(config *MosapiConfig) (*MosapiClient, error) {
	httpClient, err := NewHTTPClient(config)
	if err != nil {
		return nil, err
	}
	return &MosapiClient{
		HTTPClient: httpClient,
		Config:     config,
	}, nil
}

// MosapiConfig is the configuration for the MOSAPI client
type MosapiConfig struct {
	TLD         string
	AuthType    string
	Certificate string
	Key         string
	Username    string
	Password    string
	Version     string
	Entity      string
}

// NewMosapiClientConfig creates a new MosapiClientConfig
// TODO: FIXME: Make this configurable for now we just use our test TLD
func NewMosapiClientConfig() *MosapiConfig {
	// return &MosapiConfig{
	// 	TLD:         "example56",
	// 	AuthType:    AuthTypeCertificate,
	// 	Certificate: "./scraps/icann-tls.crt.pem",
	// 	Key:         "./scraps/icann-tls.private.key",
	// 	Version:     V2,
	// 	Entity:      EntityRegistry,
	// }

	return &MosapiConfig{
		TLD:      "build",
		AuthType: AuthTypeBasic,
		Username: "build_ry",
		Password: "build_ry",
		Version:  V2,
		Entity:   EntityRegistry,
	}
}

// BASEURL returns the base URL for the MOSAPICient given the current configuration
func (c *MosapiClient) BaseURL() string {
	return MOSAPI_URL + "/" + c.Config.Entity + "/" + c.Config.TLD + "/" + c.Config.Version
}

// Login does a GET request to the login endpoint of the MOSAPI
func (c *MosapiClient) Login() error {
	req, err := http.NewRequest("GET", "https://mosapi.icann.org/ry/"+c.Config.TLD+"/login", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Config.Username, c.Config.Password)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	// If not succcess
	if resp.StatusCode != 200 {
		return errors.Join(fmt.Errorf("login failed(%d) for %s", resp.StatusCode, c.Config.TLD), err)
	}
	// If success, set the cookies
	c.HTTPClient.Jar.SetCookies(resp.Request.URL, resp.Cookies())
	return nil
}

// Logout does a GET request to the logout endpoint of the MOSAPI
func (c *MosapiClient) Logout() error {
	req, err := http.NewRequest("GET", "https://mosapi.icann.org/ry/"+c.Config.TLD+"/logout", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	// If not succcess
	if resp.StatusCode != 200 {
		return errors.Join(fmt.Errorf("logout failed(%d) for %s", resp.StatusCode, c.Config.TLD), err)
	}
	// Remove the cookies
	c.HTTPClient.Jar.SetCookies(resp.Request.URL, nil)
	return nil
}

// GetState does a GET request to the state endpoint of the MOSAPI and returns the response as a StateResponse
func (c *MosapiClient) GetState() (*StateResponse, error) {
	req, err := http.NewRequest("GET", "https://mosapi.icann.org/ry/"+c.Config.TLD+"/v2/monitoring/state", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	// Failed to get status
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get status(%d) for %s", resp.StatusCode, c.Config.TLD)
	}
	// If success, read the response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var stateResponse StateResponse
	err = json.Unmarshal(body, &stateResponse)
	if err != nil {
		return nil, err
	}
	return &stateResponse, nil
}
