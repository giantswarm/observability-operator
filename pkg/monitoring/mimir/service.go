package mimir

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/common/secret"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

const (
	authSecretName               = "mimir-gateway-ingress"
	authSecretNamespace          = "mimir"
	mimirSpecificSecretName      = "mimir-basic-auth"
	mimirSpecificSecretNamespace = "mimir"
)

type MimirService struct {
	client.Client
	PasswordManager password.Manager
	SecretManager   secret.Manager
}

func (ms *MimirService) ConfigureMimir(ctx context.Context, mc string) error {
	logger := log.FromContext(ctx).WithValues("cluster", mc)
	logger.Info("ensuring mimir config")

	err := ms.CreateAuthSecret(ctx, logger, mc)
	if err != nil {
		logger.Error(err, "failed to create mimit auth secret")
		return errors.WithStack(err)
	}

	err = ms.CreateIngressSecret(ctx, mc, logger)
	if err != nil {
		logger.Error(err, "failed to create mimir ingress secret")
		return errors.WithStack(err)
	}

	logger.Info("ensured mimir config")

	return nil
}

func (ms *MimirService) CreateAuthSecret(ctx context.Context, logger logr.Logger, mc string) error {
	objectKey := client.ObjectKey{
		Name:      mimirSpecificSecretName,
		Namespace: mimirSpecificSecretNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		logger.Info("Building auth secret")

		password, err := ms.PasswordManager.GeneratePassword(32)
		if err != nil {
			return errors.WithStack(err)
		}

		secretdata := mc + ":" + password

		secret, err := ms.SecretManager.GenerateGenericSecret(
			mimirSpecificSecretName, mimirSpecificSecretNamespace, "credentials", secretdata)
		if err != nil {
			return errors.WithStack(err)
		}

		err = ms.Client.Create(ctx, secret)
		if err != nil {
			return errors.WithStack(err)
		}

		logger.Info("Auth secret successfully created")

		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (ms *MimirService) CreateIngressSecret(ctx context.Context, mc string, logger logr.Logger) error {
	objectKey := client.ObjectKey{
		Name:      authSecretName,
		Namespace: authSecretNamespace,
	}

	current := &corev1.Secret{}
	err := ms.Client.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// CREATE SECRET
		logger.Info("building ingress secret")

		password, err := prometheusagent.GetMimirIngressPassword(ctx)
		if err != nil {
			return errors.WithStack(err)
		}

		htpasswd, err := ms.PasswordManager.GenerateHtpasswd(mc, password)
		if err != nil {
			return errors.WithStack(err)
		}

		secret, err := ms.SecretManager.GenerateGenericSecret(authSecretName, authSecretNamespace, "auth", htpasswd)
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

	return nil
}

func (ms *MimirService) DeleteIngressSecret(ctx context.Context) error {
	objectKey := client.ObjectKey{
		Name:      authSecretName,
		Namespace: authSecretNamespace,
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
