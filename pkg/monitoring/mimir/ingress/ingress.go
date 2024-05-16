package ingress

import (
	"os/exec"

	"github.com/giantswarm/observability-operator/pkg/monitoring"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ms *MimirService) buildIngressSecret(username string, password string) (*corev1.Secret, error) {
	// Uses htpasswd to generate the password hash.
	secretData, err := exec.Command("htpasswd", "-bn", username, password).Output()
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			// The secret name is hard coded so that it's easier to use on other places.
			Name:      "mimir-gateway-ingress",
			Namespace: "mimir",
			Finalizers: []string{
				monitoring.MonitoringFinalizer,
			},
		},
		Data: map[string][]byte{
			"auth": secretData,
		},
		Type: "Opaque",
	}

	return secret, nil
}
