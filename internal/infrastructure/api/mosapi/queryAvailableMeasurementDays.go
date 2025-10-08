package mosapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// QueryAvailableMeasurementDaysResponse represents the response structure for available measurement days in a given month.
// Example of JSON response of the days for which reports are available:
// {
// "version": 2,
// "lastUpdateApiDatabase": 1422492450,
// "days": ["03", "02", "01"]
// }
type QueryAvailableMeasurementDaysResponse struct {
	Version         int      `json:"version"`
	LastUpdateApiDB int      `json:"lastUpdateApiDatabase"`
	AvailableDays   []string `json:"days"`
}

func (c *MosapiClient) QueryAvailableMeasurementDays(service string, year string, month string) ([]string, error) {
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

	url := baseURL + "/monitoring/" + service + "/measurements/" + year + "/" + month
	log.Printf("Querying URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get available measurement days (%d) for %s, year %s and month %s", resp.StatusCode, c.Config.TLD, year, month)
	}
	defer resp.Body.Close()

	var response QueryAvailableMeasurementDaysResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.AvailableDays, nil
}
