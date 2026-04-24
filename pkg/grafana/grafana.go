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

// UpsertOrganization reconciles the Grafana organization described by org with Grafana's current state.
//
// previousName is the last display name this CR successfully applied to its Grafana organization
// (typically GrafanaOrganization.Status.DisplayName). It lets the reconciler tell apart:
//
//   - a CR rename: Grafana still has the org at org.ID() under previousName → rename it to org.Name().
//   - a stale cached orgID: the org at org.ID() has a different name (and that name is not what we
//     last wrote there), so it now belongs to someone else (e.g., Grafana's DB was reset and the ID
//     got reassigned to another CR). We must not rename it; create a new org instead.
//
// Pass an empty previousName for a first-time reconcile. In that case a mismatched current name
// is always treated as a collision (safer than assuming ownership).
//
// On success org.ID() is set to the current Grafana org ID.
func (s *Service) UpsertOrganization(ctx context.Context, org *organization.Organization, previousName string) error {
	logger := log.FromContext(ctx)
	logger.Info("upserting organization")

	// Prefer matching by name. If Grafana already has an org with our desired display name,
	// adopt its ID — this handles first reconcile, a CR re-creation, and the case where
	// Grafana's DB was reset and existing IDs no longer match status.orgID.
	foundByName, err := s.FindOrgByName(org.Name())
	if err != nil && !errors.Is(err, organization.ErrOrganizationNotFound) {
		return fmt.Errorf("failed to look up organization by name: %w", err)
	}
	if foundByName != nil {
		if org.ID() != foundByName.ID() {
			logger.Info("adopting existing Grafana organization matching display name",
				"name", foundByName.Name(), "newID", foundByName.ID(), "previousID", org.ID())
		}
		org.SetID(foundByName.ID())
		return nil
	}

	// No org with our desired name exists. If we have a cached ID, check whether the
	// org still sitting at that ID is the one we previously owned (rename case) or a
	// different one (stale-ID / collision case).
	if org.ID() > 0 {
		currentOrg, err := s.findOrgByID(org.ID())
		switch {
		case err == nil:
			if previousName != "" && currentOrg.Name() == previousName {
				logger.Info("renaming organization", "id", org.ID(), "from", currentOrg.Name(), "to", org.Name())
				if _, err := s.grafanaClient.Orgs().UpdateOrg(org.ID(), &models.UpdateOrgForm{Name: org.Name()}); err != nil {
					return fmt.Errorf("failed to update organization name: %w", err)
				}
				return nil
			}
			logger.Info("cached orgID points to an organization we no longer own; creating a new one",
				"staleID", org.ID(), "currentName", currentOrg.Name(), "previousName", previousName)
			// fall through to CreateOrg
		case errors.Is(err, organization.ErrOrganizationNotFound):
			// cached ID is gone; fall through to CreateOrg
		default:
			return fmt.Errorf("failed to find organization with ID %d: %w", org.ID(), err)
		}
	}

	logger.Info("creating organization", "name", org.Name())
	createdOrg, err := s.grafanaClient.Orgs().CreateOrg(&models.CreateOrgCommand{Name: org.Name()})
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}
	org.SetID(*createdOrg.Payload.OrgID)
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

// FindOrgByName is a wrapper function used to find a Grafana organization by its name.
// It returns organization.ErrOrganizationNotFound (wrapped) when Grafana returns 404,
// letting callers distinguish a missing organization from other API failures.
func (s *Service) FindOrgByName(name string) (*organization.Organization, error) {
	org, err := s.grafanaClient.Orgs().GetOrgByName(name)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", organization.ErrOrganizationNotFound, err)
		}
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
