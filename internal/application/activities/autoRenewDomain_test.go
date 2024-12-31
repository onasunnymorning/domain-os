package activities

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MockRoundTripper is a mock implementation of http.RoundTripper
type MockRoundTripper struct {
	Response *http.Response
	Err      error
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

// AutoRenewTestSuite is the test suite for AutoRenewDomain
type AutoRenewTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

// SetupTest runs before each test
func (suite *AutoRenewTestSuite) SetupTest() {
	// Save the original transport and replace it with the mock transport
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport

	os.Setenv("API_HOST", "localhost")
	os.Setenv("API_PORT", "8080")
	os.Setenv("API_TOKEN", "somebogustoken")
}

// TearDownTest runs after each test
func (suite *AutoRenewTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *AutoRenewTestSuite) TestAutoRenewDomain_Success() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
	}

	err := AutoRenewDomain("example.com")
	suite.NoError(err, "Expected no error for successful auto-renewal")
}

func (suite *AutoRenewTestSuite) TestAutoRenewDomain_BadRequest() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(`Bad Request`)),
	}

	err := AutoRenewDomain("example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "unexpected status code: 400")
}

func (suite *AutoRenewTestSuite) TestAutoRenewDomain_NetworkError() {
	suite.mockTransport.Err = errors.New("request failed: network error")

	err := AutoRenewDomain("example.com")
	suite.Error(err, "Expected an error for network issues")
	suite.Contains(err.Error(), "request failed: network error")
}

func (suite *AutoRenewTestSuite) TestAutoRenewDomain_URLError() {
	//    override the envar
	os.Setenv("API_HOST", `loc"alhost`)

	suite.mockTransport.Err = errors.New("failed to create request: url error")

	err := AutoRenewDomain("example.com")
	suite.Error(err, "Expected an error for network issues")
	suite.Contains(err.Error(), "failed to create request: url error")
}

func TestAutoRenewTestSuite(t *testing.T) {
	suite.Run(t, new(AutoRenewTestSuite))
}
