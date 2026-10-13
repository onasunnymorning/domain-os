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
	storeDegraded           *prometheus.GaugeVec
	registrarSessions       *prometheus.GaugeVec
	registryRenewals        *prometheus.CounterVec
	policyDecisions         *prometheus.CounterVec
	auditErrors             *prometheus.CounterVec
	checkNames              prometheus.Histogram
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
			Help: "Policy decisions on validated commands, by the rule that decided and the action: allow, delay, reject or close.",
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
		m.credentialsRefreshed, m.credentialRefreshErrors, m.storeDegraded, m.registrarSessions, m.registryRenewals,
		m.policyDecisions, m.auditErrors, m.checkNames,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}
