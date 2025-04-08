package predicates

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIsGrafanaPod(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "non-Grafana pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-grafana-pod",
					Namespace: "default",
				},
			},
			expected: false,
		},
		{
			name: "Grafana pod with correct labels",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
			},
			expected: true,
		},
		{
			name: "Grafana pod with incorrect namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
			},
			expected: false,
		},
		{
			name: "Grafana pod with incorrect label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "not-grafana",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGrafanaPod(tt.pod)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGrafanaPodRecreatedPredicate_Update(t *testing.T) {
	tests := []struct {
		name     string
		oldPod   *corev1.Pod
		newPod   *corev1.Pod
		expected bool
	}{
		{
			name:     "nil old object",
			oldPod:   nil,
			newPod:   &corev1.Pod{},
			expected: false,
		},
		{
			name:     "nil new object",
			oldPod:   &corev1.Pod{},
			newPod:   nil,
			expected: false,
		},
		{
			name: "non-Grafana pod",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-grafana-pod",
					Namespace: "default",
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-grafana-pod",
					Namespace: "default",
				},
			},
			expected: false,
		},
		{
			name: "Grafana pod not ready to ready",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Grafana pod ready to not ready",
			oldPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			newPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grafana-pod",
					Namespace: "monitoring",
					Labels: map[string]string{
						"app.kubernetes.io/instance": "grafana",
					},
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
	}

	predicate := GrafanaPodRecreatedPredicate{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := event.UpdateEvent{
				ObjectOld: tt.oldPod,
				ObjectNew: tt.newPod,
			}
			result := predicate.Update(e)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
