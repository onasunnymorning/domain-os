package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The DS endpoint's stubs implement only what it calls; anything else
// panics through the nil interface each embeds.
type dsTLDs struct{ interfaces.TLDService }

func (dsTLDs) GetTLDByName(_ context.Context, name string, _ bool) (*entities.TLD, error) {
	if name == "test" {
		return &entities.TLD{Name: "test"}, nil
	}
	return nil, entities.ErrTLDNotFound
}

type dsDomains struct {
	interfaces.DomainService
	asked string
	err   error
}

func (d *dsDomains) GetDSRecordsPerTLD(_ context.Context, tld string) ([]dns.RR, error) {
	d.asked = tld
	if d.err != nil {
		return nil, d.err
	}
	rr, err := dns.NewRR("signed.test. 3600 IN DS 2371 13 2 49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4")
	return []dns.RR{rr}, err
}

func TestGetDSRecordsPerTLD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	domains := &dsDomains{}
	r := gin.New()
	NewTLDController(r, dsTLDs{}, domains, func(c *gin.Context) { c.Next() })
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	w := get("/tlds/TEST/dns/ds-records?format=text")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test", domains.asked, "the TLD is looked up as it is stored, in lower case")
	assert.Contains(t, w.Body.String(), "signed.test.\t3600\tIN\tDS\t2371 13 2 49FD46E6")

	assert.Equal(t, http.StatusNotFound, get("/tlds/nowhere/dns/ds-records").Code)

	domains.err = errors.New(`pq: relation "domain_ds_records" does not exist`)
	w = get("/tlds/test/dns/ds-records")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "domain_ds_records", "database detail stays in the log")
}
