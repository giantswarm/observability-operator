package client

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/users"
	"github.com/grafana/grafana-openapi-client-go/models"

	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	clientConfigNumRetries = 3
)

var (
	// clients keep track of the user logins that have already been used to create a client.
	clients      = make(map[string]bool)
	clientsMutex sync.Mutex
)

// GenerateGrafanaClient creates a new Grafana client for the given userLogin.
// Only a single client per login can be created to avoid concurency issues.
func GenerateGrafanaClient(grafanaURL *url.URL, conf config.Config, userLogin string) (*grafana.GrafanaHTTPAPI, error) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	if _, ok := clients[userLogin]; ok {
		return nil, fmt.Errorf("client already created for user %q. Only a single client per login is created to avoid concurency issues.", userLogin)
	}

	grafanaTLSConfig := TLSConfig{
		Cert: conf.Environment.GrafanaTLSCertFile,
		Key:  conf.Environment.GrafanaTLSKeyFile,
	}

	// Create the "root" admin client
	adminCredentials := AdminCredentials{
		Username: conf.Environment.GrafanaAdminUsername,
		Password: conf.Environment.GrafanaAdminPassword,
	}

	adminClient, err := generateGrafanaClient(grafanaURL, adminCredentials, grafanaTLSConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create grafana client: %w", err)
	}

	// Generate a password for the requested login
	passwordManager := password.SimpleManager{}
	userPassword, err := passwordManager.GeneratePassword(32)
	if err != nil {
		return nil, fmt.Errorf("unable to generate password: %w", err)
	}

	// Check if user exists
	user, err := adminClient.Users.GetUserByLoginOrEmail(userLogin)
	if err != nil {
		var loginOrEmailNotFoundErr *users.GetUserByLoginOrEmailNotFound
		if !errors.As(err, &loginOrEmailNotFoundErr) {
			return nil, fmt.Errorf("unable to get user %q: %w", userLogin, err)
		}
	} else {
		// Delete user since we can't update the password when login form is disabled
		_, err = adminClient.AdminUsers.AdminDeleteUser(user.Payload.ID)
		if err != nil {
			return nil, fmt.Errorf("unable to delete user %q: %w", userLogin, err)
		}
	}

	// Create user
	adminUser, err := adminClient.AdminUsers.AdminCreateUser(&models.AdminCreateUserForm{
		Login:    userLogin,
		Email:    fmt.Sprintf("%s@observability-operator", userLogin),
		Password: models.Password(userPassword),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create user %q: %w", userLogin, err)
	}

	_, err = adminClient.AdminUsers.AdminUpdateUserPermissions(adminUser.Payload.ID, &models.AdminUpdateUserPermissionsForm{
		IsGrafanaAdmin: true,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to update permissions for user %q: %w", userLogin, err)
	}

	// Create grafana client with new credentials
	userCredentials := AdminCredentials{
		Username: userLogin,
		Password: userPassword,
	}
	client, err := generateGrafanaClient(grafanaURL, userCredentials, grafanaTLSConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create grafana client for user %q: %w", userLogin, err)
	}

	// Mark the user login as used
	clients[userLogin] = true

	return client, nil
}

func generateGrafanaClient(grafanaURL *url.URL, credentials AdminCredentials, tlsConfig TLSConfig) (*grafana.GrafanaHTTPAPI, error) {
	var err error

	// Generate Grafana client
	// Get grafana admin-password and admin-user
	if credentials.Username == "" {
		return nil, fmt.Errorf("GrafanaAdminUsername not set")

	}
	if credentials.Password == "" {
		return nil, fmt.Errorf("GrafanaAdminPassword not set")

	}

	grafanaTLSConfig, err := tlsConfig.toTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build tls config: %w", err)
	}

	cfg := &grafana.TransportConfig{
		Schemes:  []string{grafanaURL.Scheme},
		BasePath: "/api",
		Host:     grafanaURL.Host,
		// We use basic auth to authenticate on grafana.
		BasicAuth: url.UserPassword(credentials.Username, credentials.Password),
		// NumRetries contains the optional number of attempted retries.
		NumRetries: clientConfigNumRetries,
		TLSConfig:  grafanaTLSConfig,
	}

	return grafana.NewHTTPClientWithConfig(nil, cfg), nil
}
