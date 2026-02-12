package grafana

import (
	"context"
	"errors"
	"fmt"

	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

// ensureFolderHierarchy ensures that the full folder hierarchy exists for the given path.
// Returns the leaf folder UID, or empty string if path is empty (General folder).
// Must be called within an organization context (see withinOrganization).
func (s *Service) ensureFolderHierarchy(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	logger := log.FromContext(ctx)

	segments := folder.ParsePath(path)
	var leafUID string

	for _, seg := range segments {
		leafUID = seg.UID()

		existing, err := s.grafanaClient.Folders().GetFolderByUID(seg.UID())
		if err != nil {
			if !isFolderNotFound(err) {
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
			continue
		}

		// Folder exists - reconcile title if someone renamed it manually in the Grafana UI.
		// The UID is derived from the path so it stays the same, but the title could have been changed.
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
	}

	return leafUID, nil
}

// CleanupOrphanedFoldersForOrg switches to the given organization context and removes
// operator-managed folders that are no longer referenced by any dashboard.
// requiredUIDs is the set of folder UIDs still needed by dashboard ConfigMaps.
func (s *Service) CleanupOrphanedFoldersForOrg(ctx context.Context, org *organization.Organization, requiredUIDs map[string]struct{}) error {
	return s.withinOrganization(ctx, org, func(ctx context.Context) error {
		logger := log.FromContext(ctx)

		// List all folders in the current org
		allFolders, err := s.grafanaClient.Folders().GetFolders(folders.NewGetFoldersParams())
		if err != nil {
			return fmt.Errorf("failed to list folders: %w", err)
		}

		// Delete operator-managed folders that are empty and unreferenced
		var errs []error
		for _, f := range allFolders.Payload {
			if !folder.IsOperatorManaged(f.UID) {
				continue
			}

			if _, ok := requiredUIDs[f.UID]; ok {
				continue
			}

			// Check if folder is empty before deleting
			counts, err := s.grafanaClient.Folders().GetFolderDescendantCounts(f.UID)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get descendant counts for folder %q: %w", f.UID, err))
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
				errs = append(errs, fmt.Errorf("orphaned folder %q is not empty, skipping deletion", f.UID))
				continue
			}

			logger.Info("deleting orphaned folder", "uid", f.UID, "title", f.Title)
			_, err = s.grafanaClient.Folders().DeleteFolder(folders.NewDeleteFolderParams().WithFolderUID(f.UID))
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to delete orphaned folder %q: %w", f.UID, err))
			}
		}

		return errors.Join(errs...)
	})
}

// isFolderNotFound checks if the error is a folder not found error.
// It first checks for the specific folder 404 error type, then falls back to the general isNotFound check.
func isFolderNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific folder 404 error type
	var folderNotFoundErr *folders.GetFolderByUIDNotFound
	if errors.As(err, &folderNotFoundErr) {
		return true
	}

	// Fallback to generic not found check
	return isNotFound(err)
}
