package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BundleReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bundle_received_total",
		Help: "Total number of bundles received via WebSocket",
	})

	BundleSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bundle_sent_total",
		Help: "Total number of bundles successfully sent to relays",
	}, []string{"relay"})

	BundleFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bundle_failed_total",
		Help: "Total number of bundles that failed to send to relays",
	}, []string{"relay"})

	RelayLatencyMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_latency_ms",
		Help:    "Latency in milliseconds for relay requests",
		Buckets: prometheus.ExponentialBuckets(10, 2, 12),
	}, []string{"relay"})
)
