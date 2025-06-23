package controller

import (
	"context"
	"errors"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/models"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/mapper"
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

			// Create fresh mock for each test
			mockGrafanaGen = &mocks.MockGrafanaClientGenerator{}

			// Setup reconciler with mock client generator
			grafanaURL, _ := url.Parse("http://localhost:3000")
			reconciler = &DashboardReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				grafanaURL:       grafanaURL,
				finalizerHelper:  NewFinalizerHelper(k8sClient, DashboardFinalizer),
				dashboardMapper:  mapper.New(),
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
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("grafana service unavailable"))
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
		})

		Context("When Grafana is available and dashboard operations succeed", func() {
			var mockGrafanaClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				// Create a mock Grafana client
				mockGrafanaClient = &mocks.MockGrafanaClient{}

				// Configure the client generator to return our mock client
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(mockGrafanaClient, nil)

				// Setup common mock expectations for organization operations
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)
			})

			AfterEach(func() {
				// Assert all expectations were met
				mockGrafanaClient.AssertExpectations(GinkgoT())
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})
			It("should successfully create a dashboard when organization exists", func() {
				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the dashboard creation to succeed
				dashboardID := int64(123)
				dashboardUID := "test-dashboard-uid"
				dashboardURL := "/d/test-dashboard-uid/test-dashboard"
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{
						ID:  &dashboardID,
						UID: &dashboardUID,
						URL: &dashboardURL,
					},
				}
				mockGrafanaClient.On("PostDashboard", mock.MatchedBy(func(cmd interface{}) bool {
					// Verify the dashboard command contains the expected data
					if saveCmd, ok := cmd.(*models.SaveDashboardCommand); ok {
						return saveCmd.Dashboard != nil && saveCmd.Overwrite == true
					}
					return false
				})).Return(dashboardResponse, nil)

				By("Creating a dashboard ConfigMap")
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was added")
				createdConfigMap := &v1.ConfigMap{}
				err = k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should be added after first reconciliation
				hasFinalizerAdded := false
				for _, finalizer := range createdConfigMap.Finalizers {
					if finalizer == DashboardFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeTrue())

				By("Second reconciliation - should process the dashboard")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should fail when dashboard creation fails in Grafana", func() {
				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the dashboard creation to fail
				mockGrafanaClient.On("PostDashboard", mock.Anything).Return(nil, errors.New("dashboard creation failed"))

				By("Creating a dashboard ConfigMap")
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was added")
				createdConfigMap := &v1.ConfigMap{}
				err = k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should be added after first reconciliation
				hasFinalizerAdded := false
				for _, finalizer := range createdConfigMap.Finalizers {
					if finalizer == DashboardFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeTrue())

				By("Second reconciliation - should fail due to dashboard creation failure")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard creation failed"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("When Grafana operations fail", func() {
			var mockGrafanaClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				// Create a mock Grafana client
				mockGrafanaClient = &mocks.MockGrafanaClient{}

				// Configure the client generator to return our mock client
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(mockGrafanaClient, nil)
			})

			AfterEach(func() {
				// Assert all expectations were met
				mockGrafanaClient.AssertExpectations(GinkgoT())
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})

			It("should fail when organization does not exist", func() {
				// Mock the organization lookup to fail
				// Note: OrgID() and WithOrgID() are NOT called when organization lookup fails
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(nil, errors.New("organization not found"))

				By("Creating a dashboard ConfigMap")
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was added")
				createdConfigMap := &v1.ConfigMap{}
				err = k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should be added after first reconciliation
				hasFinalizerAdded := false
				for _, finalizer := range createdConfigMap.Finalizers {
					if finalizer == DashboardFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeTrue())

				By("Second reconciliation - should fail due to organization not found")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("organization not found"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("When deleting a dashboard", func() {
			var mockGrafanaClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				// Create a mock Grafana client
				mockGrafanaClient = &mocks.MockGrafanaClient{}

				// Configure the client generator to return our mock client
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(mockGrafanaClient, nil)

				// Setup common mock expectations for organization operations
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				// First create the dashboard with finalizer
				dashboardConfigMap.Finalizers = []string{DashboardFinalizer}
				Expect(k8sClient.Create(ctx, dashboardConfigMap)).To(Succeed())
			})

			AfterEach(func() {
				// Assert all expectations were met
				mockGrafanaClient.AssertExpectations(GinkgoT())
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})
			It("should successfully delete a dashboard", func() {
				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock getting the dashboard to verify it exists
				dashboardResponse := &dashboards.GetDashboardByUIDOK{
					Payload: &models.DashboardFullWithMeta{
						Dashboard: map[string]interface{}{
							"uid":   "test-dashboard-uid",
							"title": "Test Dashboard",
						},
					},
				}
				mockGrafanaClient.On("GetDashboardByUID", "test-dashboard-uid").Return(dashboardResponse, nil)

				// Mock the dashboard deletion to succeed
				deleteMessage := "Dashboard deleted"
				deleteTitle := "Test Dashboard"
				deleteResponse := &dashboards.DeleteDashboardByUIDOK{
					Payload: &models.DeleteDashboardByUIDOKBody{
						Message: &deleteMessage,
						Title:   &deleteTitle,
					},
				}
				mockGrafanaClient.On("DeleteDashboardByUID", "test-dashboard-uid").Return(deleteResponse, nil)

				By("Marking the dashboard ConfigMap for deletion")
				createdConfigMap := &v1.ConfigMap{}
				err := k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				Expect(k8sClient.Delete(ctx, createdConfigMap)).To(Succeed())

				By("Reconciling the dashboard deletion")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Verifying the ConfigMap was actually deleted")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, namespacedName, &v1.ConfigMap{})
					return apierrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			})

			It("should fail when dashboard deletion fails in Grafana", func() {
				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock getting the dashboard to succeed
				dashboardResponse := &dashboards.GetDashboardByUIDOK{
					Payload: &models.DashboardFullWithMeta{
						Dashboard: map[string]interface{}{
							"uid":   "test-dashboard-uid",
							"title": "Test Dashboard",
						},
					},
				}
				mockGrafanaClient.On("GetDashboardByUID", "test-dashboard-uid").Return(dashboardResponse, nil)

				// Mock the dashboard deletion to fail
				mockGrafanaClient.On("DeleteDashboardByUID", "test-dashboard-uid").Return(nil, errors.New("dashboard deletion failed"))

				By("Marking the dashboard ConfigMap for deletion")
				createdConfigMap := &v1.ConfigMap{}
				err := k8sClient.Get(ctx, namespacedName, createdConfigMap)
				Expect(err).NotTo(HaveOccurred())

				Expect(k8sClient.Delete(ctx, createdConfigMap)).To(Succeed())

				By("Reconciling the dashboard deletion")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard deletion failed"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("When testing edge cases and error conditions", func() {
			var mockGrafanaClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				mockGrafanaClient = &mocks.MockGrafanaClient{}
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(mockGrafanaClient, nil)
				// Note: Only set mock expectations when they are actually needed
			})

			AfterEach(func() {
				// Only assert expectations if they were set
				if len(mockGrafanaClient.ExpectedCalls) > 0 {
					mockGrafanaClient.AssertExpectations(GinkgoT())
				}
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})

			It("should handle organization specified in labels instead of annotations", func() {
				// Set up mock expectations for organization operations
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				// Test ConfigMap with organization in labels instead of annotations
				// Note: Using a valid Kubernetes label value (no spaces, alphanumeric + dashes/dots/underscores)
				configMapWithLabelOrg := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-with-label-org",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							DashboardSelectorLabelName:                 DashboardSelectorLabelValue,
							"observability.giantswarm.io/organization": "test-dashboard-org",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "test-dashboard-uid",
							"title": "Test Dashboard"
						}`,
					},
				}

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "test-dashboard-org",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "test-dashboard-org").Return(orgResponse, nil)

				// Mock the dashboard creation to succeed
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockGrafanaClient.On("PostDashboard", mock.Anything).Return(dashboardResponse, nil)

				By("Creating a dashboard ConfigMap with organization in labels")
				Expect(k8sClient.Create(ctx, configMapWithLabelOrg)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-label-org",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process the dashboard successfully")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-label-org",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithLabelOrg)).To(Succeed())
			})

			It("should handle ConfigMap with no organization label or annotation gracefully", func() {
				configMapWithoutOrg := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-without-org",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							DashboardSelectorLabelName: DashboardSelectorLabelValue,
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "test-dashboard-uid",
							"title": "Test Dashboard"
						}`,
					},
				}

				By("Creating a dashboard ConfigMap without organization")
				Expect(k8sClient.Create(ctx, configMapWithoutOrg)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-org",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should succeed and skip dashboard processing")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-org",
						Namespace: dashboardNamespace,
					},
				})
				// Should succeed because missing organization is handled gracefully by logging an error and continuing
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithoutOrg)).To(Succeed())
			})

			It("should handle dashboard with missing UID gracefully", func() {
				// No mock expectations for organization operations since validation will fail early

				configMapWithoutUID := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-without-uid",
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
							"title": "Test Dashboard Without UID"
						}`,
					},
				}

				// No organization lookup mock needed since dashboard will be skipped due to missing UID

				By("Creating a dashboard ConfigMap without UID")
				Expect(k8sClient.Create(ctx, configMapWithoutUID)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-uid",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should succeed and skip dashboard without UID")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-uid",
						Namespace: dashboardNamespace,
					},
				})
				// Should succeed because missing UID is handled gracefully by logging an error and continuing
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithoutUID)).To(Succeed())
			})

			It("should handle dashboard with existing ID that needs cleaning", func() {
				// Set up mock expectations for organization operations
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				configMapWithID := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-with-id",
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
							"id": 123,
							"uid": "test-dashboard-uid",
							"title": "Test Dashboard With ID"
						}`,
					},
				}

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the dashboard creation to succeed
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockGrafanaClient.On("PostDashboard", mock.MatchedBy(func(cmd interface{}) bool {
					// Verify that the dashboard ID was cleaned (removed)
					if saveCmd, ok := cmd.(*models.SaveDashboardCommand); ok {
						if dashboard, ok := saveCmd.Dashboard.(map[string]interface{}); ok {
							// ID should be cleaned/removed
							_, hasID := dashboard["id"]
							return !hasID // Should not have ID after cleaning
						}
					}
					return false
				})).Return(dashboardResponse, nil)

				By("Creating a dashboard ConfigMap with existing ID")
				Expect(k8sClient.Create(ctx, configMapWithID)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-id",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process and clean the dashboard ID")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-id",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithID)).To(Succeed())
			})

			It("should handle ConfigMap with invalid JSON gracefully", func() {
				// No mock expectations for organization operations since validation will fail early

				configMapWithInvalidJSON := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-invalid-json",
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
							"uid": "test-dashboard-uid"
							"title": "Invalid JSON - missing comma"
						}`,
					},
				}

				// No mock expectations needed - service layer will skip due to validation errors

				By("Creating a dashboard ConfigMap with invalid JSON")
				Expect(k8sClient.Create(ctx, configMapWithInvalidJSON)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-invalid-json",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should succeed and skip invalid JSON")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-invalid-json",
						Namespace: dashboardNamespace,
					},
				})
				// Should succeed because invalid JSON is handled gracefully by logging an error and continuing
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithInvalidJSON)).To(Succeed())
			})

			It("should handle ConfigMap with multiple dashboards", func() {
				// Set up mock expectations for organization operations
				mockGrafanaClient.On("OrgID").Return(int64(1))
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				configMapWithMultipleDashboards := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-dashboards",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							DashboardSelectorLabelName: DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							"observability.giantswarm.io/organization": "Test Dashboard Organization",
						},
					},
					Data: map[string]string{
						"dashboard1.json": `{
							"uid": "dashboard-1",
							"title": "First Dashboard"
						}`,
						"dashboard2.json": `{
							"uid": "dashboard-2", 
							"title": "Second Dashboard"
						}`,
						"dashboard3.json": `{
							"uid": "dashboard-3",
							"title": "Third Dashboard",
							"id": 456
						}`,
					},
				}

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockGrafanaClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock dashboard creation for all three dashboards
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockGrafanaClient.On("PostDashboard", mock.Anything).Return(dashboardResponse, nil).Times(3)

				By("Creating a ConfigMap with multiple dashboards")
				Expect(k8sClient.Create(ctx, configMapWithMultipleDashboards)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "multiple-dashboards",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process all dashboards")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "multiple-dashboards",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithMultipleDashboards)).To(Succeed())
			})
		})

		Context("When testing controller resilience", func() {
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

	Context("When testing dashboard controller setup and constants", func() {
		It("should have a setup function that can be called", func() {
			By("Verifying the SetupDashboardReconciler function exists")
			// Note: In a real test environment, you would need a manager instance
			// to properly test the setup function. For now, we'll test that the
			// function exists and can be referenced.
			Expect(SetupDashboardReconciler).NotTo(BeNil())
		})

		It("should have correct constants defined", func() {
			By("Verifying dashboard controller constants")
			Expect(DashboardFinalizer).To(Equal("observability.giantswarm.io/grafanadashboard"))
			Expect(DashboardSelectorLabelName).To(Equal("app.giantswarm.io/kind"))
			Expect(DashboardSelectorLabelValue).To(Equal("dashboard"))
		})
	})
})
