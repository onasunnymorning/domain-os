package mosapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *MosapiClient) GetMeasurementDetails(service, year, month, day, measurementID string) (*MeasurementDetailsResponse, error) {
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

	url := fmt.Sprintf("%s/monitoring/%s/measurements/%s/%s/%s/%s", baseURL, service, year, month, day, measurementID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get status(%d) for %s", resp.StatusCode, c.Config.TLD)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	measurementDetailsResponse := &MeasurementDetailsResponse{}
	err = json.Unmarshal(body, measurementDetailsResponse)
	if err != nil {
		return nil, err
	}

	return measurementDetailsResponse, nil
}
