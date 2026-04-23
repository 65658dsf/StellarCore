package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/65658dsf/StellarCore/server/metrics"
)

const (
	namespace       = "frp"
	serverSubsystem = "server"
)

var ServerMetrics metrics.ServerMetrics = newServerMetrics()

type serverMetrics struct {
	clientCount     prometheus.Gauge
	proxyCount      *prometheus.GaugeVec
	connectionCount *prometheus.GaugeVec
	trafficIn       *prometheus.CounterVec
	trafficOut      *prometheus.CounterVec
	detectBlock     *prometheus.CounterVec
	detectAllow     *prometheus.CounterVec
	detectSusp      *prometheus.CounterVec
}

func (m *serverMetrics) NewClient() {
	m.clientCount.Inc()
}

func (m *serverMetrics) CloseClient() {
	m.clientCount.Dec()
}

func (m *serverMetrics) NewProxy(_ string, proxyType string) {
	m.proxyCount.WithLabelValues(proxyType).Inc()
}

func (m *serverMetrics) CloseProxy(_ string, proxyType string) {
	m.proxyCount.WithLabelValues(proxyType).Dec()
}

func (m *serverMetrics) OpenConnection(name string, proxyType string) {
	m.connectionCount.WithLabelValues(name, proxyType).Inc()
}

func (m *serverMetrics) CloseConnection(name string, proxyType string) {
	m.connectionCount.WithLabelValues(name, proxyType).Dec()
}

func (m *serverMetrics) AddTrafficIn(name string, proxyType string, trafficBytes int64) {
	m.trafficIn.WithLabelValues(name, proxyType).Add(float64(trafficBytes))
}

func (m *serverMetrics) AddTrafficOut(name string, proxyType string, trafficBytes int64) {
	m.trafficOut.WithLabelValues(name, proxyType).Add(float64(trafficBytes))
}

func (m *serverMetrics) AddDetectionBlock(transport string, protocol string, reason string) {
	m.detectBlock.WithLabelValues(transport, protocol, reason).Inc()
}

func (m *serverMetrics) AddDetectionAllow(transport string, protocol string, reason string) {
	m.detectAllow.WithLabelValues(transport, protocol, reason).Inc()
}

func (m *serverMetrics) AddDetectionSuspicious(transport string, protocol string, reason string) {
	m.detectSusp.WithLabelValues(transport, protocol, reason).Inc()
}

func newServerMetrics() *serverMetrics {
	m := &serverMetrics{
		clientCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "client_counts",
			Help:      "The current client counts of frps",
		}),
		proxyCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "proxy_counts",
			Help:      "The current proxy counts",
		}, []string{"type"}),
		connectionCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "connection_counts",
			Help:      "The current connection counts",
		}, []string{"name", "type"}),
		trafficIn: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "traffic_in",
			Help:      "The total in traffic",
		}, []string{"name", "type"}),
		trafficOut: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "traffic_out",
			Help:      "The total out traffic",
		}, []string{"name", "type"}),
		detectBlock: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "detect_block_total",
			Help:      "The total number of blocked traffic detections",
		}, []string{"transport", "protocol", "reason"}),
		detectAllow: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "detect_allow_total",
			Help:      "The total number of allowed traffic detections",
		}, []string{"transport", "protocol", "reason"}),
		detectSusp: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: serverSubsystem,
			Name:      "detect_suspicious_total",
			Help:      "The total number of suspicious traffic detections",
		}, []string{"transport", "protocol", "reason"}),
	}
	prometheus.MustRegister(m.clientCount)
	prometheus.MustRegister(m.proxyCount)
	prometheus.MustRegister(m.connectionCount)
	prometheus.MustRegister(m.trafficIn)
	prometheus.MustRegister(m.trafficOut)
	prometheus.MustRegister(m.detectBlock)
	prometheus.MustRegister(m.detectAllow)
	prometheus.MustRegister(m.detectSusp)
	return m
}
