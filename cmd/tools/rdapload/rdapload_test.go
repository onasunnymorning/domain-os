package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReportsCodesAndMisses(t *testing.T) {
	var n atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "rdapload-"):
			w.WriteHeader(http.StatusNotFound)
		case n.Add(1)%10 == 0:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	rep := run(context.Background(), options{
		Base: srv.URL + "/example/", Names: []string{"a.example", "b.example"},
		Miss: 0.5, Workers: 2, Duration: 300 * time.Millisecond, Insecure: true,
	})
	require.Positive(t, rep.Requests)
	assert.Positive(t, rep.Codes["200"])
	assert.Positive(t, rep.Codes["404"], "misses ask for names that do not exist")
	assert.Equal(t, rep.Codes["429"], rep.RateLimited)
	assert.Zero(t, rep.Errors)
	assert.LessOrEqual(t, rep.P50, rep.P95)
	assert.LessOrEqual(t, rep.P95, rep.Max)
}

func TestSummarise(t *testing.T) {
	rep := summarise([]sample{
		{code: 200, latency: 10 * time.Millisecond},
		{code: 200, latency: 500 * time.Millisecond},
		{code: 503, latency: time.Millisecond},
		{code: 0, latency: time.Second},
	}, 2*time.Second)
	assert.Equal(t, 4, rep.Requests)
	assert.Equal(t, 1, rep.Errors)
	assert.Equal(t, 1, rep.Shed)
	assert.Equal(t, 2, rep.OverSLA, "a transport error that took a second is over the SLA too")
	assert.InDelta(t, 2.0, rep.PerSecond, 0.01)
}

func TestCLI(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	names := filepath.Join(t.TempDir(), "names.txt")
	require.NoError(t, os.WriteFile(names, []byte("# test\na.example\n\nb.example\n"), 0o600))

	var out bytes.Buffer
	require.NoError(t, cli([]string{"-base", srv.URL + "/example", "-names", names, "-duration", "200ms", "-insecure"}, &out))
	assert.Contains(t, out.String(), "over 400 ms")

	assert.Error(t, cli([]string{"-base", srv.URL}, &out), "names are required")
	assert.Error(t, cli([]string{"-name", "a.example", "-miss", "2"}, &out))
}

func TestTruncatedBodyIsAFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("only ten b")) // then the connection ends
	}))
	defer srv.Close()

	s := lookup(context.Background(), srv.Client(), srv.URL)
	assert.Zero(t, s.code, "a 200 whose body never arrived is not a 200")
	assert.Positive(t, s.latency, "its time is still counted")
}
