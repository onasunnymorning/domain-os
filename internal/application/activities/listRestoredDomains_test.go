package activities

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/assert"
)

func TestListRestoredDomains(t *testing.T) {
	BASEURL = "http://example.com"
	BEARER_TOKEN = "test-token"
	BATCHSIZE = 10

	tests := []struct {
		name           string
		correlationID  string
		query          *queries.RestoredDomainsQuery
		mockResponse   string
		mockStatusCode int
		expectedError  error
		expectedResult []response.DomainRestoredItem
	}{
		{
			name:           "successful fetch",
			correlationID:  "test-correlation-id",
			query:          &queries.RestoredDomainsQuery{},
			mockResponse:   `{"meta":{},"data":[{"name":"example.com"}]}`,
			mockStatusCode: http.StatusOK,
			expectedError:  nil,
			expectedResult: []response.DomainRestoredItem{{Name: "example.com"}},
		},
		{
			name:           "failed to fetch domains",
			correlationID:  "test-correlation-id",
			query:          &queries.RestoredDomainsQuery{},
			mockResponse:   `{"error":"something went wrong"}`,
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  fmt.Errorf("failed to fetch domain count (500): {\"error\":\"something went wrong\"}"),
			expectedResult: nil,
		},
		{
			name:           "failed to unmarshal response",
			correlationID:  "test-correlation-id",
			query:          &queries.RestoredDomainsQuery{},
			mockResponse:   `invalid json`,
			mockStatusCode: http.StatusOK,
			expectedError:  errors.New("failed to unmarshal response"),
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			BASEURL = server.URL

			result, err := ListRestoredDomains(tt.correlationID, tt.query)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
