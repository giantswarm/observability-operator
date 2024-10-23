package grafana

import (
	"context"
	"strings"

	"github.com/grafana/grafana-openapi-client-go/client"
	grafanaAPIModels "github.com/grafana/grafana-openapi-client-go/models"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func CreateOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization *v1alpha1.GrafanaOrganization) (*Organization, error) {
	logger := log.FromContext(ctx)

	// Check if the organization name is available
	organizationInGrafana, err := getOrganizationInGrafanaByName(ctx, grafanaAPI, organization.Spec.DisplayName)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if organizationInGrafana != nil {
		logger.Error(err, "A grafana organization with the same name already exists. Please choose a different display name.")
		return nil, errors.WithStack(err)
	}

	logger.Info("Create organization in Grafana")
	createdOrg, err := grafanaAPI.Orgs.CreateOrg(&grafanaAPIModels.CreateOrgCommand{
		Name: organization.Spec.DisplayName,
	})
	if err != nil {
		logger.Error(err, "Creating organization failed")
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   *createdOrg.Payload.OrgID,
		Name: organization.Spec.DisplayName,
	}, nil
}

func UpdateOrganization(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, organization *v1alpha1.GrafanaOrganization) (*Organization, error) {
	logger := log.FromContext(ctx)

	organizationInGrafana, err := getOrganizationInGrafanaByID(ctx, grafanaAPI, organization.Status.OrgID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if organizationInGrafana == nil {
		// If the CR orgID does not exist in Grafana, then we create the organization
		return CreateOrganization(ctx, grafanaAPI, organization)
	}

	// If both name matches, there is nothing to do.
	if organizationInGrafana.Name == organization.Spec.DisplayName {
		logger.Info("The organization already exists in Grafana and does not need to be updated.")
		return organizationInGrafana, nil
	}

	// Check if the organization name is available
	organizationInGrafana, err = getOrganizationInGrafanaByName(ctx, grafanaAPI, organization.Spec.DisplayName)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if organizationInGrafana != nil {
		logger.Error(err, "A grafana organization with the same name already exists. Please choose a different display name.")
		return organizationInGrafana, errors.WithStack(err)
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = grafanaAPI.Orgs.UpdateOrg(organization.Status.OrgID, &grafanaAPIModels.UpdateOrgForm{
		Name: organization.Spec.DisplayName,
	})
	if err != nil {
		logger.Error(err, "Failed to update organization name")
		return nil, errors.WithStack(err)
	}

	return &Organization{
		ID:   organization.Status.OrgID,
		Name: organization.Spec.DisplayName,
	}, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Parsing error message to find out the error code
	return strings.Contains(err.Error(), "(status 404)")
}

// getOrganizationInGrafanaByName is a Wrapper function to get the organization in Grafana by Name
func getOrganizationInGrafanaByName(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, name string) (*Organization, error) {
	logger := log.FromContext(ctx)
	organization, err := grafanaAPI.Orgs.GetOrgByName(name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		} else {
			logger.Error(err, "Failed to get organization by name")
			return nil, errors.WithStack(err)
		}
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// getOrganizationInGrafanaByID is a Wrapper function to get the organization in Grafana by ID
func getOrganizationInGrafanaByID(ctx context.Context, grafanaAPI *client.GrafanaHTTPAPI, orgID int64) (*Organization, error) {
	logger := log.FromContext(ctx)
	organization, err := grafanaAPI.Orgs.GetOrgByID(orgID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		} else {
			logger.Error(err, "Failed to get organization by ID")
			return nil, errors.WithStack(err)
		}
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}
