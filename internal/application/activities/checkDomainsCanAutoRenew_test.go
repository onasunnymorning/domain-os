package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MapMockRoundTripper handles mapping different endpoints to specific responses
type MapMockRoundTripper struct {
	mu        sync.Mutex
	responses map[string]*http.Response
	errors    map[string]error
}

func (m *MapMockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	urlStr := req.URL.String()
	if err, ok := m.errors[urlStr]; ok {
		return nil, err
	}
	if resp, ok := m.responses[urlStr]; ok {
		return resp, nil
	}
	// Default fallback
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString(`Not Found`)),
	}, nil
}

type CheckDomainsCanAutoRenewTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MapMockRoundTripper
}

func (suite *CheckDomainsCanAutoRenewTestSuite) SetupTest() {
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MapMockRoundTripper{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
	}
	http.DefaultTransport = suite.mockTransport
}

func (suite *CheckDomainsCanAutoRenewTestSuite) TearDownTest() {
	http.DefaultTransport = suite.originalTransport
}

func (suite *CheckDomainsCanAutoRenewTestSuite) TestCheckDomainsCanAutoRenew_MixedResults() {
	// Setup responses for individual domains
	// Eligible for Auto-Renew
	suite.mockTransport.responses["http://localhost:8000/domains/domain1.com/canautorenew?correlation_id=testCorr"] = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"canAutoRenew": true}`)),
	}
	// Eligible for Expiry
	suite.mockTransport.responses["http://localhost:8000/domains/domain2.com/canautorenew?correlation_id=testCorr"] = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"canAutoRenew": false}`)),
	}
	// Failure: Bad request
	suite.mockTransport.responses["http://localhost:8000/domains/domain3.com/canautorenew?correlation_id=testCorr"] = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(`Bad Request`)),
	}
	// Failure: Network error
	suite.mockTransport.errors["http://localhost:8000/domains/domain4.com/canautorenew?correlation_id=testCorr"] = fmt.Errorf("network disconnect")

	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com"}

	// Set BASEURL to localhost:8000
	originalBaseURL := BASEURL
	BASEURL = "http://localhost:8000"
	defer func() { BASEURL = originalBaseURL }()

	result, err := CheckDomainsCanAutoRenew("testCorr", domains)
	suite.NoError(err)

	suite.ElementsMatch([]string{"domain1.com"}, result.EligibleForAutoRenew)
	suite.ElementsMatch([]string{"domain2.com"}, result.EligibleForExpiry)
	suite.Len(result.CheckFailures, 2)

	// Verify failures
	failures := make(map[string]string)
	for _, f := range result.CheckFailures {
		failures[f.DomainName] = f.Error
	}
	suite.Contains(failures["domain3.com"], "unexpected status code: 400")
	suite.Contains(failures["domain4.com"], "network disconnect")
}

func TestCheckDomainsCanAutoRenewTestSuite(t *testing.T) {
	suite.Run(t, new(CheckDomainsCanAutoRenewTestSuite))
}
