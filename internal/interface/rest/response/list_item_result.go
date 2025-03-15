package response

import (
	"encoding/base64"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
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
func (r *ListItemResult) SetMeta(ctx *gin.Context, cursor string, listLength, pageSize int, filter queries.ListItemsFilter) {
	r.Meta.PageSize = pageSize
	// Only set the cursor and nextlink if we have a non-empty cursor (meaning there is a next page)
	if cursor != "" {
		r.Meta.PageCursor = base64.URLEncoding.EncodeToString([]byte(cursor))

		// Create a NextLink that retrieves the next oage with the same filters applied
		nextLink := fmt.Sprintf("http://%s%s?pagesize=%d&cursor=%s", ctx.Request.Host, ctx.Request.URL.Path, r.Meta.PageSize, r.Meta.PageCursor)
		// If there is a filter, add it to the next link so it keeps filters the same on the next page
		if filter != nil && filter.ToQueryParams() != "" {
			nextLink += filter.ToQueryParams()
		}
		r.Meta.NextLink = nextLink
	}
}
