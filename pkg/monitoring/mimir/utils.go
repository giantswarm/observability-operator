package mimir

import (
	"context"
	"fmt"

	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func GetMimirIngressPassword(ctx context.Context, mc string) (string, error) {
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

	mimirPassword, err := prometheusagent.ReadRemoteWritePasswordFromSecret(*secret)

	return mimirPassword, err
}
