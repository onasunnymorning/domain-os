package rdap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The alert rules (prometheus/rules/rdap.rules.yml) and the Grafana dashboard
// (grafana/dashboards/rdap.json) name metrics by string. These tests fail
// when one names a metric the RDAP server does not serve, or an alert links
// to a runbook section that does not exist, so renaming a metric cannot
// leave an alert silently watching nothing. The same guard as the EPP
// server's (internal/interface/epp/ops_assets_test.go).

// servedMetrics are the names of every metric the ops port serves: those
// defined in this package (Name: "rdap_..." in its metric options), checked
// against the registry, which must hold every one.
func servedMetrics(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	defined := regexp.MustCompile(`Name:\s+"(rdap_[a-z_]+)"`)
	names := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, m := range defined.FindAllStringSubmatch(string(src), -1) {
			names[m[1]] = true
		}
	}
	require.NotEmpty(t, names)

	// Every defined name is registered: a metric defined but never
	// registered is not served.
	m := newMetrics()
	described := map[string]bool{}
	ch := make(chan *prometheus.Desc, 64)
	go func() { m.registry.Describe(ch); close(ch) }()
	fqName := regexp.MustCompile(`fqName: "([^"]+)"`)
	for d := range ch {
		if sub := fqName.FindStringSubmatch(d.String()); sub != nil {
			described[sub[1]] = true
		}
	}
	for n := range names {
		assert.True(t, described[n], "%s is defined but not registered", n)
	}
	return names
}

var rdapMetricName = regexp.MustCompile(`\brdap_[a-z_]+\b`)

// referenced are the rdap_ metrics in exprs, histogram series folded into
// their histogram.
func referenced(exprs []string) []string {
	var out []string
	for _, e := range exprs {
		for _, n := range rdapMetricName.FindAllString(e, -1) {
			for _, suffix := range []string{"_bucket", "_sum", "_count"} {
				n = strings.TrimSuffix(n, suffix)
			}
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestAlertRulesNameServedMetrics(t *testing.T) {
	raw, err := os.ReadFile("../../../prometheus/rules/rdap.rules.yml")
	require.NoError(t, err)
	var rules struct {
		Groups []struct {
			Rules []struct {
				Alert       string            `yaml:"alert"`
				Expr        string            `yaml:"expr"`
				Labels      map[string]string `yaml:"labels"`
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &rules))
	runbook, err := os.ReadFile("../../../docs/rdap/RUNBOOK.md")
	require.NoError(t, err)

	var exprs []string
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			exprs = append(exprs, r.Expr)
			assert.Contains(t, []string{"critical", "warning", "info"}, r.Labels["severity"], r.Alert)
			anchor, ok := strings.CutPrefix(r.Annotations["runbook"], "docs/rdap/RUNBOOK.md#")
			if assert.True(t, ok, "%s links to its runbook section", r.Alert) {
				assert.True(t, hasAnchor(string(runbook), anchor), "%s: RUNBOOK.md has no section #%s", r.Alert, anchor)
			}
		}
	}
	served := servedMetrics(t)
	refs := referenced(exprs)
	require.NotEmpty(t, refs)
	for _, n := range refs {
		assert.True(t, served[n], "an alert rule names %s, which the RDAP server does not serve", n)
	}
}

func TestDashboardNamesServedMetrics(t *testing.T) {
	raw, err := os.ReadFile("../../../grafana/dashboards/rdap.json")
	require.NoError(t, err)
	var dash struct {
		Panels []struct {
			Title   string `json:"title"`
			Targets []struct {
				Expr string `json:"expr"`
			} `json:"targets"`
		} `json:"panels"`
	}
	require.NoError(t, json.Unmarshal(raw, &dash))
	var exprs []string
	for _, p := range dash.Panels {
		for _, tg := range p.Targets {
			exprs = append(exprs, tg.Expr)
		}
	}
	served := servedMetrics(t)
	refs := referenced(exprs)
	require.NotEmpty(t, refs)
	for _, n := range refs {
		assert.True(t, served[n], "a dashboard panel names %s, which the RDAP server does not serve", n)
	}
}

// hasAnchor reports whether markdown has a heading whose GitHub anchor is
// anchor: lower-cased, punctuation dropped, spaces as hyphens.
func hasAnchor(markdown, anchor string) bool {
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

func TestAlertedCountersStartAtZero(t *testing.T) {
	// An alert on increase() misses the first event of a series that
	// appears with it, so these are exposed before any traffic.
	ts := startServer(t, testConfig("https://rdap.example.test/"), registry())
	metrics := opsGet(ts.Server, "/metrics").Body.String()
	for _, series := range []string{
		`rdap_lookup_errors_total{route="domain"} 0`,
		`rdap_lookup_errors_total{route="nameserver"} 0`,
		`rdap_lookup_errors_total{route="entity"} 0`,
		`rdap_shed_total{reason="in_flight"} 0`,
		`rdap_shed_total{reason="connections"} 0`,
		`rdap_policy_decisions_total{action="refused",policy="monitoring",scope="address"} 0`,
		`rdap_policy_decisions_total{action="refused",policy="monitoring",scope="aggregate"} 0`,
	} {
		assert.Contains(t, metrics, series)
	}
}
