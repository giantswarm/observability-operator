package grafana

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				URL:    "http://tempo-gateway.tempo.svc",
				Access: "proxy",
				JSONData: map[string]any{
					"serviceMap": map[string]any{
						"datasourceUid": "gs-mimir",
					},
					"tracesToLogs": map[string]any{
						"datasourceUid": "gs-loki",
						"tags":          []string{"service_name", "pod"},
						"mappedTags": []map[string]string{
							{
								"key":   "service_name",
								"value": "service",
							},
						},
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
		name         string
		organization Organization
		expectedLen  int
		checkTempo   bool
		checkShared  bool
	}{
		{
			name: "regular organization",
			organization: Organization{
				ID:        1,
				Name:      "test-org",
				TenantIDs: []string{"tenant1", "tenant2"},
			},
			expectedLen: 4, // Loki, Mimir, Alertmanager, Tempo
			checkTempo:  true,
			checkShared: false,
		},
		{
			name: "shared organization",
			organization: Organization{
				ID:        1,
				Name:      "Shared Org",
				TenantIDs: []string{"tenant1"},
			},
			expectedLen: 5, // Loki, Mimir, Alertmanager, Tempo, Cardinality
			checkTempo:  true,
			checkShared: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{}
			result := service.generateDatasources(tt.organization)

			assert.Len(t, result, tt.expectedLen)

			// Check that all datasources have the correct multi-tenant headers
			expectedHeaderValue := "tenant1|tenant2"
			if len(tt.organization.TenantIDs) == 1 {
				expectedHeaderValue = "tenant1"
			}

			for _, ds := range result {
				if ds.Name != "Mimir Cardinality" { // Cardinality has different logic
					assert.Equal(t, "X-Scope-OrgID", ds.JSONData["httpHeaderName1"])
					assert.Equal(t, expectedHeaderValue, ds.SecureJSONData["httpHeaderValue1"])
				}
				assert.True(t, ds.UID != "", "UID should not be empty")
				assert.True(t, ds.Name != "", "Name should not be empty")
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
				assert.Equal(t, "http://tempo-gateway.tempo.svc", tempoDS.URL)

				// Check tempo-specific configurations
				serviceMap, ok := tempoDS.JSONData["serviceMap"].(map[string]any)
				require.True(t, ok, "serviceMap should be present and be a map")
				assert.Equal(t, "gs-mimir", serviceMap["datasourceUid"])

				tracesToLogs, ok := tempoDS.JSONData["tracesToLogs"].(map[string]any)
				require.True(t, ok, "tracesToLogs should be present and be a map")
				assert.Equal(t, "gs-loki", tracesToLogs["datasourceUid"])
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
			name:           "cardinality datasource has predefined UID with gs- prefix",
			datasourceFunc: DatasourceMimirCardinality,
			expectedPrefix: "gs-",
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
