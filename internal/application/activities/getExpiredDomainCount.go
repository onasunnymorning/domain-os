package activities

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

func GetExpiredDomainCount() (int64, error) {
	// COUNT_ENDPOINT := fmt.Sprintf("http://%s:%s/domains/expiring/count", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	COUNT_ENDPOINT := "http://api.dos.dev.geoff.it:8080/domains/expiring/count"
	BEARER := "Bearer " + "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"

	// Set up an API client
	client := http.Client{}

	// check the total amount of domains to renew
	req, err := http.NewRequest("GET", COUNT_ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", BEARER)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch domain count (%d): %s", resp.StatusCode, body)
	}

	// Parse the result
	countResponse := &response.CountResult{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		return 0, err
	}

	return countResponse.Count, nil
}
