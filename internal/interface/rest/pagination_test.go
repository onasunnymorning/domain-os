package rest

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetPageSize(t *testing.T) {
	// Create a test context
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	// Test case 1: Valid page size
	req, err := http.NewRequest("GET", "/path?pagesize=10", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx.Request = req
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pageSize != 10 {
		t.Errorf("expected page size 10, got %d", pageSize)
	}

	// Test case 2: Page size not an integer
	req, err = http.NewRequest("GET", "/path?pagesize=abc", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	_, err = GetPageSize(ctx)
	if !errors.Is(err, ErrPageSizeNotInteger) {
		t.Errorf("expected ErrPageSizeNotInteger, got %v", err)
	}

	// Test case 3: Page size exceeds maximum
	req, err = http.NewRequest("GET", "/path?pagesize=10000", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	pageSize, err = GetPageSize(ctx)
	if !errors.Is(err, ErrMaxPageSizeExceeded) {
		t.Errorf("expected ErrMaxPageSizeExceeded, got %v", err)
	}
	if pageSize != MAX_PAGE_SIZE {
		t.Errorf("expected page size %d, got %d", MAX_PAGE_SIZE, pageSize)
	}

	// Test case 4: Page size -1
	req, err = http.NewRequest("GET", "/path?pagesize=-1", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	pageSize, err = GetPageSize(ctx)
	if !errors.Is(err, ErrPageSizeNotInteger) {
		t.Errorf("expected ErrPageSizeNotInteger, got %v", err)
	}
	if pageSize != DEFAULT_PAGE_SIZE {
		t.Errorf("expected page size %d, got %d", DEFAULT_PAGE_SIZE, pageSize)
	}
}

func TestGetAndDecodeCursor(t *testing.T) {
	// Test case 1: Empty cursor
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request, _ = http.NewRequest("GET", "/path?cursor=", nil)
	cursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %s", cursor)
	}

	// Test case 2: Invalid cursor
	req, err := http.NewRequest("GET", "/path?cursor=invalid", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	cursor, err = GetAndDecodeCursor(ctx)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Errorf("expected ErrInvalidCursor, got %v", err)
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %s", cursor)
	}

	// Test case 3: Valid cursor
	validCursor := "1729468286778740736_RAR-APEX"
	encodedCursor := base64.URLEncoding.EncodeToString([]byte(validCursor))
	req, err = http.NewRequest("GET", "/path?cursor="+encodedCursor, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	cursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cursor != validCursor {
		t.Errorf("expected cursor %s, got %s", validCursor, cursor)
	}
}
