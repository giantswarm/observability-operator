package controller

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
	"github.com/grafana/grafana-openapi-client-go/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/pkg/config"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

var _ = Describe("Grafana Organization Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			GrafanaOrgName = "test-grafana-org"
			timeout        = time.Second * 10
			interval       = time.Millisecond * 250
		)

		ctx := context.Background()

		It("should successfully reconcile a GrafanaOrganization resource", func() {
			By("Creating a new GrafanaOrganization")
			grafanaOrg := &observabilityv1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: GrafanaOrgName,
				},
				Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants:     []observabilityv1alpha1.TenantID{"testtenant"},
					RBAC: &observabilityv1alpha1.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grafanaOrg)).Should(Succeed())

			By("Checking that the GrafanaOrganization was created")
			grafanaOrgLookupKey := types.NamespacedName{Name: GrafanaOrgName}
			createdGrafanaOrg := &observabilityv1alpha1.GrafanaOrganization{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, grafanaOrgLookupKey, createdGrafanaOrg)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdGrafanaOrg.Spec.DisplayName).Should(Equal("Test Organization"))
			Expect(createdGrafanaOrg.Spec.Tenants).Should(ContainElement(observabilityv1alpha1.TenantID("testtenant")))
			Expect(createdGrafanaOrg.Spec.RBAC.Admins).Should(ContainElement("admin-org"))

			By("Cleaning up the GrafanaOrganization")
			Eventually(func() error {
				return k8sClient.Delete(ctx, grafanaOrg)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reconciling with the Grafana SSO org mapping toggle", func() {
		const (
			ssoProvider    = "generic_oauth"
			grafanaOrgID   = int64(42)
			datasourceID   = int64(7)
			ssoAdminsGroup = "sso-admins"
		)

		var (
			ctx                  context.Context
			reconciler           *GrafanaOrganizationReconciler
			mockGrafanaGen       *mocks.MockGrafanaClientGenerator
			mockGrafanaClient    *mocks.MockGrafanaClient
			mockOrgsClient       *mocks.MockOrgsClient
			mockDatasources      *mocks.MockDatasourcesClient
			grafanaOrg           *v1alpha2.GrafanaOrganization
			request              reconcile.Request
			orgName              string
			orgDisplayName       string
			newReconcilerWithSSO func(ssoOrgMappingEnabled bool) *GrafanaOrganizationReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()

			mockGrafanaGen = &mocks.MockGrafanaClientGenerator{}
			mockGrafanaClient = &mocks.MockGrafanaClient{}
			mockOrgsClient = &mocks.MockOrgsClient{}
			mockDatasources = &mocks.MockDatasourcesClient{}

			mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).
				Return(mockGrafanaClient, nil)
			mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)
			mockGrafanaClient.On("Orgs").Return(mockOrgsClient)
			mockGrafanaClient.On("Datasources").Return(mockDatasources)

			newReconcilerWithSSO = func(ssoOrgMappingEnabled bool) *GrafanaOrganizationReconciler {
				grafanaURL, _ := url.Parse("http://localhost:3000")
				return &GrafanaOrganizationReconciler{
					Client:           k8sClient,
					Scheme:           k8sClient.Scheme(),
					grafanaURL:       grafanaURL,
					finalizerHelper:  NewFinalizerHelper(k8sClient, v1alpha2.GrafanaOrganizationFinalizer),
					grafanaClientGen: mockGrafanaGen,
					cfg: config.Config{
						Grafana: config.GrafanaConfig{
							URL:                  grafanaURL,
							SSOOrgMappingEnabled: ssoOrgMappingEnabled,
						},
					},
					organizationMapper: mapper.NewOrganizationMapper(),
				}
			}
		})

		// createAndReconcile creates the GrafanaOrganization and reconciles it until the
		// organization and its datasources are configured.
		createAndReconcile := func() {
			By("Creating a new GrafanaOrganization")
			grafanaOrg = &v1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: orgName,
				},
				Spec: v1alpha2.GrafanaOrganizationSpec{
					DisplayName: orgDisplayName,
					Tenants:     []v1alpha2.TenantConfig{{Name: "ssotenant"}},
					RBAC: &v1alpha2.RBAC{
						Admins: []string{ssoAdminsGroup},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grafanaOrg)).To(Succeed())
			request = reconcile.Request{NamespacedName: types.NamespacedName{Name: orgName}}

			mockOrgsClient.On("GetOrgByName", orgDisplayName).Return(&orgs.GetOrgByNameOK{
				Payload: &models.OrgDetailsDTO{ID: grafanaOrgID, Name: orgDisplayName},
			}, nil)
			mockDatasources.On("GetDataSources").Return(&datasources.GetDataSourcesOK{
				Payload: models.DataSourceList{},
			}, nil)
			createdDatasourceID := datasourceID
			mockDatasources.On("AddDataSource", mock.AnythingOfType("*models.AddDataSourceCommand")).Return(&datasources.AddDataSourceOK{
				Payload: &models.AddDataSourceOKBody{ID: &createdDatasourceID},
			}, nil)

			By("Reconciling to add the finalizer")
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling to configure the organization")
			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the organization and its datasources were configured")
			updated := &v1alpha2.GrafanaOrganization{}
			Expect(k8sClient.Get(ctx, request.NamespacedName, updated)).To(Succeed())
			Expect(updated.Finalizers).To(ContainElement(v1alpha2.GrafanaOrganizationFinalizer))
			Expect(updated.Status.OrgID).To(Equal(grafanaOrgID))
			Expect(updated.Status.DisplayName).To(Equal(orgDisplayName))
			Expect(updated.Status.DataSources).NotTo(BeEmpty())
			mockDatasources.AssertCalled(GinkgoT(), "AddDataSource", mock.AnythingOfType("*models.AddDataSourceCommand"))
		}

		// deleteAndReconcile deletes the GrafanaOrganization and reconciles it until the
		// Grafana organization is removed and the finalizer is released.
		deleteAndReconcile := func() {
			mockOrgsClient.On("GetOrgByID", grafanaOrgID).Return(&orgs.GetOrgByIDOK{
				Payload: &models.OrgDetailsDTO{ID: grafanaOrgID, Name: orgDisplayName},
			}, nil)
			mockOrgsClient.On("DeleteOrgByID", grafanaOrgID).Return(&orgs.DeleteOrgByIDOK{}, nil)

			By("Deleting the GrafanaOrganization")
			Expect(k8sClient.Delete(ctx, grafanaOrg)).To(Succeed())

			By("Reconciling the deletion")
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			mockOrgsClient.AssertCalled(GinkgoT(), "DeleteOrgByID", grafanaOrgID)
			err = k8sClient.Get(ctx, request.NamespacedName, &v1alpha2.GrafanaOrganization{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}

		AfterEach(func() {
			// Release the finalizer and delete the organization if a spec failed halfway.
			leftover := &v1alpha2.GrafanaOrganization{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: orgName}, leftover); err == nil {
				leftover.Finalizers = nil
				_ = k8sClient.Update(ctx, leftover)
				_ = k8sClient.Delete(ctx, leftover)
			}
		})

		It("should not read or write SSO settings when SSO org mapping is disabled", func() {
			orgName = "sso-disabled-org"
			orgDisplayName = "SSO Disabled Organization"
			reconciler = newReconcilerWithSSO(false)

			createAndReconcile()
			deleteAndReconcile()

			By("Checking that the SSO settings were never touched")
			mockGrafanaClient.AssertNotCalled(GinkgoT(), "SsoSettings")
		})

		It("should write the SSO org mapping when SSO org mapping is enabled", func() {
			orgName = "sso-enabled-org"
			orgDisplayName = "SSO Enabled Organization"
			reconciler = newReconcilerWithSSO(true)

			mockSsoSettings := &mocks.MockSsoSettingsClient{}
			mockGrafanaClient.On("SsoSettings").Return(mockSsoSettings)
			mockSsoSettings.On("GetProviderSettings", ssoProvider).Return(&sso_settings.GetProviderSettingsOK{
				Payload: &models.GetProviderSettingsOKBody{
					ID:       "1",
					Provider: ssoProvider,
					Settings: map[string]any{},
				},
			}, nil)
			// Other GrafanaOrganizations may exist in the test environment, so only require
			// that this organization's admin group is part of the mapping.
			mockSsoSettings.On("UpdateProviderSettings", ssoProvider, mock.MatchedBy(func(body *models.UpdateProviderSettingsParamsBody) bool {
				orgMapping, ok := body.Settings["org_mapping"].(string)
				return ok && strings.Contains(orgMapping, ssoAdminsGroup)
			})).Return(&sso_settings.UpdateProviderSettingsNoContent{}, nil).Once()
			// On deletion the organization is no longer part of the mapping.
			mockSsoSettings.On("UpdateProviderSettings", ssoProvider, mock.Anything).
				Return(&sso_settings.UpdateProviderSettingsNoContent{}, nil).Maybe()

			createAndReconcile()

			By("Checking that the SSO settings were read and written")
			mockSsoSettings.AssertCalled(GinkgoT(), "GetProviderSettings", ssoProvider)
			mockSsoSettings.AssertNumberOfCalls(GinkgoT(), "UpdateProviderSettings", 1)

			deleteAndReconcile()
		})
	})

	Context("When deleting a GrafanaOrganization after Grafana is gone", func() {
		const (
			orgName              = "grafana-gone-org"
			adminSecretName      = "grafana-uninstalled" // nolint:gosec // G101: Secret resource name, not a credential
			adminSecretNamespace = "default"
		)

		var (
			ctx        context.Context
			reconciler *GrafanaOrganizationReconciler
			request    reconcile.Request
		)

		BeforeEach(func() {
			ctx = context.Background()
			request = reconcile.Request{NamespacedName: types.NamespacedName{Name: orgName}}

			grafanaURL, _ := url.Parse("http://localhost:3000")
			grafanaCfg := config.GrafanaConfig{
				URL:                  grafanaURL,
				AdminSecretNamespace: adminSecretNamespace,
				AdminSecretName:      adminSecretName,
			}
			reconciler = &GrafanaOrganizationReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				grafanaURL:      grafanaURL,
				finalizerHelper: NewFinalizerHelper(k8sClient, v1alpha2.GrafanaOrganizationFinalizer),
				// The real client generator, pointed at an admin Secret that does not exist,
				// as after the Grafana that owned it was uninstalled.
				grafanaClientGen:   grafanaclient.NewDefaultGrafanaClientGenerator(grafanaCfg),
				cfg:                config.Config{Grafana: grafanaCfg},
				organizationMapper: mapper.NewOrganizationMapper(),
			}

			grafanaOrg := &v1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:       orgName,
					Finalizers: []string{v1alpha2.GrafanaOrganizationFinalizer},
				},
				Spec: v1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Main Org.",
					Tenants:     []v1alpha2.TenantConfig{{Name: "default"}},
					RBAC:        &v1alpha2.RBAC{Admins: []string{"admins"}},
				},
			}
			Expect(k8sClient.Create(ctx, grafanaOrg)).To(Succeed())
			Expect(k8sClient.Delete(ctx, grafanaOrg)).To(Succeed())
		})

		AfterEach(func() {
			leftover := &v1alpha2.GrafanaOrganization{}
			if err := k8sClient.Get(ctx, request.NamespacedName, leftover); err == nil {
				leftover.Finalizers = nil
				_ = k8sClient.Update(ctx, leftover)
			}
		})

		It("should release the finalizer when the Grafana admin Secret no longer exists", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, request.NamespacedName, &v1alpha2.GrafanaOrganization{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should keep the finalizer when the Grafana admin Secret exists but is unusable", func() {
			brokenSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: adminSecretName, Namespace: adminSecretNamespace},
				Data:       map[string][]byte{"admin-user": []byte("admin")},
			}
			Expect(k8sClient.Create(ctx, brokenSecret)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, brokenSecret) })

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())

			remaining := &v1alpha2.GrafanaOrganization{}
			Expect(k8sClient.Get(ctx, request.NamespacedName, remaining)).To(Succeed())
			Expect(remaining.Finalizers).To(ContainElement(v1alpha2.GrafanaOrganizationFinalizer))
		})
	})
})
