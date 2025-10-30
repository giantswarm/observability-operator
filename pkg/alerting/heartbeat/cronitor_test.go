package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/config"
)

// mockHTTPClient is a mock implementation of HTTPClient for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestNewCronitorHeartbeatRepository(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-management-key",
			CronitorHeartbeatPingKey:       "test-ping-key",
		},
	}

	t.Run("with nil http client", func(t *testing.T) {
		repo, err := NewCronitorHeartbeatRepository(cfg, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo == nil {
			t.Fatal("expected repository, got nil")
		}
	})

	t.Run("with custom http client", func(t *testing.T) {
		mockClient := &mockHTTPClient{}
		repo, err := NewCronitorHeartbeatRepository(cfg, mockClient)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo == nil {
			t.Fatal("expected repository, got nil")
		}

		cronitorRepo := repo.(*CronitorHeartbeatRepository)
		if cronitorRepo.httpClient != mockClient {
			t.Error("expected custom http client to be used")
		}
	})
}

func TestMakeMonitor(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-key",
		},
	}

	repo := &CronitorHeartbeatRepository{
		Config:     cfg,
		httpClient: &mockHTTPClient{},
	}

	monitor := repo.makeMonitor()

	if monitor.Type != "heartbeat" {
		t.Errorf("expected type %q, got %s", "heartbeat", monitor.Type)
	}
	expectedKey := "mimir-test-cluster"
	if monitor.Key != expectedKey {
		t.Errorf("expected key %q, got %s", expectedKey, monitor.Key)
	}
	if monitor.GraceSeconds != 1800 {
		t.Errorf("expected grace_seconds %d, got %d", 1800, monitor.GraceSeconds)
	}
	if monitor.Schedule != "every 1 hour" {
		t.Errorf("expected schedule %q, got %s", "every 1 hour", monitor.Schedule)
	}
	if len(monitor.Environments) != 1 || monitor.Environments[0] != "testing" {
		t.Errorf("expected environments ['testing'], got %v", monitor.Environments)
	}
	if len(monitor.Tags) != 4 {
		t.Errorf("expected 4 tags, got %d", len(monitor.Tags))
	}
}

func TestCreateOrUpdate_CreateNew(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-management-key",
			CronitorHeartbeatPingKey:       "test-ping-key",
		},
	}

	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			callCount++

			// First call: GET to check if monitor exists (returns 404)
			if req.Method == http.MethodGet && callCount == 1 {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}

			// Second call: PUT to create monitor
			if req.Method == http.MethodPut && callCount == 2 {
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}

			// Third call: GET to ping monitor
			if req.Method == http.MethodGet && callCount == 3 {
				if req.URL.Host != "cronitor.link" {
					t.Errorf("expected ping to cronitor.link, got %s", req.URL.Host)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}

			t.Fatalf("unexpected call %d: %s %s", callCount, req.Method, req.URL)
			return nil, nil
		},
	}

	repo := &CronitorHeartbeatRepository{
		Config:     cfg,
		httpClient: mockClient,
	}

	err := repo.CreateOrUpdate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 HTTP calls (GET, PUT, PING), got %d", callCount)
	}
}

func TestCreateOrUpdate_UpdateExisting(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-management-key",
			CronitorHeartbeatPingKey:       "test-ping-key",
		},
	}

	existingMonitor := &CronitorMonitor{
		Type:         "heartbeat",
		Key:          "mimir-test-cluster",
		Name:         "mimir-test-cluster",
		GraceSeconds: 900, // Different from desired 1800
		Schedule:     "every 1 hour",
		Tags:         []string{"team:atlas"},
		Environments: []string{"testing"},
	}

	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			callCount++

			// First call: GET to check if monitor exists
			if req.Method == http.MethodGet && callCount == 1 {
				body, _ := json.Marshal(existingMonitor)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}

			// Second call: PUT to update monitor
			if req.Method == http.MethodPut && callCount == 2 {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}

			// No ping call expected for updates

			t.Fatalf("unexpected call %d: %s %s", callCount, req.Method, req.URL)
			return nil, nil
		},
	}

	repo := &CronitorHeartbeatRepository{
		Config:     cfg,
		httpClient: mockClient,
	}

	err := repo.CreateOrUpdate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (GET, PUT), got %d", callCount)
	}
}

func TestCreateOrUpdate_NoChangeNeeded(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-management-key",
		},
	}

	// Create a monitor that matches what makeMonitor() would create
	repo := &CronitorHeartbeatRepository{
		Config:     cfg,
		httpClient: &mockHTTPClient{},
	}
	desiredMonitor := repo.makeMonitor()

	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			callCount++

			// Only GET call expected
			if req.Method == http.MethodGet && callCount == 1 {
				body, _ := json.Marshal(desiredMonitor)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}

			t.Fatalf("unexpected call %d: %s %s", callCount, req.Method, req.URL)
			return nil, nil
		},
	}

	repo.httpClient = mockClient

	err := repo.CreateOrUpdate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (GET only), got %d", callCount)
	}
}

func TestDelete(t *testing.T) {
	cfg := config.Config{
		Cluster: config.ClusterConfig{
			Name:     "test-cluster",
			Pipeline: "testing",
		},
		Environment: config.EnvironmentConfig{
			CronitorHeartbeatManagementKey: "test-management-key",
		},
	}

	t.Run("successful deletion", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodDelete {
					t.Errorf("expected DELETE request, got %s", req.Method)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			},
		}

		repo := &CronitorHeartbeatRepository{
			Config:     cfg,
			httpClient: mockClient,
		}

		err := repo.Delete(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("monitor not found", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			},
		}

		repo := &CronitorHeartbeatRepository{
			Config:     cfg,
			httpClient: mockClient,
		}

		err := repo.Delete(context.Background())
		if err != nil {
			t.Fatalf("expected no error for 404, got %v", err)
		}
	})
}

func TestHasChanged(t *testing.T) {
	repo := &CronitorHeartbeatRepository{}

	tests := []struct {
		name     string
		existing *CronitorMonitor
		desired  *CronitorMonitor
		expected bool
	}{
		{
			name: "no changes",
			existing: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1", "tag2"},
				Environments: []string{"production"},
				Note:         "test note",
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1", "tag2"},
				Environments: []string{"production"},
				Note:         "test note",
			},
			expected: false,
		},
		{
			name: "grace seconds changed",
			existing: &CronitorMonitor{
				GraceSeconds: 900,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			expected: true,
		},
		{
			name: "schedule changed",
			existing: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 30 minutes",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			expected: true,
		},
		{
			name: "tags changed",
			existing: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1", "tag2"},
				Environments: []string{"production"},
			},
			expected: true,
		},
		{
			name: "environments changed",
			existing: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"testing"},
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
			},
			expected: true,
		},
		{
			name: "note changed",
			existing: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
				Note:         "old note",
			},
			desired: &CronitorMonitor{
				GraceSeconds: 1800,
				Schedule:     "every 1 hour",
				Tags:         []string{"tag1"},
				Environments: []string{"production"},
				Note:         "new note",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repo.hasChanged(tt.existing, tt.desired)
			if result != tt.expected {
				t.Errorf("expected hasChanged to return %v, got %v", tt.expected, result)
			}
		})
	}
}
