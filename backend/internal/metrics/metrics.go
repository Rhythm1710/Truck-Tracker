// Package metrics exposes Prometheus collectors used across the server.
//
// One Metrics struct is constructed at startup, registered with a custom
// prometheus.Registry (so we don't pollute the global default), and
// shared by passing the *Metrics into the components that need it.
package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	// --- Bus ---
	BusPublishTotal *prometheus.CounterVec // kind={position,heartbeat,alarm,...}
	BusDropTotal    prometheus.Counter     // dropped due to slow subscriber

	// --- WebSocket ---
	WSClients   prometheus.Gauge
	WSMessages  prometheus.Counter
	WSDropTotal prometheus.Counter // dropped due to slow client

	// --- TCP / trackers ---
	TCPConnectionsOpen   prometheus.Gauge
	TCPConnectionsTotal  prometheus.Counter
	TCPFramesTotal       *prometheus.CounterVec // kind={login,heartbeat,position,alarm,unknown}
	TCPDecodeErrorsTotal *prometheus.CounterVec // err={crc,framing,short,unsupported}

	// --- Store / DB ---
	PositionSaveDuration prometheus.Histogram
	PositionSaveErrors   prometheus.Counter
}

// New constructs and registers all collectors against reg. Returns the
// Metrics struct for handing to the components.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		BusPublishTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "bus_publish_total", Help: "Frames published to the position bus."},
			[]string{"kind"},
		),
		BusDropTotal: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "bus_drop_total", Help: "Bus messages dropped due to a slow subscriber."},
		),

		WSClients: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "ws_clients", Help: "Currently connected WebSocket clients."},
		),
		WSMessages: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "ws_messages_total", Help: "Messages written to WebSocket clients."},
		),
		WSDropTotal: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "ws_drop_total", Help: "WebSocket writes dropped due to slow client."},
		),

		TCPConnectionsOpen: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "tcp_connections_open", Help: "Currently open tracker TCP connections."},
		),
		TCPConnectionsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "tcp_connections_total", Help: "Total tracker TCP connections accepted."},
		),
		TCPFramesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "tcp_frames_total", Help: "Tracker frames parsed by kind."},
			[]string{"kind"},
		),
		TCPDecodeErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "tcp_decode_errors_total", Help: "Tracker frame decode errors by class."},
			[]string{"err"},
		),

		PositionSaveDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "position_save_duration_seconds",
			Help:    "Postgres INSERT latency for positions.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
		}),
		PositionSaveErrors: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "position_save_errors_total", Help: "Postgres INSERT errors for positions."},
		),
	}

	reg.MustRegister(
		m.BusPublishTotal, m.BusDropTotal,
		m.WSClients, m.WSMessages, m.WSDropTotal,
		m.TCPConnectionsOpen, m.TCPConnectionsTotal, m.TCPFramesTotal, m.TCPDecodeErrorsTotal,
		m.PositionSaveDuration, m.PositionSaveErrors,
	)
	return m
}
