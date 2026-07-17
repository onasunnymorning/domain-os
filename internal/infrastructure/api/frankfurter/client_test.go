package frankfurter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLatestRates_Success(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"date":"2026-07-17","base":"USD","quote":"EUR","rate":0.87209},
			{"date":"2026-07-17","base":"USD","quote":"PEN","rate":3.3903}
		]`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	rates, err := c.GetLatestRates(context.Background(), "usd", []string{"eur", "pen"})
	require.NoError(t, err)

	assert.Equal(t, "/rates", gotPath)
	assert.Contains(t, gotQuery, "base=USD", "base must be uppercased")
	assert.Contains(t, gotQuery, "quotes=EUR%2CPEN", "quotes must be uppercased and comma-joined")

	require.Len(t, rates, 2)
	assert.Equal(t, "USD", rates[0].Base)
	assert.Equal(t, "EUR", rates[0].Quote)
	assert.Equal(t, 0.87209, rates[0].Rate)

	parsed, err := rates[0].ParsedDate()
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC), parsed)
}

func TestGetLatestRates_NoQuotesParamWhenEmpty(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"date":"2026-07-17","base":"USD","quote":"EUR","rate":0.87}]`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetLatestRates(context.Background(), "USD", nil)
	require.NoError(t, err)
	assert.NotContains(t, gotQuery, "quotes=")
}

func TestGetLatestRates_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	rates, err := c.GetLatestRates(context.Background(), "XXX", nil)
	require.Error(t, err)
	assert.Nil(t, rates)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "XXX")
}

func TestGetLatestRates_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"not":"an array"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetLatestRates(context.Background(), "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestGetLatestRates_EmptyRates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetLatestRates(context.Background(), "USD", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rates returned")
}

func TestGetLatestRates_EmptyBase(t *testing.T) {
	c := NewClient()
	_, err := c.GetLatestRates(context.Background(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base currency is required")
}

func TestGetLatestRates_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetLatestRates(ctx, "USD", nil)
	require.Error(t, err)
}
