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

package v1

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	k8sClient client.Client
	cfg       *rest.Config
	testEnv   *envtest.Environment
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

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
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

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		BinaryAssetsDirectory: kubeBuilderAssets, // Use the assets we found

		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
		},
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager.
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	err = SetupAlertmanagerConfigSecretWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready.
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true}) // nolint:gosec
		if err != nil {
			return err
		}

		return conn.Close()
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	// Only check for assets if we need to clean up
	kubeBuilderAssets := getKubeBuilderAssets()
	if kubeBuilderAssets == "" {
		// If no assets were available, the test was skipped, so nothing to clean up
		return
	}

	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

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
		filepath.Join("..", "..", "..", "bin", "k8s"),        // bin/k8s/
		filepath.Join("..", "..", "..", "bin", "k8s", "k8s"), // bin/k8s/k8s/ (setup-envtest creates this nested structure)
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
