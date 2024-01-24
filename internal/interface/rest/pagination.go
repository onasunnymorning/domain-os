package rest

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DEFAULT_PAGE_SIZE = 25
	MAX_PAGE_SIZE     = 1000
)

var (
	ErrPageSizeNotInteger  = errors.New("page size must be an integer")
	ErrMaxPageSizeExceeded = fmt.Errorf("page size must be less than or equal to %d", MAX_PAGE_SIZE)
	ErrInvalidCursor       = errors.New("cursor is not a valid base64 encoded string")
)

// GetPageSize returns the page size from the request or sets the page size to the DEFAULT_PAGE_SIZE
// It returns an error if the page size is greater than the MAX_PAGE_SIZE
// For safety it will also return the DEFAULT_PAGE_SIZE if the page size is not an integer and MAX_PAGE_SIZE if the max is exceeded
func GetPageSize(ctx *gin.Context) (int, error) {
	pageSize, err := strconv.Atoi(ctx.DefaultQuery("pagesize", strconv.Itoa(DEFAULT_PAGE_SIZE)))
	if err != nil || pageSize < 0 {
		return DEFAULT_PAGE_SIZE, ErrPageSizeNotInteger
	}
	if pageSize > MAX_PAGE_SIZE {
		return MAX_PAGE_SIZE, ErrMaxPageSizeExceeded
	}
	return pageSize, nil
}

// GetAndDecodeCursor returns the cursor from the request or an empty string
// It returns an error if the cursor is not a valid base64 encoded string
func GetAndDecodeCursor(ctx *gin.Context) (string, error) {
	cursor := ctx.DefaultQuery("cursor", "")
	if cursor == "" {
		return "", nil
	}
	decodedCursor, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", ErrInvalidCursor
	}
	return string(decodedCursor), nil
}
