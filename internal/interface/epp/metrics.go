package epp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Connection outcomes for epp_connections_total.
const (
	outcomeAccepted        = "accepted"
	outcomeNodeCap         = "rejected_node_cap"
	outcomeHandshakeCap    = "rejected_handshake_cap"
	outcomeSourceCap       = "rejected_source_cap"
	outcomeDraining        = "rejected_draining"
	outcomeHandshakeFailed = "handshake_failed"
	outcomeLoginTimeout    = "login_timeout"
	outcomeSourceIP        = "rejected_source_ip"
)

// Why a session was ended by the server, for epp_sessions_ended_total.
const (
	endedCompromised = "credential_compromised"
	endedAdminKill   = "admin_kill"
	// endedSuspended is a session of a login suspended for failed creates.
	endedSuspended = "login_suspended"
)

// Audit failures, for epp_audit_errors_total.
const (
	// auditStageSync is a transform's record that could not be written
	// before its response, and was queued instead.
	auditStageSync = "sync"
	// auditStageBatch is a batch of queued records that failed and is
	// being retried.
	auditStageBatch = "batch"
	// auditStageLost is a record that could not be written at all.
	auditStageLost = "lost"
)

// Shared stores, for epp_policy_store_degraded.
const (
	storeLockout  = "lockout"
	storeSessions = "sessions"
	storeRates    = "rates"
	// storeFailedCreates is the failed-create budget's counts, strikes and
	// suspensions.
	storeFailedCreates = "failed_creates"
)

// metrics are the EPP server's Prometheus series. They live on their own
// registry, served on the ops port, so nothing else in the process can add
// to or collide with them. The names are listed in docs/epp/ARCHITECTURE.md.
type metrics struct {
	registry *prometheus.Registry

	connections      *prometheus.CounterVec
	sessionsActive   prometheus.Gauge
	handshakesActive prometheus.Gauge
	logins           *prometheus.CounterVec
	commands         *prometheus.CounterVec
	commandDuration  *prometheus.HistogramVec
	frameBytes       prometheus.Histogram
	framesRejected   *prometheus.CounterVec
	certNotAfter     prometheus.Gauge
	certReloadErrors prometheus.Counter
	draining         prometheus.Gauge

	loginFailures           *prometheus.CounterVec
	clientCertNotAfter      *prometheus.GaugeVec
	sessionsEnded           *prometheus.CounterVec
	allowedNetworks         prometheus.Gauge
	credentialsRefreshed    prometheus.Gauge
	credentialRefreshErrors prometheus.Counter
	policiesRefreshed       prometheus.Gauge
	policyRefreshErrors     prometheus.Counter
	storeDegraded           *prometheus.GaugeVec
	registrarSessions       *prometheus.GaugeVec
	registryRenewals        *prometheus.CounterVec
	policyDecisions         *prometheus.CounterVec
	auditErrors             *prometheus.CounterVec
	checkNames              prometheus.Histogram
	policyDelay             prometheus.Histogram
	loginsSuspended         *prometheus.CounterVec
	eventsLost              *prometheus.CounterVec
	createQueueWait         prometheus.Histogram
	createQueueTimeouts     prometheus.Counter
}

