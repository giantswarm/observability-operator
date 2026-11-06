package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/metrics"
)

// ConfigureDashboard configures a dashboard, ensuring folder hierarchy exists and injecting managed tag
func (s *Service) ConfigureDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureDashboard).Inc()
		return fmt.Errorf("failed to find organization: %w", err)
	}

	err = s.withinOrganization(ctx, org, func(ctx context.Context, client grafanaclient.GrafanaClient) error {
		// Ensure folder hierarchy exists and get the leaf folder UID
		folderUID, err := s.ensureFolderHierarchy(ctx, client, org.ID(), dashboard.FolderPath())
		if err != nil {
			return fmt.Errorf("failed to ensure folder hierarchy: %w", err)
		}

		dashboardContent := dashboard.Content()
		// removes the "id" field from the content which can cause conflicts during dashboard creation/update
		delete(dashboardContent, "id")

		// Inject the managed tag so operator dashboards are distinguishable
		if err := injectManagedTag(dashboardContent); err != nil {
			return fmt.Errorf("failed to inject managed tag: %w", err)
		}

		version, resource, err := buildDashboardResource(dashboard.UID(), dashboardContent, folderUID, dashboard.SourcePath())
		if err != nil {
			return fmt.Errorf("failed to build dashboard resource: %w", err)
		}

		if err := publishDashboard(ctx, client.DashboardAPI(), version, grafanaclient.OrgNamespace(org.ID()), dashboard.UID(), resource); err != nil {
			return fmt.Errorf("failed to update dashboard: %w", err)
		}

		return nil
	})
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpConfigureDashboard).Inc()
	}
	return err
}

// DeleteDashboard deletes through the legacy API: it still accepts deletes on
// managed dashboards (verified on Grafana 13.2.4).
//
// TODO: move to the app-platform API if Grafana starts enforcing the manager
// identity on legacy deletes. The check is an open TODO in Grafana:
// https://github.com/grafana/grafana/blob/v13.2.4/pkg/storage/unified/apistore/managed.go#L242
func (s *Service) DeleteDashboard(ctx context.Context, dashboard *dashboard.Dashboard) error {
	logger := log.FromContext(ctx)

	org, err := s.FindOrgByName(dashboard.Organization())
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpDeleteDashboard).Inc()
		return fmt.Errorf("failed to find organization: %w", err)
	}

	err = s.withinOrganization(ctx, org, func(ctx context.Context, client grafanaclient.GrafanaClient) error {
		_, err = client.Dashboards().DeleteDashboardByUID(dashboard.UID())
		if err != nil {
			// Return with no error in case the dashboard is already gone in Grafana.
			var notFound *dashboards.DeleteDashboardByUIDNotFound
			if errors.As(err, &notFound) {
				logger.Info("skipping deletion, dashboard not found")
				return nil
			}
			return fmt.Errorf("failed to delete dashboard: %w", err)
		}

		return nil
	})
	if err != nil {
		metrics.GrafanaAPIErrors.WithLabelValues(metrics.OpDeleteDashboard).Inc()
	}
	return err
}

const managedDashboardTag = "managed-by: observability-operator"

// injectManagedTag ensures the managed tag is present in the dashboard content's tags array.
// This is idempotent - if the tag already exists, it does nothing.
//
// The tags live at the top level for the classic flat schema and under "spec" for
// the Grafana Dashboard schema (dashboard.grafana.app/v2).
func injectManagedTag(content map[string]any) error {
	tagsHolder := content
	if dashboard.IsV2(content) {
		spec, ok := content["spec"].(map[string]any)
		if !ok {
			return fmt.Errorf("malformed v2 dashboard: missing spec")
		}
		tagsHolder = spec
	}

	tags, ok := tagsHolder[tagsKey].([]any)
	if !ok {
		tags = []any{}
	}

	for _, tag := range tags {
		if tag == managedDashboardTag {
			return nil
		}
	}

	tagsHolder[tagsKey] = append(tags, managedDashboardTag)
	return nil
}

// Annotations marking a dashboard as managed by the operator in Grafana.
const (
	managedByAnnotation  = "grafana.app/managedBy"
	managerIDAnnotation  = "grafana.app/managerId"
	sourcePathAnnotation = "grafana.app/sourcePath"
	folderAnnotation     = "grafana.app/folder"
	managerKind          = "kubectl"
	managerID            = "observability-operator"
)

// buildDashboardResource builds the app-platform Dashboard resource for the dashboard content.
// It returns the API version to write it with: "v1" for the classic flat schema, "v2" for
// the Grafana Dashboard schema (dashboard.grafana.app/v2).
// folderUID specifies the target folder; empty string means the General folder.
func buildDashboardResource(uid string, content map[string]any, folderUID string, sourcePath string) (string, json.RawMessage, error) {
	annotations := map[string]any{
		managedByAnnotation:  managerKind,
		managerIDAnnotation:  managerID,
		sourcePathAnnotation: sourcePath,
	}
	if folderUID != "" {
		annotations[folderAnnotation] = folderUID
	}

	// Only metadata.name is kept: server metadata from a copied v2 dashboard
	// (resourceVersion, generation, uid, labels, ...) is dropped.
	metadata := map[string]any{
		nameKey:       uid,
		"annotations": annotations,
	}

	version := "v1"
	resource := map[string]any{
		"apiVersion": "dashboard.grafana.app/v1",
		"kind":       "Dashboard",
		"metadata":   metadata,
		"spec":       content,
	}
	if dashboard.IsV2(content) {
		version = "v2"
		resource["apiVersion"] = content["apiVersion"]
		resource["kind"] = content["kind"]
		resource["spec"] = content["spec"]
	}

	body, err := json.Marshal(resource)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal dashboard resource: %w", err)
	}
	return version, body, nil
}

// publishDashboard creates or updates a dashboard through the Grafana app-platform API.
// The update omits resourceVersion so it overwrites the stored dashboard unconditionally.
func publishDashboard(ctx context.Context, client grafanaclient.DashboardAPIClient, version, namespace, uid string, resource json.RawMessage) error {
	_, err := client.Get(ctx, version, namespace, uid)
	if errors.Is(err, grafanaclient.ErrDashboardNotFound) {
		if err := client.Create(ctx, version, namespace, resource); err != nil {
			return fmt.Errorf("failed to create dashboard: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get dashboard: %w", err)
	}

	if err := client.Update(ctx, version, namespace, uid, resource); err != nil {
		return fmt.Errorf("failed to update dashboard: %w", err)
	}
	return nil
}

// withinOrganization runs fn against a Grafana client scoped to org. The cloned
// client is local to this call, so concurrent invocations for different orgs do
// not interfere with each other.
func (s *Service) withinOrganization(
	ctx context.Context,
	org *organization.Organization,
	fn func(ctx context.Context, client grafanaclient.GrafanaClient) error,
) error {
	logger := log.FromContext(ctx).WithValues("organization", org.Name())
	ctx = log.IntoContext(ctx, logger)
	return fn(ctx, s.grafanaClient.WithOrgID(org.ID()))
}
