package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metrics   = make(map[string]*prometheus.GaugeVec)
	summaries = make(map[string]*prometheus.SummaryVec)
)

const (
	funcTopicSubsystem = "func_topic"
	pubSubSubsystem    = "pubsub"
	websocketSubsystem = "websocket"
)

// This is Premetheus data modelling and naming convention
// https://prometheus.io/docs/practices/naming/
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
// TODO add regex evaluation against names [a-zA-Z_:][a-zA-Z0-9_:]*

// TenantsGaugeOpt is the description for rest api tenant counts
func TenantsGaugeOpt() prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "pulsar",
		Subsystem: "tenant",
		Name:      "size",
		Help:      "Plusar rest api tenant counts",
	}
}

// SiteLatencyGaugeOpt is the description for hosting site latency gauge
func SiteLatencyGaugeOpt() prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "kafkaesque",
		Subsystem: "webendpoint",
		Name:      "latency_ms",
		Help:      "kafkaesque website endpoint monitor and latency in ms",
	}
}

// MsgLatencyGaugeOpt is the description for Pulsar message latency gauge
func MsgLatencyGaugeOpt(typeName, desc string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "pulsar",
		Subsystem: typeName,
		Name:      "latency_ms",
		Help:      desc,
	}
}

// FuncLatencyGaugeOpt is the description of Pulsar Function latency gauge
func FuncLatencyGaugeOpt() prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "pulsar",
		Subsystem: "function",
		Name:      "latency_ms",
		Help:      "Plusar message latency in ms",
	}
}

// PromGaugeInt registers gauge reading in integer
func PromGaugeInt(opt prometheus.GaugeOpts, cluster string, num int) {
	PromGauge(opt, cluster, float64(num))
}

// PromGauge registers gauge reading
func PromGauge(opt prometheus.GaugeOpts, cluster string, num float64) {
	key := getMetricKey(opt)
	if promMetric, ok := metrics[key]; ok {
		promMetric.WithLabelValues(cluster).Set(num)
	} else {
		newMetric := prometheus.NewGaugeVec(opt, []string{"device"})
		prometheus.Register(newMetric)
		newMetric.WithLabelValues(cluster).Set(num)
		metrics[key] = newMetric
	}
}

// PromLatencySum expose monitoring metrics to Prometheus
func PromLatencySum(opt prometheus.GaugeOpts, cluster string, latency time.Duration) {
	key := getMetricKey(opt)
	ms := float64(latency / time.Millisecond)
	if promMetric, ok := metrics[key]; ok {
		promMetric.WithLabelValues(cluster).Set(ms)
	} else {
		newMetric := prometheus.NewGaugeVec(opt, []string{"device"})
		prometheus.Register(newMetric)
		newMetric.WithLabelValues(cluster).Set(ms)
		metrics[key] = newMetric
	}

	if summary, ok := summaries[key]; ok {
		summary.WithLabelValues(cluster).Observe(ms)
	} else {
		newSummary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace:  opt.Namespace,
			Subsystem:  opt.Subsystem,
			Name:       fmt.Sprintf("%s_hst", opt.Name),
			Help:       opt.Help,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			MaxAge:     30 * time.Minute,
			AgeBuckets: 3,
			BufCap:     500,
		}, []string{"device"})
		prometheus.MustRegister(newSummary)
		newSummary.WithLabelValues(cluster).Observe(ms)
		summaries[key] = newSummary
	}

}

func getMetricKey(opt prometheus.GaugeOpts) string {
	return fmt.Sprintf("%s-%s-%s", opt.Namespace, opt.Subsystem, opt.Name)
}

// GetGaugeType get the Prometheus Gauge Option based on type/subsystem
func GetGaugeType(nameType string) prometheus.GaugeOpts {
	if nameType == funcTopicSubsystem || strings.HasPrefix(nameType, "func_topic") {
		return MsgLatencyGaugeOpt(funcTopicSubsystem, "Plusar function input output topic latency in ms")
	}

	if nameType == websocketSubsystem {
		return MsgLatencyGaugeOpt(websocketSubsystem, "Plusar websocket pubsub topic latency in ms")
	}

	return MsgLatencyGaugeOpt(pubSubSubsystem, "Plusar pubsub message latency in ms")
}
