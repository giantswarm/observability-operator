package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestHeartbeat is a simple test that sends a single alert to Alertmanager
func TestHeartbeat(t *testing.T) {
	testCases := []helper.TestCase{
		// Heartbeat alert SHOULD go to Cronitor and OpsGenie
		{
			Alert: helper.Alert{
				Name: "Heartbeat",
				Labels: map[string]string{
					"cluster_id":   "foo",
					"installation": "bar",
					"status":       "firing",
					"team":         "baz",
				},
			},
			Expectations: []helper.Expectation{
				// Cronitor Heartbeat
				{
					URL:       "https://cronitor.link/p/cronitor-ping-key/mimir-mc-name?env=mc-pipeline",
					BodyParts: []string{`{"receiver":"cronitor-heartbeat","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"Heartbeat","cluster_id":"foo","installation":"bar","status":"firing","team":"baz"}`},
				},
				// OpsGenie Heartbeat
				{
					URL:       "https://api.opsgenie.com/v2/heartbeats/mc-name/ping",
					BodyParts: []string{`{"receiver":"heartbeat","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"Heartbeat","cluster_id":"foo","installation":"bar","status":"firing","team":"baz"}`},
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
