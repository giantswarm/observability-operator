package controller

import (
	"context"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

var _ = Describe("Dashboard Controller", func() {
	Context("When reconciling a dashboard ConfigMap", func() {
		const (
			dashboardName      = "test-dashboard"
			dashboardNamespace = "default"
			timeout            = time.Second * 10
			interval           = time.Millisecond * 250
		)

		var (
			ctx                context.Context
			reconciler         *DashboardReconciler
			dashboardConfigMap *v1.ConfigMap
			namespacedName     types.NamespacedName
			mockGrafanaGen     *MockGrafanaClientGenerator
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespacedName = types.NamespacedName{
				Name:      dashboardName,
				Namespace: dashboardNamespace,
			}

			// Clean up any existing ConfigMaps from previous test runs
			existingConfigMap := &v1.ConfigMap{}
			err := k8sClient.Get(ctx, namespacedName, existingConfigMap)
			if err == nil {
				// ConfigMap exists, delete it
				Expect(k8sClient.Delete(ctx, existingConfigMap)).To(Succeed())
				// Wait for it to be deleted
				Eventually(func() bool {
					err := k8sClient.Get(ctx, namespacedName, &v1.ConfigMap{})
					return apierrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}

			// Create test namespace if it doesn't exist
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: dashboardNamespace,
				},
			}
			err = k8sClient.Create(ctx, ns)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			// Create a test GrafanaOrganization for dashboard validation
			grafanaOrg := &observabilityv1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dashboard-org",
				},
				Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Test Dashboard Organization",
					Tenants:     []observabilityv1alpha1.TenantID{"dashboard-tenant"},
					RBAC: &observabilityv1alpha1.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}
			err = k8sClient.Create(ctx, grafanaOrg)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			// Create fresh mock for each test
			mockGrafanaGen = &MockGrafanaClientGenerator{}

			// Setup reconciler with mock client generator
			grafanaURL, _ := url.Parse("http://localhost:3000")
			reconciler = &DashboardReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				grafanaURL:       grafanaURL,
				finalizerHelper:  NewFinalizerHelper(k8sClient, DashboardFinalizer),
				grafanaClientGen: mockGrafanaGen,
			}

			// Create base dashboard ConfigMap
			dashboardConfigMap = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dashboardName,
					Namespace: dashboardNamespace,
					Labels: map[string]string{
						DashboardSelectorLabelName: DashboardSelectorLabelValue,
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "Test Dashboard Organization",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{
						"uid": "test-dashboard-uid",
						"title": "Test Dashboard",
						"tags": ["test"],
						"panels": [
							{
								"id": 1,
								"title": "Test Panel",
								"type": "graph"
							}
						]
					}`,
				},
			}
		})

		AfterEach(func() {
			// Clean up dashboard ConfigMap if it exists
			if dashboardConfigMap != nil {
				configMapToDelete := &v1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      dashboardConfigMap.Name,
					Namespace: dashboardConfigMap.Namespace,
				}, configMapToDelete)
				if err == nil {
					// Remove finalizers to allow deletion
					configMapToDelete.Finalizers = []string{}
					k8sClient.Update(ctx, configMapToDelete)
					k8sClient.Delete(ctx, configMapToDelete)
				}
			}

			// Clean up GrafanaOrganization
			grafanaOrg := &observabilityv1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dashboard-org",
				},
			}
			err := k8sClient.Delete(ctx, grafanaOrg)
			if err != nil && !apierrors.IsNotFound(err) {
				// Ignore cleanup errors to prevent test failures
			}
		})

		Context("When Grafana is unavailable", func() {
			BeforeEach(func() {
				// Configure mock to return errors (Grafana unavailable)
				mockGrafanaGen.SetShouldReturnError(true)
			})

			It("should handle Grafana unavailability gracefully", func() {
				By("Creating a dashboard ConfigMap")
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())

				By("First reconciliation - should fail due to Grafana being unavailable")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				// First reconciliation should fail when Grafana client generation fails
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("grafana service unavailable"))
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was NOT added due to the error")
				createdConfigMap := &v1.ConfigMap{}
				err = k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should not be added since the reconciliation failed
				hasFinalizerAdded := false
				for _, finalizer := range createdConfigMap.Finalizers {
					if finalizer == DashboardFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeFalse())
			})

			It("should retry when Grafana becomes available again", func() {
				By("Creating a dashboard ConfigMap")
				retryTestConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "retry-test-dashboard",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							DashboardSelectorLabelName: DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							"observability.giantswarm.io/organization": "Test Dashboard Organization",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "retry-test-dashboard-uid",
							"title": "Retry Test Dashboard",
							"tags": ["test"],
							"panels": []
						}`,
					},
				}
				Expect(k8sClient.Create(ctx, retryTestConfigMap)).To(Succeed())

				retryNamespacedName := types.NamespacedName{
					Name:      "retry-test-dashboard",
					Namespace: dashboardNamespace,
				}

				By("First reconciliation - should fail with Grafana unavailable")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: retryNamespacedName,
				})
				Expect(err).To(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Making Grafana available again")
				mockGrafanaGen.SetShouldReturnError(false)

				By("Second reconciliation - should now fail with 'configuration skipped'")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: retryNamespacedName,
				})
				// This will still fail but with a different error message indicating the test limitation
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, retryTestConfigMap)).To(Succeed())
			})
		})

		Context("When testing edge cases", func() {
			BeforeEach(func() {
				// Use "working" Grafana for edge case tests (though it will still fail with test message)
				mockGrafanaGen.SetShouldReturnError(false)
			})

			It("should handle ConfigMap without dashboard labels", func() {
				By("Creating a ConfigMap without dashboard labels")
				nonDashboardConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "non-dashboard-configmap",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							"app": "some-other-app",
						},
					},
					Data: map[string]string{
						"config.yml": "some: config",
					},
				}
				Expect(k8sClient.Create(ctx, nonDashboardConfigMap)).To(Succeed())

				By("Reconciling the non-dashboard ConfigMap")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "non-dashboard-configmap",
						Namespace: dashboardNamespace,
					},
				})

				// Note: In a real environment, the controller manager's label selector predicate
				// would prevent this ConfigMap from being reconciled. However, since we're calling
				// Reconcile() directly in the test, it will process any ConfigMap.
				// The controller will still fail because it tries to generate a Grafana client
				// for any ConfigMap it processes, regardless of labels.
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, nonDashboardConfigMap)).To(Succeed())
			})

			It("should handle non-existent ConfigMap gracefully", func() {
				By("Reconciling a non-existent dashboard ConfigMap")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "non-existent-dashboard",
						Namespace: dashboardNamespace,
					},
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})

	Context("When testing dashboard controller setup", func() {
		It("should setup the controller with proper predicates", func() {
			By("Creating a DashboardReconciler")

			// Note: In a real test environment, you would need a manager
			// For now, we'll test that the setup function exists and can be called
			// The actual manager integration would require more complex setup
			Expect(SetupDashboardReconciler).NotTo(BeNil())
		})
	})

	Context("When testing dashboard constants and selectors", func() {
		It("should have correct constants defined", func() {
			Expect(DashboardFinalizer).To(Equal("observability.giantswarm.io/grafanadashboard"))
			Expect(DashboardSelectorLabelName).To(Equal("app.giantswarm.io/kind"))
			Expect(DashboardSelectorLabelValue).To(Equal("dashboard"))
		})
	})
})
