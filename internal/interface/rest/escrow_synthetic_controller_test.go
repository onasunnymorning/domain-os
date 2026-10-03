package rest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postSynthetic(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := &EscrowController{}
	r.POST("/escrow/synthetic", c.GenerateSynthetic)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/escrow/synthetic", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestGenerateSynthetic_StreamsDeposit(t *testing.T) {
	w := postSynthetic(t, `{"tld":"Test","domains":3,"contactsPerDomain":2,"avgHostsPerDomain":1.5,"nndns":2,"seed":42}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "application/gzip", w.Header().Get("Content-Type"))
	assert.Regexp(t, `^attachment; filename="test_\d{4}-\d{2}-\d{2}_full_S1_R0\.xml\.gz"$`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "42", w.Header().Get(SyntheticSeedHeader))

	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	xml, err := io.ReadAll(zr)
	require.NoError(t, err)
	s := string(xml)
	assert.Contains(t, s, "<rdeHeader:tld>test</rdeHeader:tld>")
	assert.Equal(t, 3, strings.Count(s, "<rdeDomain:domain>"))
	assert.Equal(t, 6, strings.Count(s, "<rdeContact:contact>"))
	assert.Equal(t, 4, strings.Count(s, "<rdeHost:host>")) // floor(3 × 1.5)
	assert.Equal(t, 2, strings.Count(s, "<rdeNNDN:NNDN>"))
}

func TestGenerateSynthetic_PicksSeedWhenOmitted(t *testing.T) {
	w := postSynthetic(t, `{"tld":"test","domains":1}`)
	require.Equal(t, http.StatusOK, w.Code)
	seed, err := strconv.ParseUint(w.Header().Get(SyntheticSeedHeader), 10, 64)
	require.NoError(t, err)
	assert.Less(t, seed, uint64(1<<53), "a picked seed must fit a JavaScript number")
}

func TestGenerateSynthetic_RejectsBadInput(t *testing.T) {
	for name, body := range map[string]string{
		"no tld":           `{"domains":1}`,
		"not json":         `nope`,
		"invalid tld":      `{"tld":"-bad-"}`,
		"too many domains": `{"tld":"test","domains":250001}`,
		"five contacts":    `{"tld":"test","contactsPerDomain":5}`,
		"too many hosts":   `{"tld":"test","avgHostsPerDomain":14}`,
		"negative nndns":   `{"tld":"test","nndns":-1}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := postSynthetic(t, body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.NotEmpty(t, resp["error"])
			assert.Empty(t, w.Header().Get("Content-Disposition"))
		})
	}
}
