package dashboard_test

import (
	"errors"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

func TestNew(t *testing.T) {
	org := "test-org"
	content := map[string]any{
		"uid": "test-uid",
		"key": "value",
	}

	d := dashboard.New(org, "", content)

	if d.UID() != "test-uid" {
		t.Errorf("Expected UID %s, got %s", "test-uid", d.UID())
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
			organization: "test-org",
			content:      map[string]any{"uid": "test-uid", "key": "value"},
			expectedErrs: nil,
		},
		{
			name:         "missing UID",
			organization: "test-org",
			content:      map[string]any{"key": "value"}, // No UID in content
			expectedErrs: []error{dashboard.ErrMissingUID},
		},
		{
			name:         "missing organization",
			organization: "",
			content:      map[string]any{"uid": "test-uid", "key": "value"},
			expectedErrs: []error{dashboard.ErrMissingOrganization},
		},
		{
			name:         "invalid JSON (nil content)",
			organization: "test-org",
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
			d := dashboard.New(tt.organization, "", tt.content)
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

func TestString(t *testing.T) {
	content := map[string]any{
		"uid": "test-uid",
		"key": "value",
	}
	d := dashboard.New("test-org", "", content)
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
			content:     map[string]any{"uid": "test-uid", "title": "Dashboard"},
			expectedUID: "test-uid",
		},
		{
			name:        "missing UID",
			content:     map[string]any{"title": "Dashboard"},
			expectedUID: "",
		},
		{
			name:        "nil content",
			content:     nil,
			expectedUID: "",
		},
		{
			name:        "UID is not a string",
			content:     map[string]any{"uid": 123, "title": "Dashboard"},
			expectedUID: "",
		},
		{
			name:        "empty UID string",
			content:     map[string]any{"uid": "", "title": "Dashboard"},
			expectedUID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dashboard.New("test-org", "", tt.content)
			if d.UID() != tt.expectedUID {
				t.Errorf("Expected UID %s, got %s", tt.expectedUID, d.UID())
			}
		})
	}
}

func TestFolderPath(t *testing.T) {
	t.Run("empty folder path", func(t *testing.T) {
		d := dashboard.New("test-org", "", map[string]any{"uid": "test-uid"})
		if d.FolderPath() != "" {
			t.Errorf("Expected empty folder path, got %q", d.FolderPath())
		}
	})

	t.Run("folder path is stored and returned", func(t *testing.T) {
		d := dashboard.New("test-org", "team-a/networking", map[string]any{"uid": "test-uid"})
		if d.FolderPath() != "team-a/networking" {
			t.Errorf("Expected folder path %q, got %q", "team-a/networking", d.FolderPath())
		}
	})

	t.Run("invalid folder path fails validation", func(t *testing.T) {
		d := dashboard.New("test-org", "/leading-slash", map[string]any{"uid": "test-uid"})
		errs := d.Validate()
		if len(errs) == 0 {
			t.Error("Expected validation error for invalid folder path")
		}
	})

	t.Run("valid folder path passes validation", func(t *testing.T) {
		d := dashboard.New("test-org", "team-a/networking/alerts", map[string]any{"uid": "test-uid"})
		errs := d.Validate()
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got %v", errs)
		}
	})
}
