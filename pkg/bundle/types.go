package bundle

import (
	"context"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// BundleService is the interface implemented by BundleConfigurationService.
// The controller depends on this interface so it can be tested without a real k8s cluster.
type BundleService interface {
	Configure(ctx context.Context, cluster *clusterv1.Cluster) error
	RemoveConfiguration(ctx context.Context, cluster *clusterv1.Cluster) error
	GetObservabilityBundleAppVersion(ctx context.Context, cluster *clusterv1.Cluster) (semver.Version, error)
}

type bundleConfiguration struct {
	Apps map[string]app `yaml:"apps" json:"apps"`
}

type app struct {
	AppName string `yaml:"appName,omitempty" json:"appName,omitempty"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}