func newMetrics() *metrics {
	m := &metrics{
		registry: prometheus.NewRegistry(),
		connections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_connections_total",
			Help: "TCP connections by outcome: accepted, or why they were refused or closed early.",
		}, []string{"outcome"}),
		sessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_sessions_active",
			Help: "Sessions on this node past the TLS handshake.",
		}),
		handshakesActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_tls_handshakes_active",
			Help: "TLS handshakes in progress on this node.",
		}),
		logins: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_logins_total",
			Help: "Login attempts by result.",
		}, []string{"result"}),
		commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_commands_total",
			Help: "Commands by registrar (empty before login), command name and EPP result code.",
		}, []string{"clid", "command", "code"}),
		commandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "epp_command_duration_seconds",
			Help:    "Time from a complete frame to its response being ready, including the audit write of a transform, by command.",
			Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"command"}),
		frameBytes: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "epp_frame_bytes",
			Help:    "Size of inbound EPP frames, excluding the 4-byte header.",
			Buckets: prometheus.ExponentialBuckets(256, 2, 10), // 256 B .. 128 KiB
		}),
		framesRejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_frames_rejected_total",
			Help: "Frames answered with 2001 before dispatch, by reason.",
		}, []string{"reason"}),
		certNotAfter: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_server_cert_not_after_timestamp_seconds",
			Help: "Expiry of the server certificate currently presented, as a Unix timestamp.",
		}),
		certReloadErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "epp_server_cert_reload_errors_total",
			Help: "Failed attempts to load a changed server certificate. The previous certificate stays in use.",
		}),
		draining: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_draining",
			Help: "1 while the node is draining for shutdown.",
		}),
		loginFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_login_failures_total",
			Help: "Refused logins by reason. Every reason gets the same answer on the wire.",
		}, []string{"reason"}),
		clientCertNotAfter: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "epp_client_cert_not_after_timestamp_seconds",
			Help: "Expiry of the client certificate last used to log in, by registrar and login, as a Unix timestamp. Alert at 30 and 7 days.",
		}, []string{"clid", "login"}),
		sessionsEnded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_sessions_ended_total",
			Help: "Sessions the server ended, by reason.",
		}, []string{"reason"}),
		allowedNetworks: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_allowed_networks",
			Help: "Networks in the union of registrars' allowlists, as last loaded.",
		}),
		credentialsRefreshed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_credentials_refreshed_timestamp_seconds",
			Help: "When the allowed networks were last loaded, as a Unix timestamp.",
		}),
		credentialRefreshErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "epp_credential_refresh_errors_total",
			Help: "Failed reloads of the allowed networks or the compromised credentials.",
		}),
		policiesRefreshed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "epp_policies_refreshed_timestamp_seconds",
			Help: "When the EPP policies, assignments and registrar families were last loaded, as a Unix timestamp.",
		}),
		policyRefreshErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "epp_policy_refresh_errors_total",
			Help: "Failed reloads of the EPP policies; the node keeps the set it last loaded.",
		}),
		registrarSessions: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "epp_registrar_sessions",
			Help: "Logged-in sessions on this node, by registrar and login. Sum across nodes for the cluster.",
		}, []string{"clid", "login"}),
		registryRenewals: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_session_registry_renewals_total",
			Help: "Heartbeat renewals of this node's sessions in the cluster-wide registry, by result.",
		}, []string{"result"}),
		policyDecisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_policy_decisions_total",
			Help: "Policy decisions on the validated commands of logged-in sessions, by the rule that decided and the action: allow, delay, reject or close.",
		}, []string{"rule", "action"}),
		auditErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_audit_errors_total",
			Help: "Audit log write failures: sync (a transform's record queued instead of written before its response), batch (a queued batch being retried) and lost (a record not written at all).",
		}, []string{"stage"}),
		checkNames: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "epp_check_names",
			Help:    "Names per <check> command.",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100},
		}),
		policyDelay: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "epp_policy_delay_seconds",
			Help:    "How long commands were held for their command rate tokens (decisions with action delay).",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}),
		loginsSuspended: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_logins_suspended_total",
			Help: "Logins suspended for creating registered names past the failed-create budget, by registrar. Alert on any.",
		}, []string{"clid"}),
		createQueueWait: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "epp_create_queue_wait_seconds",
			Help:    "How long <domain:create>s waited for a slot on this node, those that waited at all.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}),
		createQueueTimeouts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "epp_create_queue_timeouts_total",
			Help: "<domain:create>s refused with 2400 for want of a slot within EPP_CREATE_QUEUE_WAIT.",
		}),
		eventsLost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "epp_events_lost_total",
			Help: "Outbox events the EPP server could not write, by event type. What they record still happened.",
		}, []string{"event"}),
		storeDegraded: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "epp_policy_store_degraded",
			Help: "1 while Redis is unreachable for a store and the node fails open to its own counts and caps (ADR 0013).",
		}, []string{"store"}),
	}
	m.registry.MustRegister(
		m.connections, m.sessionsActive, m.handshakesActive, m.logins, m.commands,
		m.commandDuration, m.frameBytes, m.framesRejected, m.certNotAfter,
		m.certReloadErrors, m.draining,
		m.loginFailures, m.clientCertNotAfter, m.sessionsEnded, m.allowedNetworks,
		m.credentialsRefreshed, m.credentialRefreshErrors, m.policiesRefreshed, m.policyRefreshErrors, m.storeDegraded, m.registrarSessions, m.registryRenewals,
		m.policyDecisions, m.auditErrors, m.checkNames, m.policyDelay, m.loginsSuspended, m.eventsLost, m.createQueueWait, m.createQueueTimeouts,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// watchCreates reports the create gate's slots in use and waiters.
func (m *metrics) watchCreates(g *createGate) {
	m.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "epp_creates_in_flight",
			Help: "<domain:create>s running on this node, at most EPP_MAX_CREATES_IN_FLIGHT.",
		}, func() float64 { running, _ := g.inFlight(); return float64(running) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "epp_creates_waiting",
			Help: "<domain:create>s waiting for a slot on this node.",
		}, func() float64 { _, waiting := g.inFlight(); return float64(waiting) }),
	)
}
