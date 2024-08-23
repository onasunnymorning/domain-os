package response

import (
	"encoding/base64"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListItemResult is the struct that adds metadata to the list item query
type ListItemResult struct {
	Meta PaginationMetaData `json:"Meta"`
	Data interface{}        `json:"Data"`
}

// PaginationMetaData is the struct that conatins the metadata for the list item query
type PaginationMetaData struct {
	PageSize   int    `json:"PageSize"`
	PageCursor string `json:"PageCursor"`
	NextLink   string `json:"NextLink"`
}

// SetCursor sets the cursor for the list item query
func (r *ListItemResult) SetMeta(ctx *gin.Context, lastItem string, listLength, pageSize int) {
	r.Meta.PageSize = pageSize
	// Only set the next cursor if we have a full page of data or if we have no more data
	if listLength > 0 && listLength == r.Meta.PageSize {
		r.Meta.PageCursor = base64.URLEncoding.EncodeToString([]byte(lastItem))
		// FIXME: add provision for the searches that may occur on list endpoints
		r.Meta.NextLink = "http://" + ctx.Request.Host + ctx.Request.URL.Path + "?pagesize=" + strconv.Itoa(r.Meta.PageSize) + "&cursor=" + r.Meta.PageCursor
	}
}
