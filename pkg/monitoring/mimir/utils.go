package mimir

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

// Checks whether Mimir is enabled in the cluster by listing the pods in the Mimir namespace.
func isMimirEnabled(ctx context.Context) bool {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Fatal(err)
	}

	mimirPods := &corev1.PodList{}
	err = c.List(ctx, mimirPods, client.InNamespace("mimir"))

	if err != nil {
		log.Fatal("error getting pods: %v\n", err)
	}

	// If pods were found in the mimir namespace, this means that mimir is running.
	return len(mimirPods.Items) > 0
}

func getMimirIngressPassword(ctx context.Context, mc string) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", err
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{}

	err = c.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("%s-remote-write-secret", mc),
		Namespace: "org-giantswarm",
	}, secret)
	if err != nil {
		return "", err
	}

	mimirPassword, err := readRemoteWritePasswordFromSecret(*secret)

	return mimirPassword, err
}
