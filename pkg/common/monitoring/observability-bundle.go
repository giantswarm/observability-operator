package monitoring

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ObservabilityBundleAppName string = "observability-bundle"

// ObservabilityBundleAppMeta returns metadata for the observability bundle app.
func ObservabilityBundleAppMeta(cluster *clusterv1.Cluster) metav1.ObjectMeta {
	metadata := metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s", cluster.GetName(), ObservabilityBundleAppName),
		Namespace: cluster.GetNamespace(),
	}

	return metadata
}

func GetObservabilityBundleAppVersion(cluster *clusterv1.Cluster, client client.Client, ctx context.Context) (version semver.Version, err error) {
	// Get observability bundle app metadata.
	appMeta := ObservabilityBundleAppMeta(cluster)
	// Retrieve the app.
	var currentApp appv1.App
	err = client.Get(ctx, types.NamespacedName{Name: appMeta.GetName(), Namespace: appMeta.GetNamespace()}, &currentApp)
	if err != nil {
		return version, err
	}
	return semver.Parse(currentApp.Spec.Version)
}
