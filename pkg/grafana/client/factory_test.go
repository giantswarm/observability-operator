package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	testAdminSecretNamespace   = "monitoring"
	testAdminSecretName        = "grafana"
	testGatewaySecretNamespace = "envoy-gateway-system"
	testGatewaySecretName      = "grafana-gateway-tls"
)

func testGrafanaConfig(gatewaySecretName string) config.GrafanaConfig {
	return config.GrafanaConfig{
		AdminSecretNamespace:      testAdminSecretNamespace,
		AdminSecretName:           testAdminSecretName,
		GatewayTLSSecretNamespace: testGatewaySecretNamespace,
		GatewayTLSSecretName:      gatewaySecretName,
	}
}

func adminSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: testAdminSecretNamespace, Name: testAdminSecretName},
		Data: map[string][]byte{
			grafanaAdminSecretUserKey:     []byte("admin"),
			grafanaAdminSecretPasswordKey: []byte("password"),
		},
	}
}

func tlsSecret(namespace, name string, cert, key []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data: map[string][]byte{
			grafanaTLSSecretCertKey: cert,
			grafanaTLSSecretKeyKey:  key,
		},
	}
}

// selfSignedCert generates a throwaway PEM-encoded certificate and key.
func selfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "observability-operator-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// recordingClient builds a fake client that records the keys of every Secret Get.
func recordingClient(objs ...client.Object) (client.Client, *[]client.ObjectKey) {
	var gets []client.ObjectKey
	c := fake.NewClientBuilder().WithObjects(objs...).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				gets = append(gets, key)
			}
			return cl.Get(ctx, key, obj, opts...)
		},
	}).Build()
	return c, &gets
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestGenerateGrafanaClient_HTTPReadsNoTLSSecret(t *testing.T) {
	ctx := context.Background()
	cert, key := selfSignedCert(t)
	// TLS Secrets exist, but must not be read for an http URL.
	c, gets := recordingClient(
		adminSecret(),
		tlsSecret(testGatewaySecretNamespace, testGatewaySecretName, cert, key),
		tlsSecret(testAdminSecretNamespace, grafanaLegacyTLSSecretName, cert, key),
	)
	g := NewDefaultGrafanaClientGenerator(testGrafanaConfig(testGatewaySecretName))
	grafanaURL := mustParseURL(t, "http://grafana.monitoring.svc")

	grafanaClient, err := g.GenerateGrafanaClient(ctx, c, grafanaURL)
	require.NoError(t, err)
	assert.NotNil(t, grafanaClient)
	assert.Equal(t, []client.ObjectKey{{Namespace: testAdminSecretNamespace, Name: testAdminSecretName}}, *gets,
		"only the admin Secret should be read for an http URL")

	tlsConfig, err := g.clientTLSConfig(ctx, c, grafanaURL)
	require.NoError(t, err)
	assert.Nil(t, tlsConfig)
}

func TestGenerateGrafanaClient_HTTPSWithoutTLSSecret(t *testing.T) {
	tests := []struct {
		name              string
		gatewaySecretName string
		expectedGets      []client.ObjectKey
	}{
		{
			name:              "empty gateway secret name skips straight to the legacy secret",
			gatewaySecretName: "",
			expectedGets: []client.ObjectKey{
				{Namespace: testAdminSecretNamespace, Name: grafanaLegacyTLSSecretName},
			},
		},
		{
			name:              "configured gateway secret name, neither secret found",
			gatewaySecretName: testGatewaySecretName,
			expectedGets: []client.ObjectKey{
				{Namespace: testGatewaySecretNamespace, Name: testGatewaySecretName},
				{Namespace: testAdminSecretNamespace, Name: grafanaLegacyTLSSecretName},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c, gets := recordingClient(adminSecret())
			g := NewDefaultGrafanaClientGenerator(testGrafanaConfig(tt.gatewaySecretName))
			grafanaURL := mustParseURL(t, "https://grafana.example.com")

			grafanaClient, err := g.GenerateGrafanaClient(ctx, c, grafanaURL)
			require.NoError(t, err)
			assert.NotNil(t, grafanaClient)

			*gets = nil
			tlsConfig, err := g.clientTLSConfig(ctx, c, grafanaURL)
			require.NoError(t, err)
			assert.Nil(t, tlsConfig, "no client certificate should be configured")
			assert.Equal(t, tt.expectedGets, *gets)
		})
	}
}

