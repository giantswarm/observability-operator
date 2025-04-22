package rules

import (
	"bytes"
	"context"
	_ "embed"
	"maps"
	"text/template"

	"github.com/blang/semver/v4"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

const (
	alloyRulesAppCatalog        = "giantswarm"
	AlloyRulesAppName           = "alloy-rules"
	alloyRulesAppNamespace      = "giantswarm"
	alloyRulesChartName         = "alloy"
	alloyRulesDeployInNamespace = "monitoring"
	alloyRulesConfigMapName     = "alloy-rules-config"
)

var (
	//go:embed templates/alloy-rules.yaml.template
	appConfig         string
	appConfigTemplate *template.Template
)

// init initializes the template for the alloy-rules configmap.
func init() {
	appConfigTemplate = template.Must(template.New("alloy-rules.yaml").Funcs(sprig.FuncMap()).Parse(appConfig))
}

type Service struct {
	Client          client.Client
	AlloyAppVersion semver.Version
}

func (s *Service) Configure(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("configuring alloy-rules")

	logger.Info("create or update alloy rules configmap")
	err := s.createOrUpdateConfigMap(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("configure alloy rules app")
	err = s.configureApp(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("alloy-rules is configured")

	return nil
}

func (s Service) createOrUpdateConfigMap(ctx context.Context) error {
	logger := log.FromContext(ctx)
	// Get list of tenants
	var tenants []string
	tenants, err := tenancy.ListTenants(ctx, s.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alloyRulesConfigMapName,
			Namespace: alloyRulesAppNamespace,
			Labels:    labels.Common,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, s.Client, configmap, func() error {
		values, err := s.generateAlloyConfig(ctx, tenants)
		if err != nil {
			logger.Error(err, "failed to generate %s", alloyRulesConfigMapName)
			return errors.WithStack(err)
		}

		data := make(map[string]string)
		data["values"] = values

		configmap.Data = data

		return nil
	})

	if err != nil {
		logger.Error(err, "failed to create or update configmap %s", alloyRulesConfigMapName)
		return errors.WithStack(err)
	}

	return nil
}

func (s *Service) generateAlloyConfig(ctx context.Context, tenants []string) (string, error) {
	data := struct {
		Tenants []string
	}{
		Tenants: tenants,
	}

	var values bytes.Buffer
	err := appConfigTemplate.Execute(&values, data)
	if err != nil {
		return "", err
	}
	return values.String(), nil
}

func (s Service) configureApp(ctx context.Context) error {
	logger := log.FromContext(ctx)

	labels := maps.Clone(labels.Common)
	labels["app-operator.giantswarm.io/version"] = "0.0.0"
	app := &appv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AlloyRulesAppName,
			Namespace: alloyRulesAppNamespace,
			Labels:    labels,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, app, func() error {
		app.Spec = appv1.AppSpec{
			Catalog:   alloyRulesAppCatalog,
			Name:      alloyRulesChartName,
			Namespace: alloyRulesDeployInNamespace,
			Version:   s.AlloyAppVersion.String(),
			Config: appv1.AppSpecConfig{
				ConfigMap: appv1.AppSpecConfigConfigMap{
					Name:      alloyRulesConfigMapName,
					Namespace: alloyRulesAppNamespace,
				},
			},
			KubeConfig: appv1.AppSpecKubeConfig{
				InCluster: true,
			},
		}

		return nil
	})

	if err != nil {
		logger.Error(err, "failed to create or update app %s", AlloyRulesAppName)
		return errors.WithStack(err)
	}

	return nil
}
