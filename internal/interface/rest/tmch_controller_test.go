package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// fakeTMCH is the service as the controller and the collector see it.
type fakeTMCH struct {
	configured bool
	status     *entities.TMCHStatus
	statusErr  error
	refresh    *queries.TMCHRefreshResult
	refreshErr error
	refreshes  int
}

func (f *fakeTMCH) Configured() bool { return f.configured }
func (f *fakeTMCH) Status(context.Context) (*entities.TMCHStatus, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if f.status == nil {
		return &entities.TMCHStatus{}, nil
	}
	return f.status, nil
}
func (f *fakeTMCH) Refresh(context.Context) (*queries.TMCHRefreshResult, error) {
	f.refreshes++
	return f.refresh, f.refreshErr
}

func tmchRouter(svc *fakeTMCH, scopes ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	grant := func(c *gin.Context) {
		c.Request = c.Request.WithContext(appcontext.WithAuthScopes(c.Request.Context(), scopes))
		c.Next()
	}
	NewTMCHController(r, svc, grant)
	return r
}

func tmchDo(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestTMCHController(t *testing.T) {
	gen := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	svc := &fakeTMCH{configured: true, status: &entities.TMCHStatus{DNL: &entities.TMCHSetStatus{Set: "dnl", GeneratedAt: gen, FetchedAt: gen.Add(time.Hour), Entries: 42}}}

	w := tmchDo(tmchRouter(svc), http.MethodGet, "/tmch")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"configured":true`)
	assert.Contains(t, w.Body.String(), `"entries":42`)
	assert.Contains(t, w.Body.String(), `"smdrl":null`, "a set never fetched is null")

	w = tmchDo(tmchRouter(svc), http.MethodPost, "/tmch/refresh")
	assert.Equal(t, http.StatusForbidden, w.Code, "refreshing needs registry:policy:admin")
	assert.Equal(t, 0, svc.refreshes)

	svc.refresh = &queries.TMCHRefreshResult{Sets: []queries.TMCHSetRefresh{{Set: "dnl", Outcome: queries.TMCHRefreshed}}}
	w = tmchDo(tmchRouter(svc, ScopeRegistryPolicyAdmin), http.MethodPost, "/tmch/refresh")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"outcome":"refreshed"`)
	assert.Equal(t, 1, svc.refreshes)

	svc.refreshErr = errors.New("smdrl: 401 Unauthorized")
	w = tmchDo(tmchRouter(svc, ScopeRegistryPolicyAdmin), http.MethodPost, "/tmch/refresh")
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "401 Unauthorized")
	assert.Contains(t, w.Body.String(), `"result"`, "the per-set outcome comes with the failure")

	svc.refreshErr = entities.ErrTMCHNotConfigured
	w = tmchDo(tmchRouter(svc, ScopeRegistryPolicyAdmin), http.MethodPost, "/tmch/refresh")
	assert.Equal(t, http.StatusConflict, w.Code)

	svc.statusErr = errors.New("db down")
	w = tmchDo(tmchRouter(svc), http.MethodGet, "/tmch")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTMCHCollector(t *testing.T) {
	gen := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	expires := gen.AddDate(5, 0, 0)
	svc := &fakeTMCH{configured: true, status: &entities.TMCHStatus{
		Certificates: &entities.TMCHSetStatus{Set: "certificates", GeneratedAt: gen.AddDate(-1, 0, 0), FetchedAt: gen, Entries: 2, ExpiresAt: &expires},
		DNL:          &entities.TMCHSetStatus{Set: "dnl", GeneratedAt: gen, FetchedAt: gen.Add(time.Hour), Entries: 42},
	}}
	reg := prometheus.NewPedanticRegistry()
	c := NewTMCHCollector(svc)
	require.NoError(t, reg.Register(c))

	assert.Equal(t, 1.0, gaugeValue(t, reg, "tmch_feed_configured", nil))
	assert.Equal(t, float64(gen.Unix()), gaugeValue(t, reg, "tmch_set_generated_timestamp_seconds", map[string]string{"set": "dnl"}))
	assert.Equal(t, float64(gen.Add(time.Hour).Unix()), gaugeValue(t, reg, "tmch_set_fetched_timestamp_seconds", map[string]string{"set": "dnl"}))
	assert.Equal(t, 42.0, gaugeValue(t, reg, "tmch_set_entries", map[string]string{"set": "dnl"}))
	assert.Equal(t, 0.0, gaugeValue(t, reg, "tmch_set_fetched_timestamp_seconds", map[string]string{"set": "smdrl"}), "0 until first fetched")
	assert.Equal(t, float64(expires.Unix()), gaugeValue(t, reg, "tmch_certificates_not_after_timestamp_seconds", nil))
	assert.Equal(t, 0.0, gaugeValue(t, reg, "tmch_status_read_errors_total", nil))

	svc.statusErr = errors.New("db down")
	assert.Equal(t, 0.0, gaugeValue(t, reg, "tmch_set_entries", map[string]string{"set": "dnl"}), "an unreadable status reads as nothing stored")
	assert.GreaterOrEqual(t, gaugeValue(t, reg, "tmch_status_read_errors_total", nil), 1.0, "every scrape that cannot read counts")
}

