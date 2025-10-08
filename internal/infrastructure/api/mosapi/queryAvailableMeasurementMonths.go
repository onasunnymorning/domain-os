package mosapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// QueryAvailableMeasurementMonthsResponse represents the response structure for available measurement months in a given year.
// Example of JSON response of the months for which reports are available:
// {
// "version": 2,
// "lastUpdateApiDatabase": 1422492450,
// "months": ["06", "05", "04", "03", "02", "01"]
// }
type QueryAvailableMeasurementMonthsResponse struct {
	Version         int      `json:"version"`
	LastUpdateApiDB int      `json:"lastUpdateApiDatabase"`
	AvailableMonths []string `json:"months"`
}

func (c *MosapiClient) QueryAvailableMeasurementMonths(service string, year string) ([]string, error) {
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

	url := baseURL + "/monitoring/" + service + "/measurements/" + year
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
		return nil, fmt.Errorf("failed to get available measurement months (%d) for %s and year %s", resp.StatusCode, c.Config.TLD, year)
	}
	defer resp.Body.Close()

	var response QueryAvailableMeasurementMonthsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.AvailableMonths, nil
}
