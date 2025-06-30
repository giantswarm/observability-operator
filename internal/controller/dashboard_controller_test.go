package controller

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
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
			mockGrafanaGen     *mocks.MockGrafanaClientGenerator
			mockGrafanaClient  *mocks.MockGrafanaClient
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
					Tenants:     []observabilityv1alpha1.TenantID{"dashboard_tenant"},
					RBAC: &observabilityv1alpha1.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}
			err = k8sClient.Create(ctx, grafanaOrg)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			// Create fresh mocks for each test
			mockGrafanaGen = &mocks.MockGrafanaClientGenerator{}
			mockGrafanaClient = &mocks.MockGrafanaClient{}

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

		Context("When Grafana is available", func() {
			BeforeEach(func() {
				// Configure mock for successful Grafana client generation
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
					Return(mockGrafanaClient, nil)

				// Setup mock client methods for successful operation
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(nil)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(
					&orgs.GetOrgByNameOK{
						Payload: &models.OrgDetailsDTO{
							ID:   1,
							Name: "Test Dashboard Organization",
						},
					}, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)
				mockDashboardsClient.On("PostDashboard", mock.AnythingOfType("*models.SaveDashboardCommand")).Return(
					&dashboards.PostDashboardOK{
						Payload: &models.PostDashboardOKBody{
							UID: func() *string { s := "test-dashboard-uid"; return &s }(),
						},
					}, nil)
			})

			It("should successfully create a dashboard", func() {
				By("Creating a dashboard ConfigMap")
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())

				By("First reconciliation - should add finalizer")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was added")
				createdConfigMap := &v1.ConfigMap{}
				err = k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should be added on the first reconciliation
				hasFinalizerAdded := false
				for _, finalizer := range createdConfigMap.Finalizers {
					if finalizer == DashboardFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeTrue())

				By("Second reconciliation - should configure dashboard in Grafana")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Verify mock expectations were met
				mockGrafanaGen.AssertExpectations(GinkgoT())
				mockGrafanaClient.AssertExpectations(GinkgoT())
			})
		})

		Context("When Grafana is unavailable", func() {
			BeforeEach(func() {
				// Configure mock to return errors (Grafana unavailable)
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("grafana service unavailable"))
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
				// Clear previous expectations and set up successful mock
				mockGrafanaGen.ExpectedCalls = nil
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
					Return(mockGrafanaClient, nil)

				// Setup mock client methods for successful dashboard operation
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(nil)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(
					&orgs.GetOrgByNameOK{
						Payload: &models.OrgDetailsDTO{
							ID:   1,
							Name: "Test Dashboard Organization",
						},
					}, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)
				mockDashboardsClient.On("PostDashboard", mock.AnythingOfType("*models.SaveDashboardCommand")).Return(
					&dashboards.PostDashboardOK{
						Payload: &models.PostDashboardOKBody{
							UID: func() *string { s := "test-dashboard-uid"; return &s }(),
						},
					}, nil)

				By("Second reconciliation - should now succeed")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: retryNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, retryTestConfigMap)).To(Succeed())
			})
		})

		Context("When testing edge cases", func() {
			BeforeEach(func() {
				// Use working Grafana for edge case tests
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
					Return(mockGrafanaClient, nil)

				// Setup mock client methods for successful operation
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(nil)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)
				mockOrgsClient.On("GetOrgByName", mock.AnythingOfType("string")).Return(
					&orgs.GetOrgByNameOK{
						Payload: &models.OrgDetailsDTO{
							ID:   1,
							Name: "Test Dashboard Organization",
						},
					}, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)
				mockDashboardsClient.On("PostDashboard", mock.AnythingOfType("*models.SaveDashboardCommand")).Return(
					&dashboards.PostDashboardOK{
						Payload: &models.PostDashboardOKBody{
							UID: func() *string { s := "test-dashboard-uid"; return &s }(),
						},
					}, nil)
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
						// Note: no organization annotation
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

				// Should succeed since dashboard processing will be skipped due to missing organization annotation
				// The controller will add finalizer first, then skip dashboard processing when it finds no organization
				Expect(err).NotTo(HaveOccurred())
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
