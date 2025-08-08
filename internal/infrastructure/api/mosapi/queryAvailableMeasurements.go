package mosapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// QueryAvailableMeasurementIDsResponse represents the response structure for available measurement IDs in a given day.
// Example of JSON response of the days for which measurements are available:
// {
// "version": 2,
// "lastUpdateApiDatabase": 1422492450,
// "measurements": ["1422492930.json", "1422492990.json", "1422493050.json",
// "1422493110.json"]
// }
type QueryAvailableMeasurementIDsResponse struct {
	Version         int      `json:"version"`
	LastUpdateApiDB int      `json:"lastUpdateApiDatabase"`
	AvailableIDs    []string `json:"measurements"`
}

func (c *MosapiClient) QueryAvailableMeasurementIDs(service, year, month, day string) ([]string, error) {
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

	url := baseURL + "/monitoring/" + service + "/measurements/" + year + "/" + month + "/" + day
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
		return nil, fmt.Errorf("failed to get available measurements (%d) for %s, year %s, month %s and day %s", resp.StatusCode, c.Config.TLD, year, month, day)
	}
	defer resp.Body.Close()

	var response QueryAvailableMeasurementIDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.AvailableIDs, nil
}
