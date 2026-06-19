package grafana

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	goruntime "github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/models"
	ttlcache "github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/mock"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

// testFolderCacheOrgID is an arbitrary organization ID used to scope folder cache
// entries in tests.
const testFolderCacheOrgID int64 = 1

func newTestService(mockClient *mocks.MockGrafanaClient) *Service {
	organizationCache := ttlcache.New(
		ttlcache.WithTTL[string, *organization.Organization](1*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, *organization.Organization](),
	)
	foldersCache := ttlcache.New(
		ttlcache.WithTTL[folderCacheKey, string](1*time.Minute),
		ttlcache.WithDisableTouchOnHit[folderCacheKey, string](),
	)

	return &Service{
		grafanaClient:     mockClient,
		organizationCache: organizationCache,
		foldersCache:      foldersCache,
	}
}

func TestEnsureFolderHierarchy_EmptyPath(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	svc := newTestService(mockClient)

	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != "" {
		t.Errorf("expected empty uid for empty path, got %q", uid)
	}
}

func TestEnsureFolderHierarchy_SingleFolder_AlreadyExists(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	expectedUID := folder.GenerateUID("team-a")

	mockFolders.On("GetFolderByUID", expectedUID).Return(&folders.GetFolderByUIDOK{
		Payload: &models.Folder{
			UID:   expectedUID,
			Title: "team-a",
		},
	}, nil)

	svc := newTestService(mockClient)
	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != expectedUID {
		t.Errorf("expected uid %q, got %q", expectedUID, uid)
	}

	mockFolders.AssertExpectations(t)
}

func TestEnsureFolderHierarchy_SingleFolder_DoesNotExist(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	expectedUID := folder.GenerateUID("team-a")

	// GetFolderByUID returns 404
	mockFolders.On("GetFolderByUID", expectedUID).Return(nil,
		goruntime.NewAPIError("not found", nil, http.StatusNotFound))

	// CreateFolder should be called
	mockFolders.On("CreateFolder", mock.MatchedBy(func(cmd *models.CreateFolderCommand) bool {
		return cmd.UID == expectedUID &&
			cmd.Title == "team-a" &&
			cmd.ParentUID == "" &&
			cmd.Description == folder.Description
	})).Return(&folders.CreateFolderOK{
		Payload: &models.Folder{UID: expectedUID, Title: "team-a"},
	}, nil)

	svc := newTestService(mockClient)
	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != expectedUID {
		t.Errorf("expected uid %q, got %q", expectedUID, uid)
	}

	mockFolders.AssertExpectations(t)
}

func TestEnsureFolderHierarchy_NestedPath(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	rootUID := folder.GenerateUID("team-a")
	leafUID := folder.GenerateUID("team-a/networking")

	// Root folder exists
	mockFolders.On("GetFolderByUID", rootUID).Return(&folders.GetFolderByUIDOK{
		Payload: &models.Folder{UID: rootUID, Title: "team-a"},
	}, nil)

	// Leaf folder doesn't exist
	mockFolders.On("GetFolderByUID", leafUID).Return(nil,
		goruntime.NewAPIError("not found", nil, http.StatusNotFound))

	// Create leaf folder with root as parent
	mockFolders.On("CreateFolder", mock.MatchedBy(func(cmd *models.CreateFolderCommand) bool {
		return cmd.UID == leafUID &&
			cmd.Title == "networking" &&
			cmd.ParentUID == rootUID
	})).Return(&folders.CreateFolderOK{
		Payload: &models.Folder{UID: leafUID, Title: "networking"},
	}, nil)

	svc := newTestService(mockClient)
	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a/networking")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != leafUID {
		t.Errorf("expected uid %q, got %q", leafUID, uid)
	}

	mockFolders.AssertExpectations(t)
}

func TestEnsureFolderHierarchy_Rename(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	expectedUID := folder.GenerateUID("team-a")

	// Folder exists but with old title
	mockFolders.On("GetFolderByUID", expectedUID).Return(&folders.GetFolderByUIDOK{
		Payload: &models.Folder{UID: expectedUID, Title: "old-team-a"},
	}, nil)

	// UpdateFolder should be called with new title
	mockFolders.On("UpdateFolder", expectedUID, mock.MatchedBy(func(cmd *models.UpdateFolderCommand) bool {
		return cmd.Title == "team-a"
	})).Return(&folders.UpdateFolderOK{
		Payload: &models.Folder{UID: expectedUID, Title: "team-a"},
	}, nil)

	svc := newTestService(mockClient)
	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != expectedUID {
		t.Errorf("expected uid %q, got %q", expectedUID, uid)
	}

	mockFolders.AssertExpectations(t)
}

