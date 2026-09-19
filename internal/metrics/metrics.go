// Package metrics registers and exposes Prometheus metrics for the routing engine.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts every routed request by route class, model, provider
	// and outcome (success | blocked | no_route | provider_error).
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "router",
		Name:      "requests_total",
		Help:      "Total routed requests.",
	}, []string{"route_class", "model", "provider", "status"})

	// RoutingOverheadMs records the time spent inside engine.Decide in milliseconds.
	RoutingOverheadMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "router",
		Name:      "routing_overhead_ms",
		Help:      "Time spent in the routing engine (ms). Routing is typically sub-millisecond, hence fine-grained low buckets.",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 50, 100, 200},
	})

	// FirstTokenMs records time-to-first-token for streaming requests.
	FirstTokenMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "router",
		Name:      "first_token_ms",
		Help:      "Time from request start to first streamed token (ms).",
		Buckets:   []float64{100, 250, 500, 750, 1000, 1500, 2500, 5000, 10000},
	}, []string{"model", "provider"})

	// ProviderDurationMs records total provider call duration.
	ProviderDurationMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "router",
		Name:      "provider_duration_ms",
		Help:      "Total provider call duration (ms).",
		Buckets:   []float64{100, 500, 1000, 2000, 5000, 10000, 30000, 60000},
	}, []string{"model", "provider", "success"})

	// EventQueueBacklog is the current number of events waiting in the async queue.
	EventQueueBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "router",
		Name:      "event_queue_backlog",
		Help:      "Number of events currently waiting in the async event queue.",
	})

	// EventQueueDropped counts events dropped because the queue was full.
	EventQueueDropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "router",
		Name:      "event_queue_dropped_total",
		Help:      "Total events dropped due to a full async event queue.",
	})

	// ProviderHealthScore tracks the current health score (0–1) per provider.
	ProviderHealthScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "router",
		Name:      "provider_health_score",
		Help:      "Current provider health score (0 = down, 1 = fully healthy).",
	}, []string{"provider"})

	// FallbacksTotal counts streaming fallback activations.
	FallbacksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "router",
		Name:      "fallbacks_total",
		Help:      "Total streaming fallback activations by reason.",
	}, []string{"provider", "reason"})
)
