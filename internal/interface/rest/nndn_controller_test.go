package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/stretchr/testify/assert"
)

func TestGetListNndnsFilterFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		query    string
		expected queries.ListNndnsFilter
	}{
		{
			name:     "Empty query parameters",
			query:    "",
			expected: queries.ListNndnsFilter{},
		},
		{
			name:  "All query parameters provided",
			query: "name_like=testname&reason_like=testreason&reason_equals=exactreason&tld_equals=.com",
			expected: queries.ListNndnsFilter{
				NameLike:     "testname",
				ReasonLike:   "testreason",
				ReasonEquals: "exactreason",
				TldEquals:    ".com",
			},
		},
		{
			name:  "Partial query parameters provided",
			query: "name_like=partial&reason_equals=exact",
			expected: queries.ListNndnsFilter{
				NameLike:     "partial",
				ReasonEquals: "exact",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request with the query parameters specified in tt.query.
			req, err := http.NewRequest(http.MethodGet, "/test?"+tt.query, nil)
			assert.NoError(t, err)

			// Create a gin context from the request.
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = req

			// Call the function being tested.
			filter, err := getListNndnsFilterFromContext(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, filter)
		})
	}
}