func TestEnsureFolderHierarchy_CreateFolderError(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	expectedUID := folder.GenerateUID("team-a")

	mockFolders.On("GetFolderByUID", expectedUID).Return(nil,
		goruntime.NewAPIError("not found", nil, http.StatusNotFound))
	mockFolders.On("CreateFolder", mock.Anything).Return(nil, errors.New("grafana unavailable"))

	svc := newTestService(mockClient)
	_, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEnsureFolderHierarchy_ThreeLevel(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	uid1 := folder.GenerateUID("a")
	uid2 := folder.GenerateUID("a/b")
	uid3 := folder.GenerateUID("a/b/c")

	// All three don't exist
	for _, uid := range []string{uid1, uid2, uid3} {
		mockFolders.On("GetFolderByUID", uid).Return(nil,
			goruntime.NewAPIError("not found", nil, http.StatusNotFound))
	}

	// All three should be created in order
	mockFolders.On("CreateFolder", mock.MatchedBy(func(cmd *models.CreateFolderCommand) bool {
		return cmd.UID == uid1 && cmd.ParentUID == ""
	})).Return(&folders.CreateFolderOK{Payload: &models.Folder{UID: uid1}}, nil)

	mockFolders.On("CreateFolder", mock.MatchedBy(func(cmd *models.CreateFolderCommand) bool {
		return cmd.UID == uid2 && cmd.ParentUID == uid1
	})).Return(&folders.CreateFolderOK{Payload: &models.Folder{UID: uid2}}, nil)

	mockFolders.On("CreateFolder", mock.MatchedBy(func(cmd *models.CreateFolderCommand) bool {
		return cmd.UID == uid3 && cmd.ParentUID == uid2
	})).Return(&folders.CreateFolderOK{Payload: &models.Folder{UID: uid3}}, nil)

	svc := newTestService(mockClient)
	uid, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "a/b/c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != uid3 {
		t.Errorf("expected leaf uid %q, got %q", uid3, uid)
	}

	mockFolders.AssertExpectations(t)
}

func TestEnsureFolderHierarchy_GetFolderNon404Error(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	mockClient.On("Folders").Return(mockFolders)

	expectedUID := folder.GenerateUID("team-a")

	// Return a 500 error, not a 404
	mockFolders.On("GetFolderByUID", expectedUID).Return(nil,
		goruntime.NewAPIError("internal server error", nil, http.StatusInternalServerError))

	svc := newTestService(mockClient)
	_, err := svc.ensureFolderHierarchy(context.Background(), mockClient, testFolderCacheOrgID, "team-a")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// testOrg returns an organization domain object for use in tests.
func testOrg() *organization.Organization {
	return organization.NewFromGrafana(1, "test-org")
}

// setupOrgContextMocks wires WithOrgID to return the same mock so the mock
// can play the role of both base and per-org client in CleanupOrphanedFoldersForOrg tests.
func setupOrgContextMocks(mockClient *mocks.MockGrafanaClient) {
	mockClient.On("WithOrgID", int64(1)).Return(mockClient)
}

func TestCleanupOrphanedFoldersForOrg_DeletesEmptyOrphanedFolder(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	orphanUID := folder.GenerateUID("old-team")

	svc := newTestService(mockClient)

	// GetFolders returns one operator-managed folder
	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: orphanUID, Title: "old-team"}},
	}, nil)

	// Folder is empty
	mockFolders.On("GetFolderDescendantCounts", orphanUID).Return(&folders.GetFolderDescendantCountsOK{
		Payload: map[string]int64{},
	}, nil)

	// DeleteFolder should be called
	mockFolders.On("DeleteFolder", mock.MatchedBy(func(params *folders.DeleteFolderParams) bool {
		return params.FolderUID == orphanUID
	})).Return(&folders.DeleteFolderOK{}, nil)

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockFolders.AssertExpectations(t)
}

func TestCleanupOrphanedFoldersForOrg_DeletesNestedEmptyHierarchy(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	uidA := folder.GenerateUID("a")
	uidB := folder.GenerateUID("a/b")
	uidC := folder.GenerateUID("a/b/c")

	svc := newTestService(mockClient)

	// GetFolders returns the hierarchy a > b > c in root-first order, which is the
	// order that previously left the parents behind.
	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{
			{UID: uidA, Title: "a"},
			{UID: uidB, Title: "b", ParentUID: uidA},
			{UID: uidC, Title: "c", ParentUID: uidB},
		},
	}, nil)

	// Descendant counts are evaluated live and reflect deletions made earlier in the
	// pass: once a leaf is deleted, its parent reports as empty. Because the loop now
	// processes deepest-first (c, then b, then a), every folder reads as empty.
	for _, uid := range []string{uidC, uidB, uidA} {
		mockFolders.On("GetFolderDescendantCounts", uid).Return(&folders.GetFolderDescendantCountsOK{
			Payload: map[string]int64{},
		}, nil)
	}

	// All three folders should be deleted.
	for _, uid := range []string{uidA, uidB, uidC} {
		mockFolders.On("DeleteFolder", mock.MatchedBy(func(params *folders.DeleteFolderParams) bool {
			return params.FolderUID == uid
		})).Return(&folders.DeleteFolderOK{}, nil)
	}

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockFolders.AssertExpectations(t)
}

