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
	outcomeLimiter         = "rejected_limiter"
	outcomeDraining        = "rejected_draining"
	outcomeHandshakeFailed = "handshake_failed"
	outcomeLoginTimeout    = "login_timeout"
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
			Help: "Commands by command name and EPP result code.",
		}, []string{"command", "code"}),
		commandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "epp_command_duration_seconds",
			Help:    "Time from a complete frame to a written response, by command.",
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
	}
	m.registry.MustRegister(
		m.connections, m.sessionsActive, m.handshakesActive, m.logins, m.commands,
		m.commandDuration, m.frameBytes, m.framesRejected, m.certNotAfter,
		m.certReloadErrors, m.draining,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}
