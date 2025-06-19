/*
Copyright 2025.

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
)

var _ = Describe("Dashboard ConfigMap Webhook", func() {
	var (
		ctx       context.Context
		validator *DashboardConfigMapValidator
		obj       *corev1.ConfigMap
		oldObj    *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctx = context.Background()
		validator = &DashboardConfigMapValidator{client: k8sClient}

		// Create a basic dashboard ConfigMap for testing
		obj = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-dashboard",
				Namespace: "default",
				Labels: map[string]string{
					"app.giantswarm.io/kind": "dashboard",
				},
				Annotations: map[string]string{
					"observability.giantswarm.io/organization": "test-org",
				},
			},
			Data: map[string]string{
				"dashboard.json": `{
					"uid": "test-dashboard",
					"title": "Test Dashboard",
					"panels": []
				}`,
			},
		}

		// Create oldObj for update tests
		oldObj = obj.DeepCopy()
		oldObj.Data["dashboard.json"] = `{
			"uid": "test-dashboard",
			"title": "Old Test Dashboard",
			"panels": []
		}`
	})

	Context("When validating dashboard ConfigMaps", func() {
		It("Should allow dashboard ConfigMaps with proper labels", func() {
			By("Testing scope filtering")
			isDashboard := validator.isDashboardConfigMap(obj)
			Expect(isDashboard).To(BeTrue())

			By("Testing ConfigMap without proper labels")
			configMapWithoutLabels := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-without-labels",
					Namespace: "default",
				},
			}
			isDashboard = validator.isDashboardConfigMap(configMapWithoutLabels)
			Expect(isDashboard).To(BeFalse())

			By("Testing ConfigMap with wrong kind label")
			configMapWithWrongKind := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-wrong-kind",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "not-dashboard",
					},
				},
			}
			isDashboard = validator.isDashboardConfigMap(configMapWithWrongKind)
			Expect(isDashboard).To(BeFalse())
		})

		It("Should allow non-dashboard ConfigMaps to pass through without validation", func() {
			By("Creating a ConfigMap without dashboard labels")
			nonDashboardConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regular-configmap",
					Namespace: "default",
					Labels: map[string]string{
						"app": "some-app",
					},
				},
				Data: map[string]string{
					"config": "some config",
				},
			}

			_, err := validator.ValidateCreate(ctx, nonDashboardConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate dashboard ConfigMaps on create", func() {
			By("Testing dashboard ConfigMap validation on create")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate dashboard ConfigMaps on update", func() {
			By("Testing dashboard ConfigMap validation on update")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow deletion without validation", func() {
			By("Testing delete operation")
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate object type correctly", func() {
			By("Testing with wrong object type")
			wrongObj := &corev1.Secret{}
			_, err := validator.ValidateCreate(ctx, wrongObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a ConfigMap object but got"))
		})
	})

	Context("When creating or updating dashboard ConfigMaps under Validating Webhook", func() {
		// TODO: Add specific validation tests here when business logic is implemented
		It("Should successfully validate basic dashboard structure", func() {
			By("Testing basic dashboard validation")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
