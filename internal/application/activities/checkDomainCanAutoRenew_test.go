package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CheckDomainCanAutoRenewTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *CheckDomainCanAutoRenewTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *CheckDomainCanAutoRenewTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *CheckDomainCanAutoRenewTestSuite) TestCheckDomainCanAutoRenew_Success() {
	body := `{"canAutoRenew": true}`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	canAutoRenew, err := CheckDomainCanAutoRenew("testCorrelationID", "example.com")
	suite.NoError(err, "Expected no error for successful response")
	suite.True(canAutoRenew, "Expected canAutoRenew to be true")
}

func (suite *CheckDomainCanAutoRenewTestSuite) TestCheckDomainCanAutoRenew_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	canAutoRenew, err := CheckDomainCanAutoRenew("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "unexpected status code: 400", "Error should include status code")
	suite.False(canAutoRenew, "Expected canAutoRenew to be false for error")
}

func (suite *CheckDomainCanAutoRenewTestSuite) TestCheckDomainCanAutoRenew_ParseError() {
	body := `invalid json`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	canAutoRenew, err := CheckDomainCanAutoRenew("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for invalid JSON")
	suite.Contains(err.Error(), "failed to parse response", "Error should indicate parse failure")
	suite.False(canAutoRenew, "Expected canAutoRenew to be false for parse error")
}

func (suite *CheckDomainCanAutoRenewTestSuite) TestCheckDomainCanAutoRenew_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	canAutoRenew, err := CheckDomainCanAutoRenew("testCorrelationID", "example.com")
	suite.Error(err, "Expected an error for network failure")
	suite.Contains(err.Error(), "request failed", "Error should indicate request failure")
	suite.False(canAutoRenew, "Expected canAutoRenew to be false for network error")
}

func TestCheckDomainCanAutoRenewTestSuite(t *testing.T) {
	suite.Run(t, new(CheckDomainCanAutoRenewTestSuite))
}
