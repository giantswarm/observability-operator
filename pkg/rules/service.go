package rules

import (
	"bytes"
	"context"
	_ "embed"
	"reflect"
	"text/template"

	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

const (
	alloyRulesAppName       = "alloy-rules"
	alloyRulesAppNamespace  = "giantswarm"
	alloyRulesConfigMapName = "alloy-rules-config"
)

var (
	//go:embed templates/alloy-rules.yaml.template
	appConfig         string
	appConfigTemplate *template.Template
)

func init() {
	appConfigTemplate = template.Must(template.New("alloy-rules.yaml").Funcs(sprig.FuncMap()).Parse(appConfig))
}

type Service struct {
	client.Client
}

func (s *Service) Configure(ctx context.Context, cluster clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("configuring alloy-rules")

	logger.Info("create or update alloy rules configmap")
	err := s.createOrUpdateConfigMap(ctx, cluster)
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

func (s *Service) CleanUp(ctx context.Context, cluster clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting alloy-rules config")

	configmap := configMap()

	err := s.Client.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete %s", alloyRulesConfigMapName)
		return errors.WithStack(err)
	}

	logger.Info("deleted alloy-rules config")
	return nil

}

func configMap() *v1.ConfigMap {
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alloyRulesConfigMapName,
			Namespace: alloyRulesAppNamespace,
			Labels:    labels.Common,
		},
	}

	return configmap
}

func (s Service) createOrUpdateConfigMap(ctx context.Context, cluster clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	// Get list of tenants
	var tenants []string
	tenants, err := tenancy.ListTenants(ctx, s.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	configmap := configMap()
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
		logger.Error(err, "failed to create or update %s", alloyRulesConfigMapName)
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
	configmap := configMap()

	// Get observability bundle app metadata.
	appObjectKey := types.NamespacedName{
		Name:      alloyRulesAppName,
		Namespace: alloyRulesAppNamespace,
	}

	var current appv1.App
	err := s.Client.Get(ctx, appObjectKey, &current)
	if err != nil {
		return errors.WithStack(err)
	}

	desired := current.DeepCopy()

	desired.Spec.Config = appv1.AppSpecConfig{
		ConfigMap: appv1.AppSpecConfigConfigMap{
			Name:      configmap.Name,
			Namespace: configmap.Namespace,
		},
	}

	if !reflect.DeepEqual(current, *desired) {
		err := s.Client.Update(ctx, desired)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}
