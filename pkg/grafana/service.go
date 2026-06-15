package grafana

import (
	"time"

	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"

	ttlcache "github.com/jellydator/ttlcache/v3"
)

type Service struct {
	grafanaClient grafanaClient.GrafanaClient
	cfg           config.Config

	// organizationCache memoizes organization lookups by name to avoid repeating the
	// same "get org by name" Grafana API call. The short TTL keeps
	// it fresh enough to pick up organizations created or renamed out-of-band.
	organizationCache *ttlcache.Cache[string, *organization.Organization]
}

func NewService(grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	// Initializing organization cache with a TTL of 1 minute.
	organizationCache := ttlcache.New(
		ttlcache.WithTTL[string, *organization.Organization](1 * time.Minute),
	)

	return &Service{
		grafanaClient:     grafanaClient,
		cfg:               cfg,
		organizationCache: organizationCache,
	}
}
