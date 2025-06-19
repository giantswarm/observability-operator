package dashboard_test

import (
	"errors"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

func TestNew(t *testing.T) {
	uid := "test-uid"
	org := "test-org"
	content := map[string]any{"key": "value"}

	d := dashboard.New(uid, org, content)

	if d.UID() != uid {
		t.Errorf("Expected UID %s, got %s", uid, d.UID())
	}
	if d.Organization() != org {
		t.Errorf("Expected organization %s, got %s", org, d.Organization())
	}

	// Test that content is copied, not referenced
	originalContent := d.Content()
	originalContent["new_key"] = "new_value"
	if len(d.Content()) != 1 {
		t.Error("Content should be copied, not referenced")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name         string
		uid          string
		organization string
		content      map[string]any
		expectedErrs []error
	}{
		{
			name:         "valid dashboard",
			uid:          "test-uid",
			organization: "test-org",
			content:      map[string]any{"key": "value"},
			expectedErrs: nil,
		},
		{
			name:         "missing UID",
			uid:          "",
			organization: "test-org",
			content:      map[string]any{"key": "value"},
			expectedErrs: []error{dashboard.ErrMissingUID},
		},
		{
			name:         "missing organization",
			uid:          "test-uid",
			organization: "",
			content:      map[string]any{"key": "value"},
			expectedErrs: []error{dashboard.ErrMissingOrganization},
		},
		{
			name:         "invalid JSON (nil content)",
			uid:          "test-uid",
			organization: "test-org",
			content:      nil,
			expectedErrs: []error{dashboard.ErrInvalidJSON},
		},
		{
			name:         "multiple errors",
			uid:          "",
			organization: "",
			content:      nil,
			expectedErrs: []error{dashboard.ErrMissingUID, dashboard.ErrMissingOrganization, dashboard.ErrInvalidJSON},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dashboard.New(tt.uid, tt.organization, tt.content)
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
	d := dashboard.New("test-uid", "test-org", map[string]any{"key": "value"})
	expected := "Dashboard{uid: test-uid, organization: test-org}"
	if d.String() != expected {
		t.Errorf("Expected String() to return %s, got %s", expected, d.String())
	}
}
