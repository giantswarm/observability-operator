package rules

import (
	"context"
	_ "embed"
	"maps"

	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/common/labels"
)

const (
	alloyRulesAppName       = "alloy-rules"
	alloyRulesAppNamespace  = "giantswarm"
	alloyRulesConfigMapName = "alloy-rules-config"
)

type Service struct {
	Client client.Client
}

func (s *Service) Delete(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting alloy-rules")

	logger.Info("delete alloy rules app")
	err := s.deleteApp(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("delete alloy rules configmap")
	err = s.deleteConfigMap(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("alloy-rules is deleted")

	return nil
}

func (s Service) deleteConfigMap(ctx context.Context) error {
	logger := log.FromContext(ctx)

	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alloyRulesConfigMapName,
			Namespace: alloyRulesAppNamespace,
			Labels:    labels.Common,
		},
	}

	err := s.Client.Delete(ctx, configmap)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "failed to delete configmap", "configmap", alloyRulesConfigMapName)
		return errors.WithStack(err)
	}
	return nil
}

func (s Service) deleteApp(ctx context.Context) error {
	logger := log.FromContext(ctx)

	labels := maps.Clone(labels.Common)
	labels["app-operator.giantswarm.io/version"] = "0.0.0"
	app := &appv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alloyRulesAppName,
			Namespace: alloyRulesAppNamespace,
			Labels:    labels,
		},
	}

	err := s.Client.Delete(ctx, app)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "failed to delete app", "app", alloyRulesAppName)
		return errors.WithStack(err)
	}

	return nil
}
