/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

var _ = Describe("Secret Webhook", func() {
	var (
		ctx       context.Context
		obj       *corev1.Secret
		oldObj    *corev1.Secret
		validator AlertmanagerConfigSecretValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		obj = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"observability.giantswarm.io/kind":   "alertmanager-config",
					"observability.giantswarm.io/tenant": "test_tenant",
				},
			},
			Data: map[string][]byte{
				alertmanager.AlertmanagerConfigKey: []byte(`global:
  smtp_smarthost: 'localhost:587'
route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'
receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/'`),
			},
		}
		oldObj = &corev1.Secret{}
		// Use the real client from the test environment
		validator = AlertmanagerConfigSecretValidator{client: k8sClient, tenantRepository: tenancy.NewKubernetesRepository(k8sClient)}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// Cleanup if needed
	})

	Context("When creating or updating Secret under Validating Webhook", func() {
		It("Should allow secrets that are not alertmanager config secrets", func() {
			By("Creating a secret without alertmanager-config label")
			nonAlertmanagerSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regular-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"app": "some-app",
					},
				},
				Data: map[string][]byte{
					"config": []byte("some config"),
				},
			}

			_, err := validator.ValidateCreate(ctx, nonAlertmanagerSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate alertmanager config secrets with proper labels", func() {
			By("Testing scope filtering")
			isAlertmanagerConfig := validator.isAlertmanagerConfigSecret(obj)
			Expect(isAlertmanagerConfig).To(BeTrue())

			By("Testing secret without proper labels")
			secretWithoutLabels := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-without-labels",
					Namespace: "test-namespace",
				},
			}
			isAlertmanagerConfig = validator.isAlertmanagerConfigSecret(secretWithoutLabels)
			Expect(isAlertmanagerConfig).To(BeFalse())

			By("Testing secret with only kind label but no tenant label")
			secretWithoutTenant := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-without-tenant",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"observability.giantswarm.io/kind": "alertmanager-config",
					},
				},
			}
			isAlertmanagerConfig = validator.isAlertmanagerConfigSecret(secretWithoutTenant)
			Expect(isAlertmanagerConfig).To(BeFalse())
		})

		It("Should validate against existing GrafanaOrganizations", func() {
			By("Creating a GrafanaOrganization with the required tenant")
			grafanaOrg := &observabilityv1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grafana-org",
				},
				Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants:     []observabilityv1alpha1.TenantID{"test_tenant"},
					RBAC: &observabilityv1alpha1.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			// Clean up after test
			defer func() {
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}()

			By("Validating that the secret now passes tenant validation")
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate that tenant exists in GrafanaOrganizations", func() {
			By("Testing with a tenant that doesn't exist")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in the list of accepted tenants"))

			By("Testing invalid alertmanager configuration with non-existent tenant")
			invalidSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"observability.giantswarm.io/kind":   "alertmanager-config",
						"observability.giantswarm.io/tenant": "test_tenant",
					},
				},
				Data: map[string][]byte{
					alertmanager.AlertmanagerConfigKey: []byte(`invalid yaml: [`),
				},
			}

			// This should fail because the tenant doesn't exist, not because of invalid config
			_, err = validator.ValidateCreate(ctx, invalidSecret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in the list of accepted tenants"))
		})
	})
})
