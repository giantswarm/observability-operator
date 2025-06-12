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

package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
)

// getKubeBuilderAssets attempts to get KUBEBUILDER_ASSETS from environment
// or find the binaries automatically. Returns empty string if neither is available.
func getKubeBuilderAssets() string {
	// First try environment variable
	if value := os.Getenv("KUBEBUILDER_ASSETS"); value != "" {
		return value
	}

	// If not set, try to find binaries automatically
	binDir := getFirstFoundEnvTestBinaryDir()
	if binDir != "" {
		logf.Log.Info("Using automatically detected envtest binaries", "path", binDir)
		return binDir
	}

	return ""
}

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// Check if we have KUBEBUILDER_ASSETS or can find them automatically
	kubeBuilderAssets := getKubeBuilderAssets()
	if kubeBuilderAssets == "" {
		Skip("KUBEBUILDER_ASSETS not set and envtest binaries not found. Run 'make setup-envtest' to set up test environment.")
	}

	ctx, cancel = context.WithCancel(context.Background())

	var err error
	err = observabilityv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// Add Cluster API types
	err = clusterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,             // Don't fail if CRDs are missing, we'll handle Cluster API separately
		BinaryAssetsDirectory: kubeBuilderAssets, // Use the assets we found
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Install Cluster API CRDs after environment starts
	By("installing Cluster API CRDs")
	err = installClusterAPICRDs(cfg)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	// Only check for assets if we need to clean up
	kubeBuilderAssets := getKubeBuilderAssets()
	if kubeBuilderAssets == "" {
		// If no assets were available, the test was skipped, so nothing to clean up
		return
	}

	// Clean up any test namespaces that might be in terminating state
	if k8sClient != nil {
		// Force cleanup any hanging namespaces
		ctx := context.Background()
		namespaces := &corev1.NamespaceList{}
		err := k8sClient.List(ctx, namespaces)
		if err == nil {
			for _, ns := range namespaces.Items {
				if ns.Name == "test-namespace" && ns.Status.Phase == corev1.NamespaceTerminating {
					// Remove finalizers to force cleanup
					ns.Finalizers = []string{}
					_ = k8sClient.Update(ctx, &ns)
				}
			}
		}
	}

	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// installClusterAPICRDs installs the Cluster API CRDs needed for testing
func installClusterAPICRDs(cfg *rest.Config) error {
	// Create a client for installing CRDs
	tempClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	// Add apiextensions scheme for CRD operations
	err = apiextensionsv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}

	// Create minimal Cluster CRD definition
	clusterCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusters.cluster.x-k8s.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "cluster.x-k8s.io",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"infrastructureRef": {
											Type: "object",
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"apiVersion": {Type: "string"},
												"kind":       {Type: "string"},
												"name":       {Type: "string"},
											},
										},
										"controlPlaneRef": {
											Type: "object",
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"apiVersion": {Type: "string"},
												"kind":       {Type: "string"},
												"name":       {Type: "string"},
											},
										},
										"controlPlaneEndpoint": {
											Type: "object",
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"host": {Type: "string"},
												"port": {Type: "integer"},
											},
										},
									},
								},
								"status": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"controlPlaneReady":   {Type: "boolean"},
										"infrastructureReady": {Type: "boolean"},
									},
								},
							},
						},
					},
				},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "clusters",
				Singular: "cluster",
				Kind:     "Cluster",
			},
		},
	}

	// Install the CRD
	err = tempClient.Create(context.Background(), clusterCRD)
	if err != nil {
		logf.Log.Error(err, "Failed to install Cluster CRD")
		return err
	}

	logf.Log.Info("Successfully installed Cluster API CRD")
	return nil
}

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	// Try common paths where envtest binaries might be located
	possiblePaths := []string{
		filepath.Join("..", "..", "bin", "k8s"),        // bin/k8s/
		filepath.Join("..", "..", "bin", "k8s", "k8s"), // bin/k8s/k8s/ (setup-envtest creates this nested structure)
	}

	for _, basePath := range possiblePaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			logf.Log.V(1).Info("Failed to read directory", "path", basePath, "error", err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				binPath := filepath.Join(basePath, entry.Name())
				// Verify this directory contains the expected binaries
				if hasEnvTestBinaries(binPath) {
					return binPath
				}
			}
		}
	}
	return ""
}

// hasEnvTestBinaries checks if a directory contains the expected envtest binaries
func hasEnvTestBinaries(path string) bool {
	expectedBinaries := []string{"kube-apiserver", "etcd", "kubectl"}
	for _, binary := range expectedBinaries {
		if _, err := os.Stat(filepath.Join(path, binary)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}
