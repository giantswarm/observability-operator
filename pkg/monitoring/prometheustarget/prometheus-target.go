package prometheustarget

import (
	"context"
	"fmt"

	"github.com/giantswarm/observability-operator/pkg/metrics"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const appInstanceLabelName = "app.kubernetes.io/instance"

// Service handles Prometheus target metrics updates.
type Service struct {
	client client.Client
}

// New creates a new Prometheus target metrics service.
func New(c client.Client) *Service {
	return &Service{
		client: c,
	}
}

// UpdateMetrics updates the Prometheus target metrics for ServiceMonitors and PodMonitors.
func (s Service) UpdateMetrics(ctx context.Context) {
	logger := log.FromContext(ctx).WithValues("component", "prometheus-target-metrics")

	// Reset all ServiceMonitor and PodMonitor metrics
	metrics.ObservabilityPrometheusTargetInfo.Reset()

	// List ServiceMonitors
	serviceMonitors := &promopv1.ServiceMonitorList{}
	err := s.list(ctx, serviceMonitors)
	if err != nil {
		logger.Info("failed to list service monitors", "error", err)
	}

	// Set ServiceMonitor metrics
	for _, sm := range serviceMonitors.Items {
		metrics.ObservabilityPrometheusTargetInfo.With(prometheus.Labels{
			"scrape_job": fmt.Sprintf("serviceMonitor/%s/%s", sm.Namespace, sm.Name),
			"app":        sm.Labels[appInstanceLabelName],
		}).Set(1)
	}

	// List PodMonitors
	podMonitors := &promopv1.PodMonitorList{}
	err = s.list(ctx, podMonitors)
	if err != nil {
		logger.Info("failed to list pod monitors", "error", err)
	}

	// Set PodMonitor metrics
	for _, pm := range podMonitors.Items {
		metrics.ObservabilityPrometheusTargetInfo.With(prometheus.Labels{
			"scrape_job": fmt.Sprintf("podMonitors/%s/%s", pm.Namespace, pm.Name),
			"app":        pm.Labels[appInstanceLabelName],
		}).Set(1)
	}
}

// list fetches a list of Kubernetes objects and handles NotFound errors gracefully.
func (s Service) list(ctx context.Context, list client.ObjectList) error {
	if err := s.client.List(ctx, list); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return no error
			return nil
		}

		// Error reading the object
		return err
	}

	return nil
}