func TestGenerateGrafanaClient_HTTPSWithValidTLSSecret(t *testing.T) {
	cert, key := selfSignedCert(t)
	expected, err := tls.X509KeyPair(cert, key)
	require.NoError(t, err)

	tests := []struct {
		name              string
		gatewaySecretName string
		secret            *corev1.Secret
	}{
		{
			name:              "gateway secret",
			gatewaySecretName: testGatewaySecretName,
			secret:            tlsSecret(testGatewaySecretNamespace, testGatewaySecretName, cert, key),
		},
		{
			name:              "legacy secret fallback",
			gatewaySecretName: testGatewaySecretName,
			secret:            tlsSecret(testAdminSecretNamespace, grafanaLegacyTLSSecretName, cert, key),
		},
		{
			name:              "legacy secret with empty gateway secret name",
			gatewaySecretName: "",
			secret:            tlsSecret(testAdminSecretNamespace, grafanaLegacyTLSSecretName, cert, key),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c, _ := recordingClient(adminSecret(), tt.secret)
			g := NewDefaultGrafanaClientGenerator(testGrafanaConfig(tt.gatewaySecretName))
			grafanaURL := mustParseURL(t, "https://grafana.example.com")

			grafanaClient, err := g.GenerateGrafanaClient(ctx, c, grafanaURL)
			require.NoError(t, err)
			assert.NotNil(t, grafanaClient)

			tlsConfig, err := g.clientTLSConfig(ctx, c, grafanaURL)
			require.NoError(t, err)
			require.NotNil(t, tlsConfig)
			assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
			require.Len(t, tlsConfig.Certificates, 1)
			assert.Equal(t, expected.Certificate, tlsConfig.Certificates[0].Certificate)
		})
	}
}

func TestGenerateGrafanaClient_MalformedTLSSecret(t *testing.T) {
	cert, key := selfSignedCert(t)

	tests := []struct {
		name   string
		secret *corev1.Secret
	}{
		{
			name:   "missing tls.crt",
			secret: tlsSecret(testGatewaySecretNamespace, testGatewaySecretName, nil, key),
		},
		{
			name:   "empty tls.key",
			secret: tlsSecret(testGatewaySecretNamespace, testGatewaySecretName, cert, []byte{}),
		},
		{
			name:   "invalid key pair",
			secret: tlsSecret(testGatewaySecretNamespace, testGatewaySecretName, []byte("not a cert"), []byte("not a key")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := recordingClient(adminSecret(), tt.secret)
			g := NewDefaultGrafanaClientGenerator(testGrafanaConfig(testGatewaySecretName))

			grafanaClient, err := g.GenerateGrafanaClient(context.Background(), c, mustParseURL(t, "https://grafana.example.com"))
			require.Error(t, err)
			assert.Nil(t, grafanaClient)
		})
	}
}

func TestGenerateGrafanaClient_TLSSecretLookupError(t *testing.T) {
	boom := errors.New("boom")
	c := fake.NewClientBuilder().WithObjects(adminSecret()).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Name == testGatewaySecretName {
				return boom
			}
			return cl.Get(ctx, key, obj, opts...)
		},
	}).Build()
	g := NewDefaultGrafanaClientGenerator(testGrafanaConfig(testGatewaySecretName))

	grafanaClient, err := g.GenerateGrafanaClient(context.Background(), c, mustParseURL(t, "https://grafana.example.com"))
	require.ErrorIs(t, err, boom)
	assert.Nil(t, grafanaClient)
}
