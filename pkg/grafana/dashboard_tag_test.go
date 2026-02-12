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

		InjectManagedTag(content)

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
		if tags[0] != ManagedDashboardTag {
			t.Errorf("Expected tag %q, got %q", ManagedDashboardTag, tags[0])
		}
	})

	t.Run("adds tag to existing tags", func(t *testing.T) {
		content := map[string]any{
			"uid":   "test-uid",
			"title": "Test Dashboard",
			"tags":  []any{"existing-tag", "another-tag"},
		}

		InjectManagedTag(content)

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 3 {
			t.Fatalf("Expected 3 tags, got %d", len(tags))
		}
		if tags[2] != ManagedDashboardTag {
			t.Errorf("Expected last tag %q, got %q", ManagedDashboardTag, tags[2])
		}
	})

	t.Run("is idempotent - does not duplicate tag", func(t *testing.T) {
		content := map[string]any{
			"uid":   "test-uid",
			"title": "Test Dashboard",
			"tags":  []any{"existing-tag"},
		}

		InjectManagedTag(content)
		InjectManagedTag(content) // Call twice

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

		InjectManagedTag(content)

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

		InjectManagedTag(content)

		tags, ok := content["tags"].([]any)
		if !ok {
			t.Fatal("tags should be a []any")
		}
		if len(tags) != 1 {
			t.Fatalf("Expected 1 tag, got %d", len(tags))
		}
	})
}
