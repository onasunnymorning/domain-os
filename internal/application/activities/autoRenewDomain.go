package activities

import (
	"fmt"
	"io"
	"net/http"
)

func AutoRenewDomain(domainName string) error {
	ENDPOINT := fmt.Sprintf("http://api.dos.dev.geoff.it:8080/domains/%s/autorenew", domainName)
	BEARER := "Bearer " + "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"

	// Set up an API client
	client := http.Client{}

	// check the total amount of domains to renew
	req, err := http.NewRequest("POST", ENDPOINT, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", BEARER)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", body)
	}

	return nil
}
