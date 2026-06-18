package controller

import (
	"context"
	"net/url"
	"testing"

	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/internal/labels"
	"github.com/giantswarm/observability-operator/internal/mapper"
	"github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

// TestDashboardCleanupOrganizationRequest verifies that dashboard ConfigMaps are
// mapped to a reconcile request keyed by their organization, which is what makes
// the controller debounce a burst of dashboard events into a single per-org cleanup.
func TestDashboardCleanupOrganizationRequest(t *testing.T) {
	r := &DashboardCleanupReconciler{dashboardMapper: mapper.New()}

	dashboardCM := func(org string) *v1.ConfigMap {
		cm := &v1.ConfigMap{
			Data: map[string]string{"dashboard.json": `{"uid":"u","title":"t"}`},
		}
		if org != "" {
			cm.Annotations = map[string]string{labels.GrafanaOrganizationKey: org}
		}
		return cm
	}

	t.Run("configmap with organization is keyed by org name", func(t *testing.T) {
		req, ok := r.organizationRequest(dashboardCM("My Org"))
		require.True(t, ok)
		require.Equal(t, reconcile.Request{NamespacedName: types.NamespacedName{Name: "My Org"}}, req)
	})

	t.Run("configmap without organization is skipped", func(t *testing.T) {
		_, ok := r.organizationRequest(dashboardCM(""))
		require.False(t, ok)
	})

	t.Run("non-configmap object is skipped", func(t *testing.T) {
		_, ok := r.organizationRequest(&v1.Secret{})
		require.False(t, ok)
	})
}

var _ = Describe("Dashboard Cleanup Controller", func() {
	const (
		orgName            = "Cleanup Test Org"
		dashboardNamespace = "default"
	)

	var (
		ctx               context.Context
		reconciler        *DashboardCleanupReconciler
		mockGrafanaGen    *mocks.MockGrafanaClientGenerator
		mockGrafanaClient *mocks.MockGrafanaClient
		keptConfigMap     *v1.ConfigMap
	)

	BeforeEach(func() {
		ctx = context.Background()

		ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dashboardNamespace}}
		_ = k8sClient.Create(ctx, ns)

		// A dashboard ConfigMap whose folder must be kept (it is still referenced).
		keptConfigMap = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kept-dashboard",
				Namespace: dashboardNamespace,
				Labels: map[string]string{
					labels.DashboardSelectorLabelName: labels.DashboardSelectorLabelValue,
				},
				Annotations: map[string]string{
					labels.GrafanaOrganizationKey: orgName,
					labels.GrafanaFolderKey:       "keep-team",
				},
			},
			Data: map[string]string{"dashboard.json": `{"uid":"kept-uid","title":"Kept"}`},
		}
		Expect(k8sClient.Create(ctx, keptConfigMap)).To(Succeed())

		mockGrafanaGen = &mocks.MockGrafanaClientGenerator{}
		mockGrafanaClient = &mocks.MockGrafanaClient{}
		mockGrafanaGen.On("GenerateGrafanaClient", mock.Anything, mock.Anything, mock.Anything).Return(mockGrafanaClient, nil)
		mockGrafanaClient.On("WithOrgID", mock.AnythingOfType("int64")).Return(mockGrafanaClient)

		grafanaURL, _ := url.Parse("http://localhost:3000")
		reconciler = &DashboardCleanupReconciler{
			Client:           k8sClient,
			Scheme:           k8sClient.Scheme(),
			grafanaURL:       grafanaURL,
			dashboardMapper:  mapper.New(),
			grafanaClientGen: mockGrafanaGen,
		}
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, keptConfigMap)
		mockGrafanaClient.AssertExpectations(GinkgoT())
		mockGrafanaGen.AssertExpectations(GinkgoT())
	})

	It("deletes orphaned operator-managed folders while keeping referenced ones", func() {
		keptUID := folder.GenerateUID("keep-team")
		orphanUID := folder.GenerateUID("old-team")

		mockOrgsClient := &mocks.MockOrgsClient{}
		mockGrafanaClient.On("Orgs").Return(mockOrgsClient)
		mockOrgsClient.On("GetOrgByName", orgName).Return(&orgs.GetOrgByNameOK{
			Payload: &models.OrgDetailsDTO{ID: 1, Name: orgName},
		}, nil)

		mockFoldersClient := &mocks.MockFoldersClient{}
		mockGrafanaClient.On("Folders").Return(mockFoldersClient)
		mockFoldersClient.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
			Payload: []*models.FolderSearchHit{
				{UID: keptUID, Title: "keep-team"},
				{UID: orphanUID, Title: "old-team"},
			},
		}, nil)
		// Only the orphaned folder is inspected and deleted; the referenced one is skipped.
		mockFoldersClient.On("GetFolderDescendantCounts", orphanUID).Return(&folders.GetFolderDescendantCountsOK{
			Payload: map[string]int64{},
		}, nil)
		mockFoldersClient.On("DeleteFolder", mock.MatchedBy(func(params *folders.DeleteFolderParams) bool {
			return params.FolderUID == orphanUID
		})).Return(&folders.DeleteFolderOK{}, nil)

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: orgName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		mockFoldersClient.AssertExpectations(GinkgoT())
	})
})
