package controller

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

func NewMetricsRegistry(prefix string) *MetricsRegistry {
	deliveriesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_notifications_deliveries_total",
			Help: "Number of delivered notifications.",
		},
		[]string{"trigger", "service", "succeeded"},
	)

	triggerEvaluationsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_notifications_trigger_eval_total",
			Help: "Number of trigger evaluations.",
		},
		[]string{"name", "triggered"},
	)

	registry := &MetricsRegistry{
		Registry:                  prometheus.NewRegistry(),
		deliveriesCounter:         deliveriesCounter,
		triggerEvaluationsCounter: triggerEvaluationsCounter,
	}
	registry.MustRegister(deliveriesCounter)
	registry.MustRegister(triggerEvaluationsCounter)
	return registry
}

type MetricsRegistry struct {
	*prometheus.Registry
	deliveriesCounter         *prometheus.CounterVec
	triggerEvaluationsCounter *prometheus.CounterVec
}

func (r *MetricsRegistry) IncDeliveriesCounter(trigger string, service string, succeeded bool) {
	r.deliveriesCounter.WithLabelValues(trigger, service, strconv.FormatBool(succeeded)).Inc()
}

func (r *MetricsRegistry) IncTriggerEvaluationsCounter(name string, triggered bool) {
	r.triggerEvaluationsCounter.WithLabelValues(name, strconv.FormatBool(triggered)).Inc()
}
