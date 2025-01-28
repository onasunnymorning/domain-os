package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// GetIANARegsitrars queries the API for all IANA registrars
func GetIANARegistrars(correlationID string) ([]entities.IANARegistrar, error) {
	ENDPOINT := fmt.Sprintf("%s/ianaregistrars", BASEURL)

	var returnVal []entities.IANARegistrar

	// Set up an API client
	client := http.Client{}

	// set the correlation ID
	qParams := map[string]string{"correlationID": correlationID}

	URL, err := getURLAndSetQueryParams(ENDPOINT, qParams)
	if err != nil {
		return nil, fmt.Errorf("failed to add query params: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", BEARER_TOKEN)

	// Hit the endpoint
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get registrar count: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	apiResponse := &response.ListItemResult{}
	var ianaRars []entities.IANARegistrar
	apiResponse.Data = &ianaRars

	err = json.Unmarshal(body, apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	// Repeat until the last page by following the next link until empty
	for apiResponse.Meta.NextLink != "" {
		URL, err := getURLAndSetQueryParams(apiResponse.Meta.NextLink, qParams)
		if err != nil {
			return nil, fmt.Errorf("failed to add query params: %w", err)
		}

		// Create the request
		req, err := http.NewRequest("GET", URL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Add("Authorization", BEARER_TOKEN)

		// Hit the endpoint
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get registrar count: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("(%d) %s", resp.StatusCode, body)
		}

		err = json.Unmarshal(body, apiResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
		}

		// Add the new batch to the returnVal
		ianaRars = *apiResponse.Data.(*[]entities.IANARegistrar)
		returnVal = append(returnVal, ianaRars...)

	}

	return returnVal, nil
}
