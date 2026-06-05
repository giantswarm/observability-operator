package grafana

import (
	"testing"
)

func TestInjectManagedTag(t *testing.T) {
	t.Run("adds tag to empty tags", func(t *testing.T) {
		content := map[string]any{
			"uid":   "test-uid",
			"title": "Test Dashboard",
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content["tags"].([]any)
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
			"uid":   "test-uid",
			"title": "Test Dashboard",
			"tags":  []any{"existing-tag", "another-tag"},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content["tags"].([]any)
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
			"uid":   "test-uid",
			"title": "Test Dashboard",
			"tags":  []any{"existing-tag"},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := injectManagedTag(content); err != nil { // Call twice
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 2 {
			t.Fatalf("Expected 2 tags (not duplicated), got %d", len(tags))
		}
	})

	t.Run("handles nil tags field", func(t *testing.T) {
		content := map[string]any{
			"uid":  "test-uid",
			"tags": nil,
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("handles tags of wrong type", func(t *testing.T) {
		content := map[string]any{
			"uid":  "test-uid",
			"tags": "not-an-array",
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("v2 schema adds tag under spec.tags", func(t *testing.T) {
		content := map[string]any{
			"apiVersion": "dashboard.grafana.app/v2",
			"kind":       "Dashboard",
			"metadata":   map[string]any{"name": "gs_cluster-overview"},
			"spec": map[string]any{
				"title": "Cluster Overview",
				"tags":  []any{"existing-tag"},
			},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The managed tag must land under spec.tags, not at the top level.
		if _, topLevel := content["tags"]; topLevel {
			t.Error("v2 dashboard should not get a top-level tags field")
		}
		spec := content["spec"].(map[string]any)
		tags, ok := spec["tags"].([]any)
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
			"apiVersion": "dashboard.grafana.app/v2",
			"metadata":   map[string]any{"name": "gs_cluster-overview"},
			"spec":       map[string]any{"tags": []any{managedDashboardTag}},
		}

		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := injectManagedTag(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spec := content["spec"].(map[string]any)
		tags := spec["tags"].([]any)
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag (not duplicated), got %d", len(tags))
		}
	})

	t.Run("v2 schema with missing spec returns an error", func(t *testing.T) {
		content := map[string]any{
			"apiVersion": "dashboard.grafana.app/v2",
			"metadata":   map[string]any{"name": "gs_cluster-overview"},
		}

		if err := injectManagedTag(content); err == nil {
			t.Fatal("expected an error for a malformed v2 dashboard without a spec")
		}

		if _, ok := content["spec"]; ok {
			t.Error("injectManagedTag should not create a spec on a malformed v2 dashboard")
		}
		if _, ok := content["tags"]; ok {
			t.Error("injectManagedTag should not add top-level tags to a v2 dashboard")
		}
	})
}
