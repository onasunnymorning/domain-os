package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
)

// newScopeTestContext builds a gin context for a request carrying the given
// X-Tenant-ID header. An empty header value means the header is not set at all.
func newScopeTestContext(t *testing.T, header string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req, err := http.NewRequest(http.MethodGet, "/zone-slavings", nil)
	require.NoError(t, err)
	if header != "" {
		req.Header.Set(TenantIDHeader, header)
	}
	ctx.Request = req
	return ctx
}

func TestOperatorScopeFromRequest(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    entities.OperatorID
		wantErr error
	}{
		{
			name:    "missing header",
			header:  "",
			want:    entities.OperatorID(""),
			wantErr: ErrMissingOperatorScope,
		},
		{
			name:    "whitespace only header",
			header:  "   ",
			want:    entities.OperatorID(""),
			wantErr: ErrMissingOperatorScope,
		},
		{
			name:    "too short to be a RyID",
			header:  "ap",
			want:    entities.OperatorID(""),
			wantErr: ErrInvalidOperatorScope,
		},
		{
			name:    "too long to be a RyID",
			header:  "seventeencharacte",
			want:    entities.OperatorID(""),
			wantErr: ErrInvalidOperatorScope,
		},
		{
			name:    "non ASCII",
			header:  "ïnvalïd",
			want:    entities.OperatorID(""),
			wantErr: ErrInvalidOperatorScope,
		},
		{
			name:   "valid",
			header: "apex",
			want:   entities.OperatorID("apex"),
		},
		{
			name:   "valid, surrounding whitespace trimmed",
			header: "  apex  ",
			want:   entities.OperatorID("apex"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := OperatorScopeFromRequest(newScopeTestContext(t, test.header))
			require.Equal(t, test.want, got)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestOperatorScopeFromRequest_IgnoresQueryParam pins the narrowing done when
// getTenantID was retired: scope comes from the header only. The old helper
// also accepted a ?tenant_id= query param, which made the scope trivially
// forgeable from a browser address bar. ADR-0006.
func TestOperatorScopeFromRequest_IgnoresQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req, err := http.NewRequest(http.MethodGet, "/zone-slavings?tenant_id=apex", nil)
	require.NoError(t, err)
	ctx.Request = req

	got, err := OperatorScopeFromRequest(ctx)
	require.ErrorIs(t, err, ErrMissingOperatorScope)
	require.Equal(t, entities.OperatorID(""), got)
}
