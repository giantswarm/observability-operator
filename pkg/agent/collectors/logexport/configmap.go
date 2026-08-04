package logexport

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"

	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/config"
)

var (
	//go:embed templates/logexport.alloy.template
	alloyLogExport         string
	alloyLogExportTemplate *template.Template

	//go:embed templates/logexport-config.yaml.template
	alloyLogExportConfig         string
	alloyLogExportConfigTemplate *template.Template
)

func init() {
	alloyLogExportTemplate = template.Must(
		template.New("logexport.alloy").Funcs(sprig.FuncMap()).Parse(alloyLogExport))
	alloyLogExportConfigTemplate = template.Must(
		template.New("logexport-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyLogExportConfig))
}

// GenerateAlloyLogExportConfigMapData renders the Helm values for alloy-logexporter.
//
// The result is a single "values" key, referenced by the alloy-logexporter
// HelmRelease that management-cluster-bases collections ships. It intentionally
// carries the whole values document -- including alloy.enabled -- because the
// collections side deliberately has no inline spec.values: Flux lets inline values
// override every valuesFrom entry, so anything set there could never be overridden
// from here.
func (s *Service) GenerateAlloyLogExportConfigMapData() (map[string]string, error) {
	values, err := generateAlloyLogExportValues(s.Config.LogExport)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"values": values,
	}, nil
}

// generateAlloyLogExportValues renders the Alloy config and embeds it in the Helm
// values document.
func generateAlloyLogExportValues(logExport config.LogExportConfig) (string, error) {
	alloyConfig, err := generateAlloyConfig(logExport)
	if err != nil {
		return "", err
	}

	data := struct {
		AlloyConfig             string
		Replicas                int
		ReleaseName             string
		WALSize                 string
		ResourcesRequestsCPU    string
		ResourcesRequestsMemory string
		ResourcesLimitsCPU      string
		ResourcesLimitsMemory   string
	}{
		AlloyConfig: alloyConfig,
		Replicas:    Replicas,
		// alloy-app names the Secret it builds from extraSecretEnv after the release.
		ReleaseName: apps.AlloyLogExporterAppName,
		WALSize:     WALSize,
		// Measured on a testing installation carrying ~880 log lines/s: two
		// replicas used ~74m CPU and ~265Mi in total. Sized here for roughly
		// three times that rate.
		ResourcesRequestsCPU:    "100m",
		ResourcesRequestsMemory: "400Mi",
		ResourcesLimitsCPU:      "1",
		ResourcesLimitsMemory:   "1Gi",
	}

	var buf bytes.Buffer
	if err := alloyLogExportConfigTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render alloy log export values: %w", err)
	}

	return buf.String(), nil
}

// generateAlloyConfig renders the Alloy river configuration.
func generateAlloyConfig(logExport config.LogExportConfig) (string, error) {
	data := struct {
		DropSelectors     []string
		Region            string
		Bucket            string
		Prefix            string
		Endpoint          string
		ForcePathStyle    bool
		Timeout           string
		QueueSize         int
		BatchMinSize      int
		BatchMaxSize      int
		BatchFlushTimeout string
	}{
		// Static for now: export the Kubernetes API audit stream. Issue 06 replaces
		// this with the selection from the customer's CR, which is why it is already
		// a list of negated drop selectors rather than a single keep selector.
		DropSelectors:     []string{`{scrape_job!="audit-logs"}`},
		Region:            logExport.Region,
		Bucket:            logExport.Bucket,
		Prefix:            logExport.Prefix,
		Endpoint:          logExport.Endpoint,
		ForcePathStyle:    logExport.ForcePathStyle,
		Timeout:           Timeout,
		QueueSize:         QueueSize,
		BatchMinSize:      BatchMinSize,
		BatchMaxSize:      BatchMaxSize,
		BatchFlushTimeout: BatchFlushTimeout,
	}

	var buf bytes.Buffer
	if err := alloyLogExportTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render alloy log export config: %w", err)
	}

	return buf.String(), nil
}
