package grafana

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/observability-operator/pkg/config"
)

func TestDatasourceTempo(t *testing.T) {
	tests := []struct {
		name     string
		expected Datasource
	}{
		{
			name: "tempo datasource configuration",
			expected: Datasource{
				Type:   "tempo",
				URL:    "http://tempo-query-frontend.tempo.svc:3200",
				Access: "proxy",
				JSONData: map[string]any{
					"serviceMap": map[string]any{
						"datasourceUid": MimirDatasourceUID,
					},
					"nodeGraph": map[string]any{
						"enabled": true,
					},
					"streamingEnabled": map[string]any{
						"metrics": true,
						"search":  true,
					},
					"tracesToLogsV2": map[string]any{
						"datasourceUid":      LokiDatasourceUID,
						"spanStartTimeShift": "-10m",
						"spanEndTimeShift":   "10m",
						"filterByTraceID":    true,
					},
					"tracesToMetrics": map[string]any{
						"datasourceUid": MimirDatasourceUID,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DatasourceTempo()

			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.URL, result.URL)
			assert.Equal(t, tt.expected.Access, result.Access)

			// Deep compare JSONData
			assert.True(t, reflect.DeepEqual(tt.expected.JSONData, result.JSONData),
				"JSONData mismatch.\nExpected: %+v\nActual: %+v", tt.expected.JSONData, result.JSONData)
		})
	}
}

func TestDatasourceLoki(t *testing.T) {
	result := DatasourceLoki()

	assert.Equal(t, "loki", result.Type)
	assert.Equal(t, "http://loki-gateway.loki.svc", result.URL)
	assert.Equal(t, "proxy", result.Access)
}

func TestDatasourceMimir(t *testing.T) {
	result := DatasourceMimir()

	assert.Equal(t, "prometheus", result.Type)
	assert.Equal(t, "http://mimir-gateway.mimir.svc/prometheus", result.URL)
	assert.Equal(t, "proxy", result.Access)

	// Check specific JSONData fields
	assert.Equal(t, "Medium", result.JSONData["cacheLevel"])
	assert.Equal(t, "POST", result.JSONData["httpMethod"])
	assert.Equal(t, true, result.JSONData["incrementalQuerying"])
	assert.Equal(t, "Mimir", result.JSONData["prometheusType"])
	assert.Equal(t, "2.9.1", result.JSONData["prometheusVersion"])
	assert.Equal(t, "60s", result.JSONData["timeInterval"])
}

func TestDatasourceMimirAlertmanager(t *testing.T) {
	result := DatasourceMimirAlertmanager()

	assert.Equal(t, "alertmanager", result.Type)
	assert.Equal(t, "http://mimir-alertmanager.mimir.svc:8080", result.URL)
	assert.Equal(t, "proxy", result.Access)

	// Check JSONData fields
	assert.Equal(t, false, result.JSONData["handleGrafanaManagedAlerts"])
	assert.Equal(t, "mimir", result.JSONData["implementation"])
}

func TestDatasourceMimirCardinality(t *testing.T) {
	result := DatasourceMimirCardinality()

	assert.Equal(t, "marcusolsson-json-datasource", result.Type)
	assert.Equal(t, "Mimir Cardinality", result.Name)
	assert.Equal(t, "gs-mimir-cardinality", result.UID)
	assert.Equal(t, "http://mimir-gateway.mimir.svc:8080/prometheus/api/v1/cardinality/", result.URL)
	assert.Equal(t, "proxy", result.Access)
}

func TestGenerateDatasources(t *testing.T) {
	tests := []struct {
		name           string
		organization   Organization
		tracingEnabled bool
		expectedLen    int
		checkTempo     bool
		checkShared    bool
		checkAlerting  bool
	}{
		{
			name: "regular organization with data-only tenants and tracing enabled",
			organization: Organization{
				ID:   1,
				Name: "test-org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data"}},
					{Name: "tenant2", Types: []string{"data"}},
				},
			},
			tracingEnabled: true,
			expectedLen:    3, // Loki, Mimir, Tempo (no Alertmanager - broken multi-tenant removed)
			checkTempo:     true,
			checkShared:    false,
			checkAlerting:  false,
		},
		{
			name: "regular organization with data-only tenants and tracing disabled",
			organization: Organization{
				ID:   1,
				Name: "test-org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data"}},
					{Name: "tenant2", Types: []string{"data"}},
				},
			},
			tracingEnabled: false,
			expectedLen:    2, // Loki, Mimir (no Tempo, no Alertmanager)
			checkTempo:     false,
			checkShared:    false,
			checkAlerting:  false,
		},
		{
			name: "organization with alerting-enabled tenants and tracing enabled",
			organization: Organization{
				ID:   1,
				Name: "test-org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data", "alerting"}},
					{Name: "tenant2", Types: []string{"data"}},
				},
			},
			tracingEnabled: true,
			expectedLen:    6, // Loki, Mimir, Tempo + Loki(tenant1), Mimir(tenant1), Alertmanager(tenant1)
			checkTempo:     true,
			checkShared:    false,
			checkAlerting:  true,
		},
		{
			name: "organization with alerting-enabled tenants and tracing disabled",
			organization: Organization{
				ID:   1,
				Name: "test-org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data", "alerting"}},
					{Name: "tenant2", Types: []string{"data"}},
				},
			},
			tracingEnabled: false,
			expectedLen:    5, // Loki, Mimir + Loki(tenant1), Mimir(tenant1), Alertmanager(tenant1)
			checkTempo:     false,
			checkShared:    false,
			checkAlerting:  true,
		},
		{
			name: "shared organization with tracing enabled",
			organization: Organization{
				ID:   1,
				Name: "Shared Org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data"}},
				},
			},
			tracingEnabled: true,
			expectedLen:    4, // Loki, Mimir, Tempo, Cardinality (no Alertmanager)
			checkTempo:     true,
			checkShared:    true,
			checkAlerting:  false,
		},
		{
			name: "shared organization with tracing disabled",
			organization: Organization{
				ID:   1,
				Name: "Shared Org",
				Tenants: []TenantConfig{
					{Name: "tenant1", Types: []string{"data"}},
				},
			},
			tracingEnabled: false,
			expectedLen:    3, // Loki, Mimir, Cardinality (no Tempo, no Alertmanager)
			checkTempo:     false,
			checkShared:    true,
			checkAlerting:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{
				cfg: config.Config{
					Tracing: config.TracingConfig{
						Enabled: tt.tracingEnabled,
					},
				},
			}
			result := service.generateDatasources(tt.organization)

			assert.Len(t, result, tt.expectedLen)

			// Check that all datasources have the correct multi-tenant headers
			expectedHeaderValue := "tenant1|tenant2"
			if len(tt.organization.Tenants) == 1 {
				expectedHeaderValue = "tenant1"
			}

			// Check multi-tenant datasources
			multiTenantDatasources := []string{"Loki", "Mimir", "Tempo"}
			perTenantCount := 0

			for _, ds := range result {
				// Check that all datasources have required fields
				assert.True(t, ds.UID != "", "UID should not be empty for %s", ds.Name)
				assert.True(t, ds.Name != "", "Name should not be empty")

				// Multi-tenant datasources should have multi-tenant headers
				if slices.Contains(multiTenantDatasources, ds.Name) && ds.Name != "Mimir Cardinality" {
					assert.Equal(t, "X-Scope-OrgID", ds.JSONData["httpHeaderName1"])
					assert.Equal(t, expectedHeaderValue, ds.SecureJSONData["httpHeaderValue1"])
				}

				// Count per-tenant datasources
				if strings.Contains(ds.Name, "(") && strings.Contains(ds.Name, ")") {
					perTenantCount++
					// Per-tenant datasources should have single-tenant headers
					assert.Equal(t, "X-Scope-OrgID", ds.JSONData["httpHeaderName1"])
					// Should not be multi-tenant header
					assert.NotEqual(t, expectedHeaderValue, ds.SecureJSONData["httpHeaderValue1"])
				}
			}

			if tt.checkTempo {
				// Find and validate Tempo datasource
				var tempoDS *Datasource
				for _, ds := range result {
					if ds.Type == "tempo" {
						tempoDS = &ds
						break
					}
				}
				require.NotNil(t, tempoDS, "Tempo datasource should be present")
				assert.Equal(t, "Tempo", tempoDS.Name)
				assert.Equal(t, "gs-tempo", tempoDS.UID)
				assert.Equal(t, "http://tempo-query-frontend.tempo.svc:3200", tempoDS.URL)

				// Check tempo-specific configurations
				serviceMap, ok := tempoDS.JSONData["serviceMap"].(map[string]any)
				require.True(t, ok, "serviceMap should be present and be a map")
				assert.Equal(t, MimirDatasourceUID, serviceMap["datasourceUid"])

				nodeGraph, ok := tempoDS.JSONData["nodeGraph"].(map[string]any)
				require.True(t, ok, "nodeGraph should be present and be a map")
				assert.Equal(t, true, nodeGraph["enabled"])

				tracesToLogsV2, ok := tempoDS.JSONData["tracesToLogsV2"].(map[string]any)
				require.True(t, ok, "tracesToLogsV2 should be present and be a map")
				assert.Equal(t, LokiDatasourceUID, tracesToLogsV2["datasourceUid"])
			}

			// Find and validate Loki datasource for derived fields
			var lokiDS *Datasource
			for _, ds := range result {
				if ds.Type == "loki" {
					lokiDS = &ds
					break
				}
			}
			require.NotNil(t, lokiDS, "Loki datasource should be present")
			assert.Equal(t, "Loki", lokiDS.Name)
			assert.Equal(t, "gs-loki", lokiDS.UID)

			// Check Loki derived fields based on tracing configuration
			if tt.tracingEnabled {
				derivedFields, ok := lokiDS.JSONData["derivedFields"].([]map[string]any)
				require.True(t, ok, "derivedFields should be present and be a slice when tracing is enabled")
				require.Len(t, derivedFields, 1, "Should have exactly one derived field")

				field := derivedFields[0]
				assert.Equal(t, "traceID", field["name"])
				assert.Equal(t, "[tT]race_?[Ii][dD]\"?[:=](\\w+)", field["matcherRegex"])
				assert.Equal(t, "gs-tempo", field["datasourceUid"])
				assert.Equal(t, "${__value.raw}", field["url"])
				assert.Equal(t, "Trace ID", field["urlDisplayLabel"])
			} else {
				_, hasDerivedFields := lokiDS.JSONData["derivedFields"]
				assert.False(t, hasDerivedFields, "derivedFields should not be present when tracing is disabled")
			}

			if tt.checkShared {
				// Validate that cardinality datasource is present for shared org
				var cardinalityDS *Datasource
				for _, ds := range result {
					if ds.Type == "marcusolsson-json-datasource" {
						cardinalityDS = &ds
						break
					}
				}
				require.NotNil(t, cardinalityDS, "Cardinality datasource should be present for shared org")
			}

			if tt.checkAlerting {
				// Validate that per-tenant alerting datasources are present
				var perTenantMimir *Datasource
				var perTenantAlertmanager *Datasource

				for _, ds := range result {
					if strings.Contains(ds.Name, "Mimir (") {
						perTenantMimir = &ds
					}
					if strings.Contains(ds.Name, "Mimir Alertmanager (") {
						perTenantAlertmanager = &ds
					}
				}

				require.NotNil(t, perTenantMimir, "Per-tenant Mimir datasource should be present for alerting tenant")
				require.NotNil(t, perTenantAlertmanager, "Per-tenant Alertmanager datasource should be present for alerting tenant")

				// Validate single-tenant headers
				assert.Equal(t, "X-Scope-OrgID", perTenantMimir.JSONData["httpHeaderName1"])
				assert.Equal(t, "tenant1", perTenantMimir.SecureJSONData["httpHeaderValue1"])
				assert.Equal(t, "X-Scope-OrgID", perTenantAlertmanager.JSONData["httpHeaderName1"])
				assert.Equal(t, "tenant1", perTenantAlertmanager.SecureJSONData["httpHeaderValue1"])
			}
		})
	}
}

