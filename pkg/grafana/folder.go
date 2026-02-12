package grafana

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/models"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/internal/mapper"
	folder "github.com/giantswarm/observability-operator/pkg/domain/folder"
)

// EnsureFolderHierarchy ensures that the full folder hierarchy exists for the given path.
// Returns the leaf folder UID, or empty string if path is empty (General folder).
func (s *Service) EnsureFolderHierarchy(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	logger := log.FromContext(ctx)

	segments := folder.ParsePath(path)
	var leafUID string

	for _, seg := range segments {
		leafUID = seg.UID()

		// Check if folder already exists
		existing, err := s.grafanaClient.Folders().GetFolderByUID(seg.UID())
		if err == nil {
			// Folder exists - check if title needs updating (rename)
			if existing.Payload.Title != seg.Title() {
				logger.Info("renaming folder", "uid", seg.UID(), "oldTitle", existing.Payload.Title, "newTitle", seg.Title())
				_, err = s.grafanaClient.Folders().UpdateFolder(seg.UID(), &models.UpdateFolderCommand{
					Title:     seg.Title(),
					Overwrite: true,
				})
				if err != nil {
					return "", fmt.Errorf("failed to update folder %q: %w", seg.FullPath(), err)
				}
			}
			continue
		}

		// If not a 404, it's a real error
		if !isNotFound(err) {
			return "", fmt.Errorf("failed to get folder %q: %w", seg.FullPath(), err)
		}

		// Folder doesn't exist - create it
		logger.Info("creating folder", "uid", seg.UID(), "title", seg.Title(), "parentUID", seg.ParentUID())
		_, err = s.grafanaClient.Folders().CreateFolder(&models.CreateFolderCommand{
			UID:         seg.UID(),
			Title:       seg.Title(),
			ParentUID:   seg.ParentUID(),
			Description: folder.Description,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create folder %q: %w", seg.FullPath(), err)
		}
	}

	return leafUID, nil
}

// CleanupOrphanedFoldersForOrg switches to the given organization context and cleans up orphaned folders.
func (s *Service) CleanupOrphanedFoldersForOrg(ctx context.Context, orgName string) error {
	logger := log.FromContext(ctx)

	org, err := s.FindOrgByName(orgName)
	if err != nil {
		logger.Error(err, "failed to find organization for folder cleanup, skipping", "organization", orgName)
		return nil // Don't fail the reconcile for cleanup issues
	}

	currentOrgID := s.grafanaClient.OrgID()
	s.grafanaClient.WithOrgID(org.ID())
	defer s.grafanaClient.WithOrgID(currentOrgID)

	return s.cleanupOrphanedFolders(ctx)
}

// cleanupOrphanedFolders removes operator-managed folders that are no longer referenced by any dashboard.
func (s *Service) cleanupOrphanedFolders(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// Collect all folder UIDs currently needed by dashboard ConfigMaps
	requiredUIDs, err := s.collectRequiredFolderUIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect required folder UIDs: %w", err)
	}

	// List all folders in the current org
	allFolders, err := s.grafanaClient.Folders().GetFolders(folders.NewGetFoldersParams())
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	// Delete operator-managed folders that are empty and unreferenced
	for _, f := range allFolders.Payload {
		if !folder.IsOperatorManaged(f.UID) {
			continue
		}

		if requiredUIDs[f.UID] {
			continue
		}

		// Check if folder is empty before deleting
		counts, err := s.grafanaClient.Folders().GetFolderDescendantCounts(f.UID)
		if err != nil {
			logger.Error(err, "failed to get descendant counts for folder", "uid", f.UID)
			continue
		}

		isEmpty := true
		for _, count := range counts.Payload {
			if count > 0 {
				isEmpty = false
				break
			}
		}

		if !isEmpty {
			continue
		}

		logger.Info("deleting orphaned folder", "uid", f.UID, "title", f.Title)
		_, err = s.grafanaClient.Folders().DeleteFolder(folders.NewDeleteFolderParams().WithFolderUID(f.UID))
		if err != nil {
			logger.Error(err, "failed to delete orphaned folder", "uid", f.UID)
		}
	}

	return nil
}

// collectRequiredFolderUIDs lists all dashboard ConfigMaps and computes the set of folder UIDs they reference.
func (s *Service) collectRequiredFolderUIDs(ctx context.Context) (map[string]bool, error) {
	var configMaps v1.ConfigMapList
	err := s.client.List(ctx, &configMaps, client.MatchingLabels{
		"app.giantswarm.io/kind": "dashboard",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list dashboard configmaps: %w", err)
	}

	requiredUIDs := make(map[string]bool)
	m := mapper.New()

	for i := range configMaps.Items {
		dashboards := m.FromConfigMap(&configMaps.Items[i])
		for _, dash := range dashboards {
			if dash.FolderPath() == "" {
				continue
			}
			segments := folder.ParsePath(dash.FolderPath())
			for _, seg := range segments {
				requiredUIDs[seg.UID()] = true
			}
		}
	}

	return requiredUIDs, nil
}
