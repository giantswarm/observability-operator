package predicates

import (
	"testing"

	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// MockObject implements client.Object but is not an App
type MockObject struct {
	metav1.ObjectMeta
}

func (m *MockObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *MockObject) DeepCopyObject() runtime.Object {
	return &MockObject{
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
	}
}

func TestAlloyRulesAppChangedPredicate_Update(t *testing.T) {
	// Create an App with a given name and config
	createApp := func(name string, config *appv1alpha1.AppSpecConfig) *appv1alpha1.App {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if config != nil {
			app.Spec.Config = *config
		}
		return app
	}

	// Test cases
	tests := []struct {
		name     string
		e        event.UpdateEvent
		expected bool
	}{
		{
			name: "old object is nil",
			e: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: createApp("alloy-rules", nil),
			},
			expected: false,
		},
		{
			name: "new object is nil",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", nil),
				ObjectNew: nil,
			},
			expected: false,
		},
		{
			name: "old object name doesn't contain alloy-rules",
			e: event.UpdateEvent{
				ObjectOld: createApp("other-app", nil),
				ObjectNew: createApp("alloy-rules", nil),
			},
			expected: false,
		},
		{
			name: "new object name doesn't contain alloy-rules",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", nil),
				ObjectNew: createApp("other-app", nil),
			},
			expected: false,
		},
		{
			name: "old object is not an App",
			e: event.UpdateEvent{
				ObjectOld: &MockObject{ObjectMeta: metav1.ObjectMeta{Name: "alloy-rules"}},
				ObjectNew: createApp("alloy-rules", nil),
			},
			expected: false,
		},
		{
			name: "new object is not an App",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", nil),
				ObjectNew: &MockObject{ObjectMeta: metav1.ObjectMeta{Name: "alloy-rules"}},
			},
			expected: false,
		},
		{
			name: "both objects are Apps with the same Config",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", &appv1alpha1.AppSpecConfig{
					ConfigMap: appv1alpha1.AppSpecConfigConfigMap{
						Name:      "config-map",
						Namespace: "default",
					},
				}),
				ObjectNew: createApp("alloy-rules", &appv1alpha1.AppSpecConfig{
					ConfigMap: appv1alpha1.AppSpecConfigConfigMap{
						Name:      "config-map",
						Namespace: "default",
					},
				}),
			},
			expected: false,
		},
		{
			name: "both objects are Apps with different Config",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", &appv1alpha1.AppSpecConfig{
					ConfigMap: appv1alpha1.AppSpecConfigConfigMap{
						Name:      "config-map-old",
						Namespace: "default",
					},
				}),
				ObjectNew: createApp("alloy-rules", &appv1alpha1.AppSpecConfig{
					ConfigMap: appv1alpha1.AppSpecConfigConfigMap{
						Name:      "config-map-new",
						Namespace: "default",
					},
				}),
			},
			expected: true,
		},
		{
			name: "new object has no config",
			e: event.UpdateEvent{
				ObjectOld: createApp("alloy-rules", &appv1alpha1.AppSpecConfig{
					ConfigMap: appv1alpha1.AppSpecConfigConfigMap{
						Name:      "config-map-old",
						Namespace: "default",
					},
				}),
				ObjectNew: createApp("alloy-rules", nil),
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			predicate := AlloyRulesAppChangedPredicate{}
			got := predicate.Update(tc.e)
			if got != tc.expected {
				t.Errorf("Update() = %v, want %v", got, tc.expected)
			}
		})
	}
}
