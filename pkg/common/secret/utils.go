package secret

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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
	current := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}

	err := providedClient.Delete(ctx, current)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
