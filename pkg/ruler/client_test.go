package ruler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/ruler"
)

func TestNewNoop(t *testing.T) {
	c := ruler.NewNoop()
	err := c.DeleteClusterRulesForTenant(context.Background(), "giantswarm", "my-cluster")
	assert.NoError(t, err)
}

func TestNewMimir_DeleteClusterRulesForTenant(t *testing.T) {
	tests := []struct {
		name           string
		clusterID      string
		listStatus     int
		listBody       any
		listBodyRaw    []byte         // overrides listBody when set; written verbatim
		deleteStatuses map[string]int // namespace → status to return on DELETE
		wantErr        bool
	}{
		{
			name:       "no rules (404 on list)",
			clusterID:  "my-cluster",
			listStatus: http.StatusNotFound,
			wantErr:    false,
		},
		{
			name:       "no namespaces (empty map)",
			clusterID:  "my-cluster",
			listStatus: http.StatusOK,
			listBody:   map[string]any{},
			wantErr:    false,
		},
		{
			name:       "deletes only namespaces matching cluster prefix",
			clusterID:  "my-cluster",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"my-cluster/rules": nil, "my-cluster/alerts": nil, "other-cluster/rules": nil},
			deleteStatuses: map[string]int{
				"my-cluster/rules":  http.StatusNoContent,
				"my-cluster/alerts": http.StatusOK,
			},
			wantErr: false,
		},
		{
			name:       "skips all namespaces when none match prefix",
			clusterID:  "my-cluster",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"other-cluster/rules": nil},
			wantErr:    false,
		},
		{
			name:       "treats 404 on delete as success",
			clusterID:  "my-cluster",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"my-cluster/gone": nil},
			deleteStatuses: map[string]int{
				"my-cluster/gone": http.StatusNotFound,
			},
			wantErr: false,
		},
		{
			name:       "returns error on unexpected delete status",
			clusterID:  "my-cluster",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"my-cluster/fail": nil},
			deleteStatuses: map[string]int{
				"my-cluster/fail": http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name:       "returns error on unexpected list status",
			clusterID:  "my-cluster",
			listStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:        "handles YAML response from Mimir (no matching namespaces)",
			clusterID:   "my-cluster",
			listStatus:  http.StatusOK,
			listBodyRaw: []byte("groups:\n- name: test\n"),
			wantErr:     false,
		},
		{
			// Real Mimir response: top-level keys are namespace names in YAML format.
			name:      "handles real Mimir YAML response with matching namespace",
			clusterID: "my-cluster",
			listStatus: http.StatusOK,
			listBodyRaw: []byte("my-cluster/rules:\n- name: group1\n  rules: []\nother-cluster/rules:\n- name: group2\n  rules: []\n"),
			deleteStatuses: map[string]int{
				"my-cluster/rules": http.StatusNoContent,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const tenantID = "giantswarm"

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tenantID, r.Header.Get(monitoring.OrgIDHeader))

				switch r.Method {
				case http.MethodGet:
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					w.WriteHeader(tt.listStatus)
					if tt.listBodyRaw != nil {
						_, _ = w.Write(tt.listBodyRaw)
					} else if tt.listBody != nil {
						b, err := yaml.Marshal(tt.listBody)
						require.NoError(t, err)
						_, _ = w.Write(b)
					}
				case http.MethodDelete:
					// extract namespace from path: /prometheus/config/v1/rules/{namespace}
					ns := r.URL.Path[len("/prometheus/config/v1/rules/"):]
					status, ok := tt.deleteStatuses[ns]
					if !ok {
						t.Errorf("unexpected DELETE for namespace %q", ns)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(status)
				default:
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer srv.Close()

			c := ruler.NewMimir(srv.URL, 30*time.Second)
			err := c.DeleteClusterRulesForTenant(context.Background(), tenantID, tt.clusterID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewLoki_DeleteClusterRulesForTenant(t *testing.T) {
	const tenantID = "giantswarm"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, tenantID, r.Header.Get(monitoring.OrgIDHeader))
		switch r.Method {
		case http.MethodGet:
			assert.Equal(t, "/loki/api/v1/rules", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	c := ruler.NewLoki(srv.URL, 30*time.Second)
	err := c.DeleteClusterRulesForTenant(context.Background(), tenantID, "my-cluster")
	assert.NoError(t, err)
}

func TestNewMulti_DeleteClusterRulesForTenant(t *testing.T) {
	const tenantID = "giantswarm"
	const clusterID = "my-cluster"

	t.Run("calls all backends", func(t *testing.T) {
		called := map[string]int{}
		clientA := &trackingClient{name: "mimir", calls: called, wantTenant: tenantID, wantCluster: clusterID}
		clientB := &trackingClient{name: "loki", calls: called, wantTenant: tenantID, wantCluster: clusterID}

		err := ruler.NewMulti(clientA, clientB).DeleteClusterRulesForTenant(context.Background(), tenantID, clusterID)
		assert.NoError(t, err)
		assert.Equal(t, 1, called["mimir"])
		assert.Equal(t, 1, called["loki"])
	})

	t.Run("calls all backends even when one errors", func(t *testing.T) {
		called := map[string]int{}
		clientA := &trackingClient{name: "mimir", calls: called, wantTenant: tenantID, wantCluster: clusterID, err: fmt.Errorf("mimir down")}
		clientB := &trackingClient{name: "loki", calls: called, wantTenant: tenantID, wantCluster: clusterID}

		err := ruler.NewMulti(clientA, clientB).DeleteClusterRulesForTenant(context.Background(), tenantID, clusterID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mimir down")
		assert.Equal(t, 1, called["mimir"], "mimir should have been called")
		assert.Equal(t, 1, called["loki"], "loki should still be called after mimir error")
	})
}

type trackingClient struct {
	name        string
	calls       map[string]int
	wantTenant  string
	wantCluster string
	err         error
}

func (tc *trackingClient) DeleteClusterRulesForTenant(_ context.Context, tenantID, clusterID string) error {
	tc.calls[tc.name]++
	if tc.wantTenant != "" && tenantID != tc.wantTenant {
		return fmt.Errorf("trackingClient %s: got tenantID %q, want %q", tc.name, tenantID, tc.wantTenant)
	}
	if tc.wantCluster != "" && clusterID != tc.wantCluster {
		return fmt.Errorf("trackingClient %s: got clusterID %q, want %q", tc.name, clusterID, tc.wantCluster)
	}
	return tc.err
}
