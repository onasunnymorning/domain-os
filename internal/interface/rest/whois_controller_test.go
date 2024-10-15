package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWhoisService is a mock implementation of the WhoisService
type MockWhoisService struct {
	mock.Mock
}

func (m *MockWhoisService) GetDomainWhois(ctx context.Context, domainName string) (*entities.WhoisResponse, error) {
	args := m.Called(ctx, domainName)
	return args.Get(0).(*entities.WhoisResponse), args.Error(1)
}

// MockGinHandler checks for the constant JWT token in the Authorization header
func MockGinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Token is valid; proceed to the next handler
		c.Next()
	}
}

func TestGetWhois_Success(t *testing.T) {
	// Create a gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a mock whois service
	mockWhoisService := new(MockWhoisService)
	expectedWhois := &entities.WhoisResponse{
		DomainName:           "example.com",
		RegistryDomainID:     "1234567890_DOM-APEX",
		RegistrarWhoisServer: "whois.example.com",
		Registrar:            "Mock Registrar",
		// Other fields...
	}

	// Set up mock response
	mockWhoisService.On("GetDomainWhois", mock.Anything, "example.com").Return(expectedWhois, nil)

	// Create the WhoisController
	NewWhoisController(router, mockWhoisService, MockGinHandler())

	// Create a request to the endpoint
	req, _ := http.NewRequest(http.MethodGet, "/whois/example.com", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Assert the response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "example.com")
	assert.Contains(t, w.Body.String(), "Mock Registrar")
	mockWhoisService.AssertExpectations(t)
}

func TestGetWhois_NotFound(t *testing.T) {
	// Create a gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a mock whois service
	mockWhoisService := new(MockWhoisService)

	// Set up mock response for not found error
	mockWhoisService.On("GetDomainWhois", mock.Anything, "notfound.com").Return((*entities.WhoisResponse)(nil), entities.ErrDomainNotFound)

	// Create the WhoisController
	NewWhoisController(router, mockWhoisService, MockGinHandler())

	// Create a request to the endpoint
	req, _ := http.NewRequest(http.MethodGet, "/whois/notfound.com", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Assert the response
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "domain not found")
	mockWhoisService.AssertExpectations(t)
}

func TestGetWhois_InternalServerError(t *testing.T) {
	// Create a gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a mock whois service
	mockWhoisService := new(MockWhoisService)

	// Set up mock response for internal server error
	mockWhoisService.On("GetDomainWhois", mock.Anything, "example.com").Return((*entities.WhoisResponse)(nil), errors.New("internal error"))

	// Create the WhoisController
	NewWhoisController(router, mockWhoisService, MockGinHandler())

	// Create a request to the endpoint
	req, _ := http.NewRequest(http.MethodGet, "/whois/example.com", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Assert the response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal error")
	mockWhoisService.AssertExpectations(t)
}