func TestCleanupOrphanedFoldersForOrg_SkipsNonOperatorManagedFolders(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	svc := newTestService(mockClient)

	// GetFolders returns a user-created folder (no gs- prefix)
	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: "user-folder-123", Title: "My Folder"}},
	}, nil)

	// GetFolderDescendantCounts and DeleteFolder should NOT be called
	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockFolders.AssertNotCalled(t, "GetFolderDescendantCounts", mock.Anything)
	mockFolders.AssertNotCalled(t, "DeleteFolder", mock.Anything)
}

func TestCleanupOrphanedFoldersForOrg_NonEmptyFolderReturnsError(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	orphanUID := folder.GenerateUID("old-team")

	svc := newTestService(mockClient)

	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: orphanUID, Title: "old-team"}},
	}, nil)

	// Folder has descendants
	mockFolders.On("GetFolderDescendantCounts", orphanUID).Return(&folders.GetFolderDescendantCountsOK{
		Payload: map[string]int64{"dashboards": 3},
	}, nil)

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error for non-empty orphaned folder, got nil")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected error to mention 'not empty', got: %v", err)
	}

	// DeleteFolder should NOT have been called
	mockFolders.AssertNotCalled(t, "DeleteFolder", mock.Anything)
}

func TestCleanupOrphanedFoldersForOrg_DeleteErrorIsReturned(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	orphanUID := folder.GenerateUID("old-team")

	svc := newTestService(mockClient)

	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: orphanUID, Title: "old-team"}},
	}, nil)

	mockFolders.On("GetFolderDescendantCounts", orphanUID).Return(&folders.GetFolderDescendantCountsOK{
		Payload: map[string]int64{},
	}, nil)

	mockFolders.On("DeleteFolder", mock.Anything).Return(nil, errors.New("permission denied"))

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error when delete fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete orphaned folder") {
		t.Errorf("expected delete error message, got: %v", err)
	}
}

func TestCleanupOrphanedFoldersForOrg_DescendantCountsErrorIsReturned(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	orphanUID := folder.GenerateUID("old-team")

	svc := newTestService(mockClient)

	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: orphanUID, Title: "old-team"}},
	}, nil)

	mockFolders.On("GetFolderDescendantCounts", orphanUID).Return(nil, errors.New("grafana error"))

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error when descendant counts fail, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get descendant counts") {
		t.Errorf("expected descendant counts error message, got: %v", err)
	}
}

func TestCleanupOrphanedFoldersForOrg_AccumulatesMultipleErrors(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	uid1 := folder.GenerateUID("folder-a")
	uid2 := folder.GenerateUID("folder-b")

	svc := newTestService(mockClient)

	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{
			{UID: uid1, Title: "folder-a"},
			{UID: uid2, Title: "folder-b"},
		},
	}, nil)

	// First folder: descendant counts fail
	mockFolders.On("GetFolderDescendantCounts", uid1).Return(nil, errors.New("error-1"))

	// Second folder: non-empty
	mockFolders.On("GetFolderDescendantCounts", uid2).Return(&folders.GetFolderDescendantCountsOK{
		Payload: map[string]int64{"dashboards": 1},
	}, nil)

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), map[string]struct{}{})
	if err == nil {
		t.Fatal("expected errors, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "failed to get descendant counts") {
		t.Errorf("expected descendant counts error, got: %v", err)
	}
	if !strings.Contains(errStr, "not empty") {
		t.Errorf("expected non-empty error, got: %v", err)
	}
}

func TestCleanupOrphanedFoldersForOrg_SkipsReferencedFolders(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockFolders := &mocks.MockFoldersClient{}
	setupOrgContextMocks(mockClient)
	mockClient.On("Folders").Return(mockFolders)

	referencedUID := folder.GenerateUID("team-a")

	svc := newTestService(mockClient)

	// Pass the referenced UID in requiredUIDs
	requiredUIDs := map[string]struct{}{
		referencedUID: {},
	}

	mockFolders.On("GetFolders", mock.Anything).Return(&folders.GetFoldersOK{
		Payload: []*models.FolderSearchHit{{UID: referencedUID, Title: "team-a"}},
	}, nil)

	err := svc.CleanupOrphanedFoldersForOrg(context.Background(), testOrg(), requiredUIDs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not attempt to check or delete a referenced folder
	mockFolders.AssertNotCalled(t, "GetFolderDescendantCounts", mock.Anything)
	mockFolders.AssertNotCalled(t, "DeleteFolder", mock.Anything)
}
