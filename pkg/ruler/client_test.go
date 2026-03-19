package ruler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/ruler"
)

func TestNewNoop(t *testing.T) {
	c := ruler.NewNoop()
	err := c.DeleteAllRulesForTenant(context.Background(), "giantswarm")
	assert.NoError(t, err)
}

func TestNewMimir_DeleteAllRulesForTenant(t *testing.T) {
	tests := []struct {
		name           string
		listStatus     int
		listBody       any
		deleteStatuses map[string]int // namespace → status to return on DELETE
		wantErr        bool
	}{
		{
			name:       "no rules (404 on list)",
			listStatus: http.StatusNotFound,
			wantErr:    false,
		},
		{
			name:       "no namespaces (empty map)",
			listStatus: http.StatusOK,
			listBody:   map[string]any{},
			wantErr:    false,
		},
		{
			name:       "deletes all namespaces",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"ns-a": nil, "ns-b": nil},
			deleteStatuses: map[string]int{
				"ns-a": http.StatusNoContent,
				"ns-b": http.StatusOK,
			},
			wantErr: false,
		},
		{
			name:       "treats 404 on delete as success",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"ns-gone": nil},
			deleteStatuses: map[string]int{
				"ns-gone": http.StatusNotFound,
			},
			wantErr: false,
		},
		{
			name:       "returns error on unexpected delete status",
			listStatus: http.StatusOK,
			listBody:   map[string]any{"ns-fail": nil},
			deleteStatuses: map[string]int{
				"ns-fail": http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name:       "returns error on unexpected list status",
			listStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const tenantID = "giantswarm"

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tenantID, r.Header.Get(monitoring.OrgIDHeader))

				switch r.Method {
				case http.MethodGet:
					w.WriteHeader(tt.listStatus)
					if tt.listBody != nil {
						require.NoError(t, json.NewEncoder(w).Encode(tt.listBody))
					}
				case http.MethodDelete:
					// extract namespace from path: /api/prom/rules/{namespace}
					ns := r.URL.Path[len("/api/prom/rules/"):]
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

			c := ruler.NewMimir(srv.URL)
			err := c.DeleteAllRulesForTenant(context.Background(), tenantID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewLoki_DeleteAllRulesForTenant(t *testing.T) {
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

	c := ruler.NewLoki(srv.URL)
	err := c.DeleteAllRulesForTenant(context.Background(), tenantID)
	assert.NoError(t, err)
}

func TestNewMulti_DeleteAllRulesForTenant(t *testing.T) {
	const tenantID = "giantswarm"

	t.Run("calls all backends", func(t *testing.T) {
		called := map[string]int{}
		clientA := &trackingClient{name: "mimir", calls: called, wantTenant: tenantID}
		clientB := &trackingClient{name: "loki", calls: called, wantTenant: tenantID}

		err := ruler.NewMulti(clientA, clientB).DeleteAllRulesForTenant(context.Background(), tenantID)
		assert.NoError(t, err)
		assert.Equal(t, 1, called["mimir"])
		assert.Equal(t, 1, called["loki"])
	})

	t.Run("calls all backends even when one errors", func(t *testing.T) {
		called := map[string]int{}
		clientA := &trackingClient{name: "mimir", calls: called, wantTenant: tenantID, err: fmt.Errorf("mimir down")}
		clientB := &trackingClient{name: "loki", calls: called, wantTenant: tenantID}

		err := ruler.NewMulti(clientA, clientB).DeleteAllRulesForTenant(context.Background(), tenantID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mimir down")
		assert.Equal(t, 1, called["mimir"], "mimir should have been called")
		assert.Equal(t, 1, called["loki"], "loki should still be called after mimir error")
	})
}

type trackingClient struct {
	name       string
	calls      map[string]int
	wantTenant string
	err        error
}

func (tc *trackingClient) DeleteAllRulesForTenant(_ context.Context, tenantID string) error {
	tc.calls[tc.name]++
	if tc.wantTenant != "" && tenantID != tc.wantTenant {
		return fmt.Errorf("trackingClient %s: got tenantID %q, want %q", tc.name, tenantID, tc.wantTenant)
	}
	return tc.err
}
