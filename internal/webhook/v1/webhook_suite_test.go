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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/webhook/testutil"
	webhookv1alpha1 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha1"
	webhookv1alpha2 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha2"
)

// k8sClient is the package-level client that will be set by the test suite
var k8sClient client.Client
var testSuite *testutil.WebhookTestSuite

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "V1 Webhook Suite")
}

var _ = BeforeSuite(func() {
	// Create a new test suite instance
	testSuite = testutil.NewWebhookTestSuite()

	// Set up the webhook test environment with all needed schemes and webhooks
	config := testutil.WebhookSuiteConfig{
		SuiteName: "V1 Webhook Suite",
		SchemeSetupFuncs: []testutil.SchemeSetupFunc{
			// Add core v1 scheme (for Secrets, ConfigMaps)
			corev1.AddToScheme,
			// Add observability v1alpha1 scheme (for GrafanaOrganization compatibility)
			observabilityv1alpha1.AddToScheme,
			// Add observability v1alpha2 scheme (for GrafanaOrganization storage version)
			observabilityv1alpha2.AddToScheme,
		},
		WebhookSetupFuncs: []testutil.WebhookSetupFunc{
			// Register v1 webhooks
			SetupAlertmanagerConfigSecretWebhookWithManager,
			SetupDashboardConfigMapWebhookWithManager,
			// Register v1alpha1 webhooks (for direct v1alpha1 validation)
			webhookv1alpha1.SetupGrafanaOrganizationWebhookWithManager,
			// Register v1alpha2 webhooks (storage version, handles converted v1alpha1 objects)
			webhookv1alpha2.SetupGrafanaOrganizationWebhookWithManager,
		},
	}

	testSuite.SetupSuite(config)

	// Set the client for use in tests
	k8sClient = testSuite.K8sClient

	// Create GrafanaOrganization CRs used by dashboard webhook tests.
	ctx := context.Background()
	for _, name := range []string{"test-org", "annotation-org", "label-org"} {
		org := &observabilityv1alpha2.GrafanaOrganization{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
				DisplayName: name,
				RBAC:        &observabilityv1alpha2.RBAC{Admins: []string{}},
				Tenants:     []observabilityv1alpha2.TenantConfig{{Name: "giantswarm"}},
			},
		}
		Expect(k8sClient.Create(ctx, org)).To(Succeed())
	}
	// Create a GrafanaOrganization where Name != DisplayName to test that the webhook
	// matches by displayName (not resource name). This exercises the regression fixed in
	// https://github.com/giantswarm/observability-operator/pull/775.
	giantswarmOrg := &observabilityv1alpha2.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{Name: "giantswarm"},
		Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
			DisplayName: "Giant Swarm",
			RBAC:        &observabilityv1alpha2.RBAC{Admins: []string{}},
			Tenants:     []observabilityv1alpha2.TenantConfig{{Name: "giantswarm"}},
		},
	}
	Expect(k8sClient.Create(ctx, giantswarmOrg)).To(Succeed())
})

var _ = AfterSuite(func() {
	// Only clean up if BeforeSuite completed successfully
	if k8sClient != nil {
		ctx := context.Background()
		for _, name := range []string{"test-org", "annotation-org", "label-org", "giantswarm"} {
			org := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			}
			_ = k8sClient.Delete(ctx, org)
		}
	}
	if testSuite != nil {
		testSuite.TeardownSuite()
	}
})
