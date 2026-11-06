package dashboard_test

import (
	"errors"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

const (
	dashboardKind         = "Dashboard"
	apiVersionKey         = "apiVersion"
	dashboardAPIVersionV2 = "dashboard.grafana.app/v2"
	gsClusterOverview     = "gs_cluster-overview"
	keyName               = "key"
	specKey               = "spec"
	kindKey               = "kind"
	metadataKey           = "metadata"
	nameKey               = "name"
	testOrg               = "test-org"
	testTitle             = "Cluster Overview"
	testUID               = "test-uid"
	titleKey              = "title"
	uidKey                = "uid"
	valueKey              = "value"
)

func TestNew(t *testing.T) {
	org := testOrg
	content := map[string]any{
		uidKey:  testUID,
		keyName: valueKey,
	}

	d := dashboard.New(org, "", "", "", content)

	if d.UID() != testUID {
		t.Errorf("Expected UID %s, got %s", testUID, d.UID())
	}
	if d.Organization() != org {
		t.Errorf("Expected organization %s, got %s", org, d.Organization())
	}

	// Test that content is copied, not referenced
	originalContent := d.Content()
	originalContent["new_key"] = "new_value"
	if len(d.Content()) != 2 { // Should still have uid and key, not new_key
		t.Error("Content should be copied, not referenced")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name         string
		organization string
		content      map[string]any
		expectedErrs []error
	}{
		{
			name:         "valid dashboard",
			organization: testOrg,
			content:      map[string]any{uidKey: testUID, keyName: valueKey},
			expectedErrs: nil,
		},
		{
			name:         "valid v2 dashboard",
			organization: testOrg,
			content: map[string]any{
				apiVersionKey: dashboardAPIVersionV2,
				kindKey:       dashboardKind,
				metadataKey:   map[string]any{nameKey: gsClusterOverview},
				specKey:       map[string]any{titleKey: testTitle},
			},
			expectedErrs: nil,
		},
		{
			name:         "valid v2alpha1 dashboard",
			organization: testOrg,
			content: map[string]any{
				apiVersionKey: "dashboard.grafana.app/v2alpha1",
				kindKey:       dashboardKind,
				metadataKey:   map[string]any{nameKey: gsClusterOverview},
				specKey:       map[string]any{titleKey: testTitle},
			},
			expectedErrs: nil,
		},
		{
			name:         "valid v2beta1 dashboard",
			organization: testOrg,
			content: map[string]any{
				apiVersionKey: "dashboard.grafana.app/v2beta1",
				kindKey:       dashboardKind,
				metadataKey:   map[string]any{nameKey: gsClusterOverview},
				specKey:       map[string]any{titleKey: testTitle},
			},
			expectedErrs: nil,
		},
		{
			name:         "valid classic dashboard with empty apiVersion",
			organization: testOrg,
			content:      map[string]any{apiVersionKey: "", uidKey: testUID},
			expectedErrs: nil,
		},
		{
			name:         "unsupported apiVersion",
			organization: testOrg,
			content:      map[string]any{apiVersionKey: "dashboard.grafana.app/v1", uidKey: testUID},
			expectedErrs: []error{dashboard.ErrUnsupportedAPIVersion},
		},
		{
			name:         "garbage apiVersion",
			organization: testOrg,
			content:      map[string]any{apiVersionKey: "not-a-version", uidKey: testUID},
			expectedErrs: []error{dashboard.ErrUnsupportedAPIVersion},
		},
		{
			name:         "non-string apiVersion",
			organization: testOrg,
			content:      map[string]any{apiVersionKey: 2.0, uidKey: testUID},
			expectedErrs: []error{dashboard.ErrUnsupportedAPIVersion},
		},
		{
			name:         "missing UID",
			organization: testOrg,
			content:      map[string]any{keyName: valueKey}, // No UID in content
			expectedErrs: []error{dashboard.ErrMissingUID},
		},
		{
			name:         "missing organization",
			organization: "",
			content:      map[string]any{uidKey: testUID, keyName: valueKey},
			expectedErrs: []error{dashboard.ErrMissingOrganization},
		},
		{
			name:         "invalid JSON (nil content)",
			organization: testOrg,
			content:      nil,
			expectedErrs: []error{dashboard.ErrMissingUID, dashboard.ErrInvalidJSON},
		},
		{
			name:         "multiple errors",
			organization: "",
			content:      nil,
			expectedErrs: []error{dashboard.ErrMissingUID, dashboard.ErrMissingOrganization, dashboard.ErrInvalidJSON},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dashboard.New(tt.organization, "", "", "", tt.content)
			errs := d.Validate()

			if len(errs) != len(tt.expectedErrs) {
				t.Fatalf("Expected %d errors, got %d: %v", len(tt.expectedErrs), len(errs), errs)
			}

			for i, expectedErr := range tt.expectedErrs {
				if !errors.Is(errs[i], expectedErr) {
					t.Errorf("Expected error %v, got %v", expectedErr, errs[i])
				}
			}
		})
	}
}

func TestUnsupportedAPIVersionMessage(t *testing.T) {
	d := dashboard.New(testOrg, "", "", "", map[string]any{apiVersionKey: "dashboard.grafana.app/v3", uidKey: testUID})
	errs := d.Validate()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}

	want := `unsupported dashboard apiVersion "dashboard.grafana.app/v3": accepted values are dashboard.grafana.app/v2, dashboard.grafana.app/v2beta1, dashboard.grafana.app/v2alpha1, or no apiVersion for a classic dashboard`
	if errs[0].Error() != want {
		t.Errorf("Expected error %q, got %q", want, errs[0].Error())
	}
}

