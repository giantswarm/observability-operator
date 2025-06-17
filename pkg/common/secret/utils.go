package secret

import (
	"context"
	"fmt"

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

	if err := providedClient.Delete(ctx, current); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete secret %s in namespace %s: %w", secretName, secretNamespace, err)
	}

	return nil
}
