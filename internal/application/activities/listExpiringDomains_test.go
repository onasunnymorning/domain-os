package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/stretchr/testify/suite"
)

type ListExpiringDomainsTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *ListExpiringDomainsTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *ListExpiringDomainsTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *ListExpiringDomainsTestSuite) TestListExpiringDomains_Success() {
	body := `{
		"meta": {
			"total": 1,
			"page": 1,
			"pagesize": 1000
		},
		"data": [
			{
				"Name": "example.com",
				"expiryDate": "2024-12-31T23:59:59Z"
			}
		]
	}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.ExpiringDomainsQuery{}
	result, err := ListExpiringDomains("testCorrelationID", query)
	suite.NoError(err, "Expected no error for successful response")
	suite.NotNil(result, "Expected a valid response")
	suite.Len(result, 1, "Expected one domain in the result")
	suite.Equal("example.com", result[0].Name, "Expected domain name to match")
}

func (suite *ListExpiringDomainsTestSuite) TestListExpiringDomains_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.ExpiringDomainsQuery{}
	result, err := ListExpiringDomains("testCorrelationID", query)
	suite.Error(err, "Expected an error for bad request")
	suite.Nil(result, "Expected no result for bad request")
	suite.Contains(err.Error(), "failed to fetch domain count", "Error should include fetch failure")
}

func (suite *ListExpiringDomainsTestSuite) TestListExpiringDomains_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	query := queries.ExpiringDomainsQuery{}
	result, err := ListExpiringDomains("testCorrelationID", query)
	suite.Error(err, "Expected an error for network failure")
	suite.Nil(result, "Expected no result for network error")
	suite.Contains(err.Error(), "failed to fetch domain count", "Error should indicate network failure")
}

func (suite *ListExpiringDomainsTestSuite) TestListExpiringDomains_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.ExpiringDomainsQuery{}
	result, err := ListExpiringDomains("testCorrelationID", query)
	suite.Error(err, "Expected an error for invalid JSON response")
	suite.Nil(result, "Expected no result for invalid JSON")
	suite.Contains(err.Error(), "failed to unmarshal response", "Error should indicate parse failure")
}

func TestListExpiringDomainsTestSuite(t *testing.T) {
	suite.Run(t, new(ListExpiringDomainsTestSuite))
}
