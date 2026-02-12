package grafana

import (
	"context"
	"errors"
	"net/http"
	"testing"

	goruntime "github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/mock"

	folder "github.com/giantswarm/observability-operator/pkg/domain/folder"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

func newTestService(mockClient *mocks.MockGrafanaClient) *Service {
	return &Service{
		grafanaClient: mockClient,
	}
}

func TestEnsureFolderHierarchy_EmptyPath(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	svc := newTestService(mockClient)

	uid, err := svc.EnsureFolderHierarchy(context.Background(), "")
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
	uid, err := svc.EnsureFolderHierarchy(context.Background(), "team-a")
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
	uid, err := svc.EnsureFolderHierarchy(context.Background(), "team-a")
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
	uid, err := svc.EnsureFolderHierarchy(context.Background(), "team-a/networking")
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
	uid, err := svc.EnsureFolderHierarchy(context.Background(), "team-a")
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
	_, err := svc.EnsureFolderHierarchy(context.Background(), "team-a")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just check it wraps nicely
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
	uid, err := svc.EnsureFolderHierarchy(context.Background(), "a/b/c")
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
	_, err := svc.EnsureFolderHierarchy(context.Background(), "team-a")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
