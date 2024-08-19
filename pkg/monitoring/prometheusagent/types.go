package prometheusagent

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

type RemoteWriteConfig struct {
	PrometheusAgentConfig *PrometheusAgentConfig `yaml:"prometheus-agent,omitempty" json:"prometheus-agent,omitempty"`
}

type PrometheusAgentConfig struct {
	ExternalLabels map[string]string     `yaml:"externalLabels,omitempty" json:"externalLabels,omitempty"`
	Image          *PrometheusAgentImage `yaml:"image,omitempty" json:"image,omitempty"`
	RemoteWrite    []*RemoteWrite        `yaml:"remoteWrite,omitempty" json:"remoteWrite,omitempty"`
	Shards         int                   `yaml:"shards,omitempty" json:"shards,omitempty"`
	Version        string                `yaml:"version,omitempty" json:"version,omitempty"`
}

type PrometheusAgentImage struct {
	Tag string `yaml:"tag" json:"tag"`
}

type RemoteWrite struct {
	promv1.RemoteWriteSpec `yaml:",inline" json:",inline"`
	Password               string `yaml:"password" json:"password"`
	Username               string `yaml:"username" json:"username"`
}
