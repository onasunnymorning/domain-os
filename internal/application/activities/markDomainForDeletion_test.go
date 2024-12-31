package activities

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MarkDomainForDeletionTestSuite struct {
	suite.Suite
	originalTransport http.RoundTripper
	mockTransport     *MockRoundTripper
}

func (suite *MarkDomainForDeletionTestSuite) SetupTest() {
	// Save the original transport and replace it with a mock
	suite.originalTransport = http.DefaultTransport
	suite.mockTransport = &MockRoundTripper{}
	http.DefaultTransport = suite.mockTransport
}

func (suite *MarkDomainForDeletionTestSuite) TearDownTest() {
	// Restore the original transport
	http.DefaultTransport = suite.originalTransport
}

func (suite *MarkDomainForDeletionTestSuite) TestMarkDomainForDeletion_Success() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("")), // Use an empty buffer
	}

	err := MarkDomainForDeletion("example.com")
	suite.NoError(err, "Expected no error for successful deletion request")
}

func (suite *MarkDomainForDeletionTestSuite) TestMarkDomainForDeletion_BadRequest() {
	body := `Bad Request`
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	err := MarkDomainForDeletion("example.com")
	suite.Error(err, "Expected an error for bad request")
	suite.Contains(err.Error(), "failed to mark domain for deletion", "Error should indicate failure to mark domain for deletion")
	suite.Contains(err.Error(), "400", "Error should include HTTP status code")
}

func (suite *MarkDomainForDeletionTestSuite) TestMarkDomainForDeletion_NetworkError() {
	suite.mockTransport.Err = fmt.Errorf("network error")

	err := MarkDomainForDeletion("example.com")
	suite.Error(err, "Expected an error for network failure")
	suite.Contains(err.Error(), "failed to mark domain for deletion", "Error should indicate failure to mark domain for deletion")
	suite.Contains(err.Error(), "network error", "Error should include network error details")
}

func (suite *MarkDomainForDeletionTestSuite) TestMarkDomainForDeletion_ReadBodyError() {
	suite.mockTransport.Response = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errorReader{}), // Simulate body read error
	}

	err := MarkDomainForDeletion("example.com")
	suite.Error(err, "Expected an error for body read failure")
	suite.Contains(err.Error(), "failed to read response body", "Error should indicate failure to read body")
}

func TestMarkDomainForDeletionTestSuite(t *testing.T) {
	suite.Run(t, new(MarkDomainForDeletionTestSuite))
}
