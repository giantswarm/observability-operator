package secret

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager interface {
	GenerateGenericSecret(secretName string, secretNamespace string, key string, value string) (*corev1.Secret, error)
}

type SimpleManager struct {
}

func (m SimpleManager) GenerateGenericSecret(secretName string, secretNamespace string,
	key string, value string) (*corev1.Secret, error) {
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

	return secret, nil
}
