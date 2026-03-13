package alertmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	common "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	pkgconfig "github.com/giantswarm/observability-operator/pkg/config"
)

func TestValidate(t *testing.T) {
	validConfig := []byte("route:\n  receiver: noop\nreceivers:\n- name: noop\n")
	validTemplate := []byte("{{ define \"myalert\" }}fired{{ end }}")

	tests := []struct {
		name        string
		data        map[string][]byte
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config, no templates",
			data:    map[string][]byte{AlertmanagerConfigKey: validConfig},
			wantErr: false,
		},
		{
			name: "valid config with valid template",
			data: map[string][]byte{
				AlertmanagerConfigKey: validConfig,
				"alert.tmpl":          validTemplate,
			},
			wantErr: false,
		},
		{
			name:        "missing alertmanager.yaml",
			data:        map[string][]byte{},
			wantErr:     true,
			errContains: "missing alertmanager.yaml",
		},
		{
			name:    "invalid alertmanager config",
			data:    map[string][]byte{AlertmanagerConfigKey: []byte("not: valid: yaml: config:")},
			wantErr: true,
		},
		{
			name: "invalid template syntax",
			data: map[string][]byte{
				AlertmanagerConfigKey: validConfig,
				"bad.tmpl":            []byte("{{ define \"broken\" }}{{ if }}{{ end }}{{ end }}"),
			},
			wantErr:     true,
			errContains: "invalid template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &v1.Secret{Data: tt.data}
			err := Validate(secret)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractTemplates(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string][]byte
		wantKeys    []string
		wantErr     bool
		errContains string
	}{
		{
			name:     "no template entries",
			data:     map[string][]byte{AlertmanagerConfigKey: []byte("config")},
			wantKeys: []string{},
		},
		{
			name: "single valid template",
			data: map[string][]byte{
				"alert.tmpl": []byte(`{{ define "myalert" }}fired{{ end }}`),
			},
			wantKeys: []string{"alert.tmpl"},
		},
		{
			name: "path prefix is stripped to base name",
			data: map[string][]byte{
				"/etc/alertmanager/alert.tmpl": []byte(`{{ define "myalert" }}fired{{ end }}`),
			},
			wantKeys: []string{"alert.tmpl"},
		},
		{
			name: "non-template keys are ignored",
			data: map[string][]byte{
				AlertmanagerConfigKey: []byte("config"),
				"alert.tmpl":          []byte(`{{ define "myalert" }}fired{{ end }}`),
			},
			wantKeys: []string{"alert.tmpl"},
		},
		{
			name: "invalid template syntax returns error",
			data: map[string][]byte{
				"bad.tmpl": []byte("{{ define \"broken\" }}{{ if }}{{ end }}{{ end }}"),
			},
			wantErr:     true,
			errContains: "invalid template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &v1.Secret{Data: tt.data}
			got, err := extractTemplates(secret)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			for _, k := range tt.wantKeys {
				assert.Contains(t, got, k)
			}
			assert.Len(t, got, len(tt.wantKeys))
		})
	}
}

func TestCountRoutes(t *testing.T) {
	tests := []struct {
		name     string
		route    *config.Route
		expected int
	}{
		{
			name:     "nil route",
			route:    nil,
			expected: 0,
		},
		{
			name: "single root route",
			route: &config.Route{
				Receiver: "default",
			},
			expected: 1,
		},
		{
			name: "route with one sub-route",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
					},
				},
			},
			expected: 2,
		},
		{
			name: "route with multiple sub-routes",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
					},
					{
						Receiver: "sub2",
					},
					{
						Receiver: "sub3",
					},
				},
			},
			expected: 4,
		},
		{
			name: "nested routes",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
						Routes: []*config.Route{
							{
								Receiver: "nested1",
							},
							{
								Receiver: "nested2",
							},
						},
					},
					{
						Receiver: "sub2",
					},
				},
			},
			expected: 5, // root + sub1 + nested1 + nested2 + sub2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countRoutes(tt.route)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigureFromSecret(t *testing.T) {
	validConfig := []byte("route:\n  receiver: noop\nreceivers:\n- name: noop\n")

	tests := []struct {
		name           string
		secretData     map[string][]byte
		responseStatus int
		wantErr        bool
		errContains    string
	}{
		{
			name: "valid config with 201 Created succeeds",
			secretData: map[string][]byte{
				AlertmanagerConfigKey: validConfig,
			},
			responseStatus: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:        "missing alertmanager.yaml key returns error",
			secretData:  map[string][]byte{},
			wantErr:     true,
			errContains: "missing alertmanager.yaml",
		},
		{
			name: "invalid alertmanager config returns parse error",
			secretData: map[string][]byte{
				AlertmanagerConfigKey: []byte("not: valid: yaml: config:"),
			},
			wantErr:     true,
			errContains: "failed to load alertmanager configuration",
		},
		{
			name: "server error returns error",
			secretData: map[string][]byte{
				AlertmanagerConfigKey: validConfig,
			},
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "failed to send configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedMethod, receivedPath, receivedTenant string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				receivedPath = r.URL.Path
				receivedTenant = r.Header.Get(common.OrgIDHeader)
				w.WriteHeader(tt.responseStatus)
			}))
			defer srv.Close()

			svc := New(pkgconfig.Config{
				Monitoring: pkgconfig.MonitoringConfig{
					AlertmanagerURL: srv.URL,
				},
			})

			secret := &v1.Secret{Data: tt.secretData}
			err := svc.ConfigureFromSecret(context.Background(), secret, "test-tenant")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, receivedMethod)
			assert.Equal(t, alertmanagerAPIPath, receivedPath)
			assert.Equal(t, "test-tenant", receivedTenant)
		})
	}
}

func TestDeleteForTenant(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "200 OK returns nil",
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "404 Not Found is treated as success (idempotent)",
			responseStatus: http.StatusNotFound,
			wantErr:        false,
		},
		{
			name:           "500 Internal Server Error returns error",
			responseStatus: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "failed to delete configuration",
		},
		{
			name:           "400 Bad Request returns error",
			responseStatus: http.StatusBadRequest,
			wantErr:        true,
			errContains:    "failed to delete configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedMethod, receivedPath, receivedTenant string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				receivedPath = r.URL.Path
				receivedTenant = r.Header.Get(common.OrgIDHeader)
				w.WriteHeader(tt.responseStatus)
			}))
			defer srv.Close()

			svc := New(pkgconfig.Config{
				Monitoring: pkgconfig.MonitoringConfig{
					AlertmanagerURL: srv.URL,
				},
			})

			err := svc.DeleteForTenant(context.Background(), "test-tenant")

			assert.Equal(t, http.MethodDelete, receivedMethod)
			assert.Equal(t, alertmanagerAPIPath, receivedPath)
			assert.Equal(t, "test-tenant", receivedTenant)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
