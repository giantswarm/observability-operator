package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
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
})
