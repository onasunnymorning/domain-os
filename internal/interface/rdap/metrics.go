package rdap

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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

// Why the node refused work, for rdap_shed_total.
const (
	shedInFlight    = "in_flight"
	shedConnections = "connections"
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
	policiesRefreshed    prometheus.Gauge
	policyRefreshErrors  prometheus.Counter
	policyDecisions      *prometheus.CounterVec
	ratesDegraded        prometheus.Gauge
	shed                 *prometheus.CounterVec
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
		policiesRefreshed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rdap_policies_refreshed_timestamp_seconds",
			Help: "When this node last loaded the TLDs' publication policies, as a Unix timestamp.",
		}),
		policyRefreshErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rdap_policy_refresh_errors_total",
			Help: "Failed reloads of the publication or access policies. The node keeps the set last loaded.",
		}),
		policyDecisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rdap_policy_decisions_total",
			Help: "Requests allowed or refused (429) by the client's access policy, with the scope that refused: address or aggregate (ADR 0022).",
		}, []string{"policy", "scope", "action"}),
		ratesDegraded: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rdap_ratelimit_degraded",
			Help: "1 while Redis cannot answer and the node limits clients with its own buckets (fail open).",
		}),
		shed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rdap_shed_total",
			Help: "Work the node refused to protect itself: requests over RDAP_MAX_IN_FLIGHT (503), connections over RDAP_MAX_CONNECTIONS (closed).",
		}, []string{"reason"}),
	}
	m.registry.MustRegister(
		m.requests, m.requestDuration, m.inFlight, m.certNotAfter, m.certReloadErrors, m.draining, m.lookupErrors, m.redactions, m.nameserversAmbiguous, m.policiesRefreshed, m.policyRefreshErrors, m.policyDecisions, m.ratesDegraded, m.shed,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	// The series alerts take increase() of start at zero, so that the first
	// event is an increase and not the series' first sample.
	for _, route := range []string{routeDomain, routeNameserver, routeEntity} {
		m.lookupErrors.WithLabelValues(route)
	}
	for _, reason := range []string{shedInFlight, shedConnections} {
		m.shed.WithLabelValues(reason)
	}
	for _, scope := range []string{scopeAddress, scopeAggregate} {
		m.policyDecisions.WithLabelValues(entities.RDAPAccessMonitoring, scope, actionRefused)
	}
	return m
}
