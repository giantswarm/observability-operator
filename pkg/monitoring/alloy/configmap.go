package alloy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

var (
	//go:embed templates/alloy-config.alloy.template
	alloyConfig         string
	alloyConfigTemplate *template.Template

	//go:embed templates/monitoring-config.yaml.template
	alloyMonitoringConfig         string
	alloyMonitoringConfigTemplate *template.Template
)

func init() {
	alloyConfigTemplate = template.Must(template.New("alloy-config.alloy").Funcs(sprig.FuncMap()).Parse(alloyConfig))
	alloyMonitoringConfigTemplate = template.Must(template.New("monitoring-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringConfig))
}

func (a *Service) GenerateAlloyMonitoringConfigMapData(ctx context.Context, cluster *clusterv1.Cluster) (map[string]string, error) {
	var values bytes.Buffer

	alloyConfig, err := a.generateAlloyConfig(ctx, cluster)
	if err != nil {
		return nil, err
	}

	data := struct {
		AlloyConfig       string
		SecretName        string
		PriorityClassName string
	}{
		AlloyConfig:       alloyConfig,
		SecretName:        commonmonitoring.AlloyMonitoringAgentAppName,
		PriorityClassName: commonmonitoring.PriorityClassName,
	}

	err = alloyMonitoringConfigTemplate.Execute(&values, data)
	if err != nil {
		return nil, err
	}

	configMapData := make(map[string]string)
	configMapData["values"] = values.String()

	return configMapData, nil
}

func (a *Service) generateAlloyConfig(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	var values bytes.Buffer

	organization, err := a.OrganizationRepository.Read(ctx, cluster)
	if err != nil {
		return "", errors.WithStack(err)
	}

	provider, err := common.GetClusterProvider(cluster)
	if err != nil {
		return "", errors.WithStack(err)
	}

	data := struct {
		RemoteWriteURLEnvVarName               string
		RemoteWriteNameEnvVarName              string
		RemoteWriteBasicAuthUsernameEnvVarName string
		RemoteWriteBasicAuthPasswordEnvVarName string
		RemoteWriteTimeout                     string
		RemoteWriteTLSInsecureSkipVerify       bool

		QueueConfigCapacity          int
		QueueConfigMaxSamplesPerSend int
		QueueConfigMaxShards         int

		ExternalLabelsClusterID       string
		ExternalLabelsClusterType     string
		ExternalLabelsCustomer        string
		ExternalLabelsInstallation    string
		ExternalLabelsOrganization    string
		ExternalLabelsPipeline        string
		ExternalLabelsProvider        string
		ExternalLabelsRegion          string
		ExternalLabelsServicePriority string
	}{
		RemoteWriteURLEnvVarName:               AlloyRemoteWriteURLEnvVarName,
		RemoteWriteNameEnvVarName:              AlloyRemoteWriteNameEnvVarName,
		RemoteWriteBasicAuthUsernameEnvVarName: AlloyRemoteWriteBasicAuthUsernameEnvVarName,
		RemoteWriteBasicAuthPasswordEnvVarName: AlloyRemoteWriteBasicAuthPasswordEnvVarName,
		RemoteWriteTimeout:                     commonmonitoring.RemoteWriteTimeout,
		RemoteWriteTLSInsecureSkipVerify:       a.ManagementCluster.InsecureCA,

		QueueConfigCapacity:          commonmonitoring.QueueConfigCapacity,
		QueueConfigMaxSamplesPerSend: commonmonitoring.QueueConfigMaxSamplesPerSend,
		QueueConfigMaxShards:         commonmonitoring.QueueConfigMaxShards,

		ExternalLabelsClusterID:       cluster.Name,
		ExternalLabelsClusterType:     common.GetClusterType(cluster, a.ManagementCluster),
		ExternalLabelsCustomer:        a.ManagementCluster.Customer,
		ExternalLabelsInstallation:    a.ManagementCluster.Name,
		ExternalLabelsOrganization:    organization,
		ExternalLabelsPipeline:        a.ManagementCluster.Pipeline,
		ExternalLabelsProvider:        provider,
		ExternalLabelsRegion:          a.ManagementCluster.Region,
		ExternalLabelsServicePriority: commonmonitoring.GetServicePriority(cluster),
	}

	err = alloyConfigTemplate.Execute(&values, data)
	if err != nil {
		return "", err
	}

	return values.String(), nil
}

func ConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, ConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common(),
		},
	}

	return configmap
}
