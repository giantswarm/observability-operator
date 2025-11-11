package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestDefault tests that alerts without specific routing go to root receiver
// Root receiver has no configuration, so no HTTP requests should be made
func TestDefault(t *testing.T) {
	// With the default configuration an alert should not be sent to any receiver
	testCases := []helper.TestCase{
		{
			Alert: helper.Alert{
				Name: "GenericAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"severity":     "page",
					"team":         "test-team",
					"status":       "firing",
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
