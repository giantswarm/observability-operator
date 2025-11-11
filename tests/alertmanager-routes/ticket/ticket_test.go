//go:build integration

package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestTeamsTicketRouting tests GitHub webhook routing for all teams in a single test
func TestTeamsTicketRouting(t *testing.T) {
	testCases := []helper.TestCase{
		// Atlas team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketAtlasAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "atlas",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/atlas",
					BodyParts: []string{`{"receiver":"team_atlas_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketAtlasAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"atlas"}`},
				},
			},
		},
		// Phoenix team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketPhoenixAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "phoenix",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/phoenix",
					BodyParts: []string{`{"receiver":"team_phoenix_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketPhoenixAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"phoenix"}`},
				},
			},
		},
		// Shield team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketShieldAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "shield",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/shield",
					BodyParts: []string{`{"receiver":"team_shield_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketShieldAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"shield"}`},
				},
			},
		},
		// Rocket team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketRocketAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "rocket",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/rocket",
					BodyParts: []string{`{"receiver":"team_rocket_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketRocketAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"rocket"}`},
				},
			},
		},
		// Honeybadger team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketHoneybadgerAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "honeybadger",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/honeybadger",
					BodyParts: []string{`{"receiver":"team_honeybadger_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketHoneybadgerAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"honeybadger"}`},
				},
			},
		},
		// Tenet team ticket alert
		{
			Alert: helper.Alert{
				Name: "TestTicketTeamAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "tenet",
					"severity":     "ticket",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "http://localhost:8081/v1/webhook?owner=giantswarm&repo=giantswarm&labels=team/tenet",
					BodyParts: []string{`{"receiver":"team_tenet_github","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestTicketTeamAlert","cluster_id":"test-cluster","installation":"test-installation","pipeline":"stable","provider":"aws","severity":"ticket","status":"firing","team":"tenet"}`},
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
