package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PurgeDomainTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *PurgeDomainTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *PurgeDomainTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *PurgeDomainTestSuite) TestPurgeDomain_Success() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusNoContent,                    // 204 No Content
		Body:       io.NopCloser(bytes.NewBufferString("")), // Empty body
	}

	err := PurgeDomain("testCorrelationID", "example.com")
	suite.NoError(err, "Expected no error for successful domain purge")
}

func (suite *PurgeDomainTestSuite) TestPurgeDomain_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	err := PurgeDomain("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "400", "Error should include HTTP status code")
	suite.Contains(err.Error(), "Bad Request", "Error should include response body")
}

func (suite *PurgeDomainTestSuite) TestPurgeDomain_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	err := PurgeDomain("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for network failure")
	suite.Contains(err.Error(), "failed to purge domain", "Error should indicate failure to purge domain")
	suite.Contains(err.Error(), "network error", "Error should include network error details")
}

func (suite *PurgeDomainTestSuite) TestPurgeDomain_ReadBodyError() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       io.NopCloser(&errorReader{}), // Simulate body read error
	}

	err := PurgeDomain("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for body read failure")
	suite.Contains(err.Error(), "failed to read response body", "Error should indicate failure to read body")
}

func TestPurgeDomainTestSuite(t *testing.T) {
	suite.Run(t, new(PurgeDomainTestSuite))
}
