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
)

var ErrOrganizationNotFound = errors.New("organization not found")

var SharedOrg = Organization{
	ID:   1,
	Name: "Shared Org",
}

// UpsertOrganization creates or updates an organization in Grafana based on the provided Organization struct.
func (s *Service) UpsertOrganization(ctx context.Context, organization *Organization) error {
	logger := log.FromContext(ctx)
	logger.Info("upserting organization")

	// Get the current organization stored in Grafana
	currentOrganization, err := s.findOrgByID(organization.ID)
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			foundByNameOrganization, err := s.FindOrgByName(organization.Name)
			if err == nil && foundByNameOrganization != nil {
				// If the organization does not exist in Grafana, but we found it by name, we can use that ID.
				logger.Info("found organization with the same name", foundByNameOrganization.Name, "id", foundByNameOrganization.ID)
				organization.ID = foundByNameOrganization.ID
				return nil
			}

			logger.Info("organization name not found, creating")

			// If organization does not exist in Grafana, create it
			createdOrg, err := s.grafanaClient.Orgs().CreateOrg(&models.CreateOrgCommand{
				Name: organization.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to create organization: %w", err)
			}
			logger.Info("created organization")

			organization.ID = *createdOrg.Payload.OrgID
			return nil
		}

		return fmt.Errorf("failed to find organization with ID %d: %w", organization.ID, err)
	}

	// If both name matches, there is nothing to do.
	if currentOrganization.Name == organization.Name {
		logger.Info("the organization already exists in Grafana and does not need to be updated.")
		return nil
	}

	// if the name of the CR is different from the name of the org in Grafana, update the name of the org in Grafana using the CR's display name.
	_, err = s.grafanaClient.Orgs().UpdateOrg(organization.ID, &models.UpdateOrgForm{
		Name: organization.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to update organization name: %w", err)
	}

	logger.Info("updated organization")

	return nil
}

func (s *Service) deleteOrganization(ctx context.Context, organization Organization) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting organization")
	_, err := s.findOrgByID(organization.ID)
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			logger.Info("organization id was not found, skipping deletion")
			// If the CR orgID does not exist in Grafana, then we create the organization
			return nil
		}
		return fmt.Errorf("failed to find organization: %w", err)
	}

	_, err = s.grafanaClient.Orgs().DeleteOrgByID(organization.ID)
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
func (s *Service) FindOrgByName(name string) (*Organization, error) {
	organization, err := s.grafanaClient.Orgs().GetOrgByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by name: %w", err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// findOrgByID is a wrapper function used to find a Grafana organization by its id
func (s *Service) findOrgByID(orgID int64) (*Organization, error) {
	if orgID == 0 {
		return nil, ErrOrganizationNotFound
	}

	organization, err := s.grafanaClient.Orgs().GetOrgByID(orgID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", ErrOrganizationNotFound, err)
		}

		return nil, fmt.Errorf("failed to get organization by id: %w", err)
	}

	return &Organization{
		ID:   organization.Payload.ID,
		Name: organization.Payload.Name,
	}, nil
}

// PublishDashboard creates or updates a dashboard in Grafana
func (s *Service) PublishDashboard(dashboard map[string]any) error {
	_, err := s.grafanaClient.Dashboards().PostDashboard(&models.SaveDashboardCommand{
		Dashboard: any(dashboard),
		Message:   "Added by observability-operator",
		Overwrite: true, // allows dashboard to be updated by the same UID

	})
	if err != nil {
		return fmt.Errorf("failed to publish dashboard: %w", err)
	}
	return nil
}