func TestDatasourceMerge(t *testing.T) {
	base := Datasource{
		Type:   "tempo",
		URL:    "http://base-url",
		Access: "proxy",
		JSONData: map[string]any{
			"baseField": "baseValue",
		},
		SecureJSONData: map[string]string{
			"baseSecret": "baseSecretValue",
		},
	}

	override := Datasource{
		Name: "Merged Tempo",
		UID:  "merged-tempo",
		JSONData: map[string]any{
			"overrideField": "overrideValue",
			"baseField":     "overriddenValue", // Should override base
		},
		SecureJSONData: map[string]string{
			"overrideSecret": "overrideSecretValue",
		},
	}

	result := base.Merge(override)

	// Check that override values take precedence
	assert.Equal(t, "Merged Tempo", result.Name)
	assert.Equal(t, "merged-tempo", result.UID)

	// Check that base values are preserved when not overridden
	assert.Equal(t, "tempo", result.Type)
	assert.Equal(t, "http://base-url", result.URL)
	assert.Equal(t, "proxy", result.Access)

	// Check JSONData merging
	assert.Equal(t, "overrideValue", result.JSONData["overrideField"])
	assert.Equal(t, "overriddenValue", result.JSONData["baseField"]) // Should be overridden

	// Check SecureJSONData merging
	assert.Equal(t, "baseSecretValue", result.SecureJSONData["baseSecret"])
	assert.Equal(t, "overrideSecretValue", result.SecureJSONData["overrideSecret"])
}

func TestDatasourceUIDPrefix(t *testing.T) {
	tests := []struct {
		name           string
		datasourceFunc func() Datasource
		expectedPrefix string
		hasUID         bool
	}{
		{
			name:           "tempo datasource has no predefined UID",
			datasourceFunc: DatasourceTempo,
			hasUID:         false,
		},
		{
			name:           "loki datasource has no predefined UID",
			datasourceFunc: DatasourceLoki,
			hasUID:         false,
		},
		{
			name:           "mimir datasource has no predefined UID",
			datasourceFunc: DatasourceMimir,
			hasUID:         false,
		},
		{
			name:           "cardinality datasource has predefined UID",
			datasourceFunc: DatasourceMimirCardinality,
			hasUID:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.datasourceFunc()

			if tt.hasUID {
				assert.NotEmpty(t, result.UID)
				assert.Contains(t, result.UID, tt.expectedPrefix)
			} else {
				assert.Empty(t, result.UID, "Datasource should not have predefined UID")
			}
		})
	}
}
