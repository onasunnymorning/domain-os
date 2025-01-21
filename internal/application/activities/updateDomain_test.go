package activities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
)

// UpdateDomainTestSuite is our testify suite for testing the UpdateDomain function.
type UpdateDomainTestSuite struct {
	suite.Suite

	// We'll store the original values of these globals so we can restore them after tests
	originalBaseURL     string
	originalBearerToken string

	// Test server to mock responses
	testServer *httptest.Server
}

// SetupSuite runs before the suite starts; we create our test server and override the global vars here.
func (suite *UpdateDomainTestSuite) SetupSuite() {
	// Save original values so we can restore them in TearDownSuite
	suite.originalBaseURL = BASEURL
	suite.originalBearerToken = BEARER_TOKEN

	// Create a test server to mock the remote endpoint
	suite.testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check path and method
		if r.URL.Path == "/domains/test-domain/" && r.Method == http.MethodPut {
			// For demonstration, check if correlationID was properly set in query
			if r.URL.Query().Get("correlationID") != "test-corr-id" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"missing or incorrect correlationID"}`))
				return
			}

			// Optionally, check for Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != suite.originalBearerToken {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid token"}`))
				return
			}

			// Mock a successful response
			// We might decode the domain from the request or just return a fixed response
			updatedDomain := entities.Domain{
				Name: "test-domain",
				// fill out additional fields as needed
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&updatedDomain)
		} else if r.URL.Path == "/domains/error-domain/" && r.Method == http.MethodPut {
			// Mock a failure response
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal server error`))
		} else {
			// Default: Not found
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`not found`))
		}
	}))

	// Override the global BASEURL and BEARER_TOKEN to point to the test server
	BASEURL = suite.testServer.URL
	BEARER_TOKEN = "test-bearer-token"
}

// TearDownSuite runs once after the entire suite finishes.
func (suite *UpdateDomainTestSuite) TearDownSuite() {
	// Restore original values
	BASEURL = suite.originalBaseURL
	BEARER_TOKEN = suite.originalBearerToken

	// Close the test server
	suite.testServer.Close()
}

// TestUpdateDomain_Success checks a valid update scenario.
func (suite *UpdateDomainTestSuite) TestUpdateDomain_Success() {
	correlationID := "test-corr-id"
	domain := entities.Domain{
		Name: "test-domain", // must match the route in our httptest server
	}

	// Make sure we use the same bearer token that the server expects
	BEARER_TOKEN = suite.originalBearerToken

	updatedDomain, err := UpdateDomain(correlationID, domain)

	// We expect no error
	suite.Require().NoError(err)
	suite.Assert().NotNil(updatedDomain)
	// Check that the name was updated correctly
	suite.Assert().Equal("test-domain", updatedDomain.Name.String())
}

// TestUpdateDomain_AuthFail checks an authentication problem scenario.
func (suite *UpdateDomainTestSuite) TestUpdateDomain_AuthFail() {
	correlationID := "test-corr-id"
	domain := entities.Domain{
		Name: "test-domain",
	}

	// Provide a mismatched token that triggers a 401 in the test server
	BEARER_TOKEN = "wrong-token"

	updatedDomain, err := UpdateDomain(correlationID, domain)
	suite.Require().Error(err)
	suite.Assert().Contains(err.Error(), "failed to update domain (401)")
	suite.Assert().Nil(updatedDomain)
}

// TestUpdateDomain_ServerError checks how the function handles non-200 responses from the server.
func (suite *UpdateDomainTestSuite) TestUpdateDomain_ServerError() {
	correlationID := "test-corr-id"
	domain := entities.Domain{
		Name: "error-domain",
	}

	// Correct token so the server goes to the 500 branch
	BEARER_TOKEN = suite.originalBearerToken

	updatedDomain, err := UpdateDomain(correlationID, domain)
	suite.Require().Error(err)
	suite.Assert().Contains(err.Error(), "failed to update domain (500): internal server error")
	suite.Assert().Nil(updatedDomain)
}

// TestUpdateDomain_NotFound checks handling of a 404 response from the server.
func (suite *UpdateDomainTestSuite) TestUpdateDomain_NotFound() {
	correlationID := "test-corr-id"
	domain := entities.Domain{
		Name: "does-not-exist-domain",
	}

	// Use correct token
	BEARER_TOKEN = suite.originalBearerToken

	updatedDomain, err := UpdateDomain(correlationID, domain)
	suite.Require().Error(err)
	// We check the status code in the error string
	suite.Assert().Contains(err.Error(), "failed to update domain (404)")
	suite.Assert().Nil(updatedDomain)
}

// Finally, tell Go to run our suite.
func TestUpdateDomainTestSuite(t *testing.T) {
	suite.Run(t, new(UpdateDomainTestSuite))
}
