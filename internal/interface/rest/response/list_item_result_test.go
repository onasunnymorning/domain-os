package response

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestListItemResult_SetMeta(t *testing.T) {
	// Setup
	ctx := gin.Context{
		Request: httptest.NewRequest("GET", "/my/path", nil),
	}
	ctx.Request.Host = "localhost:8080"
	lastItem := "co.apex"
	lastItemBase64 := base64.URLEncoding.EncodeToString([]byte(lastItem))

	// Test case 1: Full page of data
	r := ListItemResult{}
	r.SetMeta(&ctx, lastItem, 25, 25)

	require.Equal(t, 25, r.Meta.PageSize, "page size mismatch")
	require.Equal(t, lastItemBase64, r.Meta.PageCursor, "cursor mismatch")
	require.Equal(t, fmt.Sprintf("http://%s/my/path?pagesize=25&cursor=%s", ctx.Request.Host, lastItemBase64), r.Meta.NextLink, "next link mismatch")
}
