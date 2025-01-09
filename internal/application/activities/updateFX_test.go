package activities

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type UpdateFXTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *UpdateFXTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *UpdateFXTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *UpdateFXTestSuite) TestPurgeDomain_Success() {
	body := `"message": "Successfully synced FX rates"`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	err := UpdateFX("testCorrelationID", "usd")
	suite.NoError(err, "Expected no error for successful FX sync")
}

func (suite *UpdateFXTestSuite) TestPurgeDomain_BadRequest() {
	body := `"error": "error retrieving FX rates"`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	err := UpdateFX("testCorrelationID", "m")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "500", "error retrieving FX rates")
}

func TestUpdateFXTestSuite(t *testing.T) {
	suite.Run(t, new(UpdateFXTestSuite))
}
