package controller

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
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
	"github.com/giantswarm/observability-operator/internal/labels"
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
						labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
					},
					Annotations: map[string]string{
						labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
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
			var mockGrafanaClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				// Create a mock Grafana client
				mockGrafanaClient = &mocks.MockGrafanaClient{}

				// Configure mock for successful Grafana client generation
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
					Return(mockGrafanaClient, nil)

				// Setup mock client methods for successful operation
	
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

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)
			})

			AfterEach(func() {
				// Assert all expectations were met
				mockGrafanaClient.AssertExpectations(GinkgoT())
				mockGrafanaGen.AssertExpectations(GinkgoT())
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
			})
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
	
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)
			})

			AfterEach(func() {
				// Assert all expectations were met
				mockGrafanaClient.AssertExpectations(GinkgoT())
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})
			It("should successfully create a dashboard when organization exists", func() {
				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

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
				mockDashboardsClient.On("PostDashboard", mock.MatchedBy(func(cmd interface{}) bool {
					// Verify the dashboard command contains the expected data
					if saveCmd, ok := cmd.(*models.SaveDashboardCommand); ok {
						return saveCmd.Dashboard != nil && saveCmd.Overwrite == true
					}
					return false
				})).Return(dashboardResponse, nil)

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)

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
				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock the dashboard creation to fail
				mockDashboardsClient.On("PostDashboard", mock.Anything).Return(nil, errors.New("dashboard creation failed"))

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
				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to fail
				// Note: OrgID() and WithOrgID() are NOT called when organization lookup fails
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(nil, errors.New("organization not found"))

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
				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock getting the dashboard to verify it exists
				dashboardResponse := &dashboards.GetDashboardByUIDOK{
					Payload: &models.DashboardFullWithMeta{
						Dashboard: map[string]interface{}{
							"uid":   "test-dashboard-uid",
							"title": "Test Dashboard",
						},
					},
				}
				mockDashboardsClient.On("GetDashboardByUID", "test-dashboard-uid").Return(dashboardResponse, nil)

				// Mock the dashboard deletion to succeed
				deleteMessage := "Dashboard deleted"
				deleteTitle := "Test Dashboard"
				deleteResponse := &dashboards.DeleteDashboardByUIDOK{
					Payload: &models.DeleteDashboardByUIDOKBody{
						Message: &deleteMessage,
						Title:   &deleteTitle,
					},
				}
				mockDashboardsClient.On("DeleteDashboardByUID", "test-dashboard-uid").Return(deleteResponse, nil)

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)

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
				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock getting the dashboard to succeed
				dashboardResponse := &dashboards.GetDashboardByUIDOK{
					Payload: &models.DashboardFullWithMeta{
						Dashboard: map[string]interface{}{
							"uid":   "test-dashboard-uid",
							"title": "Test Dashboard",
						},
					},
				}
				mockDashboardsClient.On("GetDashboardByUID", "test-dashboard-uid").Return(dashboardResponse, nil)

				// Mock the dashboard deletion to fail
				mockDashboardsClient.On("DeleteDashboardByUID", "test-dashboard-uid").Return(nil, errors.New("dashboard deletion failed"))

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
			})

			AfterEach(func() {
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})

			It("should handle organization specified in labels instead of annotations", func() {
				// Set up mock expectations for organization operations
	
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "test-dashboard-org",
					},
				}
				mockOrgsClient.On("GetOrgByName", "test-dashboard-org").Return(orgResponse, nil)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock the dashboard creation to succeed
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockDashboardsClient.On("PostDashboard", mock.Anything).Return(dashboardResponse, nil)

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)

				// Test ConfigMap with organization in labels instead of annotations
				// Note: Using a valid Kubernetes label value (no spaces, alphanumeric + dashes/dots/underscores)
				configMapWithLabelOrg := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-with-label-org",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName:    labels.DashboardSelectorLabelValue,
							labels.GrafanaOrganizationAnnotation: "test-dashboard-org",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "test-dashboard-uid",
							"title": "Test Dashboard"
						}`,
					},
				}

				By("Creating a dashboard ConfigMap with organization in labels")
				Expect(k8sClient.Create(ctx, configMapWithLabelOrg)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-label-org",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process the dashboard successfully")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
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

			It("should fail when ConfigMap has no organization label or annotation (defensive validation)", func() {
				configMapWithoutOrg := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-without-org",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
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

				By("Second reconciliation - should fail due to defensive validation")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-org",
						Namespace: dashboardNamespace,
					},
				})
				// Should fail because defensive validation catches the missing organization
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
				Expect(err.Error()).To(ContainSubstring("organization is missing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithoutOrg)).To(Succeed())
			})

			It("should fail when dashboard has missing UID (defensive validation)", func() {
				// No mock expectations for organization operations since validation will fail early
				// The controller should skip Grafana calls when dashboard has no UID

				configMapWithoutUID := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-without-uid",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"title": "Test Dashboard Without UID"
						}`,
					},
				}

				By("Creating a dashboard ConfigMap without UID")
				Expect(k8sClient.Create(ctx, configMapWithoutUID)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-uid",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should fail due to defensive validation")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-without-uid",
						Namespace: dashboardNamespace,
					},
				})
				// Should fail because defensive validation catches the missing UID
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
				Expect(err.Error()).To(ContainSubstring("UID is missing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithoutUID)).To(Succeed())
			})

			It("should handle dashboard with existing ID that needs cleaning", func() {
				// Set up mock expectations for organization operations
	
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)

				configMapWithID := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-with-id",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
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
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock the dashboard creation to succeed
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockDashboardsClient.On("PostDashboard", mock.MatchedBy(func(cmd interface{}) bool {
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
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-with-id",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process and clean the dashboard ID")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
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

			It("should fail when ConfigMap has invalid JSON (defensive validation)", func() {
				// No mock expectations for organization operations since validation will fail early
				// The controller should skip Grafana calls when JSON is invalid

				configMapWithInvalidJSON := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dashboard-invalid-json",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "test-dashboard-uid"
							"title": "Invalid JSON - missing comma"
						}`,
					},
				}

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
				Expect(result).To(Equal(reconcile.Result{}))

				By("Second reconciliation - should fail due to defensive validation")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "dashboard-invalid-json",
						Namespace: dashboardNamespace,
					},
				})
				// Should fail because defensive validation catches the invalid JSON (nil content)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
				Expect(err.Error()).To(ContainSubstring("invalid JSON format"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, configMapWithInvalidJSON)).To(Succeed())
			})

			It("should handle ConfigMap with multiple dashboards", func() {
				// Set up mock expectations for organization operations
	
				mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

				// Mock the Orgs service
				mockOrgsClient := &mocks.MockOrgsClient{}
				mockGrafanaClient.On("Orgs").Return(mockOrgsClient)

				// Mock the Dashboards service
				mockDashboardsClient := &mocks.MockDashboardsClient{}
				mockGrafanaClient.On("Dashboards").Return(mockDashboardsClient)

				// Mock the Folders service for cleanup
				mockFoldersClient := &mocks.MockFoldersClient{}
				mockGrafanaClient.On("Folders").Return(mockFoldersClient)
				mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
					Payload: []*models.FolderSearchHit{},
				}, nil)

				// Mock the organization lookup to succeed
				orgResponse := &orgs.GetOrgByNameOK{
					Payload: &models.OrgDetailsDTO{
						ID:   int64(2),
						Name: "Test Dashboard Organization",
					},
				}
				mockOrgsClient.On("GetOrgByName", "Test Dashboard Organization").Return(orgResponse, nil)

				// Mock dashboard creation for all three dashboards
				dashboardResponse := &dashboards.PostDashboardOK{
					Payload: &models.PostDashboardOKBody{},
				}
				mockDashboardsClient.On("PostDashboard", mock.Anything).Return(dashboardResponse, nil).Times(3)

				configMapWithMultipleDashboards := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-dashboards",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
						},
						// Note: no organization annotation
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

				By("Creating a ConfigMap with multiple dashboards")
				Expect(k8sClient.Create(ctx, configMapWithMultipleDashboards)).To(Succeed())

				By("First reconciliation - should add finalizer only")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "multiple-dashboards",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Second reconciliation - should process all dashboards")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
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

		Context("When testing defensive validation at controller level", func() {
			var validationMockClient *mocks.MockGrafanaClient

			BeforeEach(func() {
				// Create a separate mock client for validation tests
				validationMockClient = &mocks.MockGrafanaClient{}
				// Configure the client generator to return our validation-specific mock
				mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(validationMockClient, nil)
			})

			AfterEach(func() {
				// Only assert expectations for the validation mock if it has any
				if len(validationMockClient.ExpectedCalls) > 0 {
					validationMockClient.AssertExpectations(GinkgoT())
				}
				mockGrafanaGen.AssertExpectations(GinkgoT())
			})

			It("should reject ConfigMap with invalid dashboard during reconciliation (defensive validation)", func() {
				By("Creating ConfigMap with invalid dashboard (missing UID)")
				invalidConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-dashboard",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						Annotations: map[string]string{
							labels.GrafanaOrganizationAnnotation: "Test Dashboard Organization",
						},
					},
					Data: map[string]string{
						"dashboard.json": `{
							"title": "Dashboard without UID",
							"panels": []
						}`,
					},
				}

				// Create the ConfigMap in the cluster
				Expect(k8sClient.Create(ctx, invalidConfigMap)).To(Succeed())

				By("First reconciliation should add finalizer")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "invalid-dashboard",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Second reconciliation should fail due to validation")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "invalid-dashboard",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
				Expect(err.Error()).To(ContainSubstring("UID is missing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, invalidConfigMap)).To(Succeed())
			})

			It("should reject ConfigMap with dashboard missing organization during reconciliation", func() {
				By("Creating ConfigMap with invalid dashboard (missing organization)")
				invalidConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-org-dashboard",
						Namespace: dashboardNamespace,
						Labels: map[string]string{
							labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
						},
						// No organization annotation
					},
					Data: map[string]string{
						"dashboard.json": `{
							"uid": "test-uid",
							"title": "Dashboard without organization",
							"panels": []
						}`,
					},
				}

				// Create the ConfigMap in the cluster
				Expect(k8sClient.Create(ctx, invalidConfigMap)).To(Succeed())

				By("First reconciliation should add finalizer")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "invalid-org-dashboard",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Second reconciliation should fail due to validation")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "invalid-org-dashboard",
						Namespace: dashboardNamespace,
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
				Expect(err.Error()).To(ContainSubstring("organization is missing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, invalidConfigMap)).To(Succeed())
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
			Expect(labels.DashboardSelectorLabelName).To(Equal("app.giantswarm.io/kind"))
			Expect(labels.DashboardSelectorLabelValue).To(Equal("dashboard"))
		})
	})
})
