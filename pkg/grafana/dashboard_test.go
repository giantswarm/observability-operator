package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/mock"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

const (
	giantSwarm            = "Giant Swarm"
	testDashboard         = "Test Dashboard"
	alerting              = "alerting"
	apiVersionKey         = "apiVersion"
	dashboardAPIVersionV2 = "dashboard.grafana.app/v2"
	dataKey               = "data"
	existingTag           = "existing-tag"
	gsClusterOverview     = "gs_cluster-overview"
	metadataKey           = "metadata"
	oldTeam               = "old-team"
	teamA                 = "team-a"
	tenant1               = "tenant1"
	tenant2               = "tenant2"
	testUID               = "test-uid"
)

func TestInjectManagedTag(t *testing.T) {
	t.Run("adds tag to empty tags", func(t *testing.T) {
		content := map[string]any{
			uidKey:   testUID,
			titleKey: testDashboard,
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content[tagsKey].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
		if tags[0] != managedDashboardTag {
			t.Errorf("Expected tag %q, got %q", managedDashboardTag, tags[0])
		}
	})

	t.Run("adds tag to existing tags", func(t *testing.T) {
		content := map[string]any{
			uidKey:   testUID,
			titleKey: testDashboard,
			tagsKey:  []any{existingTag, "another-tag"},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content[tagsKey].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 3 {
			t.Fatalf("Expected 3 tags, got %d", len(tags))
		}
		if tags[2] != managedDashboardTag {
			t.Errorf("Expected last tag %q, got %q", managedDashboardTag, tags[2])
		}
	})

	t.Run("is idempotent - does not duplicate tag", func(t *testing.T) {
		content := map[string]any{
			uidKey:   testUID,
			titleKey: testDashboard,
			tagsKey:  []any{existingTag},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := injectManagedTag(content); err != nil { // Call twice
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content[tagsKey].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 2 {
			t.Fatalf("Expected 2 tags (not duplicated), got %d", len(tags))
		}
	})

	t.Run("handles nil tags field", func(t *testing.T) {
		content := map[string]any{
			uidKey:  testUID,
			tagsKey: nil,
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content[tagsKey].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("handles tags of wrong type", func(t *testing.T) {
		content := map[string]any{
			uidKey:  testUID,
			tagsKey: "not-an-array",
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content[tagsKey].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("v2 schema adds tag under spec.tags", func(t *testing.T) {
		content := map[string]any{
			apiVersionKey: dashboardAPIVersionV2,
			"kind":        "Dashboard",
			metadataKey:   map[string]any{nameKey: gsClusterOverview},
			"spec": map[string]any{
				titleKey: "Cluster Overview",
				tagsKey:  []any{existingTag},
			},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The managed tag must land under spec.tags, not at the top level.
		if _, topLevel := content[tagsKey]; topLevel {
			t.Error("v2 dashboard should not get a top-level tags field")
		}
		spec := content["spec"].(map[string]any)
		tags, ok := spec[tagsKey].([]any)
		if !ok {
			t.Fatal("spec.tags should be a []any")
		}
		if len(tags) != 2 {
			t.Fatalf("Expected 2 tags, got %d", len(tags))
		}
		if tags[1] != managedDashboardTag {
			t.Errorf("Expected last tag %q, got %q", managedDashboardTag, tags[1])
		}
	})

	t.Run("v2 schema is idempotent", func(t *testing.T) {
		content := map[string]any{
			apiVersionKey: dashboardAPIVersionV2,
			metadataKey:   map[string]any{nameKey: gsClusterOverview},
			"spec":        map[string]any{tagsKey: []any{managedDashboardTag}},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spec := content["spec"].(map[string]any)
		tags := spec[tagsKey].([]any)
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag (not duplicated), got %d", len(tags))
		}
	})

	t.Run("v2 schema with missing spec returns an error", func(t *testing.T) {
		content := map[string]any{
			apiVersionKey: dashboardAPIVersionV2,
			metadataKey:   map[string]any{nameKey: gsClusterOverview},
		}

		if err := injectManagedTag(content); err == nil {
			t.Fatal("expected an error for a malformed v2 dashboard without a spec")
		}

		if _, ok := content["spec"]; ok {
			t.Error("injectManagedTag should not create a spec on a malformed v2 dashboard")
		}
		if _, ok := content[tagsKey]; ok {
			t.Error("injectManagedTag should not add top-level tags to a v2 dashboard")
		}
	})
}

// sourceLinkJSON is the dashboard link injectSourceLink appends.
const sourceLinkJSON = `{
	"title": "giantswarm/dashboards",
	"type": "link",
	"url": "https://github.com/giantswarm/dashboards",
	"tooltip": "Source of truth for this dashboard",
	"icon": "external link",
	"targetBlank": true,
	"tags": [],
	"asDropdown": false,
	"includeVars": false,
	"keepTime": false
}`

// otherLinkJSON is a dashboard link unrelated to the source repository.
const otherLinkJSON = `{"title": "Runbook", "type": "link", "url": "https://example.com/runbook"}`

// mustUnmarshal decodes a JSON document into a generic value.
func mustUnmarshal(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", data, err)
	}
	return v
}

// assertJSONEqual fails the test when got and want encode different JSON documents.
func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	if !reflect.DeepEqual(mustUnmarshal(t, got), mustUnmarshal(t, []byte(want))) {
		t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func TestInjectSourceLink(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "classic without links creates links",
			content: `{"uid": "test-uid"}`,
			want:    `{"uid": "test-uid", "links": [` + sourceLinkJSON + `]}`,
		},
		{
			name:    "classic with other links appends the link",
			content: `{"uid": "test-uid", "links": [` + otherLinkJSON + `]}`,
			want:    `{"uid": "test-uid", "links": [` + otherLinkJSON + `, ` + sourceLinkJSON + `]}`,
		},
		{
			name:    "classic with the repository link keeps links unchanged",
			content: `{"uid": "test-uid", "links": [{"title": "custom", "url": "https://github.com/giantswarm/dashboards"}]}`,
			want:    `{"uid": "test-uid", "links": [{"title": "custom", "url": "https://github.com/giantswarm/dashboards"}]}`,
		},
		{
			name:    "v2 without links creates spec.links",
			content: `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {}}`,
			want:    `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {"links": [` + sourceLinkJSON + `]}}`,
		},
		{
			name:    "v2 with other links appends the link to spec.links",
			content: `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {"links": [` + otherLinkJSON + `]}}`,
			want:    `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {"links": [` + otherLinkJSON + `, ` + sourceLinkJSON + `]}}`,
		},
		{
			name:    "v2 with the repository link keeps spec.links unchanged",
			content: `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {"links": [` + sourceLinkJSON + `]}}`,
			want:    `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}, "spec": {"links": [` + sourceLinkJSON + `]}}`,
		},
		{
			name:    "v2 without spec returns an error",
			content: `{"apiVersion": "dashboard.grafana.app/v2", "metadata": {"name": "test-uid"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := mustUnmarshal(t, []byte(tt.content)).(map[string]any)

			err := injectSourceLink(content)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := json.Marshal(content)
			if err != nil {
				t.Fatalf("failed to marshal content: %v", err)
			}
			assertJSONEqual(t, got, tt.want)
		})
	}
}

func TestBuildDashboardResource(t *testing.T) {
	const sourcePath = "configmap monitoring/test-dashboard"

	tests := []struct {
		name        string
		content     string
		folderUID   string
		wantVersion string
		want        string
	}{
		{
			name:        "classic in the General folder",
			content:     `{"uid": "test-uid", "title": "Test Dashboard", "tags": ["a"]}`,
			wantVersion: "v1",
			want: `{
				"apiVersion": "dashboard.grafana.app/v1",
				"kind": "Dashboard",
				"metadata": {
					"name": "test-uid",
					"annotations": {
						"grafana.app/managedBy": "kubectl",
						"grafana.app/managerId": "observability-operator",
						"grafana.app/sourcePath": "configmap monitoring/test-dashboard"
					}
				},
				"spec": {"uid": "test-uid", "title": "Test Dashboard", "tags": ["a"]}
			}`,
		},
		{
			name:        "classic in a folder",
			content:     `{"uid": "test-uid", "title": "Test Dashboard"}`,
			folderUID:   "folder-uid",
			wantVersion: "v1",
			want: `{
				"apiVersion": "dashboard.grafana.app/v1",
				"kind": "Dashboard",
				"metadata": {
					"name": "test-uid",
					"annotations": {
						"grafana.app/managedBy": "kubectl",
						"grafana.app/managerId": "observability-operator",
						"grafana.app/sourcePath": "configmap monitoring/test-dashboard",
						"grafana.app/folder": "folder-uid"
					}
				},
				"spec": {"uid": "test-uid", "title": "Test Dashboard"}
			}`,
		},
		{
			name:        "v2 in a folder",
			content:     `{"apiVersion": "dashboard.grafana.app/v2", "kind": "Dashboard", "metadata": {"name": "test-uid"}, "spec": {"title": "Test Dashboard"}}`,
			folderUID:   "folder-uid",
			wantVersion: "v2",
			want: `{
				"apiVersion": "dashboard.grafana.app/v2",
				"kind": "Dashboard",
				"metadata": {
					"name": "test-uid",
					"annotations": {
						"grafana.app/managedBy": "kubectl",
						"grafana.app/managerId": "observability-operator",
						"grafana.app/sourcePath": "configmap monitoring/test-dashboard",
						"grafana.app/folder": "folder-uid"
					}
				},
				"spec": {"title": "Test Dashboard"}
			}`,
		},
		{
			name: "v2 copied from Grafana drops server metadata",
			content: `{
				"apiVersion": "dashboard.grafana.app/v2",
				"kind": "Dashboard",
				"metadata": {
					"name": "test-uid",
					"namespace": "default",
					"uid": "0b1c2d3e-k8s-uid",
					"resourceVersion": "1762250000000000",
					"generation": 3,
					"creationTimestamp": "2026-11-04T10:00:00Z",
					"labels": {"grafana.app/deprecatedInternalID": "42"},
					"annotations": {
						"grafana.app/managedBy": "kubectl",
						"grafana.app/updatedBy": "user:abc",
						"grafana.app/folder": "old-folder"
					}
				},
				"spec": {"title": "Test Dashboard"}
			}`,
			wantVersion: "v2",
			want: `{
				"apiVersion": "dashboard.grafana.app/v2",
				"kind": "Dashboard",
				"metadata": {
					"name": "test-uid",
					"annotations": {
						"grafana.app/managedBy": "kubectl",
						"grafana.app/managerId": "observability-operator",
						"grafana.app/sourcePath": "configmap monitoring/test-dashboard"
					}
				},
				"spec": {"title": "Test Dashboard"}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := mustUnmarshal(t, []byte(tt.content)).(map[string]any)

			version, got, err := buildDashboardResource(testUID, content, tt.folderUID, sourcePath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != tt.wantVersion {
				t.Errorf("expected version %q, got %q", tt.wantVersion, version)
			}
			assertJSONEqual(t, got, tt.want)
		})
	}
}

func TestPublishDashboard(t *testing.T) {
	const namespace = "org-2"
	resource := json.RawMessage(`{"metadata": {"name": "test-uid"}}`)

	tests := []struct {
		name    string
		getErr  error
		setup   func(api *mocks.MockDashboardAPIClient)
		wantErr bool
	}{
		{
			name:   "creates a missing dashboard",
			getErr: fmt.Errorf("wrapped: %w", grafanaclient.ErrDashboardNotFound),
			setup: func(api *mocks.MockDashboardAPIClient) {
				api.On("Create", mock.Anything, "v1", namespace, resource).Return(nil)
			},
		},
		{
			name: "updates an existing dashboard",
			setup: func(api *mocks.MockDashboardAPIClient) {
				api.On("Update", mock.Anything, "v1", namespace, testUID, resource).Return(nil)
			},
		},
		{
			name:    "returns the get error",
			getErr:  errors.New("status 500"),
			setup:   func(api *mocks.MockDashboardAPIClient) {},
			wantErr: true,
		},
		{
			name:   "returns the create error",
			getErr: grafanaclient.ErrDashboardNotFound,
			setup: func(api *mocks.MockDashboardAPIClient) {
				api.On("Create", mock.Anything, "v1", namespace, resource).Return(errors.New("status 400"))
			},
			wantErr: true,
		},
		{
			name: "returns the update error",
			setup: func(api *mocks.MockDashboardAPIClient) {
				api.On("Update", mock.Anything, "v1", namespace, testUID, resource).Return(errors.New("status 403"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &mocks.MockDashboardAPIClient{}
			var existing json.RawMessage
			if tt.getErr == nil {
				existing = json.RawMessage(`{}`)
			}
			api.On("Get", mock.Anything, "v1", namespace, testUID).Return(existing, tt.getErr)
			tt.setup(api)

			err := publishDashboard(context.Background(), api, "v1", namespace, testUID, resource)
			if tt.wantErr != (err != nil) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestConfigureDashboard(t *testing.T) {
	tests := []struct {
		name          string
		orgID         int64
		content       string
		wantVersion   string
		wantNamespace string
	}{
		{
			name:          "classic dashboard in org 1",
			orgID:         1,
			content:       `{"id": 7, "uid": "test-uid", "title": "Test Dashboard"}`,
			wantVersion:   "v1",
			wantNamespace: "default",
		},
		{
			name:          "v2 dashboard in another org",
			orgID:         5,
			content:       `{"apiVersion": "dashboard.grafana.app/v2", "kind": "Dashboard", "metadata": {"name": "test-uid"}, "spec": {"title": "Test Dashboard"}}`,
			wantVersion:   "v2",
			wantNamespace: "org-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockGrafanaClient{}
			mockOrgs := &mocks.MockOrgsClient{}
			api := &mocks.MockDashboardAPIClient{}

			mockClient.On("Orgs").Return(mockOrgs)
			mockOrgs.On("GetOrgByName", giantSwarm).Return(&orgs.GetOrgByNameOK{
				Payload: &models.OrgDetailsDTO{ID: tt.orgID, Name: giantSwarm},
			}, nil)
			mockClient.On("WithOrgID", tt.orgID).Return(mockClient)
			mockClient.On("DashboardAPI").Return(api)

			var created json.RawMessage
			api.On("Get", mock.Anything, tt.wantVersion, tt.wantNamespace, testUID).Return(nil, grafanaclient.ErrDashboardNotFound)
			api.On("Create", mock.Anything, tt.wantVersion, tt.wantNamespace, mock.Anything).
				Run(func(args mock.Arguments) { created = args.Get(3).(json.RawMessage) }).
				Return(nil)

			content := mustUnmarshal(t, []byte(tt.content)).(map[string]any)
			d := dashboard.New(giantSwarm, "", "monitoring", "test-dashboard", content)

			svc := newTestService(mockClient)
			if err := svc.ConfigureDashboard(context.Background(), d); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			mockClient.AssertExpectations(t)
			api.AssertExpectations(t)

			resource := mustUnmarshal(t, created).(map[string]any)
			metadata := resource["metadata"].(map[string]any)
			annotations := metadata["annotations"].(map[string]any)
			if annotations[sourcePathAnnotation] != "configmap monitoring/test-dashboard" {
				t.Errorf("unexpected sourcePath annotation %v", annotations[sourcePathAnnotation])
			}

			spec := resource["spec"].(map[string]any)
			if _, hasID := spec["id"]; hasID {
				t.Error("spec should not contain the dashboard id")
			}
			if tags := spec[tagsKey].([]any); len(tags) != 1 || tags[0] != managedDashboardTag {
				t.Errorf("expected the managed tag, got %v", tags)
			}
			if links := spec[linksKey].([]any); len(links) != 1 {
				t.Errorf("expected the source link, got %v", links)
			}
		})
	}
}
