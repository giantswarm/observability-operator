package mimir

import (
	"context"

	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type MimirService struct {
	client.Client
}

func (ms *MimirService) ReconcileMimirConfig(ctx context.Context, mc string) error {
	logger := log.FromContext(ctx).WithValues("cluster", mc)
	logger.Info("ensuring mimir config")

	err := ms.CreateOrUpdateIngressSecret(ctx, mc, logger)
	if err != nil {
		logger.Error(err, "failed to create or update mimir config")
		return errors.WithStack(err)
	}

	logger.Info("ensured mimir config")

	return nil
}

func (ms *MimirService) CreateOrUpdateIngressSecret(ctx context.Context, mc string, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      "mimir-gateway-ingress",
		Namespace: "mimir",
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// CREATE SECRET
		logger.Info("building ingress secret")

		password, err := GetMimirIngressPassword(ctx, mc)
		if err != nil {
			return errors.WithStack(err)
		}

		secret, err := BuildIngressSecret(mc, password)
		if err != nil {
			return errors.WithStack(err)
		}
		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("ingress secret successfully created")

		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	// UPDATE SECRET
	/* password, err := readRemoteWritePasswordFromSecret(*current)
	if err != nil {
		return errors.WithStack(err)
	}

	desired, err := pas.buildRemoteWriteSecret(cluster, password, mimirEnabled)
	if err != nil {
		return errors.WithStack(err)
	}
	if !reflect.DeepEqual(current.Data, desired.Data) || !reflect.DeepEqual(current.Finalizers, desired.Finalizers) {
		err = pas.Client.Update(ctx, desired)
		if err != nil {
			return errors.WithStack(err)
		}
	} */

	return nil
}

func (ms *MimirService) DeleteIngressSecret(ctx context.Context) error {
	objectKey := client.ObjectKey{
		Name:      "mimir-gateway-ingress",
		Namespace: "mimir",
	}
	current := &corev1.Secret{}
	// Get the current secret if it exists.
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the secret is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	// Delete the finalizer
	desired := current.DeepCopy()
	controllerutil.RemoveFinalizer(desired, monitoring.MonitoringFinalizer)
	err = ms.Client.Patch(ctx, current, client.MergeFrom(desired))
	if err != nil {
		return errors.WithStack(err)
	}

	err = ms.Client.Delete(ctx, desired)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
