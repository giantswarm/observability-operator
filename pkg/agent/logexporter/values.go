package logexporter

import (
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/common/apps"
)

var (
	//go:embed templates/alloy-logexporter.alloy.template
	alloyConfig         string
	alloyConfigTemplate *template.Template

	//go:embed templates/alloy-logexporter-config.yaml.template
	alloyValues         string
	alloyValuesTemplate *template.Template

	// Alloy component labels are identifiers, so anything else in a namespace or name has
	// to be folded away.
	nonIdentifier = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func init() {
	alloyConfigTemplate = template.Must(template.New("alloy-logexporter.alloy").Funcs(sprig.FuncMap()).Parse(alloyConfig))
	alloyValuesTemplate = template.Must(template.New("alloy-logexporter-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyValues))
}

// Credentials are the resolved contents of a destination's credentialsRef Secret.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// export is one LogExport translated into everything the templates need.
type export struct {
	// Slug suffixes every Alloy component belonging to this export. It has to be stable:
	// the WAL keys queued records by component ID, so renaming a component orphans records
	// that were already buffered.
	Slug     string
	Ref      string
	Pipeline pipeline
	S3       *observabilityv1alpha1.S3Destination
}

// RenderValues renders the Helm values document for alloy-logexporter, covering every
// LogExport on the installation. The result is the `values` key of the ConfigMap the
// HelmRelease reads last, so it is also what switches the app on.
//
// Order is not taken from the caller: exports are sorted so that the same set of
// resources always renders byte-identically.
func RenderValues(exports []observabilityv1alpha1.LogExport) (string, error) {
	rendered, err := buildExports(exports)
	if err != nil {
		return "", err
	}

	config, err := render(alloyConfigTemplate, struct {
		Exports           []export
		Port              int
		WALDirectory      string
		ClusterIDField    string
		ExportTimeout     string
		QueueSize         int
		QueueConsumers    int
		BatchMinSize      int
		BatchMaxSize      int
		BatchFlushTimeout string
		RetryMaxAttempts  int
		RetryMaxBackoff   string
	}{
		Exports:           rendered,
		Port:              Port,
		WALDirectory:      WALDirectory,
		ClusterIDField:    ClusterIDField,
		ExportTimeout:     ExportTimeout,
		QueueSize:         QueueSize,
		QueueConsumers:    QueueConsumers,
		BatchMinSize:      BatchMinSize,
		BatchMaxSize:      BatchMaxSize,
		BatchFlushTimeout: BatchFlushTimeout,
		RetryMaxAttempts:  RetryMaxAttempts,
		RetryMaxBackoff:   RetryMaxBackoff,
	})
	if err != nil {
		return "", err
	}

	return render(alloyValuesTemplate, struct {
		AlloyConfig  string
		AppName      string
		Port         int
		Replicas     int
		RunAsUser    int
		WALMountPath string
		WALDirectory string
		WALSize      string
	}{
		AlloyConfig:  config,
		AppName:      apps.AlloyLogExporterAppName,
		Port:         Port,
		Replicas:     Replicas,
		RunAsUser:    RunAsUser,
		WALMountPath: WALMountPath,
		WALDirectory: WALDirectory,
		WALSize:      WALSize,
	})
}

// SecretEnv returns the environment the exporter needs for static credentials, keyed by
// variable name, ready for common.GenerateSecretData.
//
// The awss3 exporter has no credential fields: it uses the AWS SDK's default chain, which
// reads the process environment. Environment variables are per-container, not per
// exporter, so only one export can carry static credentials. Additional destinations have
// to authenticate by workload identity with roleARN, which is per exporter.
func SecretEnv(exports []observabilityv1alpha1.LogExport, creds map[client.ObjectKey]Credentials) (map[string]string, error) {
	env := map[string]string{}
	var credentialed string

	for _, e := range sorted(exports) {
		if e.Spec.Destination.S3 == nil || e.Spec.Destination.S3.CredentialsRef == nil {
			continue
		}
		ref := client.ObjectKey{Namespace: e.Namespace, Name: e.Name}
		if credentialed != "" {
			return nil, fmt.Errorf("%s and %s both set spec.destination.s3.credentialsRef, but static credentials reach the exporter as environment variables and cannot be set per destination: use roleARN on all but one", credentialed, ref)
		}
		credentialed = ref.String()

		c, ok := creds[ref]
		if !ok {
			return nil, fmt.Errorf("no resolved credentials for %s", ref)
		}
		env[AccessKeyIDEnv] = c.AccessKeyID
		env[SecretAccessKeyEnv] = c.SecretAccessKey
	}

	return env, nil
}

func buildExports(exports []observabilityv1alpha1.LogExport) ([]export, error) {
	if len(exports) == 0 {
		return nil, fmt.Errorf("no LogExport to render")
	}

	out := make([]export, 0, len(exports))
	slugs := map[string]string{}

	for _, e := range sorted(exports) {
		ref := client.ObjectKey{Namespace: e.Namespace, Name: e.Name}.String()

		if e.Spec.Destination.Type != observabilityv1alpha1.LogExportDestinationS3 {
			return nil, fmt.Errorf("%s: destination type %q is not implemented yet, only %q is", ref, e.Spec.Destination.Type, observabilityv1alpha1.LogExportDestinationS3)
		}
		if e.Spec.Destination.S3 == nil {
			return nil, fmt.Errorf("%s: destination type is %q with no s3 block", ref, observabilityv1alpha1.LogExportDestinationS3)
		}

		slug := slugify(e.Namespace, e.Name)
		if other, taken := slugs[slug]; taken {
			return nil, fmt.Errorf("LogExports %s and %s both render to the Alloy component name %q; recreate one under a name that does not fold to the same identifier", other, ref, slug)
		}
		slugs[slug] = ref

		p, err := translate(e.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ref, err)
		}

		out = append(out, export{
			Slug:     slug,
			Ref:      ref,
			Pipeline: p,
			S3:       e.Spec.Destination.S3,
		})
	}

	return out, nil
}

// slugify builds the component suffix. Folding non-identifier characters can collide
// (a-b/c and a/b-c), which buildExports rejects rather than working around, because the
// alternative -- numbering the components -- renames them whenever an export is added.
func slugify(namespace, name string) string {
	return nonIdentifier.ReplaceAllString(namespace, "_") + "_" + nonIdentifier.ReplaceAllString(name, "_")
}

func sorted(exports []observabilityv1alpha1.LogExport) []observabilityv1alpha1.LogExport {
	out := slices.Clone(exports)
	slices.SortFunc(out, func(a, b observabilityv1alpha1.LogExport) int {
		return strings.Compare(a.Namespace+"/"+a.Name, b.Namespace+"/"+b.Name)
	})
	return out
}

func render(tmpl *template.Template, data any) (string, error) {
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed to execute %s template: %w", tmpl.Name(), err)
	}
	return buffer.String(), nil
}
