package secret

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateGenericSecret(secretName string, secretNamespace string,
	key string, value string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
		Type: "Opaque",
	}

	return secret
}

func DeleteSecret(secretName string, secretNamespace string,
	ctx context.Context, providedClient client.Client) error {
	objectKey := client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	current := &corev1.Secret{}
	// Get the current secret if it exists.
	err := providedClient.Get(ctx, objectKey, current)
	if apierrors.IsNotFound(err) {
		// Ignore cases where the secret is not found (if it was manually deleted, for instance).
		return nil
	} else if err != nil {
		return errors.WithStack(err)
	}

	err = providedClient.Delete(ctx, current)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
