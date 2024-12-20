package activities

import (
	"fmt"
	"net/http"
	"os"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

var (
	BASE_URL  = "http://" + os.Getenv("API_HOST") + ":" + os.Getenv("API_PORT")
	URL       = BASE_URL + "/domains/expiring"
	BATCHSIZE = 1000
)

// ListExpiringDomains takes an ExpiringDomainsQuery and returns a list of domains that are expiring before the given date. It gets these through the admin API.
func ListExpiringDomains(query queries.ExpiringDomainsQuery) ([]entities.Domain, error) {

	resp, err := http.Get(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// This is a placeholder function that should be implemented in the future.
	return nil, nil
}

// getBatch returns a []Domains from the API
func getBatch(url string) ([]entities.Domain, error) {
	q, err := queries.NewExpiringDomainsQuery("", "", "")
	if err != nil {
		return nil, err
	}
	fmt.Println(q)

	return nil, nil
}
