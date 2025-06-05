package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Alertmanager Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			SecretName      = "test-alertmanager-config"
			SecretNamespace = "default"
			timeout         = time.Second * 10
			interval        = time.Millisecond * 250
		)

		ctx := context.Background()

		It("should successfully reconcile an Alertmanager config Secret", func() {
			By("Creating a new Secret with Alertmanager configuration")
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SecretName,
					Namespace: SecretNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "alertmanager",
						"app.kubernetes.io/component": "config",
					},
				},
				Data: map[string][]byte{
					"alertmanager.yml": []byte(`
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alertmanager@example.org'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/'
`),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			By("Checking that the Secret was created")
			secretLookupKey := types.NamespacedName{Name: SecretName, Namespace: SecretNamespace}
			createdSecret := &v1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSecret.Data).Should(HaveKey("alertmanager.yml"))
			Expect(string(createdSecret.Data["alertmanager.yml"])).Should(ContainSubstring("receivers:"))

			By("Cleaning up the Secret")
			Eventually(func() error {
				return k8sClient.Delete(ctx, secret)
			}, timeout, interval).Should(Succeed())
		})
	})
})
