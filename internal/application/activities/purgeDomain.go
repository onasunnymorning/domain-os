package activities

import (
	"fmt"
	"io"
	"net/http"
)

// PurgeDomain purges (deletes) a domain from the system.
func PurgeDomain(domainName string) error {
	ENDPOINT := fmt.Sprintf("http://api.dos.dev.geoff.it:8080/domains/%s", domainName)
	BEARER := "Bearer " + "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"

	// Set up an API client
	client := http.Client{}

	// Delete the domain
	req, err := http.NewRequest("DELETE", ENDPOINT, nil)
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

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("(%d) %s", resp.StatusCode, body)
	}

	return nil
}
