package activities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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
	result, err := ListPurgeableDomains(context.Background(), "testCorrelationID", query)
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
	result, err := ListPurgeableDomains(context.Background(), "testCorrelationID", query)
	suite.Error(err, "Expected an error for bad request")
	suite.Nil(result, "Expected no result for bad request")
	suite.Contains(err.Error(), "(400)", "Error should include status code")
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(context.Background(), "testCorrelationID", query)
	suite.Error(err, "Expected an error for network failure")
	suite.Nil(result, "Expected no result for network error")
	suite.Contains(err.Error(), "failed to fetch purgeable domains", "Error should indicate network failure")
}

// TestListPurgeableDomains_SerializesQuery is a regression test: a previous
// version of this activity accepted the query but never put it on the wire,
// so the server silently evaluated its own time.Now() instead of the
// workflow's locked reference time.
func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_SerializesQuery() {
	body := `{"meta": {}, "data": []}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	cutoff := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	query := queries.PurgeableDomainsQuery{
		Before:   cutoff,
		ClID:     entities.ClIDType("registrarX"),
		TLD:      entities.DomainName("com"),
		PageSize: 500,
	}
	_, err := ListPurgeableDomains(context.Background(), "testCorrelationID", query)
	suite.NoError(err)

	suite.Require().NotNil(suite.mockTransport.LastRequest, "Expected the request to be captured")
	params := suite.mockTransport.LastRequest.URL.Query()
	suite.Equal(cutoff.Format(time.RFC3339), params.Get("before"), "Expected the reference time cutoff to be serialized")
	suite.Equal("registrarX", params.Get("clid"), "Expected the ClID filter to be serialized")
	suite.Equal("com", params.Get("tld"), "Expected the TLD filter to be serialized")
	suite.Equal("500", params.Get("pagesize"), "Expected the page size to be serialized")
	suite.Equal("testCorrelationID", params.Get("correlation_id"), "Expected the correlation ID to be serialized")
}

// TestListPurgeableDomains_DefaultsPageSize verifies the BATCHSIZE default
// applies when the query does not specify a page size.
func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_DefaultsPageSize() {
	body := `{"meta": {}, "data": []}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	_, err := ListPurgeableDomains(context.Background(), "testCorrelationID", queries.PurgeableDomainsQuery{})
	suite.NoError(err)

	suite.Require().NotNil(suite.mockTransport.LastRequest)
	params := suite.mockTransport.LastRequest.URL.Query()
	suite.Equal(fmt.Sprintf("%d", BATCHSIZE), params.Get("pagesize"))
	suite.Empty(params.Get("clid"), "Empty ClID must not be serialized")
	suite.Empty(params.Get("tld"), "Empty TLD must not be serialized")
}

func (suite *ListPurgeableDomainsTestSuite) TestListPurgeableDomains_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	query := queries.PurgeableDomainsQuery{}
	result, err := ListPurgeableDomains(context.Background(), "testCorrelationID", query)
	suite.Error(err, "Expected an error for invalid JSON response")
	suite.Nil(result, "Expected no result for invalid JSON")
	suite.Contains(err.Error(), "failed to unmarshal response", "Error should indicate parse failure")
}

func TestListPurgeableDomainsTestSuite(t *testing.T) {
	suite.Run(t, new(ListPurgeableDomainsTestSuite))
}
