package rest

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TMCHCollector serves the TMCH data's freshness as Prometheus metrics on the
// admin API (the process Prometheus already scrapes; the worker that
// refreshes the data has no metrics port). Each scrape reads the stored
// status, so the metrics say what every process would read, not what one
// node last did. The alerts in prometheus/rules/tmch.rules.yml are on these.
type TMCHCollector struct {
	svc     interfaces.TMCHService
	timeout time.Duration

	configured  *prometheus.Desc
	generated   *prometheus.Desc
	fetched     *prometheus.Desc
	entries     *prometheus.Desc
	certsExpire *prometheus.Desc
	readErrors  *prometheus.Desc
	errs        atomic.Int64
}

// NewTMCHCollector builds the collector; register it on the registry the
// admin API serves.
func NewTMCHCollector(svc interfaces.TMCHService) *TMCHCollector {
	return &TMCHCollector{
		svc:     svc,
		timeout: 5 * time.Second,
		configured: prometheus.NewDesc("tmch_feed_configured",
			"1 when this installation has a TMCH feed (TMCH_USERNAME and TMCH_PASSWORD set), else 0.", nil, nil),
		generated: prometheus.NewDesc("tmch_set_generated_timestamp_seconds",
			"When the Clearinghouse generated the stored copy of a TMCH data set, as a Unix timestamp; 0 until first fetched.", []string{"set"}, nil),
		fetched: prometheus.NewDesc("tmch_set_fetched_timestamp_seconds",
			"When this registry last fetched a TMCH data set, as a Unix timestamp; 0 until first fetched.", []string{"set"}, nil),
		entries: prometheus.NewDesc("tmch_set_entries",
			"How many entries the stored copy of a TMCH data set holds.", []string{"set"}, nil),
		certsExpire: prometheus.NewDesc("tmch_certificates_not_after_timestamp_seconds",
			"When the last certificate of the stored TMCH CA bundle expires, as a Unix timestamp; 0 until first fetched.", nil, nil),
		readErrors: prometheus.NewDesc("tmch_status_read_errors_total",
			"Scrapes on which the stored TMCH status could not be read.", nil, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *TMCHCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.configured
	ch <- c.generated
	ch <- c.fetched
	ch <- c.entries
	ch <- c.certsExpire
	ch <- c.readErrors
}

// Collect implements prometheus.Collector.
func (c *TMCHCollector) Collect(ch chan<- prometheus.Metric) {
	configured := 0.0
	if c.svc.Configured() {
		configured = 1
	}
	ch <- prometheus.MustNewConstMetric(c.configured, prometheus.GaugeValue, configured)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	status, err := c.svc.Status(ctx)
	if err != nil {
		c.errs.Add(1)
		status = &entities.TMCHStatus{}
	}
	ch <- prometheus.MustNewConstMetric(c.readErrors, prometheus.CounterValue, float64(c.errs.Load()))

	for _, set := range []struct {
		name   string
		status *entities.TMCHSetStatus
	}{
		{entities.TMCHSetCertificates, status.Certificates},
		{entities.TMCHSetSMDRL, status.SMDRL},
		{entities.TMCHSetDNL, status.DNL},
	} {
		var generated, fetched, entries float64
		if set.status != nil {
			generated = float64(set.status.GeneratedAt.Unix())
			fetched = float64(set.status.FetchedAt.Unix())
			entries = float64(set.status.Entries)
		}
		ch <- prometheus.MustNewConstMetric(c.generated, prometheus.GaugeValue, generated, set.name)
		ch <- prometheus.MustNewConstMetric(c.fetched, prometheus.GaugeValue, fetched, set.name)
		ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, entries, set.name)
	}
	var expires float64
	if status.Certificates != nil && status.Certificates.ExpiresAt != nil {
		expires = float64(status.Certificates.ExpiresAt.Unix())
	}
	ch <- prometheus.MustNewConstMetric(c.certsExpire, prometheus.GaugeValue, expires)
}
