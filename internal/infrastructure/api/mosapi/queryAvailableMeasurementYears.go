package mosapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// QueryAvailableMeasurementYearsResponse represents the response structure for available measurement years.
// Example of JSON response of the years for which reports are available:
// {
// "version": 2,
// "lastUpdateApiDatabase": 1422492450,
// "years": ["2018", "2017", "2016"]
// }
type QueryAvailableMeasurementYearsResponse struct {
	Version         int      `json:"version"`
	LastUpdateApiDB int      `json:"lastUpdateApiDatabase"`
	AvailableYears  []string `json:"years"`
}

func (c *MosapiClient) QueryAvailableMeasurementYears(service string) ([]string, error) {
	if c.Config.AuthType == AuthTypeBasic {
		err := c.Login()
		if err != nil {
			return nil, err
		}
		defer c.Logout()
	}

	baseURL, err := c.BaseURL()
	if err != nil {
		return nil, err
	}

	url := baseURL + "/monitoring/" + service + "/measurements"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get available measurement years (%d) for %s", resp.StatusCode, c.Config.TLD)
	}
	defer resp.Body.Close()

	var response QueryAvailableMeasurementYearsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.AvailableYears, nil
}
