package grafana

import (
	"testing"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

func TestPrepareForGrafanaAPI(t *testing.T) {
	content := map[string]any{
		"uid":   "test-uid",
		"title": "Test Dashboard",
		"id":    123, // Should be removed
	}

	d := dashboard.New("test-org", content)
	prepared := prepareForGrafanaAPI(d)

	if _, hasID := prepared["id"]; hasID {
		t.Error("Expected 'id' field to be removed from prepared content")
	}

	if prepared["uid"] != "test-uid" {
		t.Error("Expected 'uid' field to be preserved")
	}

	if prepared["title"] != "Test Dashboard" {
		t.Error("Expected 'title' field to be preserved")
	}

	// Ensure original content is not modified (domain object should return copy)
	originalContent := d.Content()
	if _, hasID := originalContent["id"]; !hasID {
		t.Error("Original domain object content should still have 'id' field")
	}
}

func TestPrepareForGrafanaAPIWithoutID(t *testing.T) {
	content := map[string]any{
		"uid":   "test-uid",
		"title": "Test Dashboard",
		// No ID field
	}

	d := dashboard.New("test-org", content)
	prepared := prepareForGrafanaAPI(d)

	if len(prepared) != 2 {
		t.Errorf("Expected 2 fields in prepared content, got %d", len(prepared))
	}

	if prepared["uid"] != "test-uid" {
		t.Error("Expected 'uid' field to be preserved")
	}

	if prepared["title"] != "Test Dashboard" {
		t.Error("Expected 'title' field to be preserved")
	}
}
