package grafana

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/models"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

// UpsertOrganization creates or updates an organization in Grafana based on the provided domain organization.
func (s *Service) UpsertOrganization(ctx context.Context, org *organization.Organization) error {
	logger := log.FromContext(ctx)
	logger.Info("upserting organization")

	// Get the current organization stored in Grafana
	currentOrg, err := s.findOrgByID(org.ID())
	if err != nil {
		if errors.Is(err, organization.ErrOrganizationNotFound) {
			foundOrgByName, err := s.FindOrgByName(org.Name())
			if err == nil && foundOrgByName != nil {
				// If the organization does not exist in Grafana, but we found it by name, we can use that ID.
				logger.Info("found organization with the same name", "name", foundOrgByName.Name(), "id", foundOrgByName.ID())
				org.SetID(foundOrgByName.ID())
				return nil
			}

			logger.Info("organization name not found, creating")

			// If organization does not exist in Grafana, create it
			createdOrg, err := s.grafanaClient.Orgs().CreateOrg(&models.CreateOrgCommand{
				Name: org.Name(),
			})
			if err != nil {
				return fmt.Errorf("failed to create organization: %w", err)
			}
			logger.Info("created organization")

			org.SetID(*createdOrg.Payload.OrgID)
			return nil
		}

		return fmt.Errorf("failed to find organization with ID %d: %w", org.ID(), err)
	}

	// If both name matches, there is nothing to do.
	if currentOrg.Name() == org.Name() {
		logger.Info("the organization already exists in Grafana and does not need to be updated.")
		return nil
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = s.grafanaClient.Orgs().UpdateOrg(org.ID(), &models.UpdateOrgForm{
		Name: org.Name(),
	})
	if err != nil {
		return fmt.Errorf("failed to update organization name: %w", err)
	}

	logger.Info("updated organization")

	return nil
}

func (s *Service) deleteOrganization(ctx context.Context, org *organization.Organization) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting organization")
	_, err := s.findOrgByID(org.ID())
	if err != nil {
		if errors.Is(err, organization.ErrOrganizationNotFound) {
			logger.Info("organization id was not found, skipping deletion")
			// If the CR orgID does not exist in Grafana, then we create the organization
			return nil
		}
		return fmt.Errorf("failed to find organization: %w", err)
	}

	_, err = s.grafanaClient.Orgs().DeleteOrgByID(org.ID())
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}
	logger.Info("deleted organization")

	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *runtime.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsCode(http.StatusNotFound)
	}

	return false
}

// FindOrgByName is a wrapper function used to find a Grafana organization by its name
func (s *Service) FindOrgByName(name string) (*organization.Organization, error) {
	org, err := s.grafanaClient.Orgs().GetOrgByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by name: %w", err)
	}

	return organization.NewFromGrafana(org.Payload.ID, org.Payload.Name), nil
}

// findOrgByID is a wrapper function used to find a Grafana organization by its id
func (s *Service) findOrgByID(orgID int64) (*organization.Organization, error) {
	if orgID == 0 {
		return nil, organization.ErrOrganizationNotFound
	}

	org, err := s.grafanaClient.Orgs().GetOrgByID(orgID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", organization.ErrOrganizationNotFound, err)
		}

		return nil, fmt.Errorf("failed to get organization by id: %w", err)
	}

	return organization.NewFromGrafana(org.Payload.ID, org.Payload.Name), nil
}
