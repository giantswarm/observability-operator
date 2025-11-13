//go:build integration

package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestPipelineTestingRouting tests that pipeline=testing goes not go to PagerDuty
// unless all_pipelines=true is set
func TestPipelineTestingRouting(t *testing.T) {
	testCases := []helper.TestCase{
		// Alert with all_pipelines=true - should go to PagerDuty
		{
			Alert: helper.Alert{
				Name: "TestTestingAllPipelineAlert",
				Labels: map[string]string{
					"all_pipelines": "true",
					"cluster_id":    "test-cluster",
					"installation":  "test-installation",
					"pipeline":      "testing",
					"provider":      "aws",
					"severity":      "page",
					"status":        "firing",
					"team":          "foo",
				},
			},
			Expectations: []helper.Expectation{
				// PagerDuty expectation ONLY for all_pipelines=true alert
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`"routing_key":"foo-pagerduty-token"`, `"alertname":"TestTestingAllPipelineAlert","all_pipelines":"true"`},
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
