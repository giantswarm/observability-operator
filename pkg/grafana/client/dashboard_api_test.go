package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	method    string
	path      string
	body      string
	user      string
	password  string
	userAgent string
	orgID     string
}

func newTestDashboardAPIClient(t *testing.T, status int, respBody string) (*dashboardAPIClient, *recordedRequest) {
	t.Helper()

	recorded := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		user, password, _ := r.BasicAuth()
		*recorded = recordedRequest{
			method:    r.Method,
			path:      r.URL.Path,
			body:      string(body),
			user:      user,
			password:  password,
			userAgent: r.UserAgent(),
			orgID:     r.Header.Get(orgIDHeader),
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	c := newDashboardAPIClient(server.Client(), baseURL, url.UserPassword("admin", "secret"), 1)
	return c.withOrgID(5), recorded
}

const org5DashboardPath = "/apis/dashboard.grafana.app/v1/namespaces/org-5/dashboards/abc"

func TestDashboardAPIClient(t *testing.T) {
	dashboard := json.RawMessage(`{"apiVersion":"dashboard.grafana.app/v1","kind":"Dashboard","metadata":{"name":"abc"},"spec":{"title":"Test"}}`)

	tests := []struct {
		name         string
		status       int
		respBody     string
		call         func(c DashboardAPIClient) (json.RawMessage, error)
		wantMethod   string
		wantPath     string
		wantReqBody  string
		wantRespBody string
		wantErr      bool
		wantNotFound bool
	}{
		{
			name:     "get",
			status:   http.StatusOK,
			respBody: string(dashboard),
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return c.Get(context.Background(), "v1", "org-5", "abc")
			},
			wantMethod:   http.MethodGet,
			wantPath:     org5DashboardPath,
			wantRespBody: string(dashboard),
		},
		{
			name:     "get not found",
			status:   http.StatusNotFound,
			respBody: `{"kind":"Status","code":404}`,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return c.Get(context.Background(), "v1", "org-5", "abc")
			},
			wantMethod:   http.MethodGet,
			wantPath:     org5DashboardPath,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:     "get server error",
			status:   http.StatusInternalServerError,
			respBody: `boom`,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return c.Get(context.Background(), "v1", "org-5", "abc")
			},
			wantMethod: http.MethodGet,
			wantPath:   org5DashboardPath,
			wantErr:    true,
		},
		{
			name:   "create",
			status: http.StatusCreated,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return nil, c.Create(context.Background(), "v2", "default", dashboard)
			},
			wantMethod:  http.MethodPost,
			wantPath:    "/apis/dashboard.grafana.app/v2/namespaces/default/dashboards",
			wantReqBody: string(dashboard),
		},
		{
			name:     "create conflict",
			status:   http.StatusConflict,
			respBody: `already exists`,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return nil, c.Create(context.Background(), "v2", "default", dashboard)
			},
			wantMethod:  http.MethodPost,
			wantPath:    "/apis/dashboard.grafana.app/v2/namespaces/default/dashboards",
			wantReqBody: string(dashboard),
			wantErr:     true,
		},
		{
			name:   "update",
			status: http.StatusOK,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return nil, c.Update(context.Background(), "v1", "org-5", "abc", dashboard)
			},
			wantMethod:  http.MethodPut,
			wantPath:    org5DashboardPath,
			wantReqBody: string(dashboard),
		},
		{
			name:     "update forbidden",
			status:   http.StatusForbidden,
			respBody: `Cannot change resource manager kind`,
			call: func(c DashboardAPIClient) (json.RawMessage, error) {
				return nil, c.Update(context.Background(), "v1", "org-5", "abc", dashboard)
			},
			wantMethod:  http.MethodPut,
			wantPath:    org5DashboardPath,
			wantReqBody: string(dashboard),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorded := newTestDashboardAPIClient(t, tt.status, tt.respBody)

			got, err := tt.call(c)

			assert.Equal(t, tt.wantMethod, recorded.method)
			assert.Equal(t, tt.wantPath, recorded.path)
			assert.Equal(t, tt.wantReqBody, recorded.body)
			assert.Equal(t, "admin", recorded.user)
			assert.Equal(t, "secret", recorded.password)
			assert.Equal(t, "observability-operator", recorded.userAgent)
			assert.Equal(t, "5", recorded.orgID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantNotFound, errors.Is(err, ErrDashboardNotFound))
				return
			}
			require.NoError(t, err)
			if tt.wantRespBody != "" {
				assert.JSONEq(t, tt.wantRespBody, string(got))
			}
		})
	}
}

func TestOrgNamespace(t *testing.T) {
	tests := []struct {
		name  string
		orgID int64
		want  string
	}{
		{name: "org 1", orgID: 1, want: "default"},
		{name: "org 5", orgID: 5, want: "org-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, OrgNamespace(tt.orgID))
		})
	}
}
