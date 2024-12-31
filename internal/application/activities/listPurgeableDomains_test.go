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

type ListPurgeableDomainsTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *ListPurgeableDomainsTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *ListPurgeableDomainsTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_Success() {
	body := `{
		"meta": {
			"total": 2,
			"page": 1,
			"pagesize": 1000
		},
		"data": [
			{
				"Name": "example1.com",
				"expiryDate": "2024-12-31T23:59:59Z"
			},
			{
				"Name": "example2.com",
				"expiryDate": "2025-01-01T23:59:59Z"
			}
		]
	}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(query)
	suite.NoError(err, "Expected no error for successful response")
	suite.NotNil(result, "Expected a valid response")
	suite.Len(result, 2, "Expected two domains in the result")
	suite.Equal("example1.com", result[0].Name, "Expected first domain name to match")
	suite.Equal("example2.com", result[1].Name, "Expected second domain name to match")
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(query)
	suite.Error(err, "Expected an error for bad request")
	suite.Nil(result, "Expected no result for bad request")
	suite.Contains(err.Error(), "failed to fetch domain count", "Error should include fetch failure")
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(query)
	suite.Error(err, "Expected an error for network failure")
	suite.Nil(result, "Expected no result for network error")
	suite.Contains(err.Error(), "failed to fetch domain count", "Error should indicate network failure")
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(query)
	suite.Error(err, "Expected an error for invalid JSON response")
	suite.Nil(result, "Expected no result for invalid JSON")
	suite.Contains(err.Error(), "failed to unmarshal response", "Error should indicate parse failure")
}

func TestListPurgeableDomainsTestSuite(t *testing.T) {
	suite.Run(t, new(ListPurgeableDomainsTestSuite))
}