func gaugeValue(t *testing.T, g prometheus.Gatherer, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := g.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
	next:
		for _, m := range f.GetMetric() {
			for k, v := range labels {
				found := false
				for _, l := range m.GetLabel() {
					if l.GetName() == k && l.GetValue() == v {
						found = true
					}
				}
				if !found {
					continue next
				}
			}
			if m.GetGauge() != nil {
				return m.GetGauge().GetValue()
			}
			return m.GetCounter().GetValue()
		}
	}
	t.Fatalf("metric %s%v not gathered", name, labels)
	return 0
}

// TestTMCHAlertRulesNameServedMetrics is the EPP ops-assets check for the
// TMCH rules: every tmch_ metric a rule names is one the collector
// describes, severities are the three the runbook knows, and every runbook
// link resolves to a section.
func TestTMCHAlertRulesNameServedMetrics(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "prometheus", "rules", "tmch.rules.yml"))
	require.NoError(t, err)
	var rules struct {
		Groups []struct {
			Name  string `yaml:"name"`
			Rules []struct {
				Alert       string            `yaml:"alert"`
				Expr        string            `yaml:"expr"`
				Labels      map[string]string `yaml:"labels"`
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &rules))
	runbook, err := os.ReadFile(filepath.Join(root, "docs", "epp", "RUNBOOK.md"))
	require.NoError(t, err)

	served := map[string]bool{}
	descs := make(chan *prometheus.Desc, 16)
	NewTMCHCollector(&fakeTMCH{}).Describe(descs)
	close(descs)
	for d := range descs {
		served[regexp.MustCompile(`fqName: "([a-z_]+)"`).FindStringSubmatch(d.String())[1]] = true
	}

	metricRe := regexp.MustCompile(`tmch_[a-z_]+`)
	count := 0
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			count++
			assert.Contains(t, []string{"critical", "warning", "info"}, r.Labels["severity"], r.Alert)
			link := r.Annotations["runbook"]
			require.True(t, strings.HasPrefix(link, "docs/epp/RUNBOOK.md#"), "%s runbook link %q", r.Alert, link)
			assert.True(t, hasRunbookAnchor(string(runbook), strings.TrimPrefix(link, "docs/epp/RUNBOOK.md#")), "%s links to a missing runbook section %q", r.Alert, link)
			for _, m := range metricRe.FindAllString(r.Expr, -1) {
				assert.True(t, served[m], "%s names %s, which the collector does not serve", r.Alert, m)
			}
		}
	}
	assert.Greater(t, count, 0)
}

func hasRunbookAnchor(markdown, anchor string) bool {
	drop := regexp.MustCompile(`[^a-z0-9 -]`)
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "#")))
		if strings.ReplaceAll(drop.ReplaceAllString(title, ""), " ", "-") == anchor {
			return true
		}
	}
	return false
}
