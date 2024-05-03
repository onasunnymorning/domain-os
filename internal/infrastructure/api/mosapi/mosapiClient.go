package mosapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

const (
	AuthTypeBasic       = "basic"
	AuthTypeCertificate = "certificate"
	MOSAPI_URL          = "https://mosapi.icann.org"
	MOSAPI_OTE_URL      = "https://mosapi-ote.icann.org"
	V2                  = "v2"
	EntityRegistry      = "ry"
	EntityRegistrar     = "rr"
	ServiceEPP          = "EPP"
	ServiceDNS          = "DNS"
	ServiceDNSSEC       = "DNSSEC"
	ServiceRDDS         = "RDDS"
)

var (
	SupportedAuthTypes          = []string{AuthTypeBasic, AuthTypeCertificate}
	SupportedVersions           = []string{V2}
	SupportedEntities           = []string{EntityRegistry} // We only support the registry entity for now
	SupportedMOSAPIEnvironments = []string{"PROD", "OTE"}
	SupportedServices           = []string{ServiceEPP, ServiceDNS, ServiceDNSSEC, ServiceRDDS}

	ErrUnsupportedAuthType    = errors.New("unsupported auth type")
	ErrUnsupportedEnvironment = errors.New("unsupported environment - only PROD or OTE are supported")
	ErrUnsupportedService     = errors.New("unsupported service - only EPP, DNS, DNSSEC, RDDS are supported")
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
	Environment string
}

// NewMosapiClientConfig creates a new MosapiClientConfig
// TODO: FIXME: Make this configurable for now we just use our test TLD
func NewMosapiClientConfig() *MosapiConfig {
	return &MosapiConfig{
		TLD:         "example56",
		AuthType:    AuthTypeCertificate,
		Certificate: "./scraps/icann-tls.crt.pem",
		Key:         "./scraps/icann-tls.private-nopass.key",
		Version:     V2,
		Entity:      EntityRegistry,
		Environment: "OTE",
	}

	// return &MosapiConfig{
	// 	TLD:      "build",
	// 	AuthType: AuthTypeBasic,
	// 	Username: "build_ry",
	// 	Password: "ntw{-N+k!H9X%h~^",
	// 	Version:  V2,
	// 	Entity:   EntityRegistry,
	// }
}

// BASEURL returns the base URL for the MOSAPICient given the current configuration. It supports PROD or OTE environments
func (c *MosapiClient) BaseURL(env string) (string, error) {
	if !slices.Contains(SupportedMOSAPIEnvironments, strings.ToUpper(env)) {
		return "", ErrUnsupportedEnvironment
	}
	if strings.ToUpper(env) == "PROD" {
		return MOSAPI_URL + "/" + c.Config.Entity + "/" + c.Config.TLD + "/" + c.Config.Version, nil
	}
	return MOSAPI_OTE_URL + "/" + c.Config.Entity + "/" + c.Config.TLD + "/" + c.Config.Version, nil
}

// Login does a GET request to the login endpoint of the MOSAPI
func (c *MosapiClient) Login() error {
	baseURL, err := c.BaseURL(c.Config.Environment)
	if err != nil {
		return err
	}
	baseURL = strings.TrimSuffix(baseURL, "/v2") // the login endpoint does not have the version
	req, err := http.NewRequest("GET", baseURL+"/"+c.Config.TLD+"/login", nil)
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
	baseURL, err := c.BaseURL(c.Config.Environment)
	if err != nil {
		return err
	}
	baseURL = strings.TrimSuffix(baseURL, "/v2") // the login endpoint does not have the version
	req, err := http.NewRequest("GET", baseURL+"/"+c.Config.TLD+"/logout", nil)
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
	baseURL, err := c.BaseURL(c.Config.Environment)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", baseURL+"/monitoring/state", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Failed to get status
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get status(%d) for %s", resp.StatusCode, c.Config.TLD)
	}
	// If success, read the response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var stateResponse StateResponse
	err = json.Unmarshal(body, &stateResponse)
	if err != nil {
		return nil, err
	}
	return &stateResponse, nil
}

// GetAlarm does a GET request to the alarm endpoint of the MOSAPI and returns the response as an AlarmResponse. Use this to check if a service is currently alarmed
func (c *MosapiClient) GetAlarm(service string) (*AlarmResponse, error) {
	if !slices.Contains(SupportedServices, strings.ToUpper(service)) {
		return nil, ErrUnsupportedService
	}
	baseURL, err := c.BaseURL(c.Config.Environment)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", baseURL+"/monitoring/"+strings.ToLower(service)+"/alarmed", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Failed to get status
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get alarm status(%d) for %s", resp.StatusCode, c.Config.TLD)
	}
	// If success, read the response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var alarmResponse AlarmResponse
	err = json.Unmarshal(body, &alarmResponse)
	if err != nil {
		return nil, err
	}
	return &alarmResponse, nil
}

// GetDowntime does a GET request to the downtime endpoint of the MOSAPI and returns the response as a DowntimeResponse
func (c *MosapiClient) GetDowntime(service string) (*DowntimeResponse, error) {
	if !slices.Contains(SupportedServices, strings.ToUpper(service)) {
		return nil, ErrUnsupportedService
	}
	baseURL, err := c.BaseURL(c.Config.Environment)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", baseURL+"/monitoring/"+strings.ToLower(service)+"/downtime", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Failed to get status
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get downtime status(%d) for %s", resp.StatusCode, c.Config.TLD)
	}
	// If success, read the response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var downtimeResponse DowntimeResponse
	err = json.Unmarshal(body, &downtimeResponse)
	if err != nil {
		return nil, err
	}
	return &downtimeResponse, nil
}