func TestIsV2(t *testing.T) {
	tests := []struct {
		name        string
		content     map[string]any
		wantV2      bool
		wantVersion string
	}{
		{name: "v2", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v2"}, wantV2: true, wantVersion: "v2"},
		{name: "v2alpha1", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v2alpha1"}, wantV2: true, wantVersion: "v2alpha1"},
		{name: "v2beta1", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v2beta1"}, wantV2: true, wantVersion: "v2beta1"},
		{name: "absent apiVersion", content: map[string]any{uidKey: testUID}},
		{name: "nil content", content: nil},
		{name: "v1", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v1"}},
		{name: "v20", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v20"}},
		{name: "v2 with unknown suffix", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v2gamma1"}},
		{name: "v2alpha without number", content: map[string]any{apiVersionKey: "dashboard.grafana.app/v2alpha"}},
		{name: "other group", content: map[string]any{apiVersionKey: "example.com/v2"}},
		{name: "garbage", content: map[string]any{apiVersionKey: "not-a-version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashboard.IsV2(tt.content); got != tt.wantV2 {
				t.Errorf("Expected IsV2 %v, got %v", tt.wantV2, got)
			}
			if got := dashboard.V2Version(tt.content); got != tt.wantVersion {
				t.Errorf("Expected V2Version %q, got %q", tt.wantVersion, got)
			}
		})
	}
}

func TestString(t *testing.T) {
	content := map[string]any{
		uidKey:  testUID,
		keyName: valueKey,
	}
	d := dashboard.New(testOrg, "", "", "", content)
	expected := "Dashboard{uid: test-uid, organization: test-org}"
	if d.String() != expected {
		t.Errorf("Expected String() to return %s, got %s", expected, d.String())
	}
}

func TestUIDExtraction(t *testing.T) {
	tests := []struct {
		name        string
		content     map[string]any
		expectedUID string
	}{
		{
			name:        "valid UID",
			content:     map[string]any{uidKey: testUID, titleKey: dashboardKind},
			expectedUID: testUID,
		},
		{
			name:        "missing UID",
			content:     map[string]any{titleKey: dashboardKind},
			expectedUID: "",
		},
		{
			name:        "nil content",
			content:     nil,
			expectedUID: "",
		},
		{
			name:        "UID is not a string",
			content:     map[string]any{uidKey: 123, titleKey: dashboardKind},
			expectedUID: "",
		},
		{
			name:        "empty UID string",
			content:     map[string]any{uidKey: "", titleKey: dashboardKind},
			expectedUID: "",
		},
		{
			name: "v2 schema reads UID from metadata.name",
			content: map[string]any{
				apiVersionKey: dashboardAPIVersionV2,
				kindKey:       dashboardKind,
				metadataKey:   map[string]any{nameKey: gsClusterOverview},
				specKey:       map[string]any{titleKey: testTitle},
			},
			expectedUID: gsClusterOverview,
		},
		{
			name: "v2 schema ignores top-level uid",
			content: map[string]any{
				apiVersionKey: dashboardAPIVersionV2,
				uidKey:        "should-be-ignored",
				metadataKey:   map[string]any{nameKey: gsClusterOverview},
			},
			expectedUID: gsClusterOverview,
		},
		{
			name: "v2 schema missing metadata.name",
			content: map[string]any{
				apiVersionKey: dashboardAPIVersionV2,
				metadataKey:   map[string]any{},
			},
			expectedUID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dashboard.New(testOrg, "", "", "", tt.content)
			if d.UID() != tt.expectedUID {
				t.Errorf("Expected UID %s, got %s", tt.expectedUID, d.UID())
			}
		})
	}
}

func TestFolderPath(t *testing.T) {
	t.Run("empty folder path", func(t *testing.T) {
		d := dashboard.New(testOrg, "", "", "", map[string]any{uidKey: testUID})
		if d.FolderPath() != "" {
			t.Errorf("Expected empty folder path, got %q", d.FolderPath())
		}
	})

	t.Run("folder path is stored and returned", func(t *testing.T) {
		d := dashboard.New(testOrg, "team-a/networking", "", "", map[string]any{uidKey: testUID})
		if d.FolderPath() != "team-a/networking" {
			t.Errorf("Expected folder path %q, got %q", "team-a/networking", d.FolderPath())
		}
	})

	t.Run("invalid folder path fails validation", func(t *testing.T) {
		d := dashboard.New(testOrg, "/leading-slash", "", "", map[string]any{uidKey: testUID})
		errs := d.Validate()
		if len(errs) == 0 {
			t.Error("Expected validation error for invalid folder path")
		}
	})

	t.Run("valid folder path passes validation", func(t *testing.T) {
		d := dashboard.New(testOrg, "team-a/networking/alerts", "", "", map[string]any{uidKey: testUID})
		errs := d.Validate()
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got %v", errs)
		}
	})
}

func TestSourcePath(t *testing.T) {
	tests := []struct {
		name            string
		sourceNamespace string
		sourceName      string
		expected        string
	}{
		{
			name:            "namespace and name",
			sourceNamespace: "monitoring",
			sourceName:      "cluster-overview",
			expected:        "configmap monitoring/cluster-overview",
		},
		{
			name:            "empty source",
			sourceNamespace: "",
			sourceName:      "",
			expected:        "configmap /",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dashboard.New(testOrg, "", tt.sourceNamespace, tt.sourceName, map[string]any{uidKey: testUID})
			if d.SourcePath() != tt.expected {
				t.Errorf("Expected source path %q, got %q", tt.expected, d.SourcePath())
			}
		})
	}
}
