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

package testutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// WebhookTestSuite provides common functionality for webhook test suites
type WebhookTestSuite struct {
	Ctx       context.Context
	Cancel    context.CancelFunc
	K8sClient client.Client
	Cfg       *rest.Config
	TestEnv   *envtest.Environment
}

// WebhookSetupFunc is a function type for setting up webhooks with a manager
type WebhookSetupFunc func(mgr ctrl.Manager) error

// SchemeSetupFunc is a function type for adding types to a scheme
type SchemeSetupFunc func(s *runtime.Scheme) error

// WebhookSuiteConfig holds configuration for setting up a webhook test suite
type WebhookSuiteConfig struct {
	SuiteName         string
	SchemeSetupFuncs  []SchemeSetupFunc
	WebhookSetupFuncs []WebhookSetupFunc
}

// NewWebhookTestSuite creates a new webhook test suite with common functionality
func NewWebhookTestSuite() *WebhookTestSuite {
	return &WebhookTestSuite{}
}

// SetupSuite sets up the test environment for webhook testing
func (s *WebhookTestSuite) SetupSuite(config WebhookSuiteConfig) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// Check if we have KUBEBUILDER_ASSETS or can find them automatically
	kubeBuilderAssets := getKubeBuilderAssets()
	if kubeBuilderAssets == "" {
		Skip("KUBEBUILDER_ASSETS not set and envtest binaries not found. Run 'make setup-envtest' to set up test environment.")
	}

	s.Ctx, s.Cancel = context.WithCancel(context.Background())

	// Setup schemes
	var err error
	for _, setupFunc := range config.SchemeSetupFuncs {
		err = setupFunc(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
	}

	By("bootstrapping test environment")
	s.TestEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		BinaryAssetsDirectory: kubeBuilderAssets,

		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
		},
	}

	s.Cfg, err = s.TestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(s.Cfg).NotTo(BeNil())

	s.K8sClient, err = client.New(s.Cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(s.K8sClient).NotTo(BeNil())

	// Start webhook server using Manager
	webhookInstallOptions := &s.TestEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(s.Cfg, ctrl.Options{
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

	// Setup webhooks
	for _, setupFunc := range config.WebhookSetupFuncs {
		err = setupFunc(mgr)
		Expect(err).NotTo(HaveOccurred())
	}

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(s.Ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// Wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true}) // nolint:gosec
		if err != nil {
			return err
		}
		return conn.Close()
	}).Should(Succeed())
}

// TeardownSuite tears down the test environment
func (s *WebhookTestSuite) TeardownSuite() {
	By("tearing down the test environment")

	// Only check for assets if we need to clean up
	kubeBuilderAssets := getKubeBuilderAssets()
	if kubeBuilderAssets == "" {
		// If no assets were available, the test was skipped, so nothing to clean up
		return
	}

	s.Cancel()
	err := s.TestEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
}

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

// SetupWebhookTestSuite is a simplified helper function that sets up a webhook test suite
// with common default configurations
func SetupWebhookTestSuite(webhookSetupFuncs ...WebhookSetupFunc) *WebhookTestSuite {
	suite := NewWebhookTestSuite()

	config := WebhookSuiteConfig{
		SuiteName: "Webhook Test Suite",
		SchemeSetupFuncs: []SchemeSetupFunc{
			// Add core scheme
			func(s *runtime.Scheme) error {
				return nil // corev1 is already added by default
			},
			// Add observability scheme
			func(s *runtime.Scheme) error {
				// Import the API package and add its scheme
				// This will be done in the specific webhook suite files
				return nil
			},
		},
		WebhookSetupFuncs: webhookSetupFuncs,
	}

	suite.SetupSuite(config)
	return suite
}
