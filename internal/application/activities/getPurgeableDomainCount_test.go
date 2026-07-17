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

type GetPurgeableDomainCountTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *GetPurgeableDomainCountTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *GetPurgeableDomainCountTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *GetPurgeableDomainCountTestSuite) TestGetPurgeableDomainCount_Success() {
	body := `{"count": 50}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetPurgeableDomainCount(context.Background(), "testCorrelationID", queries.PurgeableDomainsQuery{})
	suite.NoError(err, "Expected no error for successful response")
	suite.NotNil(result, "Expected a valid response")
	suite.Equal(int64(50), result.Count, "Expected count to match")
}

// TestGetPurgeableDomainCount_SerializesQuery is a regression test: a previous
// version of this activity accepted the query but never put it on the wire.
func (suite *GetPurgeableDomainCountTestSuite) TestGetPurgeableDomainCount_SerializesQuery() {
	body := `{"count": 0}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	cutoff := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	query := queries.PurgeableDomainsQuery{
		Before: cutoff,
		ClID:   entities.ClIDType("registrarX"),
		TLD:    entities.DomainName("com"),
	}
	_, err := GetPurgeableDomainCount(context.Background(), "testCorrelationID", query)
	suite.NoError(err)

	suite.Require().NotNil(suite.mockTransport.LastRequest, "Expected the request to be captured")
	params := suite.mockTransport.LastRequest.URL.Query()
	suite.Equal(cutoff.Format(time.RFC3339), params.Get("before"), "Expected the reference time cutoff to be serialized")
	suite.Equal("registrarX", params.Get("clid"), "Expected the ClID filter to be serialized")
	suite.Equal("com", params.Get("tld"), "Expected the TLD filter to be serialized")
}

func (suite *GetPurgeableDomainCountTestSuite) TestGetPurgeableDomainCount_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetPurgeableDomainCount(context.Background(), "testCorrelationID", queries.PurgeableDomainsQuery{})
	suite.Error(err, "Expected an error for bad request")
	suite.Nil(result, "Expected no result for bad request")
	suite.Contains(err.Error(), "(400)", "Error should include status code")
}

func (suite *GetPurgeableDomainCountTestSuite) TestGetPurgeableDomainCount_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	result, err := GetPurgeableDomainCount(context.Background(), "testCorrelationID", queries.PurgeableDomainsQuery{})
	suite.Error(err, "Expected an error for network failure")
	suite.Nil(result, "Expected no result for network error")
	suite.Contains(err.Error(), "failed to fetch domain count", "Error should indicate network failure")
}

func (suite *GetPurgeableDomainCountTestSuite) TestGetPurgeableDomainCount_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	result, err := GetPurgeableDomainCount(context.Background(), "testCorrelationID", queries.PurgeableDomainsQuery{})
	suite.Error(err, "Expected an error for invalid JSON response")
	suite.Nil(result, "Expected no result for invalid JSON")
	suite.Contains(err.Error(), "failed to parse response body", "Error should indicate parse failure")
}

func TestGetPurgeableDomainCountTestSuite(t *testing.T) {
	suite.Run(t, new(GetPurgeableDomainCountTestSuite))
}
