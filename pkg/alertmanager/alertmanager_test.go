package alertmanager

import (
	"testing"

	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/assert"
)

func TestCountRoutes(t *testing.T) {
	tests := []struct {
		name     string
		route    *config.Route
		expected int
	}{
		{
			name:     "nil route",
			route:    nil,
			expected: 0,
		},
		{
			name: "single root route",
			route: &config.Route{
				Receiver: "default",
			},
			expected: 1,
		},
		{
			name: "route with one sub-route",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
					},
				},
			},
			expected: 2,
		},
		{
			name: "route with multiple sub-routes",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
					},
					{
						Receiver: "sub2",
					},
					{
						Receiver: "sub3",
					},
				},
			},
			expected: 4,
		},
		{
			name: "nested routes",
			route: &config.Route{
				Receiver: "default",
				Routes: []*config.Route{
					{
						Receiver: "sub1",
						Routes: []*config.Route{
							{
								Receiver: "nested1",
							},
							{
								Receiver: "nested2",
							},
						},
					},
					{
						Receiver: "sub2",
					},
				},
			},
			expected: 5, // root + sub1 + nested1 + nested2 + sub2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countRoutes(tt.route)
			assert.Equal(t, tt.expected, result)
		})
	}
}
