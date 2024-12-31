package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ExpireDomainTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *ExpireDomainTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *ExpireDomainTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *ExpireDomainTestSuite) TestExpireDomain_Success() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}

	err := ExpireDomain("example.com")
	suite.NoError(err, "Expected no error for successful response")
}

func (suite *ExpireDomainTestSuite) TestExpireDomain_BadRequest() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString("Bad Request")),
	}

	err := ExpireDomain("example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "unexpected status code: 400", "Error should include status code")
}

func (suite *ExpireDomainTestSuite) TestExpireDomain_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	err := ExpireDomain("example.com")
	suite.Error(err, "Expected an error for network failure")
	suite.Contains(err.Error(), "request failed", "Error should indicate request failure")
}

func (suite *ExpireDomainTestSuite) TestExpireDomain_ReadBodyError() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errorReader{}), // Simulate a read error
	}

	err := ExpireDomain("example.com")
	suite.Error(err, "Expected an error for body read failure")
	suite.Contains(err.Error(), "failed to read response", "Error should indicate body read failure")
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func (e *errorReader) Close() error {
	return nil
}

func TestExpireDomainTestSuite(t *testing.T) {
	suite.Run(t, new(ExpireDomainTestSuite))
}
