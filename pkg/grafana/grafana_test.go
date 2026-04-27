package grafana

import (
	"context"
	"errors"
	"net/http"
	"testing"

	goruntime "github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	"github.com/giantswarm/observability-operator/pkg/grafana/client/mocks"
)

func notFoundErr() error {
	return goruntime.NewAPIError("not found", nil, http.StatusNotFound)
}

func int64Ptr(v int64) *int64 { return &v }

func newOrg(id int64, name string) *organization.Organization {
	return organization.New(id, name, nil, nil, nil, nil)
}

func TestUpsertOrganization_AdoptsByName(t *testing.T) {
	// Grafana already has an org with our display name at ID 7 — even though our
	// cached status.orgID is stale (2). We should adopt ID 7 and touch nothing else.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Giant Swarm").Return(&orgs.GetOrgByNameOK{
		Payload: &models.OrgDetailsDTO{ID: 7, Name: "Giant Swarm"},
	}, nil)

	svc := newTestService(mockClient)
	org := newOrg(2, "Giant Swarm")

	err := svc.UpsertOrganization(context.Background(), org, "something-stale")
	require.NoError(t, err)
	require.Equal(t, int64(7), org.ID())

	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_CreatesWhenNoCachedIDAndNoNameMatch(t *testing.T) {
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Giant Swarm").Return(nil, notFoundErr())
	mockOrgs.On("CreateOrg", mock.MatchedBy(func(cmd *models.CreateOrgCommand) bool {
		return cmd.Name == "Giant Swarm"
	})).Return(&orgs.CreateOrgOK{
		Payload: &models.CreateOrgOKBody{OrgID: int64Ptr(42)},
	}, nil)

	svc := newTestService(mockClient)
	org := newOrg(0, "Giant Swarm")

	err := svc.UpsertOrganization(context.Background(), org, "")
	require.NoError(t, err)
	require.Equal(t, int64(42), org.ID())

	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_RenamesInPlaceWhenPreviousNameMatches(t *testing.T) {
	// User renamed the CR: spec.displayName changed from "Old" to "New".
	// Grafana has org 2 = "Old" (still ours). previousName="Old" unlocks rename.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "New").Return(nil, notFoundErr())
	mockOrgs.On("GetOrgByID", int64(2)).Return(&orgs.GetOrgByIDOK{
		Payload: &models.OrgDetailsDTO{ID: 2, Name: "Old"},
	}, nil)
	mockOrgs.On("UpdateOrg", int64(2), mock.MatchedBy(func(cmd *models.UpdateOrgForm) bool {
		return cmd.Name == "New"
	})).Return(&orgs.UpdateOrgOK{}, nil)

	svc := newTestService(mockClient)
	org := newOrg(2, "New")

	err := svc.UpsertOrganization(context.Background(), org, "Old")
	require.NoError(t, err)
	require.Equal(t, int64(2), org.ID())

	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_StaleIDCollision_CreatesFreshInsteadOfClobbering(t *testing.T) {
	// Two CRs ended up with status.orgID=2 after a Grafana DB reset. The first CR
	// re-created "Giant Swarm" at ID=2; now this second CR reconciles with a stale
	// orgID=2 but displayName "Hello World". Grafana has ID 2 = "Giant Swarm"
	// (owned by the other CR), and status.displayName on this CR was "Hello World"
	// — so the name at our cached ID is NOT what we last wrote there. The fix
	// must NOT rename org 2 (that would clobber the sibling CR) and instead create
	// a fresh org for us.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Hello World").Return(nil, notFoundErr())
	mockOrgs.On("GetOrgByID", int64(2)).Return(&orgs.GetOrgByIDOK{
		Payload: &models.OrgDetailsDTO{ID: 2, Name: "Giant Swarm"},
	}, nil)
	mockOrgs.On("CreateOrg", mock.MatchedBy(func(cmd *models.CreateOrgCommand) bool {
		return cmd.Name == "Hello World"
	})).Return(&orgs.CreateOrgOK{
		Payload: &models.CreateOrgOKBody{OrgID: int64Ptr(99)},
	}, nil)

	svc := newTestService(mockClient)
	org := newOrg(2, "Hello World")

	err := svc.UpsertOrganization(context.Background(), org, "Hello World")
	require.NoError(t, err)
	require.Equal(t, int64(99), org.ID())

	// Critically: UpdateOrg must NOT have been called on the sibling's org.
	mockOrgs.AssertNotCalled(t, "UpdateOrg", mock.Anything, mock.Anything)
	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_EmptyPreviousNameTreatsIDMismatchAsCollision(t *testing.T) {
	// First-ever reconcile after adding status.displayName: previousName is empty.
	// Even though status.orgID=2 is set, we refuse to rename whatever sits there
	// because we can't prove it was ours. Create fresh.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Giant Swarm").Return(nil, notFoundErr())
	mockOrgs.On("GetOrgByID", int64(2)).Return(&orgs.GetOrgByIDOK{
		Payload: &models.OrgDetailsDTO{ID: 2, Name: "Something Else"},
	}, nil)
	mockOrgs.On("CreateOrg", mock.Anything).Return(&orgs.CreateOrgOK{
		Payload: &models.CreateOrgOKBody{OrgID: int64Ptr(55)},
	}, nil)

	svc := newTestService(mockClient)
	org := newOrg(2, "Giant Swarm")

	err := svc.UpsertOrganization(context.Background(), org, "")
	require.NoError(t, err)
	require.Equal(t, int64(55), org.ID())

	mockOrgs.AssertNotCalled(t, "UpdateOrg", mock.Anything, mock.Anything)
	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_CachedIDGone_CreatesFresh(t *testing.T) {
	// Grafana's DB was reset: our cached ID and the name both 404. Create fresh.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Giant Swarm").Return(nil, notFoundErr())
	mockOrgs.On("GetOrgByID", int64(2)).Return(nil, notFoundErr())
	mockOrgs.On("CreateOrg", mock.Anything).Return(&orgs.CreateOrgOK{
		Payload: &models.CreateOrgOKBody{OrgID: int64Ptr(2)},
	}, nil)

	svc := newTestService(mockClient)
	org := newOrg(2, "Giant Swarm")

	err := svc.UpsertOrganization(context.Background(), org, "Giant Swarm")
	require.NoError(t, err)
	require.Equal(t, int64(2), org.ID())

	mockOrgs.AssertExpectations(t)
}

func TestUpsertOrganization_FindByNameTransientError_Propagates(t *testing.T) {
	// Any non-404 error while looking up by name must surface — we must NOT
	// silently create a duplicate org on a transient Grafana hiccup.
	mockClient := &mocks.MockGrafanaClient{}
	mockOrgs := &mocks.MockOrgsClient{}
	mockClient.On("Orgs").Return(mockOrgs)

	mockOrgs.On("GetOrgByName", "Giant Swarm").Return(nil, errors.New("connection refused"))

	svc := newTestService(mockClient)
	org := newOrg(2, "Giant Swarm")

	err := svc.UpsertOrganization(context.Background(), org, "Giant Swarm")
	require.Error(t, err)

	mockOrgs.AssertNotCalled(t, "CreateOrg", mock.Anything)
	mockOrgs.AssertNotCalled(t, "UpdateOrg", mock.Anything, mock.Anything)
	mockOrgs.AssertExpectations(t)
}
