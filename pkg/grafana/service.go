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

	// foldersCache memoizes the leaf folder UID for a given folder path. Resolving a
	// path otherwise issues a Grafana API call per segment to check/create each folder;
	// caching the resolved UID avoids re-walking unchanged hierarchies.
	// The short TTL bounds how long a folder deleted out-of-band stays
	// "resolved" in the cache.
	foldersCache *ttlcache.Cache[string, string]
}

func NewService(grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	// Initializing organization cache with a TTL of 1 minute.
	organizationCache := ttlcache.New(
		// TODO: adjust cache TTL when sharing the grafana service at the controller level.
		ttlcache.WithTTL[string, *organization.Organization](10 * time.Second),
	)

	// Initializing folder path cache with a TTL of 1 minute.
	foldersCache := ttlcache.New(
		// TODO: adjust cache TTL when sharing the grafana service at the controller level.
		ttlcache.WithTTL[string, string](10 * time.Second),
	)

	return &Service{
		grafanaClient:     grafanaClient,
		cfg:               cfg,
		organizationCache: organizationCache,
		foldersCache:      foldersCache,
	}
}
