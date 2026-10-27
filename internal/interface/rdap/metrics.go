package rdap

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Routes, for the route label. A fixed set, so a request path can never
// mint a new series.
const (
	routeHelp       = "help"
	routeDomain     = "domain"
	routeNameserver = "nameserver"
	routeEntity     = "entity"
	routeSearch     = "search"
	routeInvalid    = "invalid"
)

// metrics are the RDAP server's Prometheus series. They live on their own
// registry, served on the ops port, as the EPP server's do. The names are
// listed in docs/rdap/ARCHITECTURE.md §5.
type metrics struct {
	registry *prometheus.Registry

	requests         *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	inFlight         prometheus.Gauge
	certNotAfter     prometheus.Gauge
	certReloadErrors prometheus.Counter
	draining         prometheus.Gauge
	lookupErrors     *prometheus.CounterVec
	redactions       *prometheus.CounterVec

	nameserversAmbiguous prometheus.Counter
}

func newMetrics() *metrics {
	m := &metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rdap_requests_total",
			Help: "Requests by route and HTTP status code.",
		}, []string{"route", "code"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "rdap_request_duration_seconds",
			Help: "Time from request read to response written, by route. The SLA's round-trip limit is 400ms.",
			// Dense around the SLA's 400ms so its percentiles are exact enough.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.8, 1.6, 3.2},
		}, []string{"route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rdap_requests_in_flight",
			Help: "Requests being served on this node.",
		}),
		certNotAfter: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rdap_server_cert_not_after_timestamp_seconds",
			Help: "Expiry of the server certificate currently presented, as a Unix timestamp.",
		}),
		certReloadErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rdap_server_cert_reload_errors_total",
			Help: "Failed attempts to load a changed server certificate. The previous certificate stays in use.",
		}),
		draining: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rdap_draining",
			Help: "1 while the node is draining for shutdown.",
		}),
		lookupErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rdap_lookup_errors_total",
			Help: "Lookups that failed reading the registry (answered 500), by route.",
		}, []string{"route"}),
		redactions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rdap_redactions_total",
			Help: "Contact fields withheld from responses, by TLD, field and RFC 9537 method. Never the values.",
		}, []string{"tld", "field", "method"}),
		nameserversAmbiguous: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rdap_nameserver_ambiguous_total",
			Help: "Nameserver lookups that matched hosts of more than one registrar, answered with the one the rule picked (ADR 0020 decision 5).",
		}),
	}
	m.registry.MustRegister(
		m.requests, m.requestDuration, m.inFlight, m.certNotAfter, m.certReloadErrors, m.draining, m.lookupErrors, m.redactions, m.nameserversAmbiguous,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}
