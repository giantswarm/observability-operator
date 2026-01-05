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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/webhook/testutil"
	webhookv1alpha2 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha2"
)

// k8sClient is the package-level client that will be set by the test suite
var k8sClient client.Client
var testSuite *testutil.WebhookTestSuite

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "V1Alpha1 Webhook Suite")
}

var _ = BeforeSuite(func() {
	// Create a new test suite instance
	testSuite = testutil.NewWebhookTestSuite()

	// Set up the webhook test environment with all needed schemes and webhooks
	config := testutil.WebhookSuiteConfig{
		SuiteName: "V1Alpha1 Webhook Suite",
		SchemeSetupFuncs: []testutil.SchemeSetupFunc{
			// Add core v1 scheme (for Secrets, ConfigMaps)
			corev1.AddToScheme,
			// Add observability v1alpha1 scheme (for GrafanaOrganization compatibility)
			observabilityv1alpha1.AddToScheme,
			// Add observability v1alpha2 scheme (for GrafanaOrganization storage version)
			observabilityv1alpha2.AddToScheme,
		},
		WebhookSetupFuncs: []testutil.WebhookSetupFunc{
			// Register v1alpha1 webhooks (for direct v1alpha1 validation)
			SetupGrafanaOrganizationWebhookWithManager,
			// Register v1alpha2 webhooks (storage version, handles converted v1alpha1 objects)
			webhookv1alpha2.SetupGrafanaOrganizationWebhookWithManager,
		},
	}

	testSuite.SetupSuite(config)

	// Set the client for use in tests
	k8sClient = testSuite.K8sClient
})

var _ = AfterSuite(func() {
	testSuite.TeardownSuite()
})
