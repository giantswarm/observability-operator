package metrics

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BackgroundMetricsCollector runs periodic metrics collection
type BackgroundMetricsCollector struct {
	client    client.Client
	collector *GrafanaOrganizationCollector
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewBackgroundMetricsCollector creates a new background metrics collector
func NewBackgroundMetricsCollector(client client.Client, interval time.Duration) *BackgroundMetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackgroundMetricsCollector{
		client:    client,
		collector: NewGrafanaOrganizationCollector(client),
		interval:  interval,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the background metrics collection
func (b *BackgroundMetricsCollector) Start() {
	logger := log.FromContext(b.ctx).WithName("metrics-collector")
	logger.Info("Starting background metrics collection", "interval", b.interval)

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	// Collect metrics immediately on start
	if err := b.collector.CollectMetrics(b.ctx); err != nil {
		logger.Error(err, "Failed to collect initial metrics")
	}

	for {
		select {
		case <-ticker.C:
			if err := b.collector.CollectMetrics(b.ctx); err != nil {
				logger.Error(err, "Failed to collect metrics")
			}
		case <-b.ctx.Done():
			logger.Info("Stopping background metrics collection")
			return
		}
	}
}

// Stop stops the background metrics collection
func (b *BackgroundMetricsCollector) Stop() {
	b.cancel()
}
